package check

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// A part number is a fact about the design, so every model joins it, provider or not (agni issue
// 748). It used to be built by the params constructor alone, so component.mpn read empty on any model
// built the other way (agni issue 757).
func TestEveryModelJoinsTheDesignsMPNs(t *testing.T) {
	if got := NewModel(supplyDesign("+5V", false, "LM1117")).ComponentMPN("U1"); got != "LM1117" {
		t.Errorf("component attribute: ComponentMPN(U1) = %q on a model with no provider, want LM1117", got)
	}
	if got := NewModel(supplyDesign("+5V", true, "LM1117")).ComponentMPN("U1"); got != "LM1117" {
		t.Errorf("BOM line: ComponentMPN(U1) = %q on a model with no provider, want LM1117", got)
	}
	d := supplyDesign("+5V", true, "FROM-BOM")
	d.Components[0].Mpn = "FROM-ATTRIBUTE"
	if got := NewModel(d).ComponentMPN("U1"); got != "FROM-BOM" {
		t.Errorf("ComponentMPN(U1) = %q, want the BOM line to win over the component's own mpn", got)
	}
}

// A nil board or provider is the same as leaving the option out, so a caller can pass whatever it
// holds without a nil check of its own.
func TestNilOptionsAreTheSameAsOmittingThem(t *testing.T) {
	m := NewModel(supplyDesign("+5V", false, "LM1117"), WithBoard(nil), WithParamProvider(nil))
	if m.HasBoard() || m.HasParams() {
		t.Errorf("HasBoard = %v, HasParams = %v with nil options, want both false", m.HasBoard(), m.HasParams())
	}
	if m.PartSpec("U1") != nil {
		t.Error("PartSpec(U1) is non-nil on a model with no provider")
	}
}

func TestWithBoardAttachesTheBoardTier(t *testing.T) {
	bg := &geom.BoardGeometry{Nets: []*geom.NetCopper{{
		Net:      "+5V",
		Segments: []*geom.TrackSegment{{Layer: "F.Cu", Width: 250}},
	}}}
	m := NewModel(supplyDesign("+5V", false, ""), WithBoard(bg))
	if !m.HasBoard() || len(m.BoardNets()) != 1 || len(m.BoardNets()[0].Segments) != 1 {
		t.Errorf("HasBoard = %v, BoardNets = %+v; want the one net and its segment", m.HasBoard(), m.BoardNets())
	}
	// The netlist half is unaffected by the board half.
	if m.ComponentMPN("U1") != "" || len(m.Pins()) != 1 {
		t.Errorf("attaching a board changed the netlist facts: pins %d", len(m.Pins()))
	}
}
