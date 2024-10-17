package handler

import (
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/dmarc-analyzer/dmarc-analyzer/backend/db"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/model"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/util"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/publicsuffix"
)

func HandleDomainList(c *gin.Context) {
	var domains []string
	err := db.DB.Model(&model.DmarcReportingFull{}).Select("distinct(domain)").Pluck("domain", &domains).Error
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	c.JSON(200, domains)
}

func HandleDomainSummary(c *gin.Context) {
	domain := c.Param("domain")
	startDate := c.Query("start")
	endDate := c.Query("end")

	start, end := util.ParseDate(startDate, endDate)

	gr := DomainSummaryResp{
		Domain: domain,
	}

	summary := []DmarcReportingSummary{}
	counts := DomainSummaryCounts{}

	// harvest all reports that have been received in the last 'dayCount' days:

	summary, counts = QSummary(domain, start, end)

	gr.StartDate = time.Unix(start, 0).Format(time.RFC3339Nano)
	gr.EndDate = time.Unix(end, 0).Format(time.RFC3339Nano)

	gr.Summary = summary
	gr.DomainSummaryCounts = counts
	if len(summary) > 0 {
		gr.MaxVolume = summary[0].TotalCount
	}
	c.JSON(200, gr)
}

// DmarcReportingSummary structure used on summary table
type DmarcReportingSummary struct {
	Source               string `json:"source"` // could be name, domain_name, or IP
	TotalCount           int64  `json:"total_count"`
	DispositionPassCount int64  `json:"pass_count"`
	SPFAlignedCount      int64  `json:"spf_aligned_count"`
	DKIMAlignedCount     int64  `json:"dkim_aligned_count"`
	FullyAlignedCount    int64  `json:"fully_aligned_count"`
	SourceType           string `json:"source_type"`
}

// DomainSummaryCounts structure used on calculating the whole volume and passing rate in the time range of one domain
type DomainSummaryCounts struct {
	ReportCount       int64 `json:"report_count"`
	MessageCount      int64 `json:"message_count"`
	DKIMAlignedCount  int64 `json:"dkim_aligned_count"`
	SPFAlignedCount   int64 `json:"spf_aligned_count"`
	FullyAlignedCount int64 `json:"fully_aligned_count"`
}

// DmarcReportingDefault structure used to
type DmarcReportingDefault struct {
	MessageCount  int64             `json:"message_count"`
	SourceIP      string            `json:"source_ip"`
	ESP           string            `json:"esp"`
	HostName      string            `json:"host_name"`
	DomainName    string            `json:"domain_name"`
	Country       string            `json:"country"`
	Disposition   string            `json:"disposition"`
	EvalDKIM      string            `json:"eval_dkim"`
	EvalSPF       string            `json:"eval_spf"`
	ReverseLookup model.StringArray `json:"reverse_lookup"`
}

func (d DmarcReportingDefault) Label() (source string, sourceType string) {

	// last resort is to show IP:
	source = d.SourceIP
	sourceType = "IP"

	// these are ordered from most-preferred to next-to-least preferred:
	if len(d.ESP) > 0 {
		source = d.ESP
		sourceType = "ESP"
	} else if len(d.DomainName) > 0 {
		source = d.DomainName
		sourceType = "DomainName"
	} else if len(d.HostName) > 0 {
		source = d.HostName
		sourceType = "HostName"
	} else if len(d.ReverseLookup[0]) > 0 {
		revsource, _ := GetOrgDomain(d.ReverseLookup[0])
		if len(revsource) > 0 {
			revsource = strings.TrimRight(revsource, ".")
			if len(revsource) > 0 {
				source = revsource
				sourceType = "ReverseLookup"
			}
		}
	}

	return
}

// GetOrgDomain returns the org domain for the input domain according to the
// mechanisms of code in the publicsuffix package
func GetOrgDomain(domain string) (orgDomain string, err error) {

	// trailing periods should be stripped before splitting:
	domain = strings.TrimRight(domain, ".")

	// resolve organizational domain:
	icannDomain, isIcann := publicsuffix.PublicSuffix(domain)

	if !isIcann {
		// bad domain spec
		//  -- not sure how we will ever be here if HostDomain is compliant
		err = fmt.Errorf("bad organizational domain: %s", domain)
		return
	}

	domainLabels := strings.Split(domain, ".")
	icannLabels := strings.Split(icannDomain, ".")
	icl := len(icannLabels)
	dcl := len(domainLabels)
	orgDomain = ""

	if dcl-icl <= 1 {
		// domain is org domain:
		orgDomain = domain
		return
	}
	for j := dcl - icl - 1; j < dcl; j++ {
		orgDomain = orgDomain + domainLabels[j] + "."
	}

	return
}

