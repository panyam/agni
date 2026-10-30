package odb

import (
	"fmt"
	"io/fs"
	"math"
	"path"
	"slices"
	"strings"

	"github.com/panyam/agni/core/num"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// ReadBoardGeometry parses the board-geometry sidecar out of the ODB++ job rooted at fsys.
//
// Fidelity: lossy-bounded (C6). This producer carries the board outline (the step's profile), the
// layer stack, component placements with their pads (each pad's shape and size from the copper
// feature eda/data ties to its toeprint, its drill from the drill layer), routed copper (lines, and
// arcs approximated as polylines) grouped by the net eda/data assigns each feature, vias (a drill
// hit and its copper lands), copper surfaces on a net as zones (each island's outline, holes
// dropped), and silkscreen and fabrication lines and text. A pad drawn by a user symbol or as a
// surface is carried as its extent with the shape "custom", not its outline. Negative features and
// barcodes are not carried. Coordinates are integer nanometres (unit_nm=1).
//
// Layers are renamed into the contract's KiCad-style vocabulary (the first copper layer F.Cu, the
// last B.Cu, inner ones In1.Cu up, and the silkscreen, mask and paste on each side F./B.SilkS,
// .Mask, .Paste), so the format-neutral renderer and the board rules read an ODB++ board as they
// read every other one.
func ReadBoardGeometry(fsys fs.FS, sourceFile string) (*geom.BoardGeometry, error) {
	p, err := parseJob(fsys)
	if err != nil {
		return nil, err
	}
	return p.toBoard(sourceFile)
}

// netRef is the net eda/data puts a feature on, and the kind of subnet it belongs to.
type netRef struct {
	net string
	typ string
}

type boardBuilder struct {
	p        *parsedJob
	src      string
	names    map[string]string // upper-cased matrix name -> contract layer name
	byUpper  map[string]matrixLayer
	features map[string]*featuresFile // upper-cased matrix name -> parsed features
	featNet  map[string]map[int]netRef
	drills   map[[2]int64]int64          // drill hits by micron position, built by drillAt
	pads     map[string]map[[2]int64]int // copper pads by layer and micron position, built by padAt
	users    map[string]symbolShape
	g        *geom.BoardGeometry
}

func (p *parsedJob) toBoard(src string) (*geom.BoardGeometry, error) {
	b := &boardBuilder{
		p: p, src: src,
		names:    layerNames(p.matrix),
		byUpper:  map[string]matrixLayer{},
		features: map[string]*featuresFile{},
		featNet:  map[string]map[int]netRef{},
		g:        &geom.BoardGeometry{DesignRef: p.jobName(src), UnitNm: 1, Prov: &geom.Provenance{SourceFile: src}},
	}
	for _, l := range p.matrix.layers {
		b.byUpper[strings.ToUpper(l.name)] = l
	}
	b.indexNets()
	for _, l := range p.matrix.layers {
		if l.context != "BOARD" || l.typ == "COMPONENT" {
			continue
		}
		kind := strings.ToLower(l.typ)
		if l.isCopper() {
			kind = "signal"
		}
		b.g.Layers = append(b.g.Layers, &geom.BoardLayer{Number: num.Int32(l.row), Name: b.names[strings.ToUpper(l.name)], Kind: kind})
	}
	if err := b.outline(); err != nil {
		return nil, err
	}
	if err := b.placements(); err != nil {
		return nil, err
	}
	if err := b.copper(); err != nil {
		return nil, err
	}
	if err := b.artwork(); err != nil {
		return nil, err
	}
	return b.g, nil
}

// indexNets records, for every feature eda/data names, the net it is on.
func (b *boardBuilder) indexNets() {
	for _, n := range b.p.eda.nets {
		name := n.name
		if name == noNet {
			name = ""
		}
		for _, s := range n.subnets {
			for _, f := range s.fids {
				layer := b.edaLayer(f.layer)
				if layer == "" {
					continue
				}
				m := b.featNet[layer]
				if m == nil {
					m = map[int]netRef{}
					b.featNet[layer] = m
				}
				m[f.feat] = netRef{net: name, typ: s.typ}
			}
		}
	}
}

// edaLayer is the upper-cased matrix name of an eda/data LYR entry.
func (b *boardBuilder) edaLayer(i int) string {
	if i < 0 || i >= len(b.p.eda.layers) {
		return ""
	}
	return strings.ToUpper(b.p.eda.layers[i])
}

// layerFeatures parses one layer's features file the first time it is asked for. A layer with no
// file has no features.
func (b *boardBuilder) layerFeatures(upper string) (*featuresFile, error) {
	if ff, ok := b.features[upper]; ok {
		return ff, nil
	}
	raw, err := b.p.readOptional(b.p.layerFile(upper, "features"))
	if err != nil {
		return nil, err
	}
	ff := &featuresFile{}
	if raw != nil {
		if ff, err = parseFeatures(raw); err != nil {
			return nil, fmt.Errorf("%s: %w", strings.ToLower(upper), err)
		}
	}
	b.features[upper] = ff
	return ff, nil
}

// symbol is what a feature's symbol says about its copper. A name the standard set does not know is
// a user symbol, drawn by the job under symbols/<name>/, and takes the extent of what it draws.
func (b *boardBuilder) symbol(ff *featuresFile, i int) symbolShape {
	if i < 0 || i >= len(ff.symbols) {
		return symbolShape{}
	}
	s := parseSymbol(ff.symbols[i])
	if s.w == 0 && s.h == 0 && s.shape == ff.symbols[i].name {
		return b.userSymbol(ff.symbols[i].name)
	}
	return s
}

// userSymbol reads a job's own symbol and sizes it by the box around everything it draws, a line's
// or arc's stroke and a pad's symbol included. A symbol the job does not define keeps its name and
// no size.
func (b *boardBuilder) userSymbol(name string) symbolShape {
	if s, ok := b.users[name]; ok {
		return s
	}
	if b.users == nil {
		b.users = map[string]symbolShape{}
	}
	out := symbolShape{shape: name}
	if raw, err := b.p.readOptional(path.Join("symbols", strings.ToLower(name), "features")); err == nil && raw != nil {
		if ff, err := parseFeatures(raw); err == nil {
			var pts []pt
			for i := range ff.features {
				f := &ff.features[i]
				half := int64(0)
				if f.sym >= 0 && f.sym < len(ff.symbols) {
					if s := parseSymbol(ff.symbols[f.sym]); s.w > 0 || s.h > 0 {
						half = max(s.w, s.h) / 2
					}
				}
				switch f.kind {
				case featLine, featArc:
					for _, p := range []pt{f.a, f.b} {
						pts = append(pts, pt{p.x - half, p.y - half}, pt{p.x + half, p.y + half})
					}
				case featPad:
					pts = append(pts, pt{f.a.x - half, f.a.y - half}, pt{f.a.x + half, f.a.y + half})
				case featSurface:
					for _, isl := range f.islands {
						for _, c := range isl {
							pts = append(pts, c.pts...)
						}
					}
				}
			}
			if w, h := extent(pts); w > 0 || h > 0 {
				out = symbolShape{w: w, h: h, shape: "custom"}
			}
		}
	}
	b.users[name] = out
	return out
}

func (b *boardBuilder) outline() error {
	raw, err := b.p.readOptional(b.p.stepFile("profile"))
	if err != nil || raw == nil {
		return err
	}
	ff, err := parseFeatures(raw)
	if err != nil {
		return fmt.Errorf("profile: %w", err)
	}
	out := &geom.BoardOutline{}
	for _, f := range ff.features {
		for _, isl := range f.islands {
			for _, c := range isl {
				out.Paths = append(out.Paths, closedPolyline(c.pts))
			}
		}
	}
	if len(out.Paths) > 0 {
		b.g.Outline = out
	}
	return nil
}

// placements builds each component's placement, its pads read off the copper eda/data ties to each
// toeprint.
func (b *boardBuilder) placements() error {
	pads := map[string]map[[2]int][]fid{} // side letter -> (component, toeprint) -> features
	for _, n := range b.p.eda.nets {
		for _, s := range n.subnets {
			if s.typ != "TOP" {
				continue
			}
			if pads[s.side] == nil {
				pads[s.side] = map[[2]int][]fid{}
			}
			key := [2]int{s.comp, s.toep}
			pads[s.side][key] = append(pads[s.side][key], s.fids...)
		}
	}
	for si, comps := range b.p.sides {
		letter := "T"
		sideLayer := "F.Cu"
		if !b.p.sideTop[si] {
			letter, sideLayer = "B", "B.Cu"
		}
		for ci := range comps {
			c := &comps[ci]
			if skipRefDes(c.ref) {
				continue
			}
			rot := normDeg(-c.rot)
			pl := &geom.ComponentPlacement{
				RefDes:      c.ref,
				At:          &geom.Point{X: c.at.x, Y: c.at.y},
				RotationDeg: rot,
				Layer:       sideLayer,
				Mirror:      c.back,
				Prov:        &geom.Provenance{SourceFile: b.src, SourceId: c.ref},
			}
			var pkg *edaPkg
			if c.pkg >= 0 && c.pkg < len(b.p.eda.pkgs) {
				pkg = &b.p.eda.pkgs[c.pkg]
			}
			for ti, t := range c.pins {
				pad := &geom.Pad{Number: t.name, At: localOffset(c.at, t.at, rot, c.back), Net: b.p.eda.netName(t.net)}
				through := pkg != nil && t.num >= 0 && t.num < len(pkg.pins) && pkg.pins[t.num].typ == "T"
				others, err := b.padShape(pad, pads[letter][[2]int{ci, ti}], sideLayer, t.at, &through)
				if err != nil {
					return err
				}
				switch {
				case through:
					pad.Layers = []string{"*.Cu"}
				case pad.Shape != "":
					pad.Layers = []string{sideLayer}
				default:
					// No copper and no hole: a toeprint whose only features are a paste or mask
					// opening, which is where it is drawn.
					pad.Layers = others
				}
				pl.Pads = append(pl.Pads, pad)
			}
			b.g.Placements = append(b.g.Placements, pl)
		}
	}
	return nil
}

// padShape fills a pad's shape, size, rotation and drill from the features eda/data ties to its
// toeprint, preferring the land on the component's own side. A land drawn as a surface (KiCad's
// custom pads) takes its outline's extent. A drill hit makes the pad through-hole whatever the
// package said, and one eda/data does not tie to the pin (a non-plated hole, on a drill layer of its
// own) is found by the pin's position, for a through-hole or copperless pad.
func (b *boardBuilder) padShape(pad *geom.Pad, fids []fid, sideLayer string, at pt, through *bool) (others []string, err error) {
	var best, surface *feature
	var bestFF *featuresFile
	for _, f := range fids {
		upper := b.edaLayer(f.layer)
		ml, ok := b.byUpper[upper]
		if !ok {
			continue
		}
		ff, err := b.layerFeatures(upper)
		if err != nil {
			return nil, err
		}
		if f.feat < 0 || f.feat >= len(ff.features) {
			continue
		}
		feat := &ff.features[f.feat]
		if !ml.isCopper() && ml.typ != "DRILL" && f.typ != 'H' && !slices.Contains(others, b.names[upper]) {
			others = append(others, b.names[upper])
		}
		switch {
		case ml.typ == "DRILL" || f.typ == 'H':
			pad.Drill = b.symbol(ff, feat.sym).hole()
			*through = true
		case ml.isCopper() && feat.kind == featPad:
			if best == nil || b.names[upper] == sideLayer {
				best, bestFF = feat, ff
			}
		case ml.isCopper() && feat.kind == featSurface:
			if surface == nil || b.names[upper] == sideLayer {
				surface = feat
			}
		}
	}
	bare := best == nil && surface == nil
	if bare && len(fids) == 0 {
		// A toeprint eda/data names nothing for, which Altium writes when several pins share a name
		// (every SH pin of a shield pointing at the first): the land at the pin's position. One that
		// names only paste or mask features is a stencil or mask opening and takes no copper, or a
		// paste window over a thermal pad would take the pad's.
		f, ff, err := b.padAt(sideLayer, at)
		if err != nil {
			return nil, err
		}
		best, bestFF = f, ff
		if best == nil {
			// A custom land eda/data ties to no pin (KiCad's mounting pads on no net, whose
			// subnets name only their paste openings): the smallest copper surface on no net
			// around the pin.
			if surface, err = b.surfaceAround(sideLayer, at); err != nil {
				return nil, err
			}
		}
	}
	// Only a pad the package calls through-hole, or one with no copper at all (a non-plated hole),
	// takes a drill by position; an SMD land under a via-in-pad would otherwise take the via's.
	if pad.Drill == 0 && (*through || (bare && best == nil)) {
		d, err := b.drillAt(at)
		if err != nil {
			return nil, err
		}
		if d > 0 {
			pad.Drill, *through = d, true
		}
	}
	switch {
	case best != nil:
		s := b.symbol(bestFF, best.sym)
		pad.Shape = s.shape
		pad.Size = &geom.Point{X: s.w, Y: s.h}
		pad.RotationDeg = normDeg(-best.rot)
	case surface != nil:
		w, h := islandsExtent(surface.islands)
		pad.Shape = "custom"
		pad.Size = &geom.Point{X: w, Y: h}
	case pad.Drill > 0:
		// A hole with no copper, which is what a non-plated hole is.
		pad.Shape = "circle"
		pad.Size = &geom.Point{X: pad.Drill, Y: pad.Drill}
	}
	return others, nil
}

// drillAt is the size of the drill hit on any drill layer at p, to the micron, 0 when there is none.
// The index is built the first time it is asked for.
func (b *boardBuilder) drillAt(p pt) (int64, error) {
	if b.drills == nil {
		b.drills = map[[2]int64]int64{}
		for _, l := range b.p.matrix.layers {
			if l.typ != "DRILL" {
				continue
			}
			ff, err := b.layerFeatures(strings.ToUpper(l.name))
			if err != nil {
				return 0, err
			}
			for i := range ff.features {
				f := &ff.features[i]
				if f.kind == featPad {
					b.drills[micron(f.a)] = b.symbol(ff, f.sym).hole()
				}
			}
		}
	}
	k := micron(p)
	for dx := int64(-1); dx <= 1; dx++ {
		for dy := int64(-1); dy <= 1; dy++ {
			if d, ok := b.drills[[2]int64{k[0] + dx, k[1] + dy}]; ok {
				return d, nil
			}
		}
	}
	return 0, nil
}

// padAt is the copper pad feature at p on the copper layer the contract names layer, to the micron,
// nil when there is none. Each layer's index is built the first time it is asked for.
func (b *boardBuilder) padAt(layer string, p pt) (*feature, *featuresFile, error) {
	var upper string
	for u, n := range b.names {
		if n == layer && b.byUpper[u].isCopper() {
			upper = u
		}
	}
	if upper == "" {
		return nil, nil, nil
	}
	ff, err := b.layerFeatures(upper)
	if err != nil {
		return nil, nil, err
	}
	if b.pads == nil {
		b.pads = map[string]map[[2]int64]int{}
	}
	idx := b.pads[upper]
	if idx == nil {
		idx = map[[2]int64]int{}
		for i := range ff.features {
			if f := &ff.features[i]; f.kind == featPad && f.positive {
				idx[micron(f.a)] = i
			}
		}
		b.pads[upper] = idx
	}
	k := micron(p)
	for dx := int64(-1); dx <= 1; dx++ {
		for dy := int64(-1); dy <= 1; dy++ {
			if i, ok := idx[[2]int64{k[0] + dx, k[1] + dy}]; ok {
				return &ff.features[i], ff, nil
			}
		}
	}
	return nil, nil, nil
}

// surfaceAround is the smallest positive surface on the named copper layer whose outline's box holds
// p and that eda/data puts on no net, nil when there is none. A surface on a net is a pour or a
// route, never a part's land, which is what keeps a ground pour under a pad from being taken for it.
func (b *boardBuilder) surfaceAround(layer string, p pt) (*feature, error) {
	var upper string
	for u, n := range b.names {
		if n == layer && b.byUpper[u].isCopper() {
			upper = u
		}
	}
	if upper == "" {
		return nil, nil
	}
	ff, err := b.layerFeatures(upper)
	if err != nil {
		return nil, err
	}
	var best *feature
	var bestArea float64
	for i := range ff.features {
		f := &ff.features[i]
		if f.kind != featSurface || !f.positive {
			continue
		}
		if ref, ok := b.featNet[upper][i]; ok && ref.net != "" {
			continue
		}
		var pts []pt
		for _, isl := range f.islands {
			for _, c := range isl {
				pts = append(pts, c.pts...)
			}
		}
		if len(pts) == 0 {
			continue
		}
		minX, maxX, minY, maxY := pts[0].x, pts[0].x, pts[0].y, pts[0].y
		for _, q := range pts[1:] {
			minX, maxX = min(minX, q.x), max(maxX, q.x)
			minY, maxY = min(minY, q.y), max(maxY, q.y)
		}
		if p.x < minX || p.x > maxX || p.y < minY || p.y > maxY {
			continue
		}
		if area := float64(maxX-minX) * float64(maxY-minY); best == nil || area < bestArea {
			best, bestArea = f, area
		}
	}
	return best, nil
}

func micron(p pt) [2]int64 {
	return [2]int64{int64(math.Round(float64(p.x) / 1000)), int64(math.Round(float64(p.y) / 1000))}
}

// islandsExtent is the width and height of the box around a surface's outlines.
func islandsExtent(islands [][]contour) (int64, int64) {
	var pts []pt
	for _, isl := range islands {
		for _, c := range isl {
			pts = append(pts, c.pts...)
		}
	}
	return extent(pts)
}

func extent(pts []pt) (int64, int64) {
	if len(pts) == 0 {
		return 0, 0
	}
	minX, maxX, minY, maxY := pts[0].x, pts[0].x, pts[0].y, pts[0].y
	for _, p := range pts[1:] {
		minX, maxX = min(minX, p.x), max(maxX, p.x)
		minY, maxY = min(minY, p.y), max(maxY, p.y)
	}
	return maxX - minX, maxY - minY
}

// copper groups each copper layer's lines and arcs by net, and builds the vias and zones.
func (b *boardBuilder) copper() error {
	byNet := map[string]*geom.NetCopper{}
	get := func(name string) *geom.NetCopper {
		nc := byNet[name]
		if nc == nil {
			nc = &geom.NetCopper{Net: name}
			byNet[name] = nc
		}
		return nc
	}
	for _, l := range b.p.matrix.layers {
		if !l.isCopper() {
			continue
		}
		upper := strings.ToUpper(l.name)
		ff, err := b.layerFeatures(upper)
		if err != nil {
			return err
		}
		layer := b.names[upper]
		nets := b.featNet[upper]
		for i := range ff.features {
			f := &ff.features[i]
			ref, onNet := nets[i]
			if !f.positive || !onNet || ref.typ == "TOP" || ref.typ == "VIA" {
				continue
			}
			switch f.kind {
			case featLine:
				nc := get(ref.net)
				nc.Segments = append(nc.Segments, &geom.TrackSegment{A: gp(f.a), B: gp(f.b), Width: b.symbol(ff, f.sym).width(), Layer: layer})
			case featArc:
				nc := get(ref.net)
				w := b.symbol(ff, f.sym).width()
				pts := flattenArc(f.a, f.b, f.center, f.cw)
				for k := 1; k < len(pts); k++ {
					nc.Segments = append(nc.Segments, &geom.TrackSegment{A: gp(pts[k-1]), B: gp(pts[k]), Width: w, Layer: layer})
				}
			case featSurface:
				for _, isl := range f.islands {
					if len(isl) == 0 {
						continue
					}
					b.g.Zones = append(b.g.Zones, &geom.Zone{Net: ref.net, Layer: layer, Outline: closedPolyline(isl[0].pts)})
				}
			}
		}
	}
	if err := b.vias(get); err != nil {
		return err
	}
	for _, n := range b.p.eda.nets {
		name := n.name
		if name == noNet {
			name = ""
		}
		if nc := byNet[name]; nc != nil {
			b.g.Nets = append(b.g.Nets, nc)
			delete(byNet, name)
		}
	}
	return nil
}

// vias builds one Via per VIA subnet: at its drill hit, drilled to the hit's size, its land the
// widest copper pad under it, spanning the drill layer's start and end.
func (b *boardBuilder) vias(get func(string) *geom.NetCopper) error {
	for _, n := range b.p.eda.nets {
		name := n.name
		if name == noNet {
			name = ""
		}
		for _, s := range n.subnets {
			if s.typ != "VIA" {
				continue
			}
			v := &geom.Via{}
			have := false
			for _, f := range s.fids {
				upper := b.edaLayer(f.layer)
				ml, ok := b.byUpper[upper]
				if !ok {
					continue
				}
				ff, err := b.layerFeatures(upper)
				if err != nil {
					return err
				}
				if f.feat < 0 || f.feat >= len(ff.features) {
					continue
				}
				feat := &ff.features[f.feat]
				w := b.symbol(ff, feat.sym).width()
				switch {
				case ml.typ == "DRILL" || f.typ == 'H':
					v.At, v.Drill, have = gp(feat.a), w, true
					v.LayerFrom = b.names[strings.ToUpper(ml.startName)]
					v.LayerTo = b.names[strings.ToUpper(ml.endName)]
				case ml.isCopper() && feat.kind == featPad:
					if v.At == nil {
						v.At = gp(feat.a)
					}
					if w > v.Size {
						v.Size = w
					}
				}
			}
			if !have && v.At == nil {
				continue
			}
			if v.LayerFrom == "" {
				v.LayerFrom, v.LayerTo = "F.Cu", "B.Cu"
			}
			nc := get(name)
			nc.Vias = append(nc.Vias, v)
		}
	}
	return nil
}

// artwork carries the silkscreen and fabrication layers' lines, arcs, outlines and text.
func (b *boardBuilder) artwork() error {
	for _, l := range b.p.matrix.layers {
		upper := strings.ToUpper(l.name)
		layer := b.names[upper]
		if l.context != "BOARD" || !drawnArtwork(l, layer) {
			continue
		}
		ff, err := b.layerFeatures(upper)
		if err != nil {
			return err
		}
		mirror := strings.HasPrefix(layer, "B.")
		for i := range ff.features {
			f := &ff.features[i]
			if !f.positive {
				continue
			}
			switch f.kind {
			case featLine:
				b.g.Graphics = append(b.g.Graphics, &geom.BoardGraphic{Shape: polyShape([]pt{f.a, f.b}), Layer: layer, Width: b.symbol(ff, f.sym).width()})
			case featArc:
				b.g.Graphics = append(b.g.Graphics, &geom.BoardGraphic{Shape: polyShape(flattenArc(f.a, f.b, f.center, f.cw)), Layer: layer, Width: b.symbol(ff, f.sym).width()})
			case featSurface:
				for _, isl := range f.islands {
					for _, c := range isl {
						b.g.Graphics = append(b.g.Graphics, &geom.BoardGraphic{Shape: &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: closedPolyline(c.pts).Points}, Layer: layer})
					}
				}
			case featText:
				b.g.Texts = append(b.g.Texts, &geom.BoardText{Text: f.text, At: gp(f.a), RotationDeg: normDeg(-f.rot), Height: f.textH, Layer: layer, Mirror: mirror || f.mirror, Kind: "gr"})
			}
		}
	}
	return nil
}

