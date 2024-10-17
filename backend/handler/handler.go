package handler

import (
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dmarc-analyzer/dmarc-analyzer/backend/db"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/model"
	"github.com/dmarc-analyzer/dmarc-analyzer/backend/util"
	"github.com/gin-gonic/gin"
)

func HandleDomainList(c *gin.Context) {
	var domains []string
	err := db.DB.Model(&model.DmarcReportEntry{}).Select("distinct(domain)").Pluck("domain", &domains).Error
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

	// harvest all reports that have been received in the last 'dayCount' days:
	summary, counts := QSummary(domain, start, end)

	resp := &DomainSummaryResp{
		Domain:              domain,
		Summary:             summary,
		DomainSummaryCounts: counts,
		StartDate:           time.Unix(start, 0).Format(time.RFC3339Nano),
		EndDate:             time.Unix(end, 0).Format(time.RFC3339Nano),
	}

	c.JSON(200, resp)
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
		revsource, _ := util.GetOrgDomain(d.ReverseLookup[0])
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

	err := db.DB.Model(&model.DmarcReportEntry{}).
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

	qargs := []interface{}{domain, start, end, source}
	qsql := `
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
	  po_comment
FROM dmarc_reporting_entries dre cross join lateral unnest(coalesce(nullif(reverse_lookup,'{}'),array[null::text])) as revlookup(i)
WHERE dre.domain = $1
AND dre.end_date >= $2
AND dre.end_date <= $3
`

	if net.ParseIP(source) != nil {
		qsql += `AND source_ip = $4::inet`
	} else {
		qsql += `AND (esp = $4 OR
	( esp = '' AND ( domain_name = $4 OR ( domain_name = '' AND (revlookup.i LIKE $5)))))`
		source2 := fmt.Sprintf("%s%s%s", "%", source, ".")
		qargs = append(qargs, source2)
	}

	qsql += `
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
ORDER BY count DESC LIMIT 2500
	`

	drArray := []DmarcReportingForwarded{}
	err := db.DB.Raw(qsql, qargs...).Scan(&drArray).Error
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	return drArray
}

func HandleDmarcChart(c *gin.Context) {
	domain := c.Param("domain")
	startDate := c.Query("start")
	endDate := c.Query("end")

	start, end := util.ParseDate(startDate, endDate)
	result, err := GetDmarcChartData(start, end, domain)
	if err != nil {
		fmt.Printf("GetDmarcChartData error: %+v", err)
	}
	c.JSON(200, result)
}

type daily struct {
	Name   string   `json:"name"`
	Series []Volume `json:"series"`
}

// Volume Name is the timestamp, Value is the pass/fail quantity on that day
type Volume struct {
	Name  int64 `json:"name"`
	Value int64 `json:"value"`
}

type DmarcChartResp struct {
	ChartData []daily `json:"chartdata"`
	Domain    string  `json:"domain"`
}

func GetDmarcChartData(start, end int64, domain string) (*DmarcChartResp, error) {

	rawData, err := GetDmarcDatedWeeklyChart(domain, start, end)

	if err != nil {
		log.Printf("ERROR on GetDmarcDatedWeeklyChart, %s", err)
	}

	chartData := []daily{}
	chartData = append(chartData, daily{Name: "pass", Series: []Volume{}})
	chartData = append(chartData, daily{Name: "fail", Series: []Volume{}})

	for i, val := range rawData.Full {
		timestamp, _ := val[0].(int64)
		pass, _ := rawData.Pass[i][1].(int64)
		fail, _ := rawData.Fail[i][1].(int64)

		chartData[0].Series = append(chartData[0].Series, Volume{Name: timestamp, Value: pass})
		chartData[1].Series = append(chartData[1].Series, Volume{Name: timestamp, Value: fail})

	}

	retVal := &DmarcChartResp{
		ChartData: chartData,
		Domain:    domain,
	}

	return retVal, err
}

// ChartContainer structure of result
type ChartContainer struct {
	Full []result `json:"full"`
	Pass []result `json:"pass"`
	Fail []result `json:"fail"`
}

type result []interface{}

// GetDmarcDatedWeeklyChart returns the weekly dmarc data
func GetDmarcDatedWeeklyChart(domain string, start, end int64) (ChartContainer, error) {
	var chart ChartContainer

	if start == 0 || end == 0 {
		now := time.Now()
		end = now.UTC().Unix()
		start = now.UTC().AddDate(0, 0, -30).Unix()
	}

	dailyResults, err := getDmarcDailyAll(domain, start, end)
	if err != nil {
		return chart, err
	}

	fmt.Fprintf(os.Stderr, "number of days returned : %d\n %v", len(dailyResults), dailyResults)

	var currentTime int64
	var lastDay int64
	for _, day := range dailyResults {

		//Pad the graph with 0 days to keep entries for every day.
		for lastDay++; lastDay < day.Day; lastDay++ {
			currentTime = ((lastDay * 86000) + start) * 1000
			chart.Full = append(chart.Full, result{currentTime, 0})
			chart.Pass = append(chart.Pass, result{currentTime, 0})
			chart.Fail = append(chart.Fail, result{currentTime, 0})
		}

		currentTime = ((day.Day * 86000) + start) * 1000
		chart.Full = append(chart.Full, result{currentTime, day.Passing + day.Failing})
		chart.Pass = append(chart.Pass, result{currentTime, day.Passing})
		chart.Fail = append(chart.Fail, result{currentTime, day.Failing})
	}

	//Pad the end of the graph with 0 days to keep entries for every day.
	//NOTE: this padding and the loop above could probably be solved by using a generated series in the query
	if currentTime > 0 {
		if currentTime+(86000*1000) < end*1000 {
			log.Println("Padding end of the chart. Current", currentTime, " and end ", end)
		} else {
			log.Println("No chart padding required. Current", currentTime, " and end ", end)
		}
		for currentTime += 86000 * 1000; currentTime < end*1000; currentTime += 86000 * 1000 {
			chart.Full = append(chart.Full, result{currentTime, 0})
			chart.Pass = append(chart.Pass, result{currentTime, 0})
			chart.Fail = append(chart.Fail, result{currentTime, 0})
		}
	} else {
		log.Println("No data returned")
	}

	return chart, nil
}

type DmarcDailyBuckets struct {
	Day     int64
	Passing int64
	Failing int64
}

func getDmarcDailyAll(domain string, timeBegin, timeEnd int64) ([]*DmarcDailyBuckets, error) {

	var results []*DmarcDailyBuckets

	//Normalize on day boundaries, using the start time as the boundary.
	days := (timeEnd - timeBegin) / 86400
	timeEnd = timeBegin + (days * 86400)

	err := db.DB.Model(&model.DmarcReportEntry{}).
		Select(`width_bucket(end_date, $1, $2, $3) as day,
SUM(case when eval_dkim = 'pass' or eval_spf = 'pass' then message_count else 0 end) as passing,
SUM(case when eval_dkim != 'pass' and eval_spf != 'pass' then message_count else 0 end) as failing`).
		Where("end_date >= $1 AND end_date < $2 AND domain = $4", timeBegin, timeEnd, days, domain).
		Group("day").
		Order("day").
		Scan(&results).Error

	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	return results, err
}