type DMARCStats struct {
	MessageCount         int64
	DispositionPassCount int64
	SPFAlignedCount      int64
	DKIMAlignedCount     int64
	FullyAlignedCount    int64
	SourceType           string
}

type DomainSummaryResp struct {
	Summary             []DmarcReportingSummary `json:"summary"`
	MaxVolume           int64                   `json:"max_volume"`
	DomainSummaryCounts DomainSummaryCounts     `json:"domain_summary_counts"`
	StartDate           string                  `json:"start_date"`
	EndDate             string                  `json:"end_date"`
	Domain              string                  `json:"domain"`
}

type DmarcReportingSummaryList []DmarcReportingSummary

func (d DmarcReportingSummaryList) Len() int      { return len(d) }
func (d DmarcReportingSummaryList) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d DmarcReportingSummaryList) Less(i, j int) bool {
	if d[i].TotalCount < d[j].TotalCount {
		return true
	}
	if d[i].TotalCount > d[j].TotalCount {
		return false
	}
	return d[i].TotalCount < d[j].TotalCount
}

// QSummary is a summary view of dmarc evaluations per sender
// Possible cases to consider:
// 1.  Sender company name is resolved        -> use name
// 2.  Sender domain but not name is resolved -> use domain
// 3.  Sender domain and name both unresolved -> use IP
func QSummary(domain string, start int64, end int64) ([]DmarcReportingSummary, DomainSummaryCounts) {

	summaryRowMax := 1000
	results := []DmarcReportingSummary{}
	thecounts := DomainSummaryCounts{}
	drArray := []DmarcReportingDefault{}

	err := db.DB.Model(&model.DmarcReportingFull{}).
		Select("SUM(message_count) AS message_count, source_ip, esp, domain_name, reverse_lookup, country, disposition, eval_dkim, eval_spf").
		Where("domain = ? AND end_date >= ? AND end_date <= ?", domain, start, end).
		Group("source_ip, esp, domain_name, reverse_lookup, country, disposition, eval_dkim, eval_spf").
		Scan(&drArray).Error
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	//----------------------------------------------------------------
	// for each row, keep counts of the respective alignments:
	//----------------------------------------------------------------
	countMap := make(map[string]*DMARCStats)
	for n := range drArray {
		dr := drArray[n]
		source, sourceType := dr.Label()

		// initialize with new source if needed:
		if countMap[source] == nil {
			countMap[source] = &DMARCStats{
				SourceType: sourceType,
			}
		}

		// keep track of counts by source:
		countMap[source].MessageCount += dr.MessageCount
		if strings.Compare(dr.EvalDKIM, "pass") == 0 && strings.Compare(dr.EvalSPF, "pass") != 0 {
			countMap[source].DKIMAlignedCount += dr.MessageCount
			countMap[source].DispositionPassCount += dr.MessageCount
		} else if strings.Compare(dr.EvalDKIM, "pass") != 0 && strings.Compare(dr.EvalSPF, "pass") == 0 {
			countMap[source].SPFAlignedCount += dr.MessageCount
			countMap[source].DispositionPassCount += dr.MessageCount
		} else if strings.Compare(dr.EvalDKIM, "pass") == 0 && strings.Compare(dr.EvalSPF, "pass") == 0 {
			countMap[source].DKIMAlignedCount += dr.MessageCount
			countMap[source].SPFAlignedCount += dr.MessageCount
			countMap[source].FullyAlignedCount += dr.MessageCount
			countMap[source].DispositionPassCount += dr.MessageCount
		}

	}

	// build up the (now very unsorted) reporting summary data)
	for key, _ := range countMap {
		summary := DmarcReportingSummary{
			Source:               key,
			TotalCount:           countMap[key].MessageCount,
			DispositionPassCount: countMap[key].DispositionPassCount,
			SPFAlignedCount:      countMap[key].SPFAlignedCount,
			DKIMAlignedCount:     countMap[key].DKIMAlignedCount,
			FullyAlignedCount:    countMap[key].FullyAlignedCount,
			SourceType:           countMap[key].SourceType,
		}

		results = append(results, summary)

		thecounts.MessageCount += countMap[key].MessageCount
		thecounts.SPFAlignedCount += countMap[key].SPFAlignedCount
		thecounts.DKIMAlignedCount += countMap[key].DKIMAlignedCount
		thecounts.FullyAlignedCount += countMap[key].FullyAlignedCount
	}

	// sort in descending order by volume:
	sort.Sort(sort.Reverse(DmarcReportingSummaryList(results))) // don't touch ¯\_(ツ)_/¯

	// build json:

	if len(results) > summaryRowMax {
		return results[0 : summaryRowMax-1], thecounts
	}
	return results, thecounts

}

