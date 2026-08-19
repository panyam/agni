package graph

import (
	"fmt"
	"sort"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/geomath"
)

// Placement is what a layout strategy produces: a position for each component node, keyed by
// ref_des. It is intentionally only positions. Turning positions into drawable geometry (node
// boxes, net hyperedge stars, labels) is shared across all strategies (see assemble), so a
// new layout algorithm implements placement only, not rendering. The struct leaves room for a
// future strategy (e.g. orthogonal routing) to also carry explicit edge routes.
type Placement struct {
	Positions map[string]*geom.Point
	// Route selects how assemble draws each net: the default hyperedge star, or orthogonal
	// L-runs (WS7-010). It is a style, not literal route points, because assemble re-spaces
	// positions (compactBySize) after the strategy returns, so routes must be computed from
	// the final positions, which only assemble has.
	Route RouteStyle
}

// RouteStyle names how nets are drawn between placed nodes.
type RouteStyle string

// Route styles: the straight hyperedge star (default) and axis-aligned orthogonal runs.
const (
	RouteStar       RouteStyle = ""
	RouteOrthogonal RouteStyle = "orthogonal"
)

// Strategy is a named layout algorithm. Place maps a design's components to node positions;
// everything downstream of placement is shared. The available strategies are returned by
// Strategies; select one by name with ByName.
type Strategy struct {
	Name  string
	Doc   string
	Place func(*ir.Design) Placement
}

// DefaultStrategy is the layout used when the caller does not request one.
const DefaultStrategy = "grid"

