package builtin

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// symbolUnresolved reports a symbol reference the reader could not open or parse (WS1-052). See
// Detail.
//
// warning, not error: a missing library is almost always a defect in the INVOCATION (no
// --symbol-path, a library not checked out, a file not mounted) rather than in the design, and the
// board it is read from may be flawless. duplicate-ref-des earns error because a collision is an
// annotation slip the designer made. Nor info: unlike a bus on a flat sheet, which is usually
// already modeled and the flag is a tripwire, an unresolved symbol is ALWAYS a real gap in the read.
var symbolUnresolved = &check.Rule{
	Name:       "symbol-unresolved",
	Severity:   "warning",
	Summary:    "A symbol reference did not resolve, so its placements carry no pins.",
	Impact:     "A component whose symbol fails to load keeps its reference designator and loses its pins, and a part with no pins has no connections. The netlist is then missing every connection those parts make, which is indistinguishable from a design where they were never drawn: connectivity rules go quiet and report a clean pass over an incomplete read. The reader already suppresses dangling-endpoint findings for the same reason (a missing pin turns a real wire end into a phantom dangle), so without this the only visible effect of a lost symbol is that the design reports LESS.",
	Remedy:     "Re-run with `--symbol-path` pointing at the library that holds the symbol. Until it resolves, the part has no pins, so every connectivity result over it rests on an incomplete read.",
	Primitives: []string{"select"},
	Reads:      []string{"unresolved_symbol"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryIntegrity,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
		check.KeySite:         check.SiteDiagnostic, // the reader knows the open failed; the IR cannot infer it
	},
	Detail: ruleDoc("symbol-unresolved"),
	Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
		return check.Report(m.UnresolvedSymbols(), func(u *ir.UnresolvedSymbol) check.Finding {
			return check.Finding{
				Kind:    check.KindSymbol,
				Subject: u.GetSymref(),
				Message: unresolvedMessage(u),
				Prov:    u.GetProv(),
			}
		})
	}),
}

// unresolvedMessage names the affected placements, because the reference alone does not say how
// much of the netlist is missing: one unresolved decorative symbol and one unresolved 100-pin FPGA
// read identically until the parts are listed.
func unresolvedMessage(u *ir.UnresolvedSymbol) string {
	// The remedy lives HERE and only here. Every connectivity rule emits a companion inconclusive
	// finding that points back to this one, so repeating the fix on each of them would print the
	// same paragraph twenty times and bury the single finding that names the cause.
	const remedy = " Re-run with --symbol-path pointing at the library that holds it."
	refs := u.GetRefDes()
	if len(refs) == 0 {
		return fmt.Sprintf("symbol %q did not resolve; pins unknown.", u.GetSymref()) + remedy
	}
	verb := "carry"
	if len(refs) == 1 {
		verb = "carries"
	}
	return fmt.Sprintf("symbol %q did not resolve, so %s %s no pins (connections absent from the netlist).",
		u.GetSymref(), strings.Join(refs, ", "), verb) + remedy
}
