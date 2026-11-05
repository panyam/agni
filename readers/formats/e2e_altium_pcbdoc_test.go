package formats

import (
	"math"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// altiumBoards are the foxBMS boards as Altium saved them (CC-BY-4.0, the altium samples artifact).
// Each board's ODB++ export, which Altium wrote from the same board, is under altiumJobs.
const altiumBoards = "../../tools/samples/altium/foxbms"

type altiumPair struct {
	name     string
	pcb, odb string
}

func altiumPairs(t *testing.T) []altiumPair {
	t.Helper()
	docs, _ := filepath.Glob(filepath.Join(altiumBoards, "*", "*.PcbDoc"))
	if len(docs) != 5 {
		// Fatal rather than skipped: an absent corpus must not read as a pass.
		t.Fatalf("found %d .PcbDoc boards under %s, want foxBMS's 5; run `make samples-oracle`", len(docs), altiumBoards)
	}
	var out []altiumPair
	for _, d := range docs {
		board := filepath.Base(filepath.Dir(d))
		out = append(out, altiumPair{name: board, pcb: d, odb: filepath.Join(altiumJobs, board, board+".odb.zip")})
	}
	return out
}

func readBoth(t *testing.T, path string) (*ir.Design, *geom.BoardGeometry) {
	t.Helper()
	l := &Loader{}
	d, err := l.ReadDesign(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	g, err := l.BoardGeometry(path)
	if err != nil {
		t.Fatalf("board %s: %v", path, err)
	}
	return d, g
}

// partition maps each net to its sorted "<ref>.<pin>" members.
func partition(d *ir.Design) map[string]string {
	out := map[string]string{}
	for _, n := range d.GetNets() {
		var m []string
		for _, c := range n.GetConnections() {
			m = append(m, c.GetComponentRef()+"."+c.GetPinRef())
		}
		sort.Strings(m)
		out[n.GetName()] = strings.Join(m, " ")
	}
	return out
}

// TestAltiumPcbDocMatchesItsODBExport holds each foxBMS .PcbDoc, read by agni, to the ODB++ job
// Altium exported from the same board, read by agni's ODB++ reader: the same parts, the same pins on
// the same nets, the same placements and pad positions, the same vias and the same track widths.
func TestAltiumPcbDocMatchesItsODBExport(t *testing.T) {
	for _, pr := range altiumPairs(t) {
		t.Run(pr.name, func(t *testing.T) {
			pd, pg := readBoth(t, pr.pcb)
			od, og := readBoth(t, pr.odb)

			ours := map[string]bool{}
			for _, c := range pd.GetComponents() {
				ours[c.GetRefDes()] = true
			}
			theirs := map[string]bool{}
			for _, c := range od.GetComponents() {
				theirs[c.GetRefDes()] = true
			}
			if len(theirs) < 50 {
				t.Fatalf("the ODB++ export lists %d parts", len(theirs))
			}
			var missing, extra []string
			for r := range theirs {
				if !ours[r] {
					missing = append(missing, r)
				}
			}
			for r := range ours {
				if !theirs[r] {
					extra = append(extra, r)
				}
			}
			sort.Strings(missing)
			sort.Strings(extra)
			if len(missing) > 0 || len(extra) > 0 {
				t.Errorf("parts: %d only in the export %v, %d only in the .PcbDoc %v", len(missing), missing, len(extra), extra)
			}

			want, got := partition(od), partition(pd)
			pins := 0
			for net, m := range want {
				if got[net] != m {
					t.Errorf("net %s: .PcbDoc %q, export %q", net, got[net], m)
				}
				pins += strings.Count(m, " ") + 1
			}
			for net := range got {
				if _, ok := want[net]; !ok {
					t.Errorf("net %s is only in the .PcbDoc: %q", net, got[net])
				}
			}
			if pins < 200 {
				t.Fatalf("compared %d pins", pins)
			}

			compareAltiumBoards(t, pg, og)
		})
	}
}

// compareAltiumBoards holds placement origins and sides, every pad's board position, the vias and
// the track widths of the .PcbDoc's geometry to the export's.
func compareAltiumBoards(t *testing.T, pg, og *geom.BoardGeometry) {
	t.Helper()
	const tol = 2540 // 1 mil
	near := func(a, b int64) bool { return math.Abs(float64(a-b)) <= tol }

	type padKey struct{ ref, pin string }
	padsOf := func(g *geom.BoardGeometry) (map[string]*geom.ComponentPlacement, map[padKey][2]float64) {
		pl := map[string]*geom.ComponentPlacement{}
		pads := map[padKey][2]float64{}
		repeated := map[padKey]bool{}
		for _, p := range g.GetPlacements() {
			pl[p.GetRefDes()] = p
			for _, pad := range p.GetPads() {
				k := padKey{p.GetRefDes(), pad.GetNumber()}
				if _, dup := pads[k]; dup {
					repeated[k] = true // several pads of one name, such as a shell's SH pads, which an export merges
				}
				x, y := boardPoint(p, pad.GetAt())
				pads[k] = [2]float64{x, y}
			}
		}
		for k := range repeated {
			delete(pads, k)
		}
		return pl, pads
	}
	ppl, ppads := padsOf(pg)
	opl, opads := padsOf(og)

	placed := 0
	for ref, o := range opl {
		p := ppl[ref]
		if p == nil {
			t.Errorf("%s: placed in the export, not in the .PcbDoc", ref)
			continue
		}
		placed++
		if !near(p.GetAt().GetX(), o.GetAt().GetX()) || !near(p.GetAt().GetY(), o.GetAt().GetY()) {
			t.Errorf("%s: at (%d, %d) nm, export (%d, %d)", ref, p.GetAt().GetX(), p.GetAt().GetY(), o.GetAt().GetX(), o.GetAt().GetY())
		}
		if p.GetMirror() != o.GetMirror() {
			t.Errorf("%s: mirror %v, export %v", ref, p.GetMirror(), o.GetMirror())
		}
	}
	matched := 0
	for k, o := range opads {
		p, ok := ppads[k]
		if !ok {
			continue // a pad name the export merged, such as several SH pads of one shell
		}
		matched++
		if math.Abs(p[0]-o[0]) > tol || math.Abs(p[1]-o[1]) > tol {
			t.Errorf("%s.%s: on the board at (%.0f, %.0f) nm, export (%.0f, %.0f)", k.ref, k.pin, p[0], p[1], o[0], o[1])
		}
	}
	if placed < 50 || matched < 200 {
		t.Fatalf("compared %d placements and %d pads", placed, matched)
	}

	// Vias are matched by net within the pad tolerance. A drilled pad of a part the export leaves
	// out, such as a mounting hole or a mechanical part on a net, comes out of ODB++ as a via, and it
	// is a pad in the .PcbDoc.
	type spot struct {
		x, y int64
		net  string
	}
	viasOf := func(g *geom.BoardGeometry) []spot {
		var out []spot
		for _, nc := range g.GetNets() {
			for _, v := range nc.GetVias() {
				out = append(out, spot{v.GetAt().GetX(), v.GetAt().GetY(), nc.GetNet()})
			}
		}
		return out
	}
	var pads []spot
	for _, p := range pg.GetPlacements() {
		for _, pad := range p.GetPads() {
			x, y := boardPoint(p, pad.GetAt())
			pads = append(pads, spot{int64(x), int64(y), pad.GetNet()})
		}
	}
	pv, ov := viasOf(pg), viasOf(og)
	used := make([]bool, len(pv))
	viasMatched := 0
	for _, o := range ov {
		found := false
		for i, p := range pv {
			if !used[i] && p.net == o.net && near(p.x, o.x) && near(p.y, o.y) {
				used[i], found = true, true
				break
			}
		}
		if !found {
			for _, p := range pads {
				if near(p.x, o.x) && near(p.y, o.y) {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("via at (%d, %d) nm on %q is in the export and not in the .PcbDoc", o.x, o.y, o.net)
			continue
		}
		viasMatched++
	}
	for i, p := range pv {
		if !used[i] {
			t.Errorf("via at (%d, %d) nm on %q is in the .PcbDoc and not in the export", p.x, p.y, p.net)
		}
	}
	if viasMatched == 0 {
		t.Fatal("matched no vias")
	}

	widths := func(g *geom.BoardGeometry) map[string]map[int64]bool {
		m := map[string]map[int64]bool{}
		for _, nc := range g.GetNets() {
			for _, s := range nc.GetSegments() {
				if m[s.GetLayer()] == nil {
					m[s.GetLayer()] = map[int64]bool{}
				}
				m[s.GetLayer()][(s.GetWidth()+50)/100] = true
			}
		}
		return m
	}
	pw, ow := widths(pg), widths(og)
	for layer, ws := range pw {
		for w := range ws {
			if !ow[layer][w] {
				t.Errorf("%s: a track %d00 nm wide, a width the export never draws on that layer", layer, w)
			}
		}
	}
}

// shapeArea is a copper shape's area less its holes', in mm², by the shoelace formula.
func shapeArea(s *geom.CopperShape) float64 {
	ring := func(pl *geom.Polyline) float64 {
		pts := pl.GetPoints()
		a := 0.0
		for i := range pts {
			j := (i + 1) % len(pts)
			a += float64(pts[i].GetX())*float64(pts[j].GetY()) - float64(pts[j].GetX())*float64(pts[i].GetY())
		}
		return math.Abs(a) / 2 / 1e12
	}
	a := ring(s.GetOutline())
	for _, h := range s.GetHoles() {
		a -= ring(h)
	}
	return a
}

// TestAltiumPoursMatchTheirODBExport holds each foxBMS .PcbDoc's filled copper to the ODB++ job
// Altium exported from the same board (agni 1410). The pours and solid regions must be the same
// shapes on the same layers and nets, with the same number of holes, and each layer and net's copper
// must cover the same area to within 0.5%.
func TestAltiumPoursMatchTheirODBExport(t *testing.T) {
	total := 0
	for _, pr := range altiumPairs(t) {
		t.Run(pr.name, func(t *testing.T) {
			_, pg := readBoth(t, pr.pcb)
			_, og := readBoth(t, pr.odb)
			ours, theirs := shapeRows(pg), shapeRows(og)
			if diffs := diffRows("shapes", ours, theirs); len(diffs) > 0 {
				t.Errorf("%d differences (copper layer, net, holes):\n  %s", len(diffs), capLines(diffs))
			}
			area := func(g *geom.BoardGeometry) map[string]float64 {
				m := map[string]float64{}
				for _, nc := range g.GetNets() {
					for _, s := range nc.GetShapes() {
						m[s.GetLayer()+" "+nc.GetNet()] += shapeArea(s)
					}
				}
				return m
			}
			oa, ta := area(pg), area(og)
			for k, a := range ta {
				if math.Abs(oa[k]-a) > 0.005*a {
					t.Errorf("%s: the .PcbDoc's copper covers %.3f mm², the export's %.3f mm²", k, oa[k], a)
				}
			}
			total += len(theirs)
		})
	}
	if total < 300 {
		t.Fatalf("compared %d shapes across the boards, too few to mean anything", total)
	}
}
