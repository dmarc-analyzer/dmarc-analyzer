package model

type DmarcReportEntry struct {
	MessageID         string      `json:"message_id" gorm:"primaryKey"`
	RecordNumber      int64       `json:"record_number" gorm:"primaryKey"`
	ReportOrgName     string      `json:"report_org_name"`
	Domain            string      `json:"domain"`
	Policy            string      `json:"policy"`
	SubdomainPolicy   string      `json:"subdomain_policy"`
	AlignDKIM         string      `json:"align_dkim"`
	AlignSPF          string      `json:"align_spf"`
	Pct               int         `json:"pct"`
	SourceIP          Inet        `json:"source_ip"`
	ESP               string      `json:"esp"`
	OrgName           string      `json:"org_name"`
	OrgID             string      `json:"org_id"`
	HostName          string      `json:"host_name"`
	DomainName        string      `json:"domain_name"`
	HostNameMatchesIP string      `json:"host_name_matches_ip"`
	City              string      `json:"city"`
	State             string      `json:"state"`
	Country           string      `json:"country"`
	Longitude         string      `json:"longitude"`
	Latitude          string      `json:"latitude"`
	ReverseLookup     StringArray `json:"reverse_lookup"`
	MessageCount      int64       `json:"message_count"`
	Disposition       string      `json:"disposition"`
	EvalDKIM          string      `json:"eval_dkim"`
	EvalSPF           string      `json:"eval_spf"`
	HeaderFrom        string      `json:"header_from"`
	EnvelopeFrom      string      `json:"envelope_from"`
	EnvelopeTo        string      `json:"envelope_to"`
	AuthDKIMDomain    StringArray `json:"auth_dkim_domain"`
	AuthDKIMSelector  StringArray `json:"auth_dkim_selector"`
	AuthDKIMResult    StringArray `json:"auth_dkim_result"`
	AuthSPFDomain     StringArray `json:"auth_spf_domain"`
	AuthSPFScope      StringArray `json:"auth_spf_scope"`
	AuthSPFResult     StringArray `json:"auth_spf_result"`
	POReason          StringArray `json:"po_reason"`
	POComment         StringArray `json:"po_comment"`
	StartDate         int64       `json:"start_date"`
	EndDate           int64       `json:"end_date"`
	LastUpdate        string      `json:"last_update"`
}
