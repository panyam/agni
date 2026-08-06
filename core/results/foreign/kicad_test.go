package foreign

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/panyam/agni/core/results"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

func readFixture(t *testing.T, name string) *checkspb.CheckResults {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := ReadKiCad(f, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ReadKiCad(%s): %v", name, err)
	}
	return doc
}

// TestImportedDocumentDeclaresItsWeakness is the assertion that matters most in this package. An
// imported document and a clean native run both look like "few findings"; nothing in the data
// distinguishes them. So the weaker artifact has to SAY it is weaker, or a consumer will eventually
// read a vendor report's silence as a pass.
func TestImportedDocumentDeclaresItsWeakness(t *testing.T) {
	for _, name := range []string{"drc.json", "erc.json"} {
		doc := readFixture(t, name)
		if doc.GetMeta().GetCoverageAxis() {
			t.Errorf("%s: coverage_axis is true; a vendor report has no such axis", name)
		}
		if doc.GetManifest() != "" || len(doc.GetAreas()) != 0 {
			t.Errorf("%s: an import must not manufacture review outcomes", name)
		}
		if len(doc.GetCatalog()) != 0 {
			t.Errorf("%s: an import must not claim a rule catalog it did not run", name)
		}
	}
}

// TestReadKiCadDRC pins the DRC path: the producer, the version, and namespaced rule names carrying
// the vendor's own vocabulary verbatim.
func TestReadKiCadDRC(t *testing.T) {
	doc := readFixture(t, "drc.json")
	if got := doc.GetMeta().GetProducer(); got != ProducerKiCadDRC {
		t.Errorf("producer = %q, want %q", got, ProducerKiCadDRC)
	}
	if doc.GetMeta().GetProducerVersion() == "" {
		t.Error("producer version is empty; two reports are only comparable once each names its build")
	}
	if doc.GetMeta().GetSchema() != results.Schema {
		t.Errorf("schema = %q, want the results schema so `agni results` can render it", doc.GetMeta().GetSchema())
	}
	if len(doc.GetFindings()) == 0 {
		t.Fatal("no findings imported")
	}
	for _, f := range doc.GetFindings() {
		if !strings.HasPrefix(f.GetRule(), RulePrefixDRC) {
			t.Errorf("rule %q is not namespaced; a foreign finding must stay visibly foreign", f.GetRule())
		}
		if f.GetSeverity() != "error" && f.GetSeverity() != "warning" && f.GetSeverity() != "info" {
			t.Errorf("severity %q is outside our vocabulary and would sort above error", f.GetSeverity())
		}
	}
}

// TestReadKiCadERC pins the ERC path, whose violations are nested per sheet rather than flat.
func TestReadKiCadERC(t *testing.T) {
	doc := readFixture(t, "erc.json")
	if got := doc.GetMeta().GetProducer(); got != ProducerKiCadERC {
		t.Errorf("producer = %q, want %q", got, ProducerKiCadERC)
	}
	if len(doc.GetFindings()) == 0 {
		t.Fatal("no findings imported from the per-sheet violations")
	}
	for _, f := range doc.GetFindings() {
		if !strings.HasPrefix(f.GetRule(), RulePrefixERC) {
			t.Errorf("rule %q is not namespaced", f.GetRule())
		}
	}
}

// TestOneFindingPerItem pins that a violation naming two items yields two findings. A clearance
// violation names both things that are too close together, and both are genuinely implicated;
// collapsing to one would silently pick a side.
func TestOneFindingPerItem(t *testing.T) {
	doc := readFixture(t, "drc.json")
	var multi int
	for _, f := range doc.GetFindings() {
		if strings.Contains(f.GetMessage(), " — ") {
			multi++
		}
	}
	if multi == 0 {
		t.Fatal("no finding carries its item description; the join has nothing to parse")
	}
	byMessage := map[string]int{}
	for _, f := range doc.GetFindings() {
		if i := strings.Index(f.GetMessage(), " — "); i > 0 {
			byMessage[f.GetMessage()[:i]]++
		}
	}
	pairs := 0
	for _, n := range byMessage {
		if n > 1 {
			pairs++
		}
	}
	if pairs == 0 {
		t.Error("no violation produced more than one finding; a two-item clearance violation should")
	}
}

// TestSeverityMapping pins that KiCad's levels land inside our vocabulary. "exclusion" is the
// interesting one: it is a violation the user acknowledged, and passing it through verbatim would sort
// it ABOVE error, because SeverityRank ranks an unrecognized level highest.
func TestSeverityMapping(t *testing.T) {
	cases := map[string]string{
		"error": "error", "warning": "warning", "exclusion": "info",
		"ignore": "info", "": "warning", "something-new": "warning",
	}
	for in, want := range cases {
		if got := severity(in); got != want {
			t.Errorf("severity(%q) = %q, want %q", in, got, want)
		}
		if results.SeverityRank(severity(in)) > results.SeverityRank("error") {
			t.Errorf("severity(%q) sorts above error", in)
		}
	}
}

// TestUnrecognizedReportIsAnError pins that a file we cannot classify fails loudly. Importing it as an
// empty document would report zero findings, which reads exactly like a design the tool was happy with.
func TestUnrecognizedReportIsAnError(t *testing.T) {
	for _, body := range []string{`{}`, `{"hello": "world"}`, `{"$schema": "https://example.com/other.v1.json"}`} {
		if _, err := ReadKiCad(strings.NewReader(body), time.Now()); err == nil {
			t.Errorf("%s: should not import as an empty document", body)
		}
	}
}
