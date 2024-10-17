package model

type AggregateReport struct {
	Organization    string                  `xml:"report_metadata>org_name"`
	Email           string                  `xml:"report_metadata>email"`
	ExtraContact    string                  `xml:"report_metadata>extra_contact_info"`
	ReportID        string                  `xml:"report_metadata>report_id"`
	DateRangeBegin  int64                   `xml:"report_metadata>date_range>begin"`
	DateRangeEnd    int64                   `xml:"report_metadata>date_range>end"`
	Errors          []string                `xml:"report_metadata>error"`
	Domain          string                  `xml:"policy_published>domain"`
	AlignDKIM       string                  `xml:"policy_published>adkim"`
	AlignSPF        string                  `xml:"policy_published>aspf"`
	Policy          string                  `xml:"policy_published>p"`
	SubdomainPolicy string                  `xml:"policy_published>sp"`
	Percentage      int                     `xml:"policy_published>pct"`
	FailureReport   string                  `xml:"policy_published>fo"`
	Records         []AggregateReportRecord `xml:"record"`
}

type AggregateReportRecord struct {
	SourceIP     string           `xml:"row>source_ip"`
	Count        int64            `xml:"row>count"`
	Disposition  string           `xml:"row>policy_evaluated>disposition"` // ignore, quarantine, reject
	EvalDKIM     string           `xml:"row>policy_evaluated>dkim"`        // pass, fail
	EvalSPF      string           `xml:"row>policy_evaluated>spf"`         // pass, fail
	POReason     []POReason       `xml:"row>policy_evaluated>reason"`
	HeaderFrom   string           `xml:"identifiers>header_from"`
	EnvelopeFrom string           `xml:"identifiers>envelope_from"`
	EnvelopeTo   string           `xml:"identifiers>envelope_to"`
	AuthDKIM     []DKIMAuthResult `xml:"auth_results>dkim"`
	AuthSPF      []SPFAuthResult  `xml:"auth_results>spf"`
}

type POReason struct {
	Reason  string `xml:"type"`
	Comment string `xml:"comment"`
}

type DKIMAuthResult struct {
	Domain      string `xml:"domain"`
	Selector    string `xml:"selector"`
	Result      string `xml:"result"`
	HumanResult string `xml:"human_result"`
}

type SPFAuthResult struct {
	Domain string `xml:"domain"`
	Scope  string `xml:"scope"`
	Result string `xml:"result"`
}
