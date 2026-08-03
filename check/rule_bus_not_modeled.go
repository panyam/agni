package check

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// busNotModeled flags a bus whose member signals are NOT resolved into distinct nets (WS1-034). It is a
// read-health tripwire, not a design defect, so it stays info severity while the reader is the known
// cause. It does NOT fire merely because a bus is present: on a flat sheet KiCad requires a member
// label on every bus tap, so the members already form nets by name (verified against kicad-cli) and the
// bus is effectively modeled — the finding is silent there. It fires when the members cannot be
// confirmed as nets (a hierarchical bus port whose members do not cross the sheet boundary, an
// alias-named bus with no member taps), where connectivity really is unmodeled. See Detail.
var busNotModeled = &Rule{
	Name:       "bus-not-modeled",
	Severity:   "info",
	Summary:    "A bus's member signals are not resolved into distinct nets.",
	Impact:     "A bus groups many signals under one drawn line. When its members do not each resolve to a net, connectivity for those signals is unmodeled: a diff, a rule, or a highlight over them is unreliable. On a flat sheet the member labels on the taps already form the nets, so this is silent; it fires where the members are genuinely unresolved (e.g. a hierarchical bus port), marking the read as incomplete rather than letting silence read as coverage.",
	Primitives: []string{"select"},
	Reads:      []string{"bus.construct", "net.names"},
	Tags: map[string]string{
		KeyCategory:     CategoryIntegrity,
		KeyTier:         "P",
		KeyDistribution: DistOpen,
		KeySite:         SiteDiagnostic, // detected by the reader from the source's bus syntax
	},
	Detail: ruleDoc("bus-not-modeled"),
	Eval: func(m Model) []Finding {
		var out []Finding
		for _, b := range m.UnmodeledBuses() {
			if busResolved(m, b) {
				continue
			}
			out = append(out, Finding{
				Kind:    KindBus, // a bus, not a net: its highlight join is the uuid (WS7-042b)
				Subject: busSubject(b),
				Message: busNotModeledMessage,
				Prov:    b.Prov,
			})
		}
		return out
	},
}

// busResolved reports whether every member signal the bus names is already a net in the design (formed
// by the member labels the taps carry). A bus with no known member set cannot be confirmed resolved, so
// it is treated as unresolved (flagged). Member names are matched bare, so a hierarchy read whose member
// nets are qualified per-sheet (`/amp1/DATA0`) does NOT match the bare `DATA0` — correctly flagging a
// bus whose members do not cross the sheet boundary.
func busResolved(m Model, b *ir.BusNotModeled) bool {
	members := b.GetMembers()
	if len(members) == 0 {
		return false
	}
	for _, mem := range members {
		if !m.HasNetName(mem) {
			return false
		}
	}
	return true
}

// busNotModeledMessage is the finding message; the specific bus and its unresolved members live in the
// finding's Subject and Prov.
const busNotModeledMessage = "bus members are not modeled as nets (connectivity unresolved)"

// busSubject names the finding after the bus: its source name (a range label `DATA[7:0]` or a bus-alias
// name), else the construct kind for a nameless one.
func busSubject(b *ir.BusNotModeled) string {
	if b.GetLabel() != "" {
		return b.GetLabel()
	}
	return b.GetKind()
}
