package builtin

import (
	"fmt"
	"strconv"
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
	Detail:              ruleDoc("symbol-unresolved"),
	Eval:                symbolUnresolvedVerdicts,
	StatesConsideredSet: true,
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

// symbolUnresolvedVerdicts decides every symbol reference the reader tried to resolve, the ones that
// failed and the ones that loaded.
//
// The rule could not do this until the reader kept both halves (agni issue 418). `unresolved_symbols`
// is a list of failures, so a rule reading only it could only ever report failures, and a design whose
// symbols all loaded produced the same nothing as a read that opened no symbol at all. That is the
// silence StatesConsideredSet exists to withhold, and it is worse here than for most rules, because an
// unresolved symbol makes the whole design report LESS: the parts keep their designators, lose their
// pins, and every connectivity rule over them goes quiet on an incomplete netlist.
//
// bus-not-modeled was already the diagnostic rule that got this right, and it got it right because
// `unmodeled_buses` holds every bus the reader saw rather than only the ones that went badly. The
// change here is the same shape moved one level down, into the readers.
//
// THE PASS CARRIES THE PIN COUNT, and that is the whole of what makes it evidence. "the symbol
// resolved" reads identically on a 100-pin FPGA and on a stale library entry that answered with an
// empty stub, and the second costs the netlist exactly what a missing file does. A count moves when
// the library does. A zero is still a Pass, because the rule's question is whether the reference
// resolved and it did; a graphic-only symbol legitimately declares no pin, so failing on the count
// would fire on title blocks. The number is there to be read.
//
// The KIND is carried through for the same reason. On KiCad a reference resolves either from the
// schematic's own lib_symbols block or from --symbol-path, and only the second can behave differently
// on someone else's machine.
func symbolUnresolvedVerdicts(m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, u := range m.UnresolvedSymbols() {
		v := check.Verdict{
			Subjects: []check.Entity{check.SymbolEntity(u.GetSymref())},
			Outcome:  check.Fail,
			Witness: &check.Witness{
				Statement: fmt.Sprintf("the reference did not resolve, so the %d placement(s) drawn with it carry no pins",
					len(u.GetRefDes())),
				Terms: []check.WitnessTerm{
					{Label: "placements without pins", Value: strconv.Itoa(len(u.GetRefDes()))},
					{Label: "pins", Value: "unknown"},
				},
			},
		}
		if refs := u.GetRefDes(); len(refs) > 0 {
			v.Context = []check.ContextSubject{check.Ctx(check.ComponentEntity(refs[0]), "placement")}
		}
		v.Finding = &check.Finding{Subject: check.SymbolEntity(u.GetSymref()), Message: unresolvedMessage(u), Prov: u.GetProv()}
		out = append(out, v)
	}
	for _, r := range m.ResolvedSymbols() {
		out = append(out, check.Verdict{
			Subjects: []check.Entity{check.SymbolEntity(r.GetSymref())},
			Outcome:  check.Pass,
			Witness: &check.Witness{
				Statement: fmt.Sprintf("the reference resolved from %s and declares %d pin(s)", resolvedSource(r.GetKind()), r.GetPinCount()),
				Terms: []check.WitnessTerm{
					{Label: "pins", Value: strconv.Itoa(int(r.GetPinCount()))},
					{Label: "source", Value: resolvedSource(r.GetKind())},
				},
			},
		})
	}
	return out
}

// resolvedSource turns a reader's construct kind into the phrase a witness can carry, so the pass
// says WHERE the answer came from. An external library is the half that can go missing on another
// machine, and a reader chasing a lost connection wants that separated from a symbol the schematic
// carries itself. An unrecognised kind falls back to the kind string rather than to "a library",
// because guessing here would state the one thing the witness exists to be precise about.
func resolvedSource(kind string) string {
	switch kind {
	case "kicad_sym_embedded":
		return "the schematic's own lib_symbols"
	case "kicad_sym_lib", "xschem_sym", "geda_sym":
		return "an external symbol library"
	}
	return kind
}
