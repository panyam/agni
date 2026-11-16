package formats

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// stackTol is how far two readings of one stack row may differ, in nanometres. Board6 writes mils
// to four places and ODB++ millimetres to three or four.
const stackTol = 1_500

// TestAltiumStackMatchesItsODBExport holds each foxBMS board's stack, read from the .PcbDoc, to the
// ODB++ job Altium exported from it (agni 1601). Altium's export writes no dielectric rows. It gives
// each copper layer the thickness of the dielectric below it in .layer_dielectric, the bottom copper
// the bottom mask's, and the board thickness as copper and dielectric together. agni's ODB++ reader
// reads .layer_dielectric as the layer's own thickness today (#1660), so the job's copper rows carry
// those values.
func TestAltiumStackMatchesItsODBExport(t *testing.T) {
	for _, pr := range altiumPairs(t) {
		t.Run(pr.name, func(t *testing.T) {
			_, pg := readBoth(t, pr.pcb)
			_, og := readBoth(t, pr.odb)
			ours, theirs := pg.GetStackup(), og.GetStackup()
			copper := rowsOf(ours, "copper")
			jobCopper := rowsOf(theirs, "copper")
			if len(copper) < 2 || len(copper) != len(jobCopper) {
				t.Fatalf("%d copper rows, the job has %d", len(copper), len(jobCopper))
			}
			below := belowEachCopper(ours)
			for i, c := range jobCopper {
				if c.Thickness == nil || below[i] == nil || abs(below[i].GetThickness()-c.GetThickness()) > stackTol {
					t.Errorf("under %s: %v, the job says %v", copper[i].GetName(), thick(below[i]), thick(c))
				}
			}
			sum := int64(0)
			for _, l := range ours.GetLayers() {
				if l.GetKind() == "copper" || l.GetKind() == "dielectric" {
					sum += l.GetThickness()
				}
			}
			if theirs.Thickness == nil || abs(sum-theirs.GetThickness()) > 4*stackTol {
				t.Errorf("copper and dielectric sum to %d nm, the job states %v", sum, theirs.Thickness)
			}
		})
	}
}

// TestAltiumStackMatchesItsASCIISave holds each OSP board's stack, read from the binary .PcbDoc, to
// the ASCII save Altium wrote of the same board (agni 1601). The ASCII's Board record names each
// layer's copper thickness, the dielectric below it and its material. The test finds each copper
// row's layer there by its name.
func TestAltiumStackMatchesItsASCIISave(t *testing.T) {
	boards, _ := filepath.Glob("../../tools/samples/altium/osp-*/*.PcbDoc")
	if len(boards) != 2 {
		t.Fatalf("found %d OSP boards, want 2; run `make samples-oracle`", len(boards))
	}
	checked := 0
	for _, pcb := range boards {
		ascii, _ := filepath.Glob(filepath.Join(filepath.Dir(pcb), "Outputs", "Assembly", "*.PcbDoc"))
		if len(ascii) != 1 {
			t.Fatalf("%s: %d ASCII saves, want 1", pcb, len(ascii))
		}
		rec := asciiBoard(t, ascii[0])
		_, g := readBoth(t, pcb)
		layers := g.GetStackup().GetLayers()
		for i, l := range layers {
			if l.GetKind() != "copper" {
				continue
			}
			n := asciiLayer(rec, l.GetName())
			if n == "" {
				t.Errorf("%s: no layer named %q in the ASCII save", pcb, l.GetName())
				continue
			}
			checked++
			if want := mils(t, rec[n+"COPTHICK"]); abs(l.GetThickness()-want) > stackTol {
				t.Errorf("%s %s: copper %d nm, the ASCII save says %d", pcb, l.GetName(), l.GetThickness(), want)
			}
			if i+1 >= len(layers) || layers[i+1].GetKind() != "dielectric" {
				continue
			}
			d := layers[i+1]
			if want := mils(t, rec[n+"DIELHEIGHT"]); abs(d.GetThickness()-want) > stackTol || d.GetMaterial() != rec[n+"DIELMATERIAL"] {
				t.Errorf("%s under %s: %d nm of %q, the ASCII save says %d of %q", pcb, l.GetName(), d.GetThickness(), d.GetMaterial(), want, rec[n+"DIELMATERIAL"])
			}
		}
		for _, m := range []struct{ layer, key string }{{"F.Mask", "TOPHEIGHT"}, {"B.Mask", "BOTTOMHEIGHT"}} {
			want := mils(t, rec[m.key])
			got := int64(-1)
			for _, l := range layers {
				if l.GetLayer() == m.layer {
					got = l.GetThickness()
				}
			}
			if abs(got-want) > stackTol {
				t.Errorf("%s %s: %d nm, the ASCII save says %d", pcb, m.layer, got, want)
			}
		}
	}
	if checked < 8 {
		t.Errorf("checked %d copper layers, want at least 8", checked)
	}
}

func rowsOf(s *geom.Stackup, kind string) []*geom.StackLayer {
	var out []*geom.StackLayer
	for _, l := range s.GetLayers() {
		if l.GetKind() == kind {
			out = append(out, l)
		}
	}
	return out
}

// belowEachCopper is the row under each copper row: its dielectric, or the bottom mask under the
// last copper.
func belowEachCopper(s *geom.Stackup) []*geom.StackLayer {
	var out []*geom.StackLayer
	ls := s.GetLayers()
	for i, l := range ls {
		if l.GetKind() != "copper" {
			continue
		}
		var next *geom.StackLayer
		if i+1 < len(ls) {
			next = ls[i+1]
		}
		out = append(out, next)
	}
	return out
}

func thick(l *geom.StackLayer) any {
	if l == nil || l.Thickness == nil {
		return "nothing"
	}
	return strconv.FormatInt(l.GetThickness(), 10) + " nm " + l.GetKind()
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// asciiBoard is the first Board record of an ASCII .PcbDoc that names layer 1's copper thickness, as
// key=value pairs. A key's first value wins, as in the binary Board6.
func asciiBoard(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "|RECORD=Board|") || !strings.Contains(line, "|LAYER1COPTHICK=") {
			continue
		}
		out := map[string]string{}
		for _, kv := range strings.Split(line, "|") {
			if k, v, ok := strings.Cut(kv, "="); ok {
				if _, seen := out[k]; !seen {
					out[k] = v
				}
			}
		}
		return out
	}
	t.Fatalf("%s: no Board record with LAYER1COPTHICK", path)
	return nil
}

// asciiLayer is the LAYER<n> key prefix of the layer the Board record names name.
func asciiLayer(rec map[string]string, name string) string {
	for n := 1; n <= 82; n++ {
		p := "LAYER" + strconv.Itoa(n)
		if rec[p+"NAME"] == name {
			return p
		}
	}
	return ""
}

func mils(t *testing.T, s string) int64 {
	t.Helper()
	v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s), "mil"), 64)
	if err != nil {
		t.Fatalf("length %q: %v", s, err)
	}
	return int64(v*25_400 + 0.5)
}
