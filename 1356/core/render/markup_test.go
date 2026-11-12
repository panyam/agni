package render

import (
	"fmt"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

func runsString(runs []textRun) string {
	if runs == nil {
		return "<plain>"
	}
	var parts []string
	for _, r := range runs {
		k := map[geom.TextRun_Kind]string{
			geom.TextRun_KIND_PLAIN: "", geom.TextRun_KIND_OVERBAR: "~",
			geom.TextRun_KIND_SUBSCRIPT: "_", geom.TextRun_KIND_SUPERSCRIPT: "^",
		}[r.kind]
		parts = append(parts, fmt.Sprintf("%s%q", k, r.text))
	}
	return strings.Join(parts, " ")
}

// TestKiCadMarkupSplitsIntoRuns reads KiCad's text markup, the forms its schematics write on Jetson
// (agni issue 1344).
func TestKiCadMarkupSplitsIntoRuns(t *testing.T) {
	cases := map[string]string{
		"~{RESET}":       `~"RESET"`,
		"V_{OUT}":        `"V" _"OUT"`,
		"I^{2}C":         `"I" ^"2" "C"`,
		"SDA{slash}MOSI": `"SDA/MOSI"`,
		"SWD.~{RST}_{n}": `"SWD." ~"RST" _"n"`,
		"~{CS{slash}SS}": `~"CS/SS"`,
		"PLAIN_NAME":     `<plain>`,
		"BUS{0..3}":      `<plain>`,
		"~{OPEN":         `<plain>`,
		"{unknown}":      `<plain>`,
		"A~{}B":          `"A" ~"" "B"`,
	}
	for in, want := range cases {
		if got := runsString(markupRuns("kicad", in)); got != want {
			t.Errorf("markupRuns(%q) = %s, want %s", in, got, want)
		}
	}
	// Another format's text prints as written, braces and all.
	if got := markupRuns("", "~{RESET}"); got != nil {
		t.Errorf("markupRuns with no markup = %s, want nil", runsString(got))
	}
}
