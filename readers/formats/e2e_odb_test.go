package formats

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/geomath"
)

// odbPair is one board read twice: from KiCad's own file, and from the ODB++ job kicad-cli exported
// from it (hack/odb_fixtures.sh). The KiCad board reader is the oracle, so every difference is one
// reader or the other getting the board wrong, and the test says which way it went.
type odbPair struct {
	kicad, odb string
	// exported names copper the exporter itself moved to another net, so the pair compares
	// everything else. demo-board plants a short, a GND track laid over a VCC one, and KiCad's ODB++
	// exporter nets copper by what touches rather than by what the file says, so the shorted
	// cluster leaves the board on VCC. Reading the job as written is right; the job disagrees with
	// its source.
	exportRenetsCopper bool
}

// odbPairs are the synthetic boards the fixtures were exported from. AGNI_ODB_PAIRS adds more as
// `board.kicad_pcb=job_dir` separated by `;`, which is how a real board's export is held to its
// source on a machine that has one, since kicad-cli is not on CI.
func odbPairs(t *testing.T) []odbPair {
	pairs := []odbPair{
		{"../../examples/tutorial-project/designs/gateway/gateway.kicad_pcb", "../../examples/common/designs/odb/gateway-layout", false},
		{"../../examples/tutorial-project/designs/gateway/gateway-rev-b.kicad_pcb", "../../examples/common/designs/odb/gateway-rev-b-layout", false},
		{"../../examples/common/designs/demo-board.kicad_pcb", "../odb/testdata/demo-board", true},
		{"../../examples/common/designs/mixer.kicad_pcb", "../odb/testdata/mixer", false},
		{"../../cmd/agni/testdata/conformance/drc.fires.kicad_pcb", "../odb/testdata/drc.fires", false},
		{"../kicad/testdata/board.kicad_pcb", "../odb/testdata/board", false},
	}
	for _, kv := range strings.Split(os.Getenv("AGNI_ODB_PAIRS"), ";") {
		if k, v, ok := strings.Cut(kv, "="); ok {
			pairs = append(pairs, odbPair{kicad: k, odb: v})
		}
	}
	return pairs
}

// pinNets is every connected pin as `REF.PIN=NET`. Comparing the whole set, names included, is the
// strict form of the oracle's partition check: KiCad wrote both files, so a net is named the same in
// each, and a pin on the wrong net shows as one line missing and one extra.
func pinNets(d *ir.Design) []string {
	var out []string
	for _, n := range d.GetNets() {
		if strings.HasPrefix(n.GetName(), "unconnected-(") {
			continue // KiCad's per-pin placeholder; ODB++ puts the same pin on $NONE$
		}
		for _, c := range n.GetConnections() {
			out = append(out, c.GetComponentRef()+"."+c.GetPinRef()+"="+odbName(n.GetName()))
		}
	}
	sort.Strings(out)
	return out
}

// odbEntry is the path a job is read by: an archive as it is, a folder by its matrix file.
func odbEntry(job string) string {
	if IsArchive(job) {
		return job
	}
	return filepath.Join(job, "matrix", "matrix")
}

// odbName is a name as an ODB++ job can spell it. The format forbids whitespace in a name, so
// KiCad's exporter writes `/USB Hub/VBUS` as `/USB_Hub/VBUS`, and the KiCad read is folded the same
// way before the two are compared.
func odbName(s string) string { return strings.ReplaceAll(s, " ", "_") }

func refValues(d *ir.Design) []string {
	var out []string
	for _, c := range d.GetComponents() {
		out = append(out, c.GetRefDes()+"="+c.GetAttributes()["Value"])
	}
	sort.Strings(out)
	return out
}

// placed is one pad, track segment or via, as what it is (key) and where it is (pts, nanometres).
// Two readers are held to the same key and to points within placedTolerance, since they compute a
// pad's position or an arc's vertices along different paths and land a nanometre or two apart.
type placed struct {
	key string
	pts [][2]int64
}

const placedTolerance = 2000 // 2 microns

var exportedPadName = regexp.MustCompile(`^(NPTH|PAD)[0-9]+$`)

