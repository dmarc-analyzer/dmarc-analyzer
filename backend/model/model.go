package model

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

type DmarcReportingFull struct {
	MessageID         string      `json:"message_id" gorm:"column:message_id;primaryKey"`
	RecordNumber      int64       `json:"record_number" gorm:"column:record_number;primaryKey"`
	Domain            string      `json:"domain" gorm:"column:domain"`
	Policy            string      `json:"policy" gorm:"column:policy"`
	SubdomainPolicy   string      `json:"subdomain_policy" gorm:"column:subdomain_policy"`
	AlignDKIM         string      `json:"align_dkim" gorm:"column:align_dkim"`
	AlignSPF          string      `json:"align_spf" gorm:"column:align_spf"`
	Pct               int         `json:"pct" gorm:"column:pct"`
	SourceIP          string      `json:"source_ip" gorm:"column:source_ip"`
	ESP               string      `json:"esp" gorm:"column:esp"`
	OrgName           string      `json:"org_name" gorm:"column:org_name"`
	OrgID             string      `json:"org_id" gorm:"column:org_id"`
	HostName          string      `json:"host_name" gorm:"column:host_name"`
	DomainName        string      `json:"domain_name" gorm:"column:domain_name"`
	HostnameMatchesIP string      `json:"host_name_matches_ip" gorm:"column:host_name_matches_ip"`
	City              string      `json:"city" gorm:"column:city"`
	State             string      `json:"state" gorm:"column:state"`
	Country           string      `json:"country" gorm:"column:country"`
	Longitude         string      `json:"longitude" gorm:"column:longitude"`
	Latitude          string      `json:"latitude" gorm:"column:latitude"`
	ReverseLookup     StringArray `json:"reverse_lookup" gorm:"column:reverse_lookup;"`
	MessageCount      int64       `json:"message_count" gorm:"column:message_count"`
	Disposition       string      `json:"disposition" gorm:"column:disposition"`
	EvalDKIM          string      `json:"eval_dkim" gorm:"column:eval_dkim"`
	EvalSPF           string      `json:"eval_spf" gorm:"column:eval_spf"`
	HeaderFrom        string      `json:"header_from" gorm:"column:header_from"`
	EnvelopeFrom      string      `json:"envelope_from" gorm:"column:envelope_from"`
	EnvelopeTo        string      `json:"envelope_to" gorm:"column:envelope_to"`
	AuthDKIMDomain    StringArray `json:"auth_dkim_domain" gorm:"column:auth_dkim_domain"`
	AuthDKIMSelector  StringArray `json:"auth_dkim_selector" gorm:"column:auth_dkim_selector"`
	AuthDKIMResult    StringArray `json:"auth_dkim_result" gorm:"column:auth_dkim_result"`
	AuthSPFDomain     StringArray `json:"auth_spf_domain" gorm:"column:auth_spf_domain"`
	AuthSPFScope      StringArray `json:"auth_spf_scope" gorm:"column:auth_spf_scope"`
	AuthSPFResult     StringArray `json:"auth_spf_result" gorm:"column:auth_spf_result"`
	POReason          StringArray `json:"po_reason" gorm:"column:po_reason"`
	POComment         StringArray `json:"po_comment" gorm:"column:po_comment"`
	StartDate         int64       `json:"start_date" gorm:"column:start_date"`
	EndDate           int64       `json:"end_date" gorm:"column:end_date"`
	LastUpdate        string      `json:"last_update" gorm:"column:last_update"`
}
