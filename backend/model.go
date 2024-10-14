package backend

type AggregateReport struct {
	MessageId       string                  `db:"MessageId"`
	Organization    string                  `xml:"report_metadata>org_name" db:"Organization"`
	Email           string                  `xml:"report_metadata>email" db:"Email"`
	ExtraContact    string                  `xml:"report_metadata>extra_contact_info" db:"ExtraContact"` // minOccurs="0"
	ReportID        string                  `xml:"report_metadata>report_id" db:"ReportID"`
	DateRangeBegin  int64                   `xml:"report_metadata>date_range>begin" db:"DateRangeBegin"`
	DateRangeEnd    int64                   `xml:"report_metadata>date_range>end" db:"DateRangeEnd"`
	Errors          []string                `xml:"report_metadata>error"`
	Domain          string                  `xml:"policy_published>domain" db:"Domain"`
	AlignDKIM       string                  `xml:"policy_published>adkim" db:"AlignDKIM"` // minOccurs="0"
	AlignSPF        string                  `xml:"policy_published>aspf" db:"AlignSPF"`   // minOccurs="0"
	Policy          string                  `xml:"policy_published>p" db:"Policy"`
	SubdomainPolicy string                  `xml:"policy_published>sp" db:"SubdomainPolicy"`
	Percentage      int                     `xml:"policy_published>pct" db:"Percentage"`
	FailureReport   string                  `xml:"policy_published>fo" db:"FailureReport"`
	Records         []AggregateReportRecord `xml:"record"`
}

type AggregateReportRecord struct {
	SourceIP           string           `xml:"row>source_ip" db:"SourceIP"`
	Count              int64            `xml:"row>count" db:"Count"`
	Disposition        string           `xml:"row>policy_evaluated>disposition" db:"Disposition"` // ignore, quarantine, reject
	EvalDKIM           string           `xml:"row>policy_evaluated>dkim" db:"EvalDKIM"`           // pass, fail
	EvalSPF            string           `xml:"row>policy_evaluated>spf" db:"EvalSPF"`             // pass, fail
	POReason           []POReason       `xml:"row>policy_evaluated>reason"`
	HeaderFrom         string           `xml:"identifiers>header_from" db:"HeaderFrom"`
	EnvelopeFrom       string           `xml:"identifiers>envelope_from" db:"EnvelopeFrom"`
	EnvelopeTo         string           `xml:"identifiers>envelope_to" db:"EnvelopeTo"` // min 0
	AuthDKIM           []DKIMAuthResult `xml:"auth_results>dkim"`                       // min 0
	AuthSPF            []SPFAuthResult  `xml:"auth_results>spf"`
	AggregateReport_id string           `db:"AggregateReport_id"`
	RecordNumber       int64            `db:"RecordNumber"`
}

type POReason struct {
	Reason             string `xml:"type" db:"Reason"`
	Comment            string `xml:"comment" db:"Comment"`
	AggregateReport_id string `db:"AggregateReport_id"`
	RecordNumber       int64  `db:"RecordNumber"`
}

type DKIMAuthResult struct {
	Domain             string `xml:"domain" db:"Domain"`
	Selector           string `xml:"selector" db:"Selector"`
	Result             string `xml:"result" db:"Result"`
	HumanResult        string `xml:"human_result" db:"HumanResult"`
	AggregateReport_id string `db:"AggregateReport_id"`
	RecordNumber       int64  `db:"RecordNumber"`
}

type SPFAuthResult struct {
	Domain             string `xml:"domain" db:"Domain"`
	Scope              string `xml:"scope" db:"Scope"`
	Result             string `xml:"result" db:"Result"`
	AggregateReport_id string `db:"AggregateReport_id"`
	RecordNumber       int64  `db:"RecordNumber"`
}