func HandleDmarcDetail(c *gin.Context) {
	domain := c.Param("domain")
	startDate := c.Query("start")
	endDate := c.Query("end")
	source := c.Query("source")
	sourceType := c.Query("source_type")

	start, end := util.ParseDate(startDate, endDate)
	result := GetDmarcReportDetail(start, end, domain, source, sourceType)

	c.JSON(200, &DmarcDetailResp{
		DetailRows: result,
	})
}

type DmarcDetailResp struct {
	DetailRows []DmarcReportingForwarded `json:"detail_rows"`
}

// DmarcReportingForwarded structure feeds the data used to generate detail table
type DmarcReportingForwarded struct {
	Count            int64             `json:"count,omitempty" db:"count"` // used with queries involving SUM(message_count) AS count
	SourceIP         string            `json:"source_ip" db:"source_ip"`
	ESP              string            `json:"esp" db:"esp"`
	DomainName       string            `json:"domain_name" db:"domain_name"`
	HostName         string            `json:"host_name" db:"host_name"`
	ReverseLookup    model.StringArray `json:"reverse_lookup" db:"reverse_lookup"`
	Country          string            `json:"country" db:"country"`
	Disposition      string            `json:"disposition" db:"disposition"`
	EvalDKIM         string            `json:"eval_dkim" db:"eval_dkim"`
	EvalSPF          string            `json:"eval_spf" db:"eval_spf"`
	HeaderFrom       string            `json:"header_from" db:"header_from"`
	EnvelopeFrom     string            `json:"envelope_from" db:"envelope_from"`
	EnvelopeTo       string            `json:"envelope_to" db:"envelope_to"`
	AuthDKIMDomain   model.StringArray `json:"auth_dkim_domain" db:"auth_dkim_domain"`
	AuthDKIMSelector model.StringArray `json:"auth_dkim_selector" db:"auth_dkim_selector"`
	AuthDKIMResult   model.StringArray `json:"auth_dkim_result" db:"auth_dkim_result"`
	AuthSPFDomain    model.StringArray `json:"auth_spf_domain" db:"auth_spf_domain"`
	AuthSPFScope     model.StringArray `json:"auth_spf_scope" db:"auth_spf_scope"`
	AuthSPFResult    model.StringArray `json:"auth_spf_result" db:"auth_spf_result"`
	POReason         model.StringArray `json:"po_reason" db:"po_reason"`
	POComment        model.StringArray `json:"po_comment" db:"po_comment"`
}

// GetDmarcReportDetail returns the dmarc report details used to be shown on detail panel
func GetDmarcReportDetail(start, end int64, domain, source, sourceType string) []DmarcReportingForwarded {
	selectTerm := `
	SELECT 	
	  SUM(message_count) AS count,
	  source_ip,
	  esp,
	  domain_name,
	  host_name,
	  revlookup.i,
	  country,
	  disposition,
	  eval_dkim,
	  eval_spf,
	  header_from,
	  envelope_from,
	  envelope_to,
	  auth_dkim_domain,
	  auth_dkim_selector,
	  auth_dkim_result,
	  auth_spf_domain,
	  auth_spf_scope,
	  auth_spf_result,
	  po_reason,
	  po_comment`

	groupTerm := `
	GROUP BY
	  source_ip,
	  esp,
	  domain_name,
	  host_name,
	  revlookup.i,
	  country,
	  disposition,
	  eval_dkim,
	  eval_spf,
	  header_from,
	  envelope_from,
	  envelope_to,
	  auth_dkim_domain,
	  auth_dkim_selector,
	  auth_dkim_result,
	  auth_spf_domain,
	  auth_spf_scope,
	  auth_spf_result,
	  po_reason,
	  po_comment
	`
	from := `FROM dmarc_reporting_fulls dre cross join lateral unnest(coalesce(nullif(reverse_lookup,'{}'),array[null::text])) as revlookup(i)`

	source2 := fmt.Sprintf("%s%s%s", "%", source, ".")

	qargs := []interface{}{domain, start, end, source}

	var qterm string

	//Dependence here on empty strings instead of nulls, and on the Label() function algorithm.
	if net.ParseIP(source) != nil {
		qterm = `AND source_ip = $4::inet`
	} else {
		qterm = `AND (esp = $4 OR
	( esp = '' AND ( domain_name = $4 OR ( domain_name = '' AND (revlookup.i LIKE $5)))))`
		qargs = append(qargs, source2)
	}

	qsql := fmt.Sprintf(`%s
%s
WHERE dre.domain = $1
AND dre.end_date >= $2
AND dre.end_date <= $3
%s
`, selectTerm, from, qterm)

	qsql = fmt.Sprintf("%s%s ORDER BY count DESC LIMIT %d", qsql, groupTerm, 2500)

	drArray := []DmarcReportingForwarded{}
	err := db.DB.Raw(qsql, qargs...).Scan(&drArray).Error
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	return drArray
}

func HandleDmarcChart(c *gin.Context) {

}
