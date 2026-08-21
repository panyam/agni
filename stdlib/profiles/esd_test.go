package profiles

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	_ "github.com/panyam/agni/stdlib/rules/builtin" // the core esd-protection rule, the parity oracle
)

// canExposed is a CAN bus brought out to a connector: CANH is clamped by a TVS sitting on it, CANL
// is not, and TXD/RXD run to the MCU with no connector on them. The asymmetry is the point — a
// fixture where every line looks alike cannot tell scoping from luck.
func canExposed() *ir.Design {
	return &ir.Design{
		Components: comps("U1", "J1", "TVS1"),
		Nets: []*ir.Net{
			net("BUS_CANH", "U1.1", "J1.1", "TVS1.1"),
			net("BUS_CANL", "U1.2", "J1.2"),
			net("BUS_TXD", "U1.3", "U2.1"),
			net("BUS_RXD", "U1.4", "U2.2"),
		},
	}
}

func esdFindings(t *testing.T, d *ir.Design) []check.Finding {
	t.Helper()
	r := esdRule(CAN, Requirement{Type: "esd"})
	if r == nil {
		t.Fatal("esdRule(CAN) returned nil; CAN declares signals, so it must compile")
	}
	return r.Findings(check.NewModel(d))
}

// TestESDRequirementScopedToExposedLines (WS3-061): the requirement reports the connector-facing line
// with no clamp, and NOT the clamped one, and NOT the two lines that never leave the board. The
// TXD/RXD half is the whole reason scope is read from the design instead of from a per-signal flag:
// they are declared CAN signals, so a flag-driven check would fail two lines that carry no exposure.
func TestESDRequirementScopedToExposedLines(t *testing.T) {
	got := map[string]bool{}
	for _, f := range esdFindings(t, canExposed()) {
		got[f.Subject] = true
	}
	if !got["BUS_CANL"] {
		t.Errorf("want BUS_CANL reported (on a connector, no clamp), got %v", got)
	}
	if got["BUS_CANH"] {
		t.Errorf("BUS_CANH has a TVS on it and must not be reported: %v", got)
	}
	for _, onboard := range []string{"BUS_TXD", "BUS_RXD"} {
		if got[onboard] {
			t.Errorf("%s never leaves the board and must not be reported: %v", onboard, got)
		}
	}
}

// TestESDRequirementSilentWithoutConnector (WS3-061): the same bus wired entirely on-board reports
// nothing. Declaring the requirement on a profile is therefore safe even where that bus is usually
// internal, which is why it ships on every connector-facing built-in rather than a hand-picked two.
func TestESDRequirementSilentWithoutConnector(t *testing.T) {
	d := canExposed()
	d.Components = comps("U1", "U2", "TVS1") // no connector anywhere
	if f := esdFindings(t, d); len(f) != 0 {
		t.Errorf("no connector on the board: want no findings, got %+v", f)
	}
}

// TestESDRequirementMatchesCoreRule is the parity check WS3-061 requires before any binding migrates
// from `rule: esd-protection + scope` to `profile: CAN`. The requirement is scoped and the catalog
// rule is design-wide, so they are compared on the nets the profile actually claims: within that
// scope the two must agree exactly. They are built to agree by construction (one shared
// external_signal_net scope, the same three exemptions at check.ProtectionReachHops), and this pins
// it so a later edit to either side cannot drift them apart silently.
func TestESDRequirementMatchesCoreRule(t *testing.T) {
	d := canExposed()
	m := check.NewModel(d)

	var core *check.Rule
	for _, r := range check.DefaultCatalog().Rules() {
		if r.Name == "esd-protection" {
			core = r
		}
	}
	if core == nil {
		t.Fatal("esd-protection not in DefaultCatalog; the parity oracle is missing")
	}

	inScope := func(subject string) bool { return strings.HasPrefix(subject, "BUS_") }
	coreSet := map[string]bool{}
	for _, f := range core.Findings(m) {
		if inScope(f.Subject) {
			coreSet[f.Subject] = true
		}
	}
	reqSet := map[string]bool{}
	for _, f := range esdFindings(t, d) {
		reqSet[f.Subject] = true
	}

	if len(coreSet) == 0 {
		t.Fatal("the core rule found nothing in scope, so this test would pass vacuously")
	}
	for n := range coreSet {
		if !reqSet[n] {
			t.Errorf("core rule reports %s but the esd requirement does not (req: %v)", n, reqSet)
		}
	}
	for n := range reqSet {
		if !coreSet[n] {
			t.Errorf("esd requirement reports %s but the core rule does not (core: %v)", n, coreSet)
		}
	}
}

// TestESDRequirementCreditsZener (WS3-061) pins a deliberate exemption that reads like a false pass
// until you know the partition: a Zener is NOT adequate ESD protection, but esd-clamp-not-tvs
// (WS3-078) is the rule that says so, and this requirement stays quiet rather than double-reporting
// the same net. The core rule exempts it for the same reason, so this is also parity.
func TestESDRequirementCreditsZener(t *testing.T) {
	d := canExposed()
	d.Components = comps("U1", "J1", "D1")
	d.Components[2].Attributes = map[string]string{"Description": "Zener diode 5V1"}
	d.Nets[0].Connections[2].ComponentRef = "D1" // the clamp on CANH is now a Zener

	got := map[string]bool{}
	for _, f := range esdFindings(t, d) {
		got[f.Subject] = true
	}
	if got["BUS_CANH"] {
		t.Errorf("a Zener-clamped net is esd-clamp-not-tvs's finding, not this one: %v", got)
	}
	// Without this the test passes vacuously: a requirement that reported nothing at all would
	// satisfy the assertion above while saying nothing about the Zener credit.
	if !got["BUS_CANL"] {
		t.Errorf("BUS_CANL is still unclamped and must still be reported: %v", got)
	}
}