// padItems is every pad of every placement keyed by its ref, number, net, shape, size, drill and
// side, at the board position its offset composes to through the placement. KiCad leaves a mounting
// or non-plated pad unnamed where its exporter names it PAD0 or NPTH0, so both read as "?", and a
// custom pad's size is left out because KiCad gives the anchor's size where ODB++ gives the outline's.
func padItems(g *geom.BoardGeometry) []placed {
	var out []placed
	for _, pl := range g.GetPlacements() {
		for _, p := range pl.GetPads() {
			if !onCopper(p) {
				continue
			}
			num := p.GetNumber()
			if num == "" || exportedPadName.MatchString(num) {
				num = "?"
			}
			size := fmt.Sprintf("%dx%d", p.GetSize().GetX(), p.GetSize().GetY())
			if p.GetShape() == "custom" {
				size = "-"
			}
			w := geomath.ComposePlacement(pl.GetAt(), pl.GetRotationDeg(), pl.GetMirror(), p.GetAt())
			out = append(out, placed{
				key: fmt.Sprintf("%s.%s net=%s %s %s drill=%d side=%s", pl.GetRefDes(), num, odbName(p.GetNet()), p.GetShape(), size, p.GetDrill(), pl.GetLayer()),
				pts: [][2]int64{{w.GetX(), w.GetY()}},
			})
		}
	}
	return out
}

// copperItems is every track segment and via on a net, a segment's ends in a fixed order so a run
// drawn the other way round still matches.
func copperItems(g *geom.BoardGeometry) []placed {
	var out []placed
	for _, nc := range g.GetNets() {
		if nc.GetNet() == "" {
			continue
		}
		net := odbName(nc.GetNet())
		for _, s := range nc.GetSegments() {
			a, b := [2]int64{s.GetA().GetX(), s.GetA().GetY()}, [2]int64{s.GetB().GetX(), s.GetB().GetY()}
			if b[0] < a[0] || (b[0] == a[0] && b[1] < a[1]) {
				a, b = b, a
			}
			out = append(out, placed{key: fmt.Sprintf("seg %s %s w=%d", net, s.GetLayer(), (s.GetWidth()+500)/1000), pts: [][2]int64{a, b}})
		}
		for _, v := range nc.GetVias() {
			out = append(out, placed{
				key: fmt.Sprintf("via %s %d/%d", net, (v.GetSize()+500)/1000, (v.GetDrill()+500)/1000),
				pts: [][2]int64{{v.GetAt().GetX(), v.GetAt().GetY()}},
			})
		}
	}
	return out
}

// onCopper reports whether a pad has a copper land, or names no layers at all as the ODB++ reader's
// copperless pads do. A KiCad pad on paste or mask alone is a stencil or mask opening, which an
// ODB++ job carries on those layers and never as a part's pad.
func onCopper(p *geom.Pad) bool {
	if len(p.GetLayers()) == 0 {
		return true
	}
	for _, l := range p.GetLayers() {
		if strings.HasSuffix(l, ".Cu") {
			return true
		}
	}
	return false
}

func (p placed) String() string {
	var b strings.Builder
	b.WriteString(p.key)
	for _, q := range p.pts {
		fmt.Fprintf(&b, " (%d,%d)um", (q[0]+500)/1000, (q[1]+500)/1000)
	}
	return b.String()
}

