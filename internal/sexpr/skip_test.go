package sexpr

import (
	"strings"
	"testing"
)

func heads(n *Node) []string {
	var out []string
	for _, k := range n.Kids {
		if k.IsList {
			out = append(out, k.Head())
		}
	}
	return out
}

// TestParseSkippingLeavesOutTheNamedLists holds ParseSkipping to dropping exactly the lists it is
// asked to, at any depth, and parsing everything else as Parse would (agni issue 946).
func TestParseSkippingLeavesOutTheNamedLists(t *testing.T) {
	src := `(board (zone (polygon (pts (xy 0 0))) (filled_polygon (layer "F.Cu") (pts (xy 1 1) (xy 2 2)))) (filled_polygon (pts)) (via (at 3 3)))`
	full, err := Parse(strings.NewReader(src), KiCadStrings)
	if err != nil {
		t.Fatal(err)
	}
	skipped, err := ParseSkipping(strings.NewReader(src), KiCadStrings, "filled_polygon")
	if err != nil {
		t.Fatal(err)
	}
	var all, left []*Node
	Collect(full, "filled_polygon", &all)
	Collect(skipped, "filled_polygon", &left)
	if len(all) != 2 || len(left) != 0 {
		t.Fatalf("filled_polygon lists: %d parsed in full, %d left after skipping, want 2 and 0", len(all), len(left))
	}
	if got := strings.Join(heads(skipped), " "); got != "zone via" {
		t.Errorf("top-level lists after skipping = %q, want %q", got, "zone via")
	}
	if got := strings.Join(heads(skipped.Child("zone")), " "); got != "polygon" {
		t.Errorf("a zone's lists after skipping = %q, want only its polygon", got)
	}
	// What is not skipped is the same tree Parse builds.
	if a, b := full.Child("via").Child("at").Arg(1).Text(), skipped.Child("via").Child("at").Arg(1).Text(); a != b || a != "3" {
		t.Errorf("an unskipped list reads %q skipped and %q in full", b, a)
	}
}

// TestParseSkippingHonoursStrings holds a skipped list's quoted strings to the tokenizer's rules, so
// a parenthesis inside one cannot end the skip early or late.
func TestParseSkippingHonoursStrings(t *testing.T) {
	src := `(board (filled_polygon (net_name "a)b(c") (note "))")) (via (at 1 2)))`
	n, err := ParseSkipping(strings.NewReader(src), KiCadStrings, "filled_polygon")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(heads(n), " "); got != "via" {
		t.Errorf("lists after skipping = %q, want %q", got, "via")
	}
}

// TestParseSkippingReportsAnUnclosedSkippedList holds a skipped list that never closes to the error
// Parse gives an unclosed list, rather than reading the rest of the file as skipped.
func TestParseSkippingReportsAnUnclosedSkippedList(t *testing.T) {
	_, err := ParseSkipping(strings.NewReader(`(board (filled_polygon (pts (xy 1 1))`), KiCadStrings, "filled_polygon")
	if err == nil || !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("err = %v, want an unexpected EOF", err)
	}
}

// TestParseSkippingSkipsOnlyAHead holds the skip to a list's head, so an atom spelled like a skipped
// head elsewhere in a list is kept.
func TestParseSkippingSkipsOnlyAHead(t *testing.T) {
	n, err := ParseSkipping(strings.NewReader(`(board (layer filled_polygon) (filled_polygon x))`), KiCadStrings, "filled_polygon")
	if err != nil {
		t.Fatal(err)
	}
	if got := n.Child("layer").Arg(1).Text(); got != "filled_polygon" {
		t.Errorf("an atom spelled like a skipped head reads %q, want it kept", got)
	}
	if len(n.Kids) != 2 {
		t.Errorf("board holds %d kids, want its head and the layer list", len(n.Kids))
	}
}
