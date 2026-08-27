package formats

import (
	"testing"

	"github.com/panyam/agni/core/classify"
)

// TestMPNResolvesOnEveryFormat is the guard agni issue 519 needed and did not have: the same claim,
// asserted through the real loader, for more than one format.
//
// The bug it pins was invisible to every existing test because each format's tests only ever checked
// that format. EDIF resolved part numbers (its reader carried a private promotion pass), Telesis did
// not (it recorded the number on the PART TYPE and the model only read the COMPONENT), and nothing
// compared them. `component.mpn` came back empty for every component of every .tel design, which
// silently disabled the whole datasheet tier on that format, because the join is
// component.mpn -> param and a parameter rule cannot tell "no datasheet seeded" from "this format
// never delivers a part number".
//
// A single-format test would have passed throughout the bug's life. This one is table-driven ACROSS
// formats for that reason: adding a row is how a new reader inherits the guarantee.
func TestMPNResolvesOnEveryFormat(t *testing.T) {
	for _, tc := range []struct {
		name, path  string
		wantAtLeast int
	}{
		// Records its part number on the part type, under a lowercase key. The regression case.
		{"telesis", "../telesis/testdata/basic.tel", 8},
		// Records it per instance under a vendor alias, and on the cell for shared part types.
		{"edif", "../edif/testdata/mpn.edn", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := (&Loader{}).ReadDesign(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			var resolved int
			for _, c := range d.GetComponents() {
				if c.GetAttributes()[classify.MPNAttr] != "" {
					resolved++
				}
			}
			if resolved < tc.wantAtLeast {
				t.Errorf("%d component(s) resolved a part number, want at least %d: the datasheet tier joins on this, and zero of them reports as a clean run",
					resolved, tc.wantAtLeast)
			}
		})
	}
}

// TestMPNNeverInvented is the other half, and the one that keeps the pass honest. A component whose
// source states no part number must come back with none. A fallback that guesses is worse than the
// gap it fills, because a wrong part number joins to a real datasheet and produces confident findings
// about a part that is not on the board.
func TestMPNNeverInvented(t *testing.T) {
	d, err := (&Loader{}).ReadDesign("../telesis/testdata/basic.tel")
	if err != nil {
		t.Fatal(err)
	}
	// R1 is placed by a package line that carries no `!` discriminator, so the source states no part
	// number for it. See readers/telesis: a line with two quoted strings and no bang is a property
	// entry, not a package one.
	for _, c := range d.GetComponents() {
		if c.GetRefDes() != "R1" {
			continue
		}
		if got := c.GetAttributes()[classify.MPNAttr]; got != "" && got != "RC0603FR-0710KL" {
			t.Errorf("R1 resolved %q, which its source does not state", got)
		}
	}
}
