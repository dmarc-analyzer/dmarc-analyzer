package backend

type AggregateReport struct {
	MessageID       string                  `gorm:"column:message_id;primaryKey"`
	Organization    string                  `xml:"report_metadata>org_name" gorm:"column:organization"`
	Email           string                  `xml:"report_metadata>email" gorm:"column:email"`
	ExtraContact    string                  `xml:"report_metadata>extra_contact_info" gorm:"column:extra_contact"` // minOccurs="0"
	ReportID        string                  `xml:"report_metadata>report_id" gorm:"column:report_id"`
	DateRangeBegin  int64                   `xml:"report_metadata>date_range>begin" gorm:"column:date_range_begin"`
	DateRangeEnd    int64                   `xml:"report_metadata>date_range>end" gorm:"column:date_range_end"`
	Errors          []string                `xml:"report_metadata>error" gorm:"-"`
	Domain          string                  `xml:"policy_published>domain" gorm:"column:domain"`
	AlignDKIM       string                  `xml:"policy_published>adkim" gorm:"column:align_dkim"` // minOccurs="0"
	AlignSPF        string                  `xml:"policy_published>aspf" gorm:"column:align_spf"`   // minOccurs="0"
	Policy          string                  `xml:"policy_published>p" gorm:"column:policy"`
	SubdomainPolicy string                  `xml:"policy_published>sp" gorm:"column:subdomain_policy"`
	Percentage      int                     `xml:"policy_published>pct" gorm:"column:percentage"`
	FailureReport   string                  `xml:"policy_published>fo" gorm:"column:failure_report"`
	Records         []AggregateReportRecord `xml:"record" gorm:"-"`
}

type AggregateReportRecord struct {
	SourceIP          string           `xml:"row>source_ip" gorm:"column:source_ip"`
	Count             int64            `xml:"row>count" gorm:"column:count"`
	Disposition       string           `xml:"row>policy_evaluated>disposition" gorm:"column:disposition"` // ignore, quarantine, reject
	EvalDKIM          string           `xml:"row>policy_evaluated>dkim" gorm:"column:eval_dkim"`          // pass, fail
	EvalSPF           string           `xml:"row>policy_evaluated>spf" gorm:"column:eval_spf"`            // pass, fail
	POReason          []POReason       `xml:"row>policy_evaluated>reason" gorm:"-"`
	HeaderFrom        string           `xml:"identifiers>header_from" gorm:"column:header_from"`
	EnvelopeFrom      string           `xml:"identifiers>envelope_from" gorm:"column:envelope_from"`
	EnvelopeTo        string           `xml:"identifiers>envelope_to" gorm:"column:envelope_to"` // min 0
	AuthDKIM          []DKIMAuthResult `xml:"auth_results>dkim" gorm:"-"`                        // min 0
	AuthSPF           []SPFAuthResult  `xml:"auth_results>spf" gorm:"-"`
	AggregateReportID string           `gorm:"column:aggregate_report_id;primaryKey"`
	RecordNumber      int64            `gorm:"column:record_number;primaryKey"`
}

type POReason struct {
	Reason            string `xml:"type" gorm:"column:reason;primaryKey"`
	Comment           string `xml:"comment" gorm:"column:comment"`
	AggregateReportID string `gorm:"column:aggregate_report_id;primaryKey"`
	RecordNumber      int64  `gorm:"column:record_number;primaryKey"`
}

type DKIMAuthResult struct {
	Domain            string `xml:"domain" gorm:"column:domain;primaryKey"`
	Selector          string `xml:"selector" gorm:"column:selector"`
	Result            string `xml:"result" gorm:"column:result"`
	HumanResult       string `xml:"human_result" gorm:"column:human_result"`
	AggregateReportID string `gorm:"column:aggregate_report_id;primaryKey"`
	RecordNumber      int64  `gorm:"column:record_number;primaryKey"`
}

type SPFAuthResult struct {
	Domain            string `xml:"domain" gorm:"column:domain;primaryKey"`
	Scope             string `xml:"scope" gorm:"column:scope"`
	Result            string `xml:"result" gorm:"column:result"`
	AggregateReportID string `gorm:"column:aggregate_report_id;primaryKey"`
	RecordNumber      int64  `gorm:"column:record_number;primaryKey"`
}
