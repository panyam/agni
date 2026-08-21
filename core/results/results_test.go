package results

import (
	"strings"
	"testing"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"google.golang.org/protobuf/proto"
)

func finding(rule, severity, ref string) *checkspb.Finding {
	return &checkspb.Finding{Subject: &checkspb.Subject{Kind: "net", Ref: ref}, Rule: rule, Severity: severity, Message: rule + " on " + ref}
}

// TestPivotOrdersWorstFirstAndKeepsAnUnknownSeverityOnTop pins the ordering an unknown severity gets.
// A provider's custom level must lead rather than sink to the bottom, because a level this build does
// not recognize is the one a reader is most likely to be surprised by.
func TestPivotOrdersWorstFirstAndKeepsAnUnknownSeverityOnTop(t *testing.T) {
	fs := []*checkspb.Finding{
		finding("a", "info", "N1"),
		finding("b", "error", "N2"),
		finding("c", "warning", "N3"),
		finding("d", "critical", "N4"),
	}
	rep := Pivot("src.edn", fs, map[string]string{}, 4)

	var got []string
	for _, s := range rep.GetSections() {
		got = append(got, s.GetSeverity())
	}
	want := []string{"critical", "error", "warning", "info"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("section order = %v, want %v", got, want)
	}
	if rep.GetSource() != "src.edn" || rep.GetRulesRun() != 4 {
		t.Errorf("source/rulesRun = %q/%d", rep.GetSource(), rep.GetRulesRun())
	}
}

// TestPivotIsIdempotentUnderItsOwnFlattening is the property that lets a document written from a
// report render back to the same report: re-pivoting a report's own findings must reproduce it. If it
// did not, the flat findings list in a document would be a lossy projection of what the terminal
// showed, and the two surfaces would silently disagree.
func TestPivotIsIdempotentUnderItsOwnFlattening(t *testing.T) {
	fs := []*checkspb.Finding{
		finding("a", "info", "N1"),
		finding("b", "error", "N2"),
		finding("a", "info", "N3"),
		finding("c", "error", "N4"),
	}
	summaries := map[string]string{"a": "A rule", "b": "B rule", "c": "C rule"}
	first := Pivot("src.edn", fs, summaries, 3)

	var flat []*checkspb.Finding
	for _, s := range first.GetSections() {
		for _, g := range s.GetRules() {
			flat = append(flat, g.GetFindings()...)
		}
	}
	if again := Pivot("src.edn", flat, summaries, 3); !proto.Equal(first, again) {
		t.Errorf("re-pivoting a report's own findings changed it:\n got %v\nwant %v", again, first)
	}
}

// TestMarshalParseRoundTrip pins that a document survives the encoding unchanged.
func TestMarshalParseRoundTrip(t *testing.T) {
	doc := &checkspb.CheckResults{
		Meta:     &checkspb.ResultsMeta{Schema: Schema, Producer: Producer, ProducerVersion: "abc123", CreatedAt: "2026-01-01T00:00:00Z"},
		Design:   &checkspb.DesignRef{Source: "d.edn", ContentHash: "sha256:00"},
		Run:      &checkspb.RunConfig{Params: true, Conventions: "acme"},
		Catalog:  []*checkspb.RuleRecord{{Name: "a", Severity: "info", Summary: "A rule", Tags: map[string]string{"category": "connectivity"}}},
		Findings: []*checkspb.Finding{finding("a", "info", "N1")},
	}
	b, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(doc, got) {
		t.Errorf("round trip changed the document:\n got %v\nwant %v", got, doc)
	}
}

// TestParseRejectsAnUnreadableSchema pins that an unknown or absent schema version is an error. A
// best-effort read of a future document would produce a findings list shorter than the run that made
// it with nothing to say so, which is the silence-as-coverage failure this whole contract rules out.
func TestParseRejectsAnUnreadableSchema(t *testing.T) {
	for name, body := range map[string]string{
		"future": `{"meta":{"schema":"agni.checks.results/v9"}}`,
		"absent": `{"findings":[]}`,
		"other":  `{"meta":{"schema":"some.other/v1"}}`,
	} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("%s: Parse should reject it", name)
		}
	}
}

// TestParseToleratesAnUnknownField pins the other side of the version rule: an ADDITIVE field is not
// a breaking change, so a document from a newer build of the same schema still reads.
func TestParseToleratesAnUnknownField(t *testing.T) {
	body := `{"meta":{"schema":"` + Schema + `","producer":"agni"},"somethingNewer":{"x":1}}`
	doc, err := Parse([]byte(body))
	if err != nil {
		t.Fatalf("an unknown field within a known schema should read: %v", err)
	}
	if doc.GetMeta().GetProducer() != "agni" {
		t.Errorf("producer = %q", doc.GetMeta().GetProducer())
	}
}