// Strategies returns the available layout strategies in a stable (name-sorted) order. The set
// is fixed at compile time (every layout lives in this package), so this builds the list
// directly rather than through a mutable registry: no package-global state, no init ordering,
// and each call is independent, so tests cannot leak strategies into one another.
func Strategies() []Strategy {
	out := []Strategy{
		{Name: "grid", Doc: "components on a ref_des-sorted square grid; edge-agnostic placeholder", Place: gridPlace},
		{Name: "layered", Doc: "components ranked into rows by connectivity (Sugiyama-style), crossings reduced per row", Place: layeredPlace},
		{Name: "stress", Doc: "stress majorization: drawn distance tracks graph distance (SMACOF, layered init, deterministic)", Place: stressPlace},
		{Name: "force", Doc: "force-directed (Fruchterman-Reingold): nodes repel, net stars attract; deterministic cooling", Place: forcePlace},
		{Name: "orthogonal", Doc: "stress positions with axis-aligned L-run routing to Manhattan-median net hubs", Place: orthoPlace},
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ByName returns the named strategy. The error lists the available names so a CLI typo is
// self-correcting.
func ByName(name string) (Strategy, error) {
	all := Strategies()
	for _, s := range all {
		if s.Name == name {
			return s, nil
		}
	}
	names := make([]string, len(all))
	for i, s := range all {
		names[i] = s.Name
	}
	return Strategy{}, fmt.Errorf("unknown layout %q (have: %s)", name, strings.Join(names, ", "))
}

// Option configures a layout call. With no options a layout draws nodes with DefaultRegistry
// (classified synthetic glyphs).
type Option func(*layoutConfig)

type layoutConfig struct{ source SymbolSource }

// WithRegistry draws nodes with the given classification/glyph registry, so a caller (the
// CLI/serve edge) can layer user classification rules on top of the defaults.
func WithRegistry(r *Registry) Option { return WithSymbolSource(r) }

// WithSymbolSource draws nodes with the given symbol source: the Registry (synthetic glyphs) or a
// FaithfulSource (the design's own artwork, re-laid-out).
func WithSymbolSource(s SymbolSource) Option { return func(c *layoutConfig) { c.source = s } }

func resolveConfig(opts []Option) layoutConfig {
	var cfg layoutConfig
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}

// LayoutWith lays a design out with the named strategy and assembles the drawable geometry.
func LayoutWith(d *ir.Design, name string, opts ...Option) (*geom.SchematicGeometry, error) {
	s, err := ByName(name)
	if err != nil {
		return nil, err
	}
	g := assemble(d, s.Place(d), resolveConfig(opts).source)
	// An auto-layout can come up short too: SymbolsFaithful draws the design's own symbols, and a
	// library that did not resolve leaves the same bodyless placement the faithful path does
	// (agni issue 354).
	geomath.MarkUndrawn(g)
	return g, nil
}

// assemble turns node positions into a full netlist-graph geometry: one box symbol per
// component at its position, each net as a hyperedge star from its member nodes to the net
// centroid, plus ref_des and net labels. It is shared by every strategy, so layouts differ
// only in placement. Node order is ref_des-sorted, and nets keep source order, so output is
// deterministic given deterministic positions.
func assemble(d *ir.Design, pl Placement, source SymbolSource) *geom.SchematicGeometry {
	if source == nil {
		source = DefaultRegistry()
	}
	refs := make([]string, 0, len(pl.Positions))
	for ref := range pl.Positions {
		refs = append(refs, ref)
	}
	sort.Strings(refs)

	// Ask the symbol source for each node's symbol: a classified synthetic glyph (Registry) or
	// the design's own artwork (FaithfulSource). The source is injected, so the layout is fixed
	// while what is drawn at each node varies.
	parts := partIndex(d)
	byRef := make(map[string]*ir.Component, len(d.Components))
	for _, c := range d.Components {
		byRef[c.RefDes] = c
	}
	symByRef := make(map[string]*geom.SymbolDef, len(refs))
	usedByCell := make(map[string]*geom.SymbolDef)
	sizes := make(map[string]nodeSize, len(refs))
	for _, ref := range refs {
		sym := source.Symbol(ref, byRef[ref], parts)
		symByRef[ref] = sym
		usedByCell[sym.CellRef] = sym
		sizes[ref] = symbolSize(sym)
	}

	// Space each node by its own symbol's size, not the largest one: a small part gets a small
	// cell, only a large (faithful) symbol expands its column/row. An all-glyph/box design fits one
	// node per pitch, so its layout is unchanged. Everything downstream (placements, net stars)
	// uses these positions.
	positions := compactBySize(pl.Positions, sizes, refs)

	placements := make([]*geom.SymbolPlacement, 0, len(refs))
	for _, ref := range refs {
		placements = append(placements, &geom.SymbolPlacement{
			RefDes:    ref,
			CellRef:   symByRef[ref].CellRef,
			Transform: &geom.Transform{Origin: positions[ref]},
		})
	}

	// One net -> one hyperedge star from each connected pin to the net centroid. Skip nets that
	// touch fewer than two distinct pins (nothing to draw between). A net where three or more pins
	// meet gets a junction dot at its hub (the centroid for the star style, the Manhattan
	// median for orthogonal routes).
	wires := make([]*geom.WireGeometry, 0, len(d.Nets))
	labels := make([]*geom.Label, 0, len(d.Nets))
	var junctions []*geom.Shape
	dotted := map[[2]int64]bool{} // one connection dot per distinct attach point
	for i, net := range d.Nets {
		pins := netPinPoints(net, positions, symByRef)
		if len(pins) < 2 {
			continue
		}
		var polys []*geom.Polyline
		var hub *geom.Point
		if pl.Route == RouteOrthogonal {
			polys, hub = routeOrthogonal(pins, i)
		} else {
			cx, cy := int64(0), int64(0)
			for _, p := range pins {
				cx += p.X
				cy += p.Y
			}
			hub = &geom.Point{X: cx / int64(len(pins)), Y: cy / int64(len(pins))}
			polys = make([]*geom.Polyline, 0, len(pins))
			for _, p := range pins {
				polys = append(polys, &geom.Polyline{Points: []*geom.Point{p, hub}})
			}
		}
		wires = append(wires, &geom.WireGeometry{Net: net.Name, NetId: net.GetId(), Polylines: polys})
		labels = append(labels, &geom.Label{Text: net.Name, Origin: hub, Height: 12})
		if len(pins) >= 3 {
			junctions = append(junctions, &geom.Shape{Kind: geom.Shape_KIND_DOT, Points: []*geom.Point{hub}})
		}
		// A connection dot at each attach point, so a reader sees where a wire actually
		// starts and ends (crucial for center-attach bodies, whose wires end mid-symbol).
		for _, p := range pins {
			k := [2]int64{p.X, p.Y}
			if !dotted[k] {
				dotted[k] = true
				junctions = append(junctions, &geom.Shape{Kind: geom.Shape_KIND_DOT, Points: []*geom.Point{p}})
			}
		}
	}

	sheet := &geom.SheetGeometry{
		Id:         "graph",
		Name:       "netlist graph",
		Placements: placements,
		Wires:      wires,
		Labels:     labels,
		Shapes:     junctions,
	}
	return &geom.SchematicGeometry{
		DesignRef: d.Name,
		UnitNm:    1,
		Symbols:   sortedByCell(usedByCell),
		Sheets:    []*geom.SheetGeometry{sheet},
	}
}
