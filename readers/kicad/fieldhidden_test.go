package kicad

import (
	"strings"
	"testing"
)

// A symbol field is hidden in each spelling KiCad has written. KiCad 6 and 7 write a bare hide
// atom inside effects, KiCad 8 and later write (hide yes). Reading only the list form drew every
// datasheet URL and footprint name on Glasgow revC3, a KiCad 7 schematic, across its sheets.
func TestFieldHiddenReadsEverySpelling(t *testing.T) {
	for _, c := range []struct {
		effects string
		hidden  bool
	}{
		{`(effects (font (size 1.27 1.27)) hide)`, true},
		{`(effects (font (size 1.27 1.27)) (hide yes))`, true},
		{`(effects (font (size 1.27 1.27)) (hide no))`, false},
		{`(effects (font (size 1.27 1.27)))`, false},
	} {
		root, err := parse(strings.NewReader(`(property "Datasheet" "x" (at 0 0 0) ` + c.effects + `)`))
		if err != nil {
			t.Fatal(err)
		}
		if got := fieldHidden(root); got != c.hidden {
			t.Errorf("%s: hidden = %v, want %v", c.effects, got, c.hidden)
		}
	}
}
