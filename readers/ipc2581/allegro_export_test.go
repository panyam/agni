package ipc2581

import (
	"strings"
	"testing"
)

// allegroShapes is a board written the way Allegro's IPC-2581 export writes one (agni 1383). Each
// track's width is an inline LineDesc, a straight run is a Line, the drill layer has no Span and
// holds each hit in a Set with no net, the inner copper keeps Allegro's name, and an unconnected pin
// sits on an invented Unused_ net.
const allegroShapes = `<?xml version="1.0" encoding="UTF-8"?>
<IPC-2581 revision="B" xmlns="http://webstds.ipc.org/2581">
  <Content>
    <DictionaryStandard units="MILLIMETER">
      <EntryStandard id="CIRCLE_1"><Circle diameter="0.600000"/></EntryStandard>
    </DictionaryStandard>
  </Content>
  <Ecad name="ecad">
    <CadData>
      <Layer name="TOP" layerFunction="CONDUCTOR" side="TOP" polarity="POSITIVE"/>
      <Layer name="DIELECTRIC_BELOW_TOP" layerFunction="DIELCORE" side="INTERNAL" polarity="POSITIVE"/>
      <Layer name="L2-GND" layerFunction="PLANE" side="INTERNAL" polarity="POSITIVE"/>
      <Layer name="BOTTOM" layerFunction="CONDUCTOR" side="BOTTOM" polarity="POSITIVE"/>
      <Layer name="IPC2581DRILL_1-3" layerFunction="DRILL" side="NONE" polarity="POSITIVE"/>
      <Step name="pcb">
        <Package name="TH2" type="OTHER">
          <Pin number="1" type="THRU"><Location x="-1.000000" y="0.000000"/><StandardPrimitiveRef id="CIRCLE_1"/></Pin>
          <Pin number="2" type="THRU"><Location x="1.000000" y="0.000000"/><StandardPrimitiveRef id="CIRCLE_1"/></Pin>
        </Package>
        <Component refDes="J1" packageRef="TH2" layerRef="TOP" part="HDR" mountType="THMT">
          <Xform rotation="90.000"/>
          <Location x="5.000000" y="5.000000"/>
        </Component>
        <LogicalNet name="N1"><PinRef pin="1" componentRef="J1"/></LogicalNet>
        <LogicalNet name="Unused_05CE8C28"><PinRef pin="2" componentRef="J1"/></LogicalNet>
        <LayerFeature layerRef="TOP">
          <Set net="N1" padUsage="NONE">
            <Pad padstackDefRef="TH60"><PinRef pin="1" componentRef="J1"/><Location x="5.000000" y="4.000000"/><StandardPrimitiveRef id="CIRCLE_1"/></Pad>
          </Set>
          <Set net="Unused_05CE8C28" padUsage="NONE">
            <Pad padstackDefRef="TH60"><PinRef pin="2" componentRef="J1"/><Location x="5.000000" y="6.000000"/><StandardPrimitiveRef id="CIRCLE_1"/></Pad>
          </Set>
          <Set net="N1" padUsage="VIA">
            <Pad padstackDefRef="VIA_250"><Location x="8.000000" y="4.000000"/><StandardPrimitiveRef id="CIRCLE_1"/></Pad>
          </Set>
          <Set net="N1">
            <Features>
              <Location x="0.0" y="0.0"/>
              <Polyline>
                <PolyBegin x="5.000000" y="4.000000"/>
                <PolyStepSegment x="6.000000" y="3.000000"/>
                <LineDesc lineEnd="ROUND" lineWidth="0.250000"/>
              </Polyline>
            </Features>
          </Set>
          <Set net="N1">
            <Features>
              <Location x="0.0" y="0.0"/>
              <Line startX="6.000000" startY="3.000000" endX="8.000000" endY="4.000000">
                <LineDesc lineEnd="ROUND" lineWidth="0.150000"/>
              </Line>
            </Features>
          </Set>
        </LayerFeature>
        <LayerFeature layerRef="L2-GND">
          <Set net="N1" padUsage="VIA">
            <Pad padstackDefRef="VIA_250"><Location x="8.000000" y="4.000000"/><StandardPrimitiveRef id="CIRCLE_1"/></Pad>
          </Set>
        </LayerFeature>
        <LayerFeature layerRef="IPC2581DRILL_1-3">
          <Set><Hole name="H1" diameter="0.2500" platingStatus="VIA" x="8.0000" y="4.0000"/></Set>
          <Set><Hole name="H2" diameter="1.0000" platingStatus="PLATED" x="5.0000" y="4.0000"/></Set>
          <Set><Hole name="H3" diameter="1.0000" platingStatus="PLATED" x="5.0000" y="6.0000"/></Set>
        </LayerFeature>
      </Step>
    </CadData>
  </Ecad>
</IPC-2581>
`

