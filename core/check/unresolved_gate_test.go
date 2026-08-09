package check

import (
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// designWithUnresolved builds a design carrying one unresolved-symbol diagnostic, the state a read
// lands in when a symbol library is missing: components present, pins absent.
func designWithUnresolved() *ir.Design {
	return &ir.Design{
		Components: []*ir.Component{{RefDes: "R1"}, {RefDes: "U1"}},
		InputDiagnostics: &ir.InputDiagnostics{
			UnresolvedSymbols: []*ir.UnresolvedSymbol{{
				Symref: "res.sym", Kind: "xschem_sym", RefDes: []string{"R1"},
				Prov: &ir.Provenance{SourceFile: "board.sch"},
			}},
		},
	}
}

// alwaysFires is a stand-in for any rule that would report a defect. Whether the gate replaces its
// output is the whole question, so it must produce something to be replaced.
func alwaysFires(reads ...string) *Rule {
	return &Rule{
		Name: "test-rule", Severity: "error", Reads: reads,
		Eval: func(Model) []Finding {
			return []Finding{{Kind: KindNet, Subject: "SIG", Message: "a defect"}}
		},
	}
}

// TestUnresolvedGateMakesConnectivityRulesInconclusive is the point of WS1-052: a rule whose
// conclusions depend on pins must not report over a netlist that is missing connections. Before the
// gate it evaluated normally and its silence (or its finding) read as authoritative.
func TestUnresolvedGateMakesConnectivityRulesInconclusive(t *testing.T) {
	for _, reads := range [][]string{{"on_net"}, {"pin.electrical_type"}, {"pin.no_connect"}, {"net.names", "on_net"}} {
		fs := Run(NewModel(designWithUnresolved()), []*Rule{alwaysFires(reads...)})
		if len(fs) != 1 {
			t.Fatalf("reads %v: findings = %d, want 1 (the gate's own)", reads, len(fs))
		}
		if !fs[0].Inconclusive {
			t.Errorf("reads %v: finding = %+v, want Inconclusive", reads, fs[0])
		}
		if fs[0].Message == "a defect" {
			t.Errorf("reads %v: the rule evaluated anyway; its verdict is unsupported by this read", reads)
		}
		if !strings.Contains(fs[0].Message, "res.sym") {
			t.Errorf("reads %v: message %q does not name the unresolved symbol", reads, fs[0].Message)
		}
		if !strings.Contains(fs[0].Message, "--symbol-path") {
			t.Errorf("reads %v: message %q does not say how to fix it", reads, fs[0].Message)
		}
	}
}

// TestUnresolvedGateLeavesOtherRulesAlone: a rule that reads only names, classes or datasheet
// params is unaffected by a lost symbol, so gating it would convert working checks into noise. The
// gate has to cost nothing where it buys nothing.
func TestUnresolvedGateLeavesOtherRulesAlone(t *testing.T) {
	for _, reads := range [][]string{{"net.names"}, {"component.class"}, {"param.esd_rating"}, {"ref_des_collision"}} {
		fs := Run(NewModel(designWithUnresolved()), []*Rule{alwaysFires(reads...)})
		if len(fs) != 1 || fs[0].Inconclusive {
			t.Errorf("reads %v: findings = %+v, want the rule's own verdict, ungated", reads, fs)
		}
	}
}

// TestUnresolvedGateInactiveOnACleanRead: with every symbol resolved the gate must vanish
// completely. A gate that fired on clean designs would be worse than the silence it replaces.
func TestUnresolvedGateInactiveOnACleanRead(t *testing.T) {
	clean := &ir.Design{Components: []*ir.Component{{RefDes: "R1"}}}
	fs := Run(NewModel(clean), []*Rule{alwaysFires("on_net")})
	if len(fs) != 1 || fs[0].Inconclusive {
		t.Errorf("findings = %+v, want the rule's own verdict on a design whose symbols all resolved", fs)
	}
}

// TestUnresolvedGateIsDesignWide pins the deliberate bluntness (and documents what per-subject
// attribution would change): only R1 lost pins, but a rule reporting on U1 is gated too. The reader
// cannot support a claim that the unflagged parts are unaffected, so the gate does not make one.
func TestUnresolvedGateIsDesignWide(t *testing.T) {
	aboutU1 := &Rule{
		Name: "about-u1", Severity: "error", Reads: []string{"on_net"},
		Eval: func(Model) []Finding {
			return []Finding{{Kind: KindComponent, Subject: "U1", Message: "a defect on U1"}}
		},
	}
	fs := Run(NewModel(designWithUnresolved()), []*Rule{aboutU1})
	if len(fs) != 1 || !fs[0].Inconclusive {
		t.Errorf("findings = %+v, want inconclusive even though only R1 lost pins (design-wide gate)", fs)
	}
}
