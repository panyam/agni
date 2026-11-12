package formats

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// stackLands lists each padstack entry's lands by copper-layer position, so two reads that name
// their layers differently ("In1.Cu" against "L2-GND") compare layer by layer (agni 1356).
func stackLands(g *geom.BoardGeometry) map[string][]string {
	pos := map[string]int{}
	n := 0
	for _, l := range g.GetLayers() {
		switch strings.ToUpper(l.GetKind()) {
		case "SIGNAL", "POWER", "CONDUCTOR", "PLANE":
			pos[l.GetName()] = n
			n++
		}
	}
	// IPC-2581's layer table keeps TOP and BOTTOM while its lands say F.Cu and B.Cu, so the outer
	// pair is found by position.
	if _, ok := pos["F.Cu"]; !ok {
		pos["F.Cu"] = 0
	}
	if _, ok := pos["B.Cu"]; !ok {
		pos["B.Cu"] = n - 1
	}
	out := map[string][]string{}
	for _, ps := range g.GetPadstacks() {
		var lands []string
		for _, l := range ps.GetLayers() {
			if p := l.GetPad(); p != nil {
				i, ok := pos[l.GetLayer()]
				if !ok {
					i = -1
				}
				// Sizes are compared in microns, which is finer than either export writes.
				lands = append(lands, fmt.Sprintf("L%d %s %d×%d", i, p.GetShape(), (p.GetSize().GetX()+500)/1000, (p.GetSize().GetY()+500)/1000))
			}
		}
		out[ps.GetId()] = lands
	}
	return out
}

// TestPadstacksAgreeAcrossParasdrsReads holds the per-layer lands of parasdr's 16.6 board to those
// its IPC-2581 export draws (agni 1356). Allegro wrote both, and the export names each land's
// padstack, so every padstack it uses must have the board's land on every copper layer, at the
// board's size. A pad Allegro draws as a shape symbol reads as custom on the board, and the export
// gives that shape a primitive of its own, so a custom land is compared by size alone.
func TestPadstacksAgreeAcrossParasdrsReads(t *testing.T) {
	brdPath, ipcPath := parasdr+"parasdr_layout.brd", parasdr+"parasdr_ipc2581.xml"
	if _, err := os.Stat(brdPath); err != nil {
		t.Fatalf("parasdr's board is missing; run `make samples-oracle`: %v", err)
	}
	l := &Loader{}
	bg, err := l.BoardGeometry(brdPath)
	if err != nil {
		t.Fatal(err)
	}
	ig, err := l.BoardGeometry(ipcPath)
	if err != nil {
		t.Fatal(err)
	}
	board, export := stackLands(bg), stackLands(ig)
	strip := func(lands []string) []string {
		out := make([]string, len(lands))
		for i, s := range lands {
			f := strings.Fields(s)
			out[i] = f[0] + " " + f[2]
		}
		return out
	}
	var ids []string
	for id := range export {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	lands := 0
	for _, id := range ids {
		want, ok := board[id]
		if !ok {
			t.Errorf("the export uses padstack %s, which the board read lacks", id)
			continue
		}
		got := export[id]
		custom := strings.Contains(strings.Join(want, " "), " custom ")
		a, b := strings.Join(want, "; "), strings.Join(got, "; ")
		if custom {
			a, b = strings.Join(strip(want), "; "), strings.Join(strip(got), "; ")
		}
		if a != b {
			t.Errorf("padstack %s:\n  board  %s\n  export %s", id, strings.Join(want, "; "), strings.Join(got, "; "))
		}
		lands += len(got)
	}
	t.Logf("%d padstacks, %d lands compared", len(ids), lands)
	if len(ids) < 10 || lands < 40 {
		t.Fatalf("compared %d padstacks and %d lands, too few to mean anything", len(ids), lands)
	}
}

// pinLands lists each placed pad's lands by copper-layer position, keyed "<ref>.<pin>", through
// the padstack entry the pad names.
func pinLands(g *geom.BoardGeometry) map[string][]string {
	byID := stackLands(g)
	out := map[string][]string{}
	for _, pl := range g.GetPlacements() {
		for _, pad := range pl.GetPads() {
			if lands, ok := byID[pad.GetPadstack()]; ok && pad.GetPadstack() != "" {
				out[pl.GetRefDes()+"."+pad.GetNumber()] = lands
			}
		}
	}
	return out
}

// TestPinLandsAgreeAcrossParasdrsReads holds every pin's per-layer lands on parasdr to the same pin's
// in each export (agni 1356). The board read takes them from the padstacks, and the IPC-2581 and
// ODB++X reads rebuild them from each layer's drawn lands.
//
// The two exports draw different sets. IPC-2581 draws every land the padstack defines, so it must
// match the board land for land. ODB++X leaves out an inner land that nothing on its layer joins,
// as Allegro's films do, so each land it draws must be the board's on that layer, and the ones it
// leaves out are counted. A pin the board draws as a shape symbol reads as custom there, and the
// exports give it a primitive of their own, turned either way, so it is compared by its extent.
func TestPinLandsAgreeAcrossParasdrsReads(t *testing.T) {
	l := &Loader{}
	read := func(path string) map[string][]string {
		g, err := l.BoardGeometry(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return pinLands(g)
	}
	board := read(parasdr + "parasdr_layout.brd")
	// land splits "L0 circle 1602×1602" into its layer and a key that compares two reads.
	land := func(s string, custom bool) (string, string) {
		f := strings.Fields(s)
		if !custom {
			return f[0], f[1] + " " + f[2]
		}
		wh := strings.Split(f[2], "×")
		sort.Strings(wh)
		return f[0], strings.Join(wh, "×")
	}
	for _, c := range []struct {
		file   string
		subset bool
	}{{"parasdr_ipc2581.xml", false}, {"parasdr_odb++.xml", true}} {
		got := read(parasdr + c.file)
		var pins []string
		for pin := range got {
			pins = append(pins, pin)
		}
		sort.Strings(pins)
		compared, bad, dropped := 0, 0, 0
		for _, pin := range pins {
			want, ok := board[pin]
			if !ok {
				continue
			}
			compared++
			custom := strings.Contains(strings.Join(want, " "), " custom ")
			wantBy := map[string]string{}
			for _, s := range want {
				ly, k := land(s, custom)
				wantBy[ly] = k
			}
			ok = len(got[pin]) == len(want) || (c.subset && len(got[pin]) <= len(want))
			for _, s := range got[pin] {
				ly, k := land(s, custom)
				if wantBy[ly] != k {
					ok = false
				}
			}
			dropped += len(want) - len(got[pin])
			if !ok {
				bad++
				if bad <= 10 {
					t.Errorf("%s pin %s:\n  board  %s\n  export %s", c.file, pin, strings.Join(want, "; "), strings.Join(got[pin], "; "))
				}
			}
		}
		t.Logf("%s: %d pins compared, %d differ, %d board lands not drawn", c.file, compared, bad, dropped)
		if compared < 400 {
			t.Errorf("%s: compared %d pins, want at least 400", c.file, compared)
		}
	}
}
