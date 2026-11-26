package check

import (
	"fmt"
	"slices"

	"github.com/panyam/agni/core/ident"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// PullUpOffBoard returns why a bus line that reaches no pull-up cannot be judged from this board, or
// "" when it can (agni issue 1911). A caller asks it only after PullUpVerdict fails. There are two
// cases, and both read the net and the nets joined to it through zero-ohm links (JoinedRail), since
// a 0 Ohm link is a net boundary in the netlist and not in the circuit.
//
// The first case is a line that reaches a connector the design declares internal. A module socket
// such as a Jetson board-to-board connector carries the line to a module that may hold the pull-up,
// and the baseboard alone cannot say. A connector a cable plugs into stays judged.
//
// The second case is a line with one part on it and only test points beside it, such as a
// multiplexer's unused channel. No second device uses that bus, so nothing on it needs a pull-up.
func PullUpOffBoard(m Model, n *ir.Net) string {
	if n == nil {
		return ""
	}
	joined := JoinedRail(m, n)
	var internal, parts []string
	tps := 0
	for _, j := range joined {
		for _, c := range j.Net.GetConnections() {
			ref := c.GetComponentRef()
			if IsVirtualRef(ref) || !m.IsFitted(ref) || ZeroOhmLink(m, ref) {
				continue
			}
			switch {
			case m.HasClass(ref, ClassTestPoint):
				tps++
			case m.HasClass(ref, ClassConnector) && !m.ExposedConnector(ref):
				internal = appendUnique(internal, ref)
				parts = appendUnique(parts, ref)
			default:
				parts = appendUnique(parts, ref)
			}
		}
	}
	if len(internal) > 0 {
		slices.SortFunc(internal, ident.CompareNatural)
		return fmt.Sprintf("the line leaves the board through %s, which the design declares internal, so the part across it may carry the pull-up", joinRefs(internal))
	}
	if len(parts) == 1 && tps > 0 {
		return fmt.Sprintf("only %s and test points sit on the line, so no second device uses the bus", parts[0])
	}
	return ""
}

func appendUnique(xs []string, x string) []string {
	if slices.Contains(xs, x) {
		return xs
	}
	return append(xs, x)
}

// joinRefs writes "J1", "J1 and J18", or "J1, J8 and J18".
func joinRefs(refs []string) string {
	switch len(refs) {
	case 1:
		return refs[0]
	case 2:
		return refs[0] + " and " + refs[1]
	}
	out := refs[0]
	for _, r := range refs[1 : len(refs)-1] {
		out += ", " + r
	}
	return out + " and " + refs[len(refs)-1]
}
