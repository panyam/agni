package builtin

import (
	"slices"
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

// resolvedDesign is a read where some references loaded and some did not, which is what a reader now
// hands over (agni issue 418). unresolvedDesign stays as it was, so the finding assertions above keep
// running against a model that knows only about failures.
func resolvedDesign(rs []*ir.ResolvedSymbol, us ...*ir.UnresolvedSymbol) *ir.Design {
	d := unresolvedDesign(us...)
	d.InputDiagnostics.ResolvedSymbols = rs
	d.InputDiagnostics.Supplied = []string{"resolved_symbols"}
	return d
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
	if fs[0].Subject.Kind != check.KindSymbol {
		t.Errorf("kind = %q, want %q (the subject is a missing file, not a placed part)", fs[0].Subject.Kind, check.KindSymbol)
	}
	if check.EntityRef(fs[0].Subject) != "res.sym" {
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

// TestSymbolUnresolvedSilentWhenClean: a design whose symbols all resolved reports no FINDING, so the
// rule is evidence of a real gap rather than a permanent fixture of every run. It is no longer silent
// in verdicts, which is the whole of agni issue 418 and is asserted below.
func TestSymbolUnresolvedSilentWhenClean(t *testing.T) {
	if fs := symbolUnresolved.Findings(check.NewModel(unresolvedDesign())); len(fs) != 0 {
		t.Errorf("findings = %v, want none for a clean read", fs)
	}
}

// TestSymbolUnresolvedStatesConsideredSet (agni issue 418): the verdicts cover every reference the
// reader tried, not only the ones that failed, and the failures still project to exactly the findings
// the rule reported before.
func TestSymbolUnresolvedStatesConsideredSet(t *testing.T) {
	if !symbolUnresolved.StatesConsideredSet {
		t.Fatal("the rule must declare a considered set, or a clean run means nothing")
	}
	d := resolvedDesign(
		[]*ir.ResolvedSymbol{
			{Symref: "Device:R", Kind: "kicad_sym_embedded", PinCount: 2},
			{Symref: "ext:U", Kind: "kicad_sym_lib", PinCount: 48},
		},
		&ir.UnresolvedSymbol{Symref: "gone.sym", Kind: "xschem_sym", RefDes: []string{"R1"}},
	)
	m := check.NewModel(d)
	vs := symbolUnresolved.Eval(m)
	if len(vs) != 3 {
		t.Fatalf("verdicts = %d, want 3 (one per reference the reader tried)", len(vs))
	}
	byOutcome := map[check.Outcome][]string{}
	for _, v := range vs {
		byOutcome[v.Outcome] = append(byOutcome[v.Outcome], check.EntityRef(v.Subjects[0]))
	}
	if got := byOutcome[check.Pass]; !slices.Equal(got, []string{"Device:R", "ext:U"}) {
		t.Errorf("passes = %v, want the two references that loaded", got)
	}
	if got := byOutcome[check.Fail]; !slices.Equal(got, []string{"gone.sym"}) {
		t.Errorf("fails = %v, want the one reference that did not", got)
	}
	if fs := symbolUnresolved.Findings(m); len(fs) != 1 || check.EntityRef(fs[0].Subject) != "gone.sym" {
		t.Errorf("findings = %+v, want the failure only, unchanged by the pass verdicts", fs)
	}
	for _, v := range vs {
		if v.Subjects[0].Kind != check.KindSymbol {
			t.Errorf("%v: kind = %q, want %q", v.Subjects[0], v.Subjects[0].Kind, check.KindSymbol)
		}
	}
}

// TestSymbolUnresolvedPassCarriesPinCount is the witness check that matters here (build/evidence.md):
// a pass statement that reads the same on every subject proves nothing, and "the symbol resolved" is
// exactly such a statement. A stale library answering with an empty stub resolves as successfully as
// the real symbol and costs the netlist just as much, so the count is what a reader inspects.
//
// Asserting the two statements DIFFER, rather than that either contains a number, is what catches a
// witness that carries a constant.
func TestSymbolUnresolvedPassCarriesPinCount(t *testing.T) {
	statement := func(pins int32) string {
		vs := symbolUnresolved.Eval(check.NewModel(resolvedDesign(
			[]*ir.ResolvedSymbol{{Symref: "Device:R", Kind: "kicad_sym_embedded", PinCount: pins}})))
		if len(vs) != 1 || vs[0].Witness == nil {
			t.Fatalf("verdicts = %+v, want one carrying a witness", vs)
		}
		return vs[0].Witness.Statement
	}
	stub, real := statement(0), statement(48)
	if stub == real {
		t.Errorf("an empty stub and a 48-pin symbol read identically (%q), so the pass is decoration", stub)
	}
	if !strings.Contains(real, "48") {
		t.Errorf("statement %q does not carry the pin count", real)
	}
}

// TestSymbolUnresolvedPassNamesSource: a reference that came off --symbol-path is the half that can
// be missing on somebody else's machine, so the witness separates it from one the schematic carries
// itself.
func TestSymbolUnresolvedPassNamesSource(t *testing.T) {
	vs := symbolUnresolved.Eval(check.NewModel(resolvedDesign([]*ir.ResolvedSymbol{
		{Symref: "Device:R", Kind: "kicad_sym_embedded", PinCount: 2},
		{Symref: "ext:U", Kind: "kicad_sym_lib", PinCount: 2},
	})))
	if len(vs) != 2 {
		t.Fatalf("verdicts = %d, want 2", len(vs))
	}
	if vs[0].Witness.Statement == vs[1].Witness.Statement {
		t.Errorf("embedded and external read identically (%q)", vs[0].Witness.Statement)
	}
	if !strings.Contains(vs[1].Witness.Statement, "external") {
		t.Errorf("external statement = %q, does not say the answer came off the symbol path", vs[1].Witness.Statement)
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
