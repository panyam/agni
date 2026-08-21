package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func unresolvedDesign(us ...*ir.UnresolvedSymbol) *ir.Design {
	return &ir.Design{
		Components:       []*ir.Component{{RefDes: "R1"}, {RefDes: "R2"}},
		InputDiagnostics: &ir.InputDiagnostics{UnresolvedSymbols: us},
	}
}

// TestSymbolUnresolvedReportsPerReference (WS1-052): one finding per missing REFERENCE, naming the
// placements it cost pins. Per-reference because one missing file is one cause; listing the parts
// is what says how much of the netlist is gone.
func TestSymbolUnresolvedReportsPerReference(t *testing.T) {
	d := unresolvedDesign(&ir.UnresolvedSymbol{
		Symref: "res.sym", Kind: "xschem_sym", RefDes: []string{"R1", "R2"},
		Prov: &ir.Provenance{SourceFile: "board.sch"},
	})
	fs := symbolUnresolved.Findings(check.NewModel(d))
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1 per unresolved reference", len(fs))
	}
	if fs[0].Kind != check.KindSymbol {
		t.Errorf("kind = %q, want %q (the subject is a missing file, not a placed part)", fs[0].Kind, check.KindSymbol)
	}
	if fs[0].Subject != "res.sym" {
		t.Errorf("subject = %q, want the symbol reference", fs[0].Subject)
	}
	for _, ref := range []string{"R1", "R2"} {
		if !strings.Contains(fs[0].Message, ref) {
			t.Errorf("message %q does not name affected placement %s", fs[0].Message, ref)
		}
	}
	// This rule is the ONE place the remedy is stated: the connectivity rules gated by the same
	// cause point here rather than each repeating it, so if it is missing here it is nowhere.
	if !strings.Contains(fs[0].Message, "--symbol-path") {
		t.Errorf("message %q does not say how to fix it", fs[0].Message)
	}
}

// TestSymbolUnresolvedSilentWhenClean: a design whose symbols all resolved reports nothing, so the
// rule is evidence of a real gap rather than a permanent fixture of every run.
func TestSymbolUnresolvedSilentWhenClean(t *testing.T) {
	if fs := symbolUnresolved.Findings(check.NewModel(unresolvedDesign())); len(fs) != 0 {
		t.Errorf("findings = %v, want none for a clean read", fs)
	}
}

// TestSymbolUnresolvedSeverityIsWarning pins the deliberate choice against error: a missing library
// is almost always a defect in the INVOCATION, and the board it is read from may be flawless.
// duplicate-ref-des earns error because the collision is in the design itself.
func TestSymbolUnresolvedSeverityIsWarning(t *testing.T) {
	if symbolUnresolved.Severity != "warning" {
		t.Errorf("severity = %q, want warning", symbolUnresolved.Severity)
	}
	if got := symbolUnresolved.Tags[check.KeySite]; got != check.SiteDiagnostic {
		t.Errorf("site = %q, want %q (only the reader knows the open failed)", got, check.SiteDiagnostic)
	}
}

// TestSymbolUnresolvedMessageWithoutPlacements: a record with no ref_des still produces a usable
// message rather than a dangling sentence fragment.
func TestSymbolUnresolvedMessageWithoutPlacements(t *testing.T) {
	d := unresolvedDesign(&ir.UnresolvedSymbol{Symref: "ghost.sym", Kind: "xschem_sym"})
	fs := symbolUnresolved.Findings(check.NewModel(d))
	if len(fs) != 1 || !strings.Contains(fs[0].Message, "ghost.sym") {
		t.Errorf("findings = %+v, want one naming ghost.sym", fs)
	}
}
