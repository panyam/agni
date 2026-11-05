package main

import (
	"fmt"
	"sort"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/core/check"
)

// boardLines summarises a board's geometry: its copper layers in stackup order, its parts by side,
// and its routed copper.
func boardLines(g *geom.BoardGeometry) string {
	var names []string
	for _, l := range g.GetLayers() {
		names = append(names, l.GetName())
	}
	top, bottom := 0, 0
	for _, pl := range g.GetPlacements() {
		if pl.GetMirror() {
			bottom++
		} else {
			top++
		}
	}
	segs, vias := 0, 0
	for _, nc := range g.GetNets() {
		segs += len(nc.GetSegments())
		vias += len(nc.GetVias())
	}
	return fmt.Sprintf("copper layers: %s\nparts:         %d on top, %d on the bottom\ncopper:        %d track segments and %d vias on %d nets, %d zones",
		strings.Join(names, " "), top, bottom, segs, vias, len(g.GetNets()), len(g.GetZones()))
}

// ruleTally counts findings per rule, most first. A real board yields hundreds of findings, and a
// line each would bury the screen (agni issue 644).
func ruleTally(fs []check.Finding) string {
	counts := map[string]int{}
	for _, f := range fs {
		counts[f.Rule]++
	}
	rules := make([]string, 0, len(counts))
	for r := range counts {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool {
		if counts[rules[i]] != counts[rules[j]] {
			return counts[rules[i]] > counts[rules[j]]
		}
		return rules[i] < rules[j]
	})
	var b strings.Builder
	fmt.Fprintf(&b, "%d findings from %d rules\n", len(fs), len(rules))
	for _, r := range rules {
		fmt.Fprintf(&b, "  %5d  %s\n", counts[r], r)
	}
	return strings.TrimRight(b.String(), "\n")
}
