package render

import (
	"sort"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// boardStrata is how one board divides into the rows a reader can show and hide (agni 1351). It
// assigns every copper layer, every copper layer's zones and every graphics layer a packed group
// of its own, so PackBoard and BoardLayers agree on the numbers. F.Cu and B.Cu keep their fixed
// groups, since a front or back SMD pad packs there whatever else the board holds.
type boardStrata struct {
	copper map[string]uint8 // copper layer -> its tracks' group
	zone   map[string]uint8 // copper layer -> its zones' group
	art    map[string]uint8 // graphics layer -> its graphics' and text's group
	colors []string         // indexed by group
	rows   []*geom.BoardLayerRow
}

// maxBoardGroups is the group count a primitive record can name, since its group is a u8. A board
// with more layers than that packs the rest into the shared fallback groups below.
const maxBoardGroups = 256

func planBoardStrata(b *geom.BoardGeometry, style Style) *boardStrata {
	s := &boardStrata{copper: map[string]uint8{}, zone: map[string]uint8{}, art: map[string]uint8{}}
	s.colors = boardGroupColors(style)
	alloc := func(fallback uint8, color string) uint8 {
		if len(s.colors) >= maxBoardGroups {
			return fallback
		}
		s.colors = append(s.colors, color)
		return uint8(len(s.colors) - 1) // #nosec G115 -- the check above holds len(s.colors) to maxBoardGroups, 256
	}

	stack := copperStack(b)
	zoned := map[string]bool{}
	for _, z := range b.GetZones() {
		if z.GetOutline() != nil {
			zoned[z.GetLayer()] = true
		}
	}
	for _, ly := range stack {
		color := copperColor(style, ly)
		switch layerSide(ly) {
		case "front":
			s.copper[ly] = groupBoardCopperFront
		case "back":
			s.copper[ly] = groupBoardCopperBack
		default:
			s.copper[ly] = alloc(groupBoardCopperInner, color)
		}
		if zoned[ly] {
			s.zone[ly] = alloc(s.copper[ly], color)
		}
	}
	for _, ly := range artLayers(b) {
		fallback := groupBoardSilk
		if strings.HasSuffix(ly, ".Fab") {
			fallback = groupBoardFab
		}
		s.art[ly] = alloc(fallback, boardArtColor(style, ly))
	}

	for _, ly := range stack {
		groups := []uint32{uint32(s.copper[ly])}
		if g, ok := s.zone[ly]; ok {
			groups = append(groups, uint32(g))
		}
		sel := layerSelector(ly)
		// A part's body pick rect sits on its side, so a hidden outer layer takes its parts' clicks
		// with it.
		switch layerSide(ly) {
		case "front":
			sel += ", .picks-front"
		case "back":
			sel += ", .picks-back"
		}
		s.rows = append(s.rows, &geom.BoardLayerRow{Name: ly, Kind: "copper", Side: layerSide(ly),
			Color: copperColor(style, ly), SvgSelector: sel, Groups: groups})
	}
	if len(b.GetOutline().GetPaths()) > 0 {
		s.rows = append(s.rows, &geom.BoardLayerRow{Name: "Edge", Kind: "edge", Color: style.BoardOutline,
			SvgSelector: ".edge", Groups: []uint32{uint32(groupBoardEdge)}})
	}
	if hasThrough(b) {
		s.rows = append(s.rows, &geom.BoardLayerRow{Name: "Through", Kind: "through", Color: style.Via,
			SvgSelector: ".through", Groups: []uint32{uint32(groupBoardThrough), uint32(groupBoardHole)}})
	}
	if len(s.zone) > 0 {
		var groups []uint32
		color := ""
		for _, ly := range stack {
			if g, ok := s.zone[ly]; ok {
				groups = append(groups, uint32(g))
				if color == "" {
					// The swatch takes the colour of the first layer that has a zone.
					color = copperColor(style, ly)
				}
			}
		}
		s.rows = append(s.rows, &geom.BoardLayerRow{Name: "Zones", Kind: "zones", Color: color,
			SvgSelector: ".zones-front, .zones-back, .zones-inner", Groups: groups})
	}
	for _, ly := range artLayers(b) {
		s.rows = append(s.rows, &geom.BoardLayerRow{Name: ly, Kind: artKind(ly), Side: artSide(ly),
			Color: boardArtColor(style, ly), SvgSelector: layerSelector(ly), Groups: []uint32{uint32(s.art[ly])}})
	}
	return s
}

// copperGroup, zoneGroup and artGroup are the packed group of a layer's copper, zones and
// graphics. A layer the plan lacks, such as an empty name, falls back to the shared group its
// side or kind used before groups were per layer.
func (s *boardStrata) copperGroup(layer string) uint8 {
	if g, ok := s.copper[layer]; ok {
		return g
	}
	return groupBoardCopperInner
}

func (s *boardStrata) zoneGroup(layer string) uint8 {
	if g, ok := s.zone[layer]; ok {
		return g
	}
	return s.copperGroup(layer)
}

func (s *boardStrata) artGroup(layer string) uint8 {
	if g, ok := s.art[layer]; ok {
		return g
	}
	if strings.HasSuffix(layer, ".Fab") {
		return groupBoardFab
	}
	return groupBoardSilk
}

// BoardLayers lists a board's layer rows, in the order the viewer's Sheets panel shows them. Each
// row carries the selector and the packed groups that hide it in BoardSVG's and PackBoard's
// output under the same style.
func BoardLayers(b *geom.BoardGeometry, opts ...Option) []*geom.BoardLayerRow {
	return planBoardStrata(b, resolveStyle(opts)).rows
}

// copperStack lists the copper layers that carry tracks, pads or zones, front first. It orders
// them by the layer table whatever a layer's kind, so a power plane sits in the stack where the
// board puts it. A layer the table lacks goes last, by name.
func copperStack(b *geom.BoardGeometry) []string {
	used := map[string]bool{}
	for _, ly := range copperLayersBackToFront(b) {
		used[ly] = true
	}
	for _, z := range b.GetZones() {
		if z.GetOutline() != nil && z.GetLayer() != "" {
			used[z.GetLayer()] = true
		}
	}
	return byLayerTable(b, used)
}

// byLayerTable sorts a set of layer names by their number in the board's layer table, and a name
// the table lacks after the rest, by name.
func byLayerTable(b *geom.BoardGeometry, set map[string]bool) []string {
	num := map[string]int32{}
	for _, l := range b.GetLayers() {
		num[l.GetName()] = l.GetNumber()
	}
	out := make([]string, 0, len(set))
	for ly := range set {
		out = append(out, ly)
	}
	sort.Slice(out, func(i, j int) bool {
		ni, iOK := num[out[i]]
		nj, jOK := num[out[j]]
		if iOK != jOK {
			return iOK
		}
		if ni != nj {
			return ni < nj
		}
		return out[i] < out[j]
	})
	return out
}

// artLayers lists the graphics layers that draw something, graphics or text, in the layer table's
// order and then by name for a layer the table lacks.
func artLayers(b *geom.BoardGeometry) []string {
	used := map[string]bool{}
	for _, gr := range b.GetGraphics() {
		if gr.GetShape() != nil && gr.GetLayer() != "" {
			used[gr.GetLayer()] = true
		}
	}
	for _, t := range boardLabels(b) {
		if t.GetAt() != nil && t.GetLayer() != "" {
			used[t.GetLayer()] = true
		}
	}
	return byLayerTable(b, used)
}

func hasThrough(b *geom.BoardGeometry) bool {
	for _, nc := range b.GetNets() {
		if len(nc.GetVias()) > 0 {
			return true
		}
	}
	for _, pl := range b.GetPlacements() {
		for _, pad := range pl.GetPads() {
			if padSide(pad) == "through" {
				return true
			}
		}
	}
	return false
}

func artKind(layer string) string {
	switch {
	case strings.HasSuffix(layer, ".SilkS"):
		return "silk"
	case strings.HasSuffix(layer, ".Fab"):
		return "fab"
	case strings.HasSuffix(layer, ".CrtYd"):
		return "courtyard"
	}
	return "graphic"
}

func artSide(layer string) string {
	switch {
	case strings.HasPrefix(layer, "F."):
		return "front"
	case strings.HasPrefix(layer, "B."):
		return "back"
	}
	return ""
}

// layerSelector matches every BoardSVG element that records layer in its data-layer.
func layerSelector(layer string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\a `)
	return `[data-layer="` + r.Replace(layer) + `"]`
}
