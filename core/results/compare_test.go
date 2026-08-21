package results

import (
	"strings"
	"testing"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

func doc(producer string, axis bool, fs ...*checkspb.Finding) *checkspb.CheckResults {
	return &checkspb.CheckResults{
		Meta:     &checkspb.ResultsMeta{Producer: producer, CoverageAxis: axis},
		Design:   &checkspb.DesignRef{Source: "d"},
		Findings: fs,
	}
}

func onComp(rule, ref string) *checkspb.Finding {
	return &checkspb.Finding{Subject: &checkspb.Subject{Kind: "component", Ref: ref}, Rule: rule}
}

func onPin(rule, ref, pin string) *checkspb.Finding {
	return &checkspb.Finding{Subject: &checkspb.Subject{Kind: "component", Ref: ref, Pin: pin}, Rule: rule}
}

func onNet(rule, name string) *checkspb.Finding {
	return &checkspb.Finding{Subject: &checkspb.Subject{Kind: "net", Ref: name}, Rule: rule}
}

func unattached(rule string) *checkspb.Finding {
	return &checkspb.Finding{Subject: &checkspb.Subject{}, Rule: rule}
}

// TestCompareSplitsByEntity pins the three-way split. It keys on the entity each tool flagged rather
// than on rule names, because two tools have two rule vocabularies and a table asserting equivalence
// between them would be an unverified mapping that rots.
func TestCompareSplitsByEntity(t *testing.T) {
	ours := doc("agni", true, onComp("r1", "U1"), onNet("r2", "GND"), onComp("r1", "R9"))
	theirs := doc("kicad", false, onComp("k1", "U1"), onNet("k2", "GND"), onComp("k3", "C7"))
	c := Compare(ours, theirs)

	if got := strings.Join(c.Both, ","); got != "component:U1,net:GND" {
		t.Errorf("both = %v", c.Both)
	}
	if got := strings.Join(c.OursOnly, ","); got != "component:R9" {
		t.Errorf("ours only = %v", c.OursOnly)
	}
	if got := strings.Join(c.TheirsOnly, ","); got != "component:C7" {
		t.Errorf("theirs only = %v", c.TheirsOnly)
	}
}

// TestPinFindingKeysToItsComponent pins the granularity choice. One tool flags "R1 pin 2" where the
// other flags "R1"; treating those as different entities would report a disagreement that is only a
// difference in reporting granularity, which is exactly the false signal a differential harness must
// not produce.
func TestPinFindingKeysToItsComponent(t *testing.T) {
	c := Compare(doc("agni", true, onPin("r1", "R1", "2")), doc("kicad", false, onComp("k1", "R1")))
	if len(c.Both) != 1 || len(c.OursOnly) != 0 || len(c.TheirsOnly) != 0 {
		t.Errorf("a pin finding and a component finding on R1 should agree: both=%v ours=%v theirs=%v",
			c.Both, c.OursOnly, c.TheirsOnly)
	}
}

// TestUnattachedFindingsAreCountedNotDropped pins that a finding naming no entity is excluded from the
// comparison AND reported. Dropping it silently would make the overlap look better than it is; counting
// it as a disagreement would invent one.
func TestUnattachedFindingsAreCountedNotDropped(t *testing.T) {
	ours := doc("agni", true, onComp("r1", "U1"))
	theirs := doc("kicad", false, onComp("k1", "U1"), unattached("k2"), unattached("k2"))
	c := Compare(ours, theirs)
	if c.TheirsNotComparable != 2 {
		t.Errorf("theirs not comparable = %d, want 2", c.TheirsNotComparable)
	}
	if len(c.TheirsOnly) != 0 {
		t.Errorf("an unattached finding must not read as a disagreement: %v", c.TheirsOnly)
	}
	if len(c.Both) != 1 {
		t.Errorf("both = %v", c.Both)
	}
}

// TestCoOccurrenceIsRankedNotAsserted pins that rule pairs are reported by observed frequency. The
// table is evidence from which an equivalence might later be inferred, never a declaration that two
// rules ask the same question.
func TestCoOccurrenceIsRankedNotAsserted(t *testing.T) {
	ours := doc("agni", true, onComp("width", "R1"), onComp("width", "R2"), onComp("other", "R1"))
	theirs := doc("kicad", false, onComp("k_width", "R1"), onComp("k_width", "R2"), onComp("k_misc", "R1"))
	c := Compare(ours, theirs)
	if len(c.CoOccurring) == 0 {
		t.Fatal("no co-occurrence reported")
	}
	top := c.CoOccurring[0]
	if top.Ours != "width" || top.Theirs != "k_width" || top.Entities != 2 {
		t.Errorf("top pair = %+v, want width/k_width on 2 entities", top)
	}
}

// TestComparisonReportLabelsAMissingCoverageAxis pins the wording that keeps an import honest in a
// report. "They flagged nothing here" from a flat violation list does not mean what it means from a run
// that records what it could not check, and a reader comparing two columns will assume it does unless
// told.
func TestComparisonReportLabelsAMissingCoverageAxis(t *testing.T) {
	var b strings.Builder
	if err := WriteComparison(&b, Compare(doc("agni", true, onComp("r", "U1")), doc("kicad", false, onComp("k", "U1")))); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if strings.Count(out, "no coverage axis") != 1 {
		t.Errorf("exactly the side without a coverage axis should be labelled:\n%s", out)
	}
	if !strings.Contains(out, "not a claim that the rules ask the same question") {
		t.Errorf("the co-occurrence table must disclaim equivalence:\n%s", out)
	}
}

// The cross-tool join must see EVERY entity a finding names, not only the one it is filed under.
//
// A clearance violation is a distance between two nets and belongs to neither: agni files it under
// one, and another tool may file the same violation under the other. Joining on the subject alone
// reported that agreement as a disagreement, which is the opposite of what this comparison is for.
func TestEntityJoinSeesContextNotOnlyTheSubject(t *testing.T) {
	filedUnderA := &checkspb.Finding{
		Rule:    "copper-clearance",
		Subject: &checkspb.Subject{Kind: "net", Ref: "GND"},
		Context: []*checkspb.ContextSubject{{Subject: &checkspb.Subject{Kind: "net", Ref: "VBUS"}, Role: "neighbour"}},
	}
	filedUnderB := &checkspb.Finding{
		Rule:    "drc/clearance",
		Subject: &checkspb.Subject{Kind: "net", Ref: "VBUS"},
	}
	idx, skipped := entityRules(&checkspb.CheckResults{Findings: []*checkspb.Finding{filedUnderA, filedUnderB}})
	if skipped != 0 {
		t.Errorf("both findings name an entity; skipped %d", skipped)
	}
	got := idx["net:VBUS"]
	if len(got) != 2 {
		t.Fatalf("net:VBUS = %v, want both rules: the two tools flagged the same net and agree", got)
	}
	// And the subject side still joins, so the wider key did not replace the narrow one.
	if len(idx["net:GND"]) != 1 {
		t.Errorf("net:GND = %v, want the rule filed under it", idx["net:GND"])
	}
}