// TestReadsAnAllegroExportsCopper reads the copper Allegro's export writes the way the board does.
func TestReadsAnAllegroExportsCopper(t *testing.T) {
	g, err := ReadBoardGeometry(strings.NewReader(allegroShapes), "allegro.xml")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, l := range g.GetLayers() {
		names = append(names, l.GetName())
	}
	if got := strings.Join(names, " "); got != "F.Cu DIELECTRIC_BELOW_TOP In1.Cu B.Cu IPC2581DRILL_1-3" {
		t.Errorf("layers = %s, want the copper named F.Cu, In1.Cu, B.Cu", got)
	}

	var widths []int64
	vias := 0
	for _, nc := range g.GetNets() {
		if nc.GetNet() != "N1" {
			if len(nc.GetSegments())+len(nc.GetVias()) > 0 {
				t.Errorf("copper on %q, want all of it on N1", nc.GetNet())
			}
			continue
		}
		for _, s := range nc.GetSegments() {
			widths = append(widths, s.GetWidth())
			if s.GetLayer() != "F.Cu" {
				t.Errorf("a segment on %s, want F.Cu", s.GetLayer())
			}
		}
		for _, v := range nc.GetVias() {
			vias++
			if v.GetDrill() != 250_000 || v.GetLayerFrom() != "F.Cu" || v.GetLayerTo() != "B.Cu" || v.GetSize() != 600_000 {
				t.Errorf("via = %v, want drill 0.25 mm through F.Cu-B.Cu with its 0.6 mm land", v)
			}
		}
	}
	if len(widths) != 2 || widths[0] != 250_000 || widths[1] != 150_000 {
		t.Errorf("N1 track widths = %v, want the Polyline's 0.25 mm and the Line's 0.15 mm", widths)
	}
	if vias != 1 {
		t.Errorf("N1 has %d vias, want the one drill hit at its land", vias)
	}

	pads := map[string][2]any{}
	for _, pl := range g.GetPlacements() {
		for _, p := range pl.GetPads() {
			pads[pl.GetRefDes()+"."+p.GetNumber()] = [2]any{p.GetNet(), p.GetDrill()}
		}
	}
	want := map[string][2]any{"J1.1": {"N1", int64(1_000_000)}, "J1.2": {"", int64(1_000_000)}}
	for pin, w := range want {
		if pads[pin] != w {
			t.Errorf("%s = net %q drill %v, want net %q drill %v", pin, pads[pin][0], pads[pin][1], w[0], w[1])
		}
	}
}

// TestUnusedNetIsNoNet reads Allegro's invented net for an unconnected pin as no net in the design,
// so the pin reads unconnected, as the board's own read has it.
func TestUnusedNetIsNoNet(t *testing.T) {
	d, err := Read(strings.NewReader(allegroShapes), "allegro.xml")
	if err != nil {
		t.Fatal(err)
	}
	var nets []string
	for _, n := range d.GetNets() {
		nets = append(nets, n.GetName())
	}
	if got := strings.Join(nets, " "); got != "N1" {
		t.Errorf("nets = %s, want N1 alone", got)
	}
}
