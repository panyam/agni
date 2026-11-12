package render

import (
	"math"

	"github.com/panyam/agni/core/svg"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// boardPickKeys writes the entity attributes that a viewer's picker reads off a board element,
// in the same vocabulary as the schematic's (wireKeys, symbolKeys, pinKeys). They are written
// only under WithPickTargets, so `agni render` and a report keep an unkeyed board. See
// docsite/content/architecture/web-picking.md#picking-the-board.
type boardPickKeys struct{ on bool }

// net keys a track, a via or a zone by its net name. Copper with no net names nothing.
func (k boardPickKeys) net(name string) []svg.Attr {
	if !k.on || name == "" {
		return nil
	}
	return []svg.Attr{svg.AEsc("data-kind", "net"), svg.AEsc("data-net", name)}
}

// component keys a footprint's body rect, and a pad with no number. Free board artwork has no
// ref and names nothing.
func (k boardPickKeys) component(ref string) []svg.Attr {
	if !k.on || ref == "" {
		return nil
	}
	return []svg.Attr{svg.AEsc("data-kind", "component"), svg.AEsc("data-ref", ref)}
}

// passThrough lets a click through the silkscreen and the designators to the copper and body
// rects under them. On a dense board a part's designator or fab outline lies over its neighbour's
// pads, and a keyed label there picked the neighbour (agni issue 1347). The schematic leaves its
// labels unkeyed for the same reason. A part still picks from its body rect.
func (k boardPickKeys) passThrough() []svg.Attr {
	if !k.on {
		return nil
	}
	return []svg.Attr{svg.A("pointer-events", "none")}
}

// pad keys a pad as a pin of its footprint, the pad number being the pin designator that
// HighlightBoardSVG matches a pin on. The pad's net rides along for a reader that wants it. A pad
// with no number, such as a mechanical hole, keys as its footprint, since a pin needs both halves.
func (k boardPickKeys) pad(pl *geom.ComponentPlacement, pad *geom.Pad) []svg.Attr {
	if !k.on || pl.GetRefDes() == "" {
		return nil
	}
	if pad.GetNumber() == "" {
		return k.component(pl.GetRefDes())
	}
	keys := []svg.Attr{svg.AEsc("data-kind", "pin"), svg.AEsc("data-ref", pl.GetRefDes()), svg.AEsc("data-pin", pad.GetNumber())}
	if n := pad.GetNet(); n != "" {
		keys = append(keys, svg.AEsc("data-net", n))
	}
	return keys
}

// drawFootprintPicks draws one invisible rect per placement over its pads' bounds, keyed as the
// component, so a click on a footprint's body between its pads picks the part. The rects split
// into picks-front and picks-back by the side the part sits on, so the layer CSS hides a hidden
// side's parts from the picker as well as from the eye. A placement with no pads gets no rect,
// and its silkscreen still picks.
func drawFootprintPicks(c *svg.Canvas, b *geom.BoardGeometry, fr sheetFrame) {
	pk := boardPickKeys{on: true}
	for _, side := range []string{"back", "front"} {
		opened := false
		for _, pl := range b.GetPlacements() {
			if placementSide(pl) != side || pl.GetRefDes() == "" || len(pl.GetPads()) == 0 {
				continue
			}
			minX, minY := int64(math.MaxInt64), int64(math.MaxInt64)
			maxX, maxY := int64(math.MinInt64), int64(math.MinInt64)
			for _, pad := range pl.GetPads() {
				wx, wy := padWorld(pl, pad)
				// The larger extent on both axes covers a pad at any rotation.
				half := max(pad.GetSize().GetX(), pad.GetSize().GetY()) / 2
				minX, maxX = min(minX, wx-half), max(maxX, wx+half)
				minY, maxY = min(minY, wy-half), max(maxY, wy+half)
			}
			if !opened {
				c.Group(svg.A("class", "picks-"+side))
				opened = true
			}
			x0, y0 := fr.tx(minX), fr.ty(maxY)
			c.El("rect", append([]svg.Attr{svg.F("x", x0), svg.F("y", y0),
				svg.F("width", fr.tx(maxX)-x0), svg.F("height", fr.ty(minY)-y0),
				svg.A("fill", "none"), svg.A("pointer-events", "all")}, pk.component(pl.GetRefDes())...)...)
		}
		if opened {
			c.GroupEnd()
		}
	}
}

// placementSide is the side of the board a part sits on, "back" for a mirrored placement or one
// on a back copper layer and "front" otherwise.
func placementSide(pl *geom.ComponentPlacement) string {
	if pl.GetMirror() || layerSide(pl.GetLayer()) == "back" {
		return "back"
	}
	return "front"
}