// drawnArtwork reports whether a non-copper layer is one the board view draws: the silkscreen,
// and a document layer that is a fabrication or courtyard drawing, as the KiCad board reader keeps
// (agni issue 946).
func drawnArtwork(l matrixLayer, name string) bool {
	if l.typ == "SILK_SCREEN" {
		return true
	}
	if l.typ != "DOCUMENT" {
		return false
	}
	switch name {
	case "F.Fab", "B.Fab", "F.CrtYd", "B.CrtYd":
		return true
	}
	return false
}

// layerNames maps each matrix layer onto the contract's layer vocabulary. The first and last
// copper layers are F.Cu and B.Cu and the ones between In1.Cu upward. A silkscreen, mask or paste
// layer takes its side from where it sits in the stack, above the copper or below it, unless its
// name says. A KiCad-produced document layer keeps the KiCad name it was exported from; anything
// else keeps the matrix's name.
func layerNames(m *matrix) map[string]string {
	out := map[string]string{}
	var copper []matrixLayer
	for _, l := range m.layers {
		if l.isCopper() {
			copper = append(copper, l)
		}
	}
	first, last := 0, 0
	if len(copper) > 0 {
		first, last = copper[0].row, copper[len(copper)-1].row
	}
	for i, l := range copper {
		name := fmt.Sprintf("In%d.Cu", i)
		switch {
		case i == 0:
			name = "F.Cu"
		case i == len(copper)-1:
			name = "B.Cu"
		}
		out[strings.ToUpper(l.name)] = name
	}
	kicadDoc := map[string]string{
		"F.FAB": "F.Fab", "B.FAB": "B.Fab", "F.COURTYARD": "F.CrtYd", "B.COURTYARD": "B.CrtYd",
		"EDGE.CUTS": "Edge.Cuts", "F.ADHESIVE": "F.Adhes", "B.ADHESIVE": "B.Adhes",
	}
	for _, l := range m.layers {
		upper := strings.ToUpper(l.name)
		if _, done := out[upper]; done {
			continue
		}
		back := l.row > last
		low := strings.ToLower(l.name)
		switch {
		case strings.HasPrefix(low, "f.") || strings.Contains(low, "top"):
			back = false
		case strings.HasPrefix(low, "b.") || strings.Contains(low, "bot"):
			back = true
		case len(copper) > 0 && l.row < first:
			back = false
		}
		side := "F."
		if back {
			side = "B."
		}
		switch l.typ {
		case "SILK_SCREEN":
			out[upper] = side + "SilkS"
		case "SOLDER_MASK":
			out[upper] = side + "Mask"
		case "SOLDER_PASTE":
			out[upper] = side + "Paste"
		default:
			if k, ok := kicadDoc[upper]; ok {
				out[upper] = k
			} else {
				out[upper] = l.name
			}
		}
	}
	return out
}

