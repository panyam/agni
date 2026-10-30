package formats

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
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

// padRows is every pad of every placement as its ref, number, net, shape and size, which is what the
// board rules and the renderer read off a pad. Position is left out on purpose, see
// TestODBPadsLandWhereKiCadPutThem.
func padRows(g *geom.BoardGeometry) []string {
	var out []string
	for _, pl := range g.GetPlacements() {
		for _, p := range pl.GetPads() {
			out = append(out, fmt.Sprintf("%s.%s net=%s %s %dx%d drill=%d side=%s", pl.GetRefDes(), p.GetNumber(), odbName(p.GetNet()),
				p.GetShape(), p.GetSize().GetX(), p.GetSize().GetY(), p.GetDrill(), pl.GetLayer()))
		}
	}
	sort.Strings(out)
	return out
}

// copperRows is every track segment and via, rounded to a micron, since KiCad's export writes
// coordinates to two decimals of a millimetre by default and a track can end between them.
func copperRows(g *geom.BoardGeometry) []string {
	um := func(v int64) int64 { return (v + 500) / 1000 }
	var out []string
	for _, nc := range g.GetNets() {
		if nc.GetNet() == "" {
			continue
		}
		for _, s := range nc.GetSegments() {
			a, b := s.GetA(), s.GetB()
			ends := []string{fmt.Sprintf("%d,%d", um(a.GetX()), um(a.GetY())), fmt.Sprintf("%d,%d", um(b.GetX()), um(b.GetY()))}
			sort.Strings(ends)
			out = append(out, fmt.Sprintf("seg %s %s %s-%s w=%d", odbName(nc.GetNet()), s.GetLayer(), ends[0], ends[1], um(s.GetWidth())))
		}
		for _, v := range nc.GetVias() {
			out = append(out, fmt.Sprintf("via %s %d,%d %d/%d", odbName(nc.GetNet()), um(v.GetAt().GetX()), um(v.GetAt().GetY()), um(v.GetSize()), um(v.GetDrill())))
		}
	}
	sort.Strings(out)
	return out
}

// diffRows reports the rows only one side has, capped so a broken reader does not print a board.
func diffRows(t *testing.T, what string, kicad, odb []string) {
	t.Helper()
	k, o := map[string]int{}, map[string]int{}
	for _, r := range kicad {
		k[r]++
	}
	for _, r := range odb {
		o[r]++
	}
	var onlyK, onlyO []string
	for r, n := range k {
		if o[r] < n {
			onlyK = append(onlyK, r)
		}
	}
	for r, n := range o {
		if k[r] < n {
			onlyO = append(onlyO, r)
		}
	}
	sort.Strings(onlyK)
	sort.Strings(onlyO)
	if len(onlyK)+len(onlyO) == 0 {
		return
	}
	cap := func(s []string) []string {
		if len(s) > 8 {
			return append(s[:8], fmt.Sprintf("... and %d more", len(s)-8))
		}
		return s
	}
	t.Errorf("%s differ: %d only in the KiCad read, %d only in the ODB++ read\n  kicad only: %s\n  odb++ only: %s",
		what, len(onlyK), len(onlyO), strings.Join(cap(onlyK), "\n              "), strings.Join(cap(onlyO), "\n              "))
}

// TestODBReadsAsItsKiCadSource holds each ODB++ fixture to the board it was exported from: the same
// parts with the same values, every pin on the same net, the same pads, and the same copper.
func TestODBReadsAsItsKiCadSource(t *testing.T) {
	for _, p := range odbPairs(t) {
		t.Run(filepath.Base(p.odb), func(t *testing.T) {
			l := &Loader{}
			kd, err := l.ReadDesign(p.kicad)
			if err != nil {
				t.Fatalf("kicad read: %v", err)
			}
			od, err := l.ReadDesign(filepath.Join(p.odb, "matrix", "matrix"))
			if err != nil {
				t.Fatalf("odb read: %v", err)
			}
			kp, op := pinNets(kd), pinNets(od)
			if len(kp) == 0 {
				t.Fatalf("the KiCad read has no connected pins, so there is nothing to compare")
			}
			diffRows(t, "pin nets", kp, op)
			diffRows(t, "parts", refValues(kd), refValues(od))

			kg, err := l.BoardGeometry(p.kicad)
			if err != nil {
				t.Fatalf("kicad board: %v", err)
			}
			og, err := l.BoardGeometry(filepath.Join(p.odb, "matrix", "matrix"))
			if err != nil {
				t.Fatalf("odb board: %v", err)
			}
			diffRows(t, "pads", padRows(kg), padRows(og))
			if p.exportRenetsCopper {
				if len(copperRows(og)) != len(copperRows(kg)) {
					t.Errorf("copper items = %d, want the KiCad read's %d", len(copperRows(og)), len(copperRows(kg)))
				}
				return
			}
			diffRows(t, "copper", copperRows(kg), copperRows(og))
		})
	}
}
