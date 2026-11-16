package formats

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAllegro166HeightsMatchItsODX holds each parasdr part's height, read from its place-bound
// outline in the 16.6 board, to the HEIGHT the ODX file Allegro exported from the board states for it
// (agni 1643). A part the ODX gives no height must have none either.
func TestAllegro166HeightsMatchItsODX(t *testing.T) {
	brd, odx := parasdr+"parasdr_layout.brd", parasdr+"parasdr_odb++.xml"
	if _, err := os.Stat(brd); err != nil {
		t.Fatalf("parasdr's board is missing; run `make samples-oracle`: %v", err)
	}
	l := &Loader{}
	bg, err := l.BoardGeometry(brd)
	if err != nil {
		t.Fatal(err)
	}
	og, err := l.BoardGeometry(odx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]*int64{}
	for _, pl := range og.GetPlacements() {
		want[pl.GetRefDes()] = pl.Height
	}
	held := 0
	for _, pl := range bg.GetPlacements() {
		w, ok := want[pl.GetRefDes()]
		if !ok {
			continue
		}
		switch {
		case w == nil && pl.Height == nil:
		case w == nil || pl.Height == nil || abs(pl.GetHeight()-*w) > 100:
			t.Errorf("%s: %v nm tall, the ODX says %v", pl.GetRefDes(), heightOf(pl.Height), heightOf(w))
		default:
			held++
		}
	}
	if held < 280 {
		t.Errorf("%d parts held to the ODX's height, want parasdr's 284", held)
	}
}

// TestAllegro172HeightsAreParts holds the BeagleBone AI's 17.2 heights to what a part can be, since
// its extract states none: dozens of parts with a height, each above zero and under 20 mm.
func TestAllegro172HeightsAreParts(t *testing.T) {
	g, err := (&Loader{}).BoardGeometry(filepath.Join(allegroBoards, "beaglebone-ai", "BeagleBone-AI.brd"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, pl := range g.GetPlacements() {
		if pl.Height == nil {
			continue
		}
		n++
		if h := pl.GetHeight(); h <= 0 || h > 20_000_000 {
			t.Errorf("%s: %d nm tall", pl.GetRefDes(), h)
		}
	}
	if n < 50 {
		t.Errorf("%d parts state a height, want the BeagleBone's 71", n)
	}
}

func heightOf(h *int64) any {
	if h == nil {
		return "none"
	}
	return *h
}
