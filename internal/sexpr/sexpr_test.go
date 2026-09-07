package sexpr

import (
	"strings"
	"testing"
)

func parse(t *testing.T, src string, mode StringMode) *Node {
	t.Helper()
	n, err := Parse(strings.NewReader(src), mode)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return n
}

func TestStructure(t *testing.T) {
	n := parse(t, `(footprint "R_0805" (at 1 2 90) (pad "1" smd))`, KiCadStrings)
	if n.Head() != "footprint" {
		t.Errorf("Head = %q, want footprint", n.Head())
	}
	if got := n.Child("at").Arg(3).Text(); got != "90" {
		t.Errorf("at arg 3 = %q, want 90", got)
	}
	if pad := n.Child("pad"); pad == nil || pad.Arg(1).Text() != "1" || !pad.Arg(1).Quoted {
		t.Errorf("pad = %+v, want quoted arg \"1\"", pad)
	}
	var pads []*Node
	Collect(n, "pad", &pads)
	if len(pads) != 1 {
		t.Errorf("Collect pad = %d, want 1", len(pads))
	}
}

// TestKiCadStrings pins the KiCad dialect: backslash escapes resolve, \" does not terminate, and a
// literal newline inside a string is kept.
func TestKiCadStrings(t *testing.T) {
	n := parse(t, "(x \"a\\nb\\tc\\\"d\")", KiCadStrings)
	if got := n.Arg(1).Text(); got != "a\nb\tc\"d" {
		t.Errorf("KiCad string = %q, want a<nl>b<tab>c\"d", got)
	}
	if got := parse(t, "(x \"line1\nline2\")", KiCadStrings).Arg(1).Text(); got != "line1\nline2" {
		t.Errorf("KiCad keeps literal newline: got %q", got)
	}
}

// TestEDIFStrings pins the EDIF dialect: no escape processing (a backslash is literal), and CR/LF
// inside a string is dropped to rejoin a column-wrapped token (WS1-026).
func TestEDIFStrings(t *testing.T) {
	if got := parse(t, "(x \"a\\nb\")", EDIFStrings).Arg(1).Text(); got != `a\nb` {
		t.Errorf("EDIF string = %q, want a\\nb literal (no escape)", got)
	}
	if got := parse(t, "(x \"SCH\nEMATIC1\")", EDIFStrings).Arg(1).Text(); got != "SCHEMATIC1" {
		t.Errorf("EDIF drops CR/LF: got %q, want SCHEMATIC1", got)
	}
}

// TestEDIFPercentEscape pins the EDIF %<code(s)>% escape (spec: characters not directly
// representable). %10% is a newline, so a multi-line annotation string keeps its line breaks
// instead of leaking a literal "%10%"; multiple codes decode in order; a percent that does not
// form a valid escape stays literal (lossless). KiCad mode leaves '%' untouched.
func TestEDIFPercentEscape(t *testing.T) {
	cases := []struct{ src, want string }{
		{`(x "1: TOC%10%2: HDR")`, "1: TOC\n2: HDR"}, // %10% -> newline (the TOC bug)
		{`(x "%72 73%")`, "HI"},                      // multiple codes decode in order
		{`(x "%9%tab")`, "\ttab"},
		{`(x "50% done")`, "50% done"}, // lone '%' not an escape -> literal
		{`(x "%abc% x")`, "%abc% x"},   // non-numeric body -> literal, both percents kept
		{`(x "100%")`, "100%"},         // trailing '%' before the closing quote -> literal
	}
	for _, c := range cases {
		if got := parse(t, c.src, EDIFStrings).Arg(1).Text(); got != c.want {
			t.Errorf("EDIF %q = %q, want %q", c.src, got, c.want)
		}
	}
	// KiCad dialect does not use %..% escapes: a percent is an ordinary byte.
	if got := parse(t, `(x "50%10%")`, KiCadStrings).Arg(1).Text(); got != "50%10%" {
		t.Errorf("KiCad string = %q, want the percent literal", got)
	}
}

// TestParenInString: a '(' inside a quoted string is not a list start in either dialect.
func TestParenInString(t *testing.T) {
	n := parse(t, `(property "Resistance (Ohm)" (value 10))`, EDIFStrings)
	if n.Arg(1).Text() != "Resistance (Ohm)" || n.Child("value") == nil {
		t.Errorf("paren-in-string mis-parsed: %+v", n)
	}
}

// TestParseRejectsTruncation pins the property that made a 97% under-read look like a clean read
// (agni issue 562): a surplus ')' closes the top-level expression early, and everything after it
// used to be discarded in silence. The board that found this is a KiCad demo whose teardrop blocks
// are each written a '(' short, so the shape below is that file in miniature: the ')' meant to close
// `teardrops` closes `pad` instead, and the cascade reaches depth 0 while a whole second footprint
// is still unread.
func TestParseRejectsTruncation(t *testing.T) {
	// The intended structure with exactly one '(' removed, which is what the board does 349 times.
	const src = `(kicad_pcb
	(footprint "A"
		(pad "1"
			(teardrops
				(curved_edges no)
				filter_ratio 0.9)
				(enabled yes)
			)
			(uuid "p")
		)
		(uuid "u")
	)
	(footprint "B"
		(pad "1")
	)
)`
	n, err := Parse(strings.NewReader(src), KiCadStrings)
	if err == nil {
		var fps []*Node
		Collect(n, "footprint", &fps)
		t.Fatalf("Parse accepted a truncating input and returned %d of 2 footprints; want an error", len(fps))
	}
	if !strings.Contains(err.Error(), "line 13") {
		t.Errorf("error does not locate where the input was abandoned: %v", err)
	}
	if !strings.Contains(err.Error(), "unread") {
		t.Errorf("error does not say input was left unread: %v", err)
	}
}

// TestParseRejectsTrailingContent: two top-level expressions is the same defect without the paren
// cascade. One file is one expression in both dialects.
func TestParseRejectsTrailingContent(t *testing.T) {
	for _, mode := range []StringMode{KiCadStrings, EDIFStrings} {
		if _, err := Parse(strings.NewReader("(a 1)\n(b 2)\n"), mode); err == nil {
			t.Errorf("mode %v: Parse accepted trailing expression", mode)
		}
		// Trailing whitespace is not trailing content: a well-formed file ends with a newline.
		if _, err := Parse(strings.NewReader("(a 1)\n\n\t \r\n"), mode); err != nil {
			t.Errorf("mode %v: Parse rejected trailing whitespace: %v", mode, err)
		}
	}
}

// TestTokenizerLineCount: the position an error quotes has to survive the tokenizer's pushbacks
// (scanAtom unreads its terminator, which may be the newline) and the newlines held inside strings.
func TestTokenizerLineCount(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"atom terminated by newline", "(a\nbare\natoms\n)\n)", "line 5"},
		{"newline inside a string", "(a \"one\ntwo\"\n)\n)", "line 4"},
		{"escaped quote inside a string", "(a \"q\\\"q\"\n)\n)", "line 3"},
	}
	for _, c := range cases {
		_, err := Parse(strings.NewReader(c.src), KiCadStrings)
		if err == nil {
			t.Errorf("%s: want an error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want %s", c.name, err, c.want)
		}
	}
}
