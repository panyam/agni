package kicad

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Every net the reader solves carries the name KiCad gives it, checked against kicad-cli's own netlist
// of each fixture (testdata/*.netnames, written by hack/kicad_net_names.sh). Nets are paired by the
// pins on them, so a wrong name fails here and a wrong partition fails the partition oracles. Power
// symbols are not pins in KiCad's netlist, so their virtual connections are left out of the pairing.
func TestFixtureNetNamesAreKiCads(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.netnames"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 20 {
		t.Fatalf("found %d .netnames oracles, want at least 20; run hack/kicad_net_names.sh", len(files))
	}
	compared := 0
	for _, f := range files {
		fixture := strings.TrimSuffix(filepath.Base(f), ".netnames") + ".kicad_sch"
		t.Run(fixture, func(t *testing.T) {
			want := readNetNames(t, f)
			open := func(rel string) ([]byte, error) { return os.ReadFile(filepath.Join("testdata", rel)) }
			d, _, err := ReadSchematicHierarchyNets(fixture, readFixture(t, fixture), open)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			for _, n := range d.GetNets() {
				var pins []string
				for _, c := range n.GetConnections() {
					if !strings.HasPrefix(c.GetComponentRef(), "#") {
						pins = append(pins, c.GetComponentRef()+"."+c.GetPinRef())
					}
				}
				if len(pins) > 0 {
					sort.Strings(pins)
					got[strings.Join(pins, " ")] = n.GetName()
				}
			}
			for pins, name := range want {
				// An unannotated part ("R?1845") is a placeholder the reader does not take as a
				// reference designator (refdes.IsPlaceholder), which is identity rather than naming.
				if strings.Contains(pins, "?") {
					continue
				}
				if g, ok := got[pins]; !ok {
					t.Errorf("KiCad's net %q (%s) has no net with the same pins here", name, pins)
				} else if g != name {
					t.Errorf("net on %s is %q, KiCad names it %q", pins, g, name)
				}
				compared++
			}
		})
	}
	if compared == 0 {
		t.Fatal("compared no nets; the oracles parsed to nothing")
	}
}

// readNetNames reads a .netnames oracle into its nets, keyed by their sorted ref.pin list.
func readNetNames(t *testing.T, path string) map[string]string {
	t.Helper()
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, pins, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("%s: line %q has no tab", path, line)
		}
		ps := strings.Fields(pins)
		sort.Strings(ps)
		out[strings.Join(ps, " ")] = name
	}
	return out
}