func near(a, b placed) bool {
	if len(a.pts) != len(b.pts) {
		return false
	}
	for i := range a.pts {
		if abs64(a.pts[i][0]-b.pts[i][0]) > placedTolerance || abs64(a.pts[i][1]-b.pts[i][1]) > placedTolerance {
			return false
		}
	}
	return true
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// diffPlaced pairs each KiCad item with an unpaired ODB++ item of the same key within the tolerance,
// looking in the 20-micron cell of its first point and the cells around it, and reports what is left
// on either side.
func diffPlaced(what string, kicad, odb []placed) []string {
	cell := func(p placed) [3]int64 { return [3]int64{0, p.pts[0][0] / 20000, p.pts[0][1] / 20000} }
	type slot struct {
		p    placed
		used bool
	}
	byKey := map[string]map[[3]int64][]*slot{}
	for _, o := range odb {
		if byKey[o.key] == nil {
			byKey[o.key] = map[[3]int64][]*slot{}
		}
		c := cell(o)
		byKey[o.key][c] = append(byKey[o.key][c], &slot{p: o})
	}
	var onlyK []string
	for _, k := range kicad {
		c, found := cell(k), false
		for dx := int64(-1); dx <= 1 && !found; dx++ {
			for dy := int64(-1); dy <= 1 && !found; dy++ {
				for _, s := range byKey[k.key][[3]int64{0, c[1] + dx, c[2] + dy}] {
					if !s.used && near(k, s.p) {
						s.used, found = true, true
						break
					}
				}
			}
		}
		if !found {
			onlyK = append(onlyK, k.String())
		}
	}
	var onlyO []string
	for _, cells := range byKey {
		for _, ss := range cells {
			for _, s := range ss {
				if !s.used {
					onlyO = append(onlyO, s.p.String())
				}
			}
		}
	}
	return onlyLines(what, onlyK, onlyO)
}

// diffRows is the rows only one side has, as onlyLines spells them.
func diffRows(what string, kicad, odb []string) []string {
	k, o := map[string]int{}, map[string]int{}
	for _, r := range kicad {
		k[r]++
	}
	for _, r := range odb {
		o[r]++
	}
	var onlyK, onlyO []string
	for r, n := range k {
		for i := o[r]; i < n; i++ {
			onlyK = append(onlyK, r)
		}
	}
	for r, n := range o {
		for i := k[r]; i < n; i++ {
			onlyO = append(onlyO, r)
		}
	}
	return onlyLines(what, onlyK, onlyO)
}

// onlyLines is one sorted line per row only one read has, `<what>: kicad only: <row>` or
// `<what>: odb++ only: <row>`, the form the oracle baseline holds.
func onlyLines(what string, onlyK, onlyO []string) []string {
	var out []string
	for _, r := range onlyK {
		out = append(out, what+": kicad only: "+r)
	}
	for _, r := range onlyO {
		out = append(out, what+": odb++ only: "+r)
	}
	sort.Strings(out)
	return out
}

// compareBoard reads one board both ways and returns every difference, nil when the two agree.
func compareBoard(p odbPair) ([]string, error) {
	l := &Loader{}
	kd, err := l.ReadDesign(p.kicad)
	if err != nil {
		return nil, fmt.Errorf("kicad read: %w", err)
	}
	od, err := l.ReadDesign(odbEntry(p.odb))
	if err != nil {
		return nil, fmt.Errorf("odb read: %w", err)
	}
	kp := pinNets(kd)
	if len(kp) == 0 {
		return nil, fmt.Errorf("the KiCad read has no connected pins, so there is nothing to compare")
	}
	diffs := append(diffRows("pin nets", kp, pinNets(od)), diffRows("parts", refValues(kd), refValues(od))...)
	kg, err := l.BoardGeometry(p.kicad)
	if err != nil {
		return nil, fmt.Errorf("kicad board: %w", err)
	}
	og, err := l.BoardGeometry(odbEntry(p.odb))
	if err != nil {
		return nil, fmt.Errorf("odb board: %w", err)
	}
	diffs = append(diffs, diffPlaced("pads", padItems(kg), padItems(og))...)
	if p.exportRenetsCopper {
		if k, o := len(copperItems(kg)), len(copperItems(og)); k != o {
			diffs = append(diffs, fmt.Sprintf("copper: %d items in the ODB++ read, %d in the KiCad read", o, k))
		}
		return diffs, nil
	}
	return append(diffs, diffPlaced("copper", copperItems(kg), copperItems(og))...), nil
}

// capLines shortens a list for a failure message, so a broken reader does not print a board.
func capLines(lines []string) string {
	if len(lines) > 12 {
		lines = append(lines[:12:12], fmt.Sprintf("... and %d more", len(lines)-12))
	}
	return strings.Join(lines, "\n  ")
}

// TestODBReadsAsItsKiCadSource holds each ODB++ fixture to the board it was exported from: the same
// parts with the same values, every pin on the same net, the same pads, and the same copper.
func TestODBReadsAsItsKiCadSource(t *testing.T) {
	for _, p := range odbPairs(t) {
		t.Run(filepath.Base(p.odb), func(t *testing.T) {
			diffs, err := compareBoard(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(diffs) > 0 {
				t.Errorf("%d differences between the two reads:\n  %s", len(diffs), capLines(diffs))
			}
		})
	}
}
