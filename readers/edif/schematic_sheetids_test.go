package edif

import (
	"os"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

func readSheets(t *testing.T, path string) []*geom.SheetGeometry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := ReadSchematic(f, path)
	if err != nil {
		t.Fatal(err)
	}
	return g.GetSheets()
}

// agni issue 1140: an export whose block cells all name their page (rename &1 "1") read as four
// sheets sharing one id and one name.
func TestRepeatedPageRenamesGetUniqueSheets(t *testing.T) {
	sheets := readSheets(t, "testdata/aggregator_dup_pages.eds")
	if len(sheets) != 4 {
		t.Fatalf("read %d sheets, want 4", len(sheets))
	}
	ids, names := map[string]bool{}, map[string]bool{}
	got := map[string][2]string{}
	for _, s := range sheets {
		ids[s.GetId()] = true
		names[s.GetName()] = true
		if len(s.GetPlacements()) != 1 {
			t.Fatalf("sheet %q holds %d placements, want 1", s.GetId(), len(s.GetPlacements()))
		}
		got[s.GetPlacements()[0].GetRefDes()] = [2]string{s.GetId(), s.GetName()}
	}
	if len(ids) != 4 || len(names) != 4 {
		t.Errorf("%d distinct ids and %d distinct names over 4 sheets, want 4 of each", len(ids), len(names))
	}
	want := map[string][2]string{
		"R1": {"&block_a/&v_a/&1", "BlockA / 1"},
		"C1": {"&block_b/&v_b/&1", "BlockB / 1"},
		"U1": {"&top/&v_top/&1", "Top / 1"},
		"J1": {"TopDesign/&1", "TopDesign / 1"},
	}
	for ref, w := range want {
		if got[ref] != w {
			t.Errorf("the sheet holding %s is (id %q, name %q), want (%q, %q)", ref, got[ref][0], got[ref][1], w[0], w[1])
		}
	}
}

// A design whose pages already have their own renames keeps them exactly as the file spells them.
func TestDistinctPageRenamesAreLeftAlone(t *testing.T) {
	var got [][2]string
	for _, s := range readSheets(t, "testdata/multisheet.eds") {
		got = append(got, [2]string{s.GetId(), s.GetName()})
	}
	want := [][2]string{{"P1", "Sheet 1"}, {"P2", "Sheet 2"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("sheets %v, want %v", got, want)
	}
}

// Two pages one owner names alike still get ids of their own, numbered in file order.
func TestAPageRenameRepeatedInOneOwnerIsNumbered(t *testing.T) {
	sheets := []*geom.SheetGeometry{{Id: "&1", Name: "1"}, {Id: "&1", Name: "1"}}
	pages := []*node{{}, {}}
	owner := pageOwner{id: "&c/&v", name: "C"}
	qualifyRepeatedSheets(sheets, pages, map[*node]pageOwner{pages[0]: owner, pages[1]: owner})
	if sheets[0].GetId() != "&c/&v/&1" || sheets[1].GetId() != "&c/&v/&1~2" {
		t.Errorf("ids %q and %q, want &c/&v/&1 and &c/&v/&1~2", sheets[0].GetId(), sheets[1].GetId())
	}
}
