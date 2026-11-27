package check

import (
	"fmt"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// UnfittedStrapPair returns why an input left floating by its unfitted resistors cannot be judged
// from the netlist, or "" when it can (agni issue 1916). The case is a strap with both positions
// drawn. One unfitted resistor runs from the net to a supply rail, and a second runs to ground.
// The designer drew both and fitted neither, so floating is one of the strap's positions. A
// four-level input such as a TUSB1046 EQ pin reads a floating pin as its own level. Only the part's
// datasheet says whether that level is valid. An input with one unfitted pull and nothing on the
// other side is not this case, and still floats (agni issue 1082).
func UnfittedStrapPair(m Model, n *ir.Net) string {
	if n == nil {
		return ""
	}
	var up, down string
	var idx map[string][]*ir.Net
	for _, c := range n.Connections {
		ref := c.ComponentRef
		if m.IsFitted(ref) || !m.HasClass(ref, ClassResistor) {
			continue
		}
		if idx == nil {
			idx = resistorNetIndex(m)
		}
		for _, far := range idx[ref] {
			switch {
			case far.Name == n.Name:
			case m.IsGroundNet(far) && down == "":
				down = ref
			case m.IsRailNet(far) && up == "":
				up = ref
			}
		}
	}
	if up == "" || down == "" {
		return ""
	}
	return fmt.Sprintf("unfitted %s pulls the net up and unfitted %s pulls it down, so floating is a strap position the designer chose, and only the part's datasheet says whether the input accepts it", up, down)
}
