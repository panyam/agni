package kicad

import (
	"strings"
	"testing"
)

// A symbol field is hidden in each spelling KiCad has written. KiCad 6 and 7 write a bare hide
// atom inside effects, KiCad 8 and 9 write (hide yes) there, and KiCad 10 writes (hide yes) in the
// property itself, beside effects. Reading only the list form drew every datasheet URL and footprint
// name on Glasgow revC3, a KiCad 7 schematic, across its sheets. Reading only effects did the same
// on Jetson rev 1.1.3, a KiCad 10 schematic (agni issue 1560). A property whose value is the word
// hide is not hidden by it.
func TestFieldHiddenReadsEverySpelling(t *testing.T) {
	for _, c := range []struct {
		effects string
		hidden  bool
	}{
		{`(effects (font (size 1.27 1.27)) hide)`, true},
		{`(effects (font (size 1.27 1.27)) (hide yes))`, true},
		{`(effects (font (size 1.27 1.27)) (hide no))`, false},
		{`(effects (font (size 1.27 1.27)))`, false},
		{`(hide yes) (effects (font (size 1.27 1.27)))`, true},
		{`(hide no) (effects (font (size 1.27 1.27)))`, false},
	} {
		root, err := parse(strings.NewReader(`(property "Datasheet" "x" (at 0 0 0) ` + c.effects + `)`))
		if err != nil {
			t.Fatal(err)
		}
		if got := fieldHidden(root); got != c.hidden {
			t.Errorf("%s: hidden = %v, want %v", c.effects, got, c.hidden)
		}
	}
	root, err := parse(strings.NewReader(`(property "Note" hide (at 0 0 0) (effects (font (size 1.27 1.27))))`))
	if err != nil {
		t.Fatal(err)
	}
	if fieldHidden(root) {
		t.Error("a property whose value is the word hide reads as hidden")
	}
}
