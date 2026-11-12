package render

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// text-markup.kicad_sch holds five labels in KiCad's markup and a part that picks an alternate
// function for pin 1. kicad-cli draws the alternate, UART_TX, where the library's GPIO1 would go,
// and text-markup.kicad-text.txt lists every text it draws (agni issue 1344).

func kicadTexts(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "readers", "kicad", "testdata", "text-markup.kicad-text.txt"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		if l := sc.Text(); l != "" && !strings.HasPrefix(l, "#") {
			out[l] = true
		}
	}
	return out
}

// TestPlacedAlternateDrawsAsKiCadDraws draws pin 1 by the alternate the part picks, in both
// backends, and pin 2 by its library name.
func TestPlacedAlternateDrawsAsKiCadDraws(t *testing.T) {
	want := kicadTexts(t)
	if !want["UART_TX"] || !want["GPIO2"] || want["GPIO1"] {
		t.Fatalf("the oracle lists %v; it should draw UART_TX and GPIO2 and not GPIO1", want)
	}
	g := readKiCadSheet(t, "text-markup.kicad_sch")
	svg := SheetSVG(g, g.Sheets[0])
	for _, name := range []string{"UART_TX", "GPIO2"} {
		if !strings.Contains(svg, ">"+name+"</text>") {
			t.Errorf("the SVG draws no %s", name)
		}
	}
	if strings.Contains(svg, ">GPIO1<") {
		t.Errorf("the SVG draws the library's GPIO1 for a pin the part gave UART_TX")
	}
	drawn := map[string]bool{}
	for _, l := range PackedLabels(PackSheet(g, g.Sheets[0])) {
		drawn[l.GetText()] = true
	}
	if !drawn["UART_TX"] || !drawn["GPIO2"] || drawn["GPIO1"] {
		t.Errorf("the overlay draws %v; want UART_TX and GPIO2, and not GPIO1", drawn)
	}
}

// TestKiCadMarkupDrawsInBothBackends draws each marked label as runs, with its markup gone, in the
// SVG and in the overlay's packed labels. kicad-cli's list keeps each label's markup as written,
// and every one of those labels must be drawn.
func TestKiCadMarkupDrawsInBothBackends(t *testing.T) {
	g := readKiCadSheet(t, "text-markup.kicad_sch")
	if g.GetTextMarkup() != "kicad" {
		t.Fatalf("text_markup = %q, want kicad", g.GetTextMarkup())
	}
	svg := SheetSVG(g, g.Sheets[0])
	for _, frag := range []string{
		`<tspan text-decoration="overline">RESET</tspan>`,
		`V<tspan baseline-shift="sub" font-size="70%">OUT</tspan>`,
		`I<tspan baseline-shift="super" font-size="70%">2</tspan>C`,
		`>SDA/MOSI</text>`,
		`SWD.<tspan text-decoration="overline">RST</tspan><tspan baseline-shift="sub" font-size="70%">n</tspan>`,
	} {
		if !strings.Contains(svg, frag) {
			t.Errorf("the SVG has no %s", frag)
		}
	}
	if strings.Contains(svg, "~{") || strings.Contains(svg, "{slash}") {
		t.Error("the SVG prints markup as text")
	}

	type packed struct {
		text string
		runs []geom.TextRun_Kind
	}
	got := map[string]packed{}
	for _, l := range PackedLabels(PackSheet(g, g.Sheets[0])) {
		var kinds []geom.TextRun_Kind
		for _, r := range l.GetRuns() {
			kinds = append(kinds, r.GetKind())
		}
		got[l.GetText()] = packed{l.GetText(), kinds}
	}
	o, s, u, p := geom.TextRun_KIND_OVERBAR, geom.TextRun_KIND_SUBSCRIPT, geom.TextRun_KIND_SUPERSCRIPT, geom.TextRun_KIND_PLAIN
	want := map[string][]geom.TextRun_Kind{
		"RESET":    {o},
		"VOUT":     {p, s},
		"I2C":      {p, u, p},
		"SDA/MOSI": {p},
		"SWD.RSTn": {p, o, s},
	}
	for text, kinds := range want {
		gp, ok := got[text]
		if !ok {
			t.Errorf("the overlay draws no %q", text)
			continue
		}
		if len(gp.runs) != len(kinds) {
			t.Errorf("%q packs runs %v, want %v", text, gp.runs, kinds)
			continue
		}
		for i := range kinds {
			if gp.runs[i] != kinds[i] {
				t.Errorf("%q packs runs %v, want %v", text, gp.runs, kinds)
				break
			}
		}
	}
	// Every label kicad-cli draws is one of these five.
	oracle := kicadTexts(t)
	for _, marked := range []string{"~{RESET}", "V_{OUT}", "I^{2}C", "SDA/MOSI", "SWD.~{RST}_{n}"} {
		if !oracle[marked] {
			t.Errorf("kicad-cli's list has no %q", marked)
		}
	}
}
