package diff

import (
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// comp builds an ir.Component with one section (library/part) and a Value attribute.
func comp(ref, lib, part, value string) *ir.Component {
	c := &ir.Component{RefDes: ref, Sections: []*ir.ComponentSection{{LibraryRef: lib, PartRef: part}}}
	if value != "" {
		c.Attributes = map[string]string{"Value": value}
	}
	return c
}

// renderPair is the report every Render test reads: one change of every component and net
// kind, produced through the public Designs path rather than a hand-assembled Report.
func renderPair() *Report {
	a := &ir.Design{
		Components: []*ir.Component{
			comp("R1", "lib", "res", "10k"), // value change -> ComponentsChanged
			comp("R9", "lib", "res", ""),    // removed
		},
		Nets: []*ir.Net{
			net("VCC", "old", "R1.1", "U1.1"),     // gains a pin -> hard
			net("SIG_OLD", "old", "U1.2", "U2.2"), // renamed
			net("DEAD", "old", "R9.1", "R9.2"),    // deleted
			net("CLK", "old", "U1.5", "U2.5"),     // net_class change -> soft
		},
	}
	a.Nets[3].NetClass = "signal"
	b := &ir.Design{
		Components: []*ir.Component{
			comp("R1", "lib", "res", "22k"),
			comp("C7", "lib", "cap", ""), // added
		},
		Nets: []*ir.Net{
			net("VCC", "new", "R1.1", "U1.1", "C7.1"),
			net("SIG_NEW", "new", "U1.2", "U2.2"),
			net("FRESH", "new", "C7.2", "U1.9"), // new
			net("CLK", "new", "U1.5", "U2.5"),
		},
	}
	b.Nets[3].NetClass = "power"
	return Designs(a, b)
}

func TestRenderSummaryAndSections(t *testing.T) {
	out := renderPair().Render(0)

	for _, want := range []string{
		"Components: +1  -1  ~1",
		"Nets:       new 1  deleted 1  renamed 1  hard 1  soft 1",
		"Components added (1):",
		"\n  C7\n",
		"Components removed (1):",
		"\n  R9\n",
		"Components changed (1):",
		`R1 Value: "10k" -> "22k"`,
		"Nets changed (5):",
		"[renamed] SIG_OLD -> SIG_NEW",
		"[hard]    VCC: +[C7.1] -[]",
		"[soft]    CLK: attributes changed",
		"[new]     FRESH",
		"[deleted] DEAD",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Render missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderLimitTruncates(t *testing.T) {
	var a, b ir.Design
	for _, ref := range []string{"C1", "C2", "C3"} {
		b.Components = append(b.Components, comp(ref, "lib", "cap", ""))
	}
	out := Designs(&a, &b).Render(2)
	if !strings.Contains(out, "  C1\n") || !strings.Contains(out, "  C2\n") {
		t.Errorf("limit 2 must list the first two items, got:\n%s", out)
	}
	if strings.Contains(out, "  C3\n") {
		t.Errorf("limit 2 must not list the third item, got:\n%s", out)
	}
	if !strings.Contains(out, "... and 1 more") {
		t.Errorf("truncation must be called out, got:\n%s", out)
	}

	// A non-positive limit lists everything with no truncation note.
	full := Designs(&a, &b).Render(-1)
	if !strings.Contains(full, "  C3\n") || strings.Contains(full, "more") {
		t.Errorf("limit<=0 must list all items with no note, got:\n%s", full)
	}
}

func TestRenderEmptyDiffIsJustCounts(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{net("GND", "x", "R1.1", "R1.2")}}
	out := Designs(d, d).Render(10)
	want := "Components: +0  -0  ~0\nNets:       new 0  deleted 0  renamed 0  hard 0  soft 0\n"
	if out != want {
		t.Errorf("empty diff = %q, want just the count header %q", out, want)
	}
}
