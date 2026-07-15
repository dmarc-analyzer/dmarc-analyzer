package backend

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/dmarc-analyzer/dmarc-analyzer/backend/model"
)

func TestDecoderAggregateReportLegacy(t *testing.T) {
	report := decodeAggregateFixture(t, "testdata/aggregate_legacy.xml")

	if report.Namespace() != "" {
		t.Fatalf("Namespace() = %q, want empty", report.Namespace())
	}
	if report.DetectedShape != model.AggregateReportShapeLegacy {
		t.Fatalf("DetectedShape = %q, want %q", report.DetectedShape, model.AggregateReportShapeLegacy)
	}
	if report.Percentage == nil || *report.Percentage != 0 {
		t.Fatalf("Percentage = %v, want pointer to 0", report.Percentage)
	}
	if len(report.ValidationWarnings) != 0 {
		t.Fatalf("ValidationWarnings = %v, want none", report.ValidationWarnings)
	}
}

func TestDecoderAggregateReportLegacyNamespace(t *testing.T) {
	report := decodeAggregateFixture(t, "testdata/aggregate_legacy_namespace.xml")

	if report.Namespace() != model.RFC7489Namespace {
		t.Fatalf("Namespace() = %q, want %q", report.Namespace(), model.RFC7489Namespace)
	}
	if report.DetectedShape != model.AggregateReportShapeLegacy {
		t.Fatalf("DetectedShape = %q, want %q", report.DetectedShape, model.AggregateReportShapeLegacy)
	}
	if report.ReportOrgName != "legacy-namespace-reporter.example" || len(report.Records) != 1 {
		t.Fatalf("legacy namespaced report was not decoded: %+v", report)
	}
}

func TestDecoderAggregateReportRFC9990(t *testing.T) {
	report := decodeAggregateFixture(t, "testdata/aggregate_rfc9990.xml")

	if report.Namespace() != model.RFC9990Namespace {
		t.Fatalf("Namespace() = %q, want %q", report.Namespace(), model.RFC9990Namespace)
	}
	if report.DetectedShape != model.AggregateReportShapeRFC9990 {
		t.Fatalf("DetectedShape = %q, want %q", report.DetectedShape, model.AggregateReportShapeRFC9990)
	}
	if report.Percentage != nil {
		t.Fatalf("Percentage = %v, want nil", report.Percentage)
	}
	if report.Version != "1.0" || report.Generator != "example-generator 1.0" {
		t.Fatalf("version/generator = %q/%q", report.Version, report.Generator)
	}
	if report.DiscoveryMethod != "treewalk" || report.NonexistentPolicy != "reject" || report.Testing != "n" {
		t.Fatalf("RFC 9990 policy fields were not retained: %+v", report)
	}
	if report.ExtraContact.Value != "https://rfc9990-reporter.example/dmarc" || report.ExtraContact.Language != "en" {
		t.Fatalf("ExtraContact = %+v", report.ExtraContact)
	}
	if len(report.Errors) != 1 || report.Errors[0].Value != "exemple d'erreur" || report.Errors[0].Language != "fr" {
		t.Fatalf("Errors = %+v", report.Errors)
	}
	if len(report.Records) != 1 || len(report.Records[0].AuthDKIM) != 2 || len(report.Records[0].AuthSPF) != 1 {
		t.Fatalf("authentication results were not retained: %+v", report.Records)
	}
	if report.Records[0].POReason[0].Comment.Language != "de" {
		t.Fatalf("policy override comment = %+v", report.Records[0].POReason[0].Comment)
	}
	if report.Records[0].AuthDKIM[0].HumanResult.Value != "signature verified" ||
		report.Records[0].AuthSPF[0].HumanResult.Language != "es" {
		t.Fatalf("human results were not retained: %+v %+v", report.Records[0].AuthDKIM, report.Records[0].AuthSPF)
	}
	if len(report.ValidationWarnings) != 0 {
		t.Fatalf("ValidationWarnings = %v, want none", report.ValidationWarnings)
	}
}

func TestDecoderAggregateReportVendorHybrid(t *testing.T) {
	report := decodeAggregateFixture(t, "testdata/aggregate_vendor_hybrid.xml")

	if report.DetectedShape != model.AggregateReportShapeVendorHybrid {
		t.Fatalf("DetectedShape = %q, want %q", report.DetectedShape, model.AggregateReportShapeVendorHybrid)
	}
	if report.NonexistentPolicy != "reject" || report.Percentage == nil || *report.Percentage != 100 {
		t.Fatalf("mixed policy fields were not retained: %+v", report)
	}
	if !slices.Contains(report.ValidationWarnings, "RFC 9990 fields found without the RFC 9990 namespace") {
		t.Fatalf("ValidationWarnings = %v", report.ValidationWarnings)
	}
}

func TestDecoderAggregateReportPreservesUnknownValues(t *testing.T) {
	report := decodeAggregateFixture(t, "testdata/aggregate_rfc9990_unknown_values.xml")

	if report.Version != "2.0" || report.Policy != "future-policy" || report.DiscoveryMethod != "future-method" {
		t.Fatalf("unknown values were not retained: %+v", report)
	}
	if len(report.ValidationWarnings) < 8 {
		t.Fatalf("ValidationWarnings = %v, want warnings for unknown values", report.ValidationWarnings)
	}
	for _, fragment := range []string{
		`unexpected RFC 9990 report version "2.0"`,
		"legacy pct element found in an RFC 9990 report",
		`unexpected RFC 9990 discovery_method value "future-method"`,
		`unexpected RFC 9990 p value "future-policy"`,
		`unexpected RFC 9990 record[0].reason[0] value "future-reason"`,
	} {
		if !warningsContain(report.ValidationWarnings, fragment) {
			t.Errorf("ValidationWarnings = %v, want %q", report.ValidationWarnings, fragment)
		}
	}
}

func decodeAggregateFixture(t *testing.T, path string) *model.AggregateReport {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	report, err := DecoderAggregateReport(file)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func warningsContain(warnings []string, fragment string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, fragment) {
			return true
		}
	}
	return false
}
