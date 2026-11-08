package render

import (
	"sort"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// BoardNet is one board net as a NetIDFunc sees it. Name is the net's name on the board. Pins lists
// every pad on the net as (ref-des, pad number), sorted, so a resolver can join the net by its
// connections when the name does not join it.
type BoardNet struct {
	Name string
	Pins [][2]string
}

// NetIDFunc answers the schematic net id (ir.Net.id) of one board net, or "" when it cannot say
// which net that is. A board file names its nets and carries no schematic id, so a board render
// asks the caller, which holds the design (agni issue 1440). A net answered "" keeps its name as
// its only key, as before.
type NetIDFunc func(BoardNet) string

// boardNetIDs resolves every net the board draws once, through the style's NetIDFunc, and returns
// the lookup a render keys with. It answers "" for every net when the render was given no
// NetIDFunc.
func boardNetIDs(b *geom.BoardGeometry, style Style) func(net string) string {
	if style.netIDs == nil || *style.netIDs == nil {
		return func(string) string { return "" }
	}
	pins := map[string][][2]string{}
	note := func(net string) {
		if _, ok := pins[net]; !ok && net != "" {
			pins[net] = nil
		}
	}
	for _, pl := range b.GetPlacements() {
		for _, pad := range pl.GetPads() {
			if n := pad.GetNet(); n != "" {
				pins[n] = append(pins[n], [2]string{pl.GetRefDes(), pad.GetNumber()})
			}
		}
	}
	for _, nc := range b.GetNets() {
		note(nc.GetNet())
	}
	for _, z := range b.GetZones() {
		note(z.GetNet())
	}
	f := *style.netIDs
	ids := make(map[string]string, len(pins))
	for name, ps := range pins {
		sort.Slice(ps, func(i, j int) bool {
			if ps[i][0] != ps[j][0] {
				return ps[i][0] < ps[j][0]
			}
			return ps[i][1] < ps[j][1]
		})
		if id := f(BoardNet{Name: name, Pins: ps}); id != "" {
			ids[name] = id
		}
	}
	return func(net string) string { return ids[net] }
}