// localOffset turns a pin's board position into the footprint-local offset the geom contract
// stores, inverting world = at + M(R(rot) * offset), so a pad composes back onto exactly the point
// the job put it at whatever angle convention its producer used.
func localOffset(at, world pt, rotDeg float64, mirror bool) *geom.Point {
	dx, dy := float64(world.x-at.x), float64(world.y-at.y)
	if mirror {
		dx = -dx
	}
	rad := -rotDeg * math.Pi / 180
	cos, sin := math.Cos(rad), math.Sin(rad)
	return &geom.Point{X: int64(math.Round(cos*dx - sin*dy)), Y: int64(math.Round(sin*dx + cos*dy))}
}

// normDeg folds an angle into (-180, 180].
func normDeg(d float64) float64 {
	d = math.Mod(d, 360)
	if d > 180 {
		d -= 360
	}
	if d <= -180 {
		d += 360
	}
	if d == 0 {
		return 0
	}
	return d
}

func gp(p pt) *geom.Point { return &geom.Point{X: p.x, Y: p.y} }

func polyShape(pts []pt) *geom.Shape {
	s := &geom.Shape{Kind: geom.Shape_KIND_POLYLINE}
	for _, p := range pts {
		s.Points = append(s.Points, gp(p))
	}
	return s
}

// closedPolyline is a contour's points with the first repeated at the end when the contour does not
// already close itself.
func closedPolyline(pts []pt) *geom.Polyline {
	pl := &geom.Polyline{}
	for _, p := range pts {
		pl.Points = append(pl.Points, gp(p))
	}
	if n := len(pts); n > 1 && pts[0] != pts[n-1] {
		pl.Points = append(pl.Points, gp(pts[0]))
	}
	return pl
}
