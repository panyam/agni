package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// houseNamedDesign places a board whose rails are named function-first, the shape the built-in
// start-anchored vocabulary (VCC / VDD / +3V3) does not match. PMIC_CORE_3V3 is the tutorial
// project's own convention, which is why that project ships a conventions.yaml.
//
// It carries one net of each kind the rule has to tell apart: a house-named rail feeding a supply
// pin, a signal that merely SWINGS at a voltage and feeds an ordinary input, and a built-in-named
// rail that already classifies correctly.
func houseNamedDesign() *ir.Design {
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{{
			Name: "IC",
			Pins: []*ir.Pin{
				{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
				{Name: "VDD2", Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
				{Name: "RX", Designator: "3", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},
			},
		}}}},
		Components: []*ir.Component{
			{RefDes: "U1", Sections: []*ir.ComponentSection{{PartRef: "IC", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			{Name: "PMIC_CORE_3V3", Prov: &ir.Provenance{SourceFile: "t"},
				Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}},
			{Name: "UART_TX_1V8", Prov: &ir.Provenance{SourceFile: "t"},
				Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "3"}}},
			{Name: "+5V", Prov: &ir.Provenance{SourceFile: "t"},
				Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "2"}}},
		},
	}
}

// The rule's reason for existing: a house-named rail that the built-in vocabulary misses is
// reported, so the run says the analysis is short rather than reporting clean.
func TestRailNotClassifiedFiresOnAHouseNamedRail(t *testing.T) {
	fs := railNotClassified.Findings(check.NewModel(houseNamedDesign()))
	if len(fs) != 1 {
		t.Fatalf("want exactly one finding (PMIC_CORE_3V3), got %d: %+v", len(fs), fs)
	}
	if check.EntityRef(fs[0].Subject) != "PMIC_CORE_3V3" {
		t.Errorf("finding must name the unclassified rail, got %q", fs[0].Subject)
	}
	for _, want := range []string{"3.3", "supply pin", "--conventions"} {
		if !strings.Contains(fs[0].Message, want) {
			t.Errorf("message must mention %q so the fix is actionable, got %q", want, fs[0].Message)
		}
	}
}

// The discriminator, and the reason the rule is not just "voltage token and not a rail". A signal
// that SWINGS at 1.8V is named the same way a 1.8V rail is, and no naming grammar separates them. It
// feeds an ordinary input rather than a supply pin, and must stay silent.
func TestRailNotClassifiedIgnoresASignalAtALevel(t *testing.T) {
	for _, f := range railNotClassified.Findings(check.NewModel(houseNamedDesign())) {
		if check.EntityRef(f.Subject) == "UART_TX_1V8" {
			t.Errorf("a signal swinging at a level is not an unclassified rail: %s", f.Message)
		}
	}
}

// A rail the built-in vocabulary already matches carries the role, so there is no gap to report.
func TestRailNotClassifiedSilentOnAnAlreadyClassifiedRail(t *testing.T) {
	for _, f := range railNotClassified.Findings(check.NewModel(houseNamedDesign())) {
		if check.EntityRef(f.Subject) == "+5V" {
			t.Errorf("a correctly classified rail must not be reported: %s", f.Message)
		}
	}
}

// The intended end state: once the project declares its rail patterns, the rule goes silent on the
// nets it was reporting. A diagnostic that keeps firing after the fix is applied is a nag, and this
// is also what proves the finding was about the CONFIG rather than the design.
func TestRailNotClassifiedGoesSilentOnceTheLexiconIsDeclared(t *testing.T) {
	d := houseNamedDesign()
	// Stamp the role the way the ingestion pass would under a project lexicon matching `_<n>V<n>`.
	for _, n := range d.Nets {
		if n.Name == "PMIC_CORE_3V3" {
			n.Roles = classify.ConventionRoles(check.NetRoleRail)
		}
	}
	if fs := railNotClassified.Findings(check.NewModel(d)); len(fs) != 0 {
		t.Errorf("declaring the lexicon must silence the rule; got %d findings: %+v", len(fs), fs)
	}
}

// Skip-not-false-pass across the inputs the rule cannot evidence.
func TestRailNotClassifiedSilentWithoutEvidence(t *testing.T) {
	// A rail-looking name with no supply pin on it: one channel only, so no finding.
	noSupply := houseNamedDesign()
	noSupply.Nets[0].Connections = []*ir.Connection{{ComponentRef: "U1", PinRef: "3"}}
	if fs := railNotClassified.Findings(check.NewModel(noSupply)); len(fs) != 0 {
		t.Errorf("one channel is not evidence; want 0 findings, got %+v", fs)
	}

	// A supply pin on a net whose name declares no voltage: nothing to report about.
	noVolts := houseNamedDesign()
	noVolts.Nets[0].Name = "PMIC_CORE"
	for _, f := range railNotClassified.Findings(check.NewModel(noVolts)) {
		if check.EntityRef(f.Subject) == "PMIC_CORE" {
			t.Errorf("a name with no voltage token cannot be evidenced: %s", f.Message)
		}
	}

	// Ground carries a role of its own and is never this rule's subject.
	gnd := houseNamedDesign()
	gnd.Nets[0].Name = "GND_0V0"
	for _, f := range railNotClassified.Findings(check.NewModel(gnd)) {
		if strings.HasPrefix(check.EntityRef(f.Subject), "GND") {
			t.Errorf("ground must be excluded: %s", f.Message)
		}
	}
}
