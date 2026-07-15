package model

import (
	"encoding/xml"
	"fmt"
)

const (
	RFC9990Namespace = "urn:ietf:params:xml:ns:dmarc-2.0"
	RFC7489Namespace = "http://dmarc.org/dmarc-xml/0.1"
)

type AggregateReportShape string

const (
	AggregateReportShapeLegacy       AggregateReportShape = "legacy"
	AggregateReportShapeRFC9990      AggregateReportShape = "rfc9990"
	AggregateReportShapeVendorHybrid AggregateReportShape = "vendor_hybrid"
	AggregateReportShapeUnknown      AggregateReportShape = "unknown"
)

type LangString struct {
	Value    string `xml:",chardata"`
	Language string `xml:"http://www.w3.org/XML/1998/namespace lang,attr"`
}

type AggregateReport struct {
	XMLName            xml.Name                `xml:"feedback"`
	Version            string                  `xml:"version"`
	ReportOrgName      string                  `xml:"report_metadata>org_name"`
	Email              string                  `xml:"report_metadata>email"`
	ExtraContact       LangString              `xml:"report_metadata>extra_contact_info"`
	ReportID           string                  `xml:"report_metadata>report_id"`
	DateRangeBegin     int64                   `xml:"report_metadata>date_range>begin"`
	DateRangeEnd       int64                   `xml:"report_metadata>date_range>end"`
	Errors             []LangString            `xml:"report_metadata>error"`
	Generator          string                  `xml:"report_metadata>generator"`
	Domain             string                  `xml:"policy_published>domain"`
	DiscoveryMethod    string                  `xml:"policy_published>discovery_method"`
	AlignDKIM          string                  `xml:"policy_published>adkim"`
	AlignSPF           string                  `xml:"policy_published>aspf"`
	Policy             string                  `xml:"policy_published>p"`
	SubdomainPolicy    string                  `xml:"policy_published>sp"`
	NonexistentPolicy  string                  `xml:"policy_published>np"`
	Percentage         *int                    `xml:"policy_published>pct"`
	FailureReport      string                  `xml:"policy_published>fo"`
	Testing            string                  `xml:"policy_published>testing"`
	Records            []AggregateReportRecord `xml:"record"`
	DetectedShape      AggregateReportShape    `xml:"-"`
	ValidationWarnings []string                `xml:"-"`
}

func (report *AggregateReport) Finalize() {
	report.ValidationWarnings = nil

	switch report.XMLName.Space {
	case RFC9990Namespace:
		report.DetectedShape = AggregateReportShapeRFC9990
	case "", RFC7489Namespace:
		if report.hasRFC9990Fields() {
			report.DetectedShape = AggregateReportShapeVendorHybrid
			report.warn("RFC 9990 fields found without the RFC 9990 namespace")
		} else {
			report.DetectedShape = AggregateReportShapeLegacy
		}
	default:
		report.DetectedShape = AggregateReportShapeUnknown
		report.warn("unrecognized feedback namespace %q", report.XMLName.Space)
	}

	if report.DetectedShape == AggregateReportShapeRFC9990 {
		if report.Version != "" && report.Version != "1.0" {
			report.warn("unexpected RFC 9990 report version %q", report.Version)
		}
		if report.Percentage != nil {
			report.warn("legacy pct element found in an RFC 9990 report")
		}
		report.validateRFC9990Values()
	}
}

func (report *AggregateReport) Namespace() string {
	return report.XMLName.Space
}

func (report *AggregateReport) hasRFC9990Fields() bool {
	return report.Version != "" || report.Generator != "" || report.DiscoveryMethod != "" ||
		report.NonexistentPolicy != "" || report.Testing != ""
}

func (report *AggregateReport) validateRFC9990Values() {
	warnUnknown(report, "discovery_method", report.DiscoveryMethod, "", "psl", "treewalk")
	warnUnknown(report, "p", report.Policy, "none", "quarantine", "reject")
	warnUnknown(report, "sp", report.SubdomainPolicy, "", "none", "quarantine", "reject")
	warnUnknown(report, "np", report.NonexistentPolicy, "", "none", "quarantine", "reject")
	warnUnknown(report, "adkim", report.AlignDKIM, "", "r", "s")
	warnUnknown(report, "aspf", report.AlignSPF, "", "r", "s")
	warnUnknown(report, "testing", report.Testing, "", "n", "y")

	for i, record := range report.Records {
		warnUnknown(report, fmt.Sprintf("record[%d].disposition", i), record.Disposition,
			"none", "pass", "quarantine", "reject")
		warnUnknown(report, fmt.Sprintf("record[%d].dkim", i), record.EvalDKIM, "pass", "fail")
		warnUnknown(report, fmt.Sprintf("record[%d].spf", i), record.EvalSPF, "pass", "fail")

		for j, reason := range record.POReason {
			warnUnknown(report, fmt.Sprintf("record[%d].reason[%d]", i, j), reason.Reason,
				"local_policy", "mailing_list", "other", "policy_test_mode", "trusted_forwarder")
		}
		for j, result := range record.AuthDKIM {
			warnUnknown(report, fmt.Sprintf("record[%d].dkim_auth[%d]", i, j), result.Result,
				"none", "pass", "fail", "policy", "neutral", "temperror", "permerror")
		}
		for j, result := range record.AuthSPF {
			warnUnknown(report, fmt.Sprintf("record[%d].spf_auth[%d].scope", i, j), result.Scope, "", "mfrom")
			warnUnknown(report, fmt.Sprintf("record[%d].spf_auth[%d]", i, j), result.Result,
				"none", "pass", "fail", "softfail", "policy", "neutral", "temperror", "permerror")
		}
	}
}

func warnUnknown(report *AggregateReport, field, value string, allowed ...string) {
	for _, candidate := range allowed {
		if value == candidate {
			return
		}
	}
	report.warn("unexpected RFC 9990 %s value %q", field, value)
}

func (report *AggregateReport) warn(format string, args ...any) {
	report.ValidationWarnings = append(report.ValidationWarnings, fmt.Sprintf(format, args...))
}

type AggregateReportRecord struct {
	SourceIP     string           `xml:"row>source_ip"`
	Count        int64            `xml:"row>count"`
	Disposition  string           `xml:"row>policy_evaluated>disposition"`
	EvalDKIM     string           `xml:"row>policy_evaluated>dkim"`
	EvalSPF      string           `xml:"row>policy_evaluated>spf"`
	POReason     []POReason       `xml:"row>policy_evaluated>reason"`
	HeaderFrom   string           `xml:"identifiers>header_from"`
	EnvelopeFrom string           `xml:"identifiers>envelope_from"`
	EnvelopeTo   string           `xml:"identifiers>envelope_to"`
	AuthDKIM     []DKIMAuthResult `xml:"auth_results>dkim"`
	AuthSPF      []SPFAuthResult  `xml:"auth_results>spf"`
}

type POReason struct {
	Reason  string     `xml:"type"`
	Comment LangString `xml:"comment"`
}

type DKIMAuthResult struct {
	Domain      string     `xml:"domain"`
	Selector    string     `xml:"selector"`
	Result      string     `xml:"result"`
	HumanResult LangString `xml:"human_result"`
}

type SPFAuthResult struct {
	Domain      string     `xml:"domain"`
	Scope       string     `xml:"scope"`
	Result      string     `xml:"result"`
	HumanResult LangString `xml:"human_result"`
}
