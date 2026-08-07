package check

import (
	"testing"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// pinRoleDesign places one component of the given ref-des with the given pin names, so the class the
// ref-des prefix implies drives the role gating (Q -> transistor, U -> IC).
func pinRoleDesign(refDes string, pinNames ...string) *ir.Design {
	part := &ir.PartType{Name: "P"}
	for i, n := range pinNames {
		part.Pins = append(part.Pins, &ir.Pin{Name: n, Designator: string(rune('1' + i))})
	}
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{part}}},
		Components: []*ir.Component{{
			RefDes: refDes, Sections: []*ir.ComponentSection{{PartRef: "P", LibraryRef: "lib"}},
			Prov: &ir.Provenance{SourceFile: "t"},
		}},
	}
}

func roleOf(m Model, refDes, designator string) PinRole { return m.PinRole(refDes, designator) }

// TestTransistorTerminalRoles (WS3-117): a transistor's G/S/D pin names reach the gate, source and
// drain roles, in both their short and spelled-out forms.
func TestTransistorTerminalRoles(t *testing.T) {
	m := NewModel(pinRoleDesign("Q1", "G", "S", "D"))
	for _, c := range []struct {
		des  string
		want PinRole
	}{{"1", RoleGate}, {"2", RoleSource}, {"3", RoleDrain}} {
		if got := roleOf(m, "Q1", c.des); got != c.want {
			t.Errorf("pin %s role = %q, want %q", c.des, got, c.want)
		}
	}

	spelled := NewModel(pinRoleDesign("Q2", "GATE", "SOURCE", "DRAIN"))
	for _, c := range []struct {
		des  string
		want PinRole
	}{{"1", RoleGate}, {"2", RoleSource}, {"3", RoleDrain}} {
		if got := roleOf(spelled, "Q2", c.des); got != c.want {
			t.Errorf("spelled pin %s role = %q, want %q", c.des, got, c.want)
		}
	}
}

// TestTerminalRolesAreClassGated is the assertion that matters most in this change. The terminal
// vocabularies are the shortest pin names on a board, so applying them to any part would mis-role
// most of a design — and a WRONG role is worse than a missing one, because a topology rule then walks
// a path that does not exist and reports on it.
//
// U1 is an IC (not a transistor) whose pins are named exactly like a FET's. Every one must stay
// unknown.
func TestTerminalRolesAreClassGated(t *testing.T) {
	m := NewModel(pinRoleDesign("U1", "G", "S", "D"))
	for _, des := range []string{"1", "2", "3"} {
		if got := roleOf(m, "U1", des); got != RoleUnknown {
			t.Errorf("pin %s on an IC = %q, want unknown: terminal roles are transistor-only", des, got)
		}
	}
}

// TestTerminalRolesDoNotShadowPowerAndGround: a transistor's supply and ground pins still resolve
// through the rail lexicon. The terminal switch runs first, so a regression that matched too broadly
// would silently take these over.
func TestTerminalRolesDoNotShadowPowerAndGround(t *testing.T) {
	m := NewModel(pinRoleDesign("Q1", "VDD", "GND"))
	if got := roleOf(m, "Q1", "1"); got != RolePower {
		t.Errorf("VDD on a transistor = %q, want power", got)
	}
	if got := roleOf(m, "Q1", "2"); got != RoleGround {
		t.Errorf("GND on a transistor = %q, want ground", got)
	}
}

// TestTerminalVocabIsAnchored: the patterns are whole-name anchored, so a pin whose name merely
// STARTS with a terminal letter is not one. Without anchoring, "SDA" and "SCLK" would read as source
// and "DIR" as drain on any transistor-classed part.
func TestTerminalVocabIsAnchored(t *testing.T) {
	m := NewModel(pinRoleDesign("Q1", "SDA", "SCLK", "DIR", "GPIO"))
	for _, des := range []string{"1", "2", "3", "4"} {
		if got := roleOf(m, "Q1", des); got != RoleUnknown {
			t.Errorf("pin %s = %q, want unknown: terminal patterns are whole-name anchored", des, got)
		}
	}
}

// TestTerminalRolesHonourConventions: a house that names its gate "DRV" declares that in the lexicon
// rather than patching the engine, which is the whole reason these are vocabularies and not literals
// (C20, and the polarity tokens above them are the counter-example this deliberately does not copy).
func TestTerminalRolesHonourConventions(t *testing.T) {
	v, err := classify.BuildRoleVocab(classify.RoleVocabConfig{
		Gate: classify.VocabPatterns{Patterns: []string{`^DRV$`}},
	})
	if err != nil {
		t.Fatalf("BuildRoleVocab: %v", err)
	}
	m := NewModel(pinRoleDesign("Q1", "DRV"), WithLexicon(&classify.Lexicon{Role: v}))
	if got := roleOf(m, "Q1", "1"); got != RoleGate {
		t.Errorf("DRV under a house lexicon = %q, want gate", got)
	}
	// The built-ins survive the merge: an override extends rather than replaces unless asked.
	if got := roleOf(NewModel(pinRoleDesign("Q2", "G"), WithLexicon(&classify.Lexicon{Role: v})), "Q2", "1"); got != RoleGate {
		t.Errorf("built-in G under a house lexicon = %q, want gate", got)
	}
}
