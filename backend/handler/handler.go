package handler

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/dmarc-analyzer/dmarc-analyzer/backend/db"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/model"
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

	t := time.Now()
	tStart, err := time.Parse(time.RFC3339Nano, startDate)
	if err != nil {
		log.Println("ERROR reading start time", err)
		tStart = time.Now().AddDate(0, 0, -30)
	}
	tEnd, err := time.Parse(time.RFC3339Nano, endDate)
	if err != nil {
		log.Println("ERROR reading end time", err)
		tEnd = time.Now()
	}

	if tEnd.After(t) {
		tEnd = t
	}

	start := tStart.Unix()
	end := tEnd.Unix()

	gr := getSummaryReturn{
		Domain: domain,
	}

	summary := []DmarcReportingSummary{}
	counts := DomainSummaryCounts{}

	// harvest all reports that have been received in the last 'dayCount' days:

	summary, counts = QSummary(domain, start, end)

	gr.StartDate = tStart.Format(time.RFC3339Nano)
	gr.EndDate = tEnd.Format(time.RFC3339Nano)

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

type getSummaryReturn struct {
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

}

func HandleDmarcChart(c *gin.Context) {

}
