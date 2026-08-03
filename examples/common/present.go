package common

import (
	"fmt"
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// StatsLines renders the summary agni's `stats` command prints, as a string an example hands
// to a demokit step. It splits the counts into two groups so a reader can see at a glance what
// is format-neutral and what is not:
//
//   - "netlist": components and nets, the format-neutral identity. These are the same for the
//     same board read from any format (the diff engine proves it in the multi-format example).
//   - "format detail": structural (tier 1) and physical (tier 2) counts a reader populates only
//     when its format carries them (CONSTRAINTS C9). Zero rows are omitted, so each format
//     shows just what it carries and a bare netlist shows no physical rows at all.
func StatsLines(d *ir.Design) string {
	sectionsTotal, multi := 0, 0
	for _, c := range d.Components {
		sectionsTotal += len(c.Sections)
		if len(c.Sections) > 1 {
			multi++
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "design:              %s\n", d.Name)
	if d.SourceFormat != "" {
		fmt.Fprintf(&b, "source format:       %s\n", d.SourceFormat)
	}

	row := func(label string, n int, note string) string {
		l := label + ":"
		if note != "" {
			return fmt.Sprintf("  %-16s %d %s\n", l, n, note)
		}
		return fmt.Sprintf("  %-16s %d\n", l, n)
	}

	// Netlist identity: always shown, always the same for a given board across formats.
	b.WriteString("\nnetlist:\n")
	b.WriteString(row("components", len(d.Components), "(unique ref_des)"))
	b.WriteString(row("nets", len(d.Nets), ""))

	// Format detail: shown only for the counts this reader actually populated.
	type entry struct {
		label, note string
		n           int
	}
	detail := []entry{
		{"libraries", "", len(d.Libraries)},
		{"sections", "(source instances)", sectionsTotal},
		{"multi-section", "(one ref_des, several sections)", multi},
		{"footprints", "", len(d.Footprints)},
		{"layers", "", len(d.Layers)},
	}
	if d.Stackup != nil {
		detail = append(detail, entry{"stackup layers", "", len(d.Stackup.Layers)})
	}
	detail = append(detail, entry{"bom lines", "", len(d.Bom)})

	var shown strings.Builder
	for _, e := range detail {
		if e.n > 0 {
			shown.WriteString(row(e.label, e.n, e.note))
		}
	}
	if shown.Len() > 0 {
		b.WriteString("\nformat detail:\n")
		b.WriteString(shown.String())
	}
	return strings.TrimRight(b.String(), "\n")
}

// NetLines renders each net as "NAME: refdes.pin, refdes.pin, ..." for narration, capped at
// limit nets (0 = all). The (component ref_des, pin) pair is the stable cross-format key that
// diff and checks use, never a format-native id.
func NetLines(d *ir.Design, limit int) string {
	var b strings.Builder
	for i, n := range d.Nets {
		if limit > 0 && i >= limit {
			fmt.Fprintf(&b, "... and %d more net(s)\n", len(d.Nets)-limit)
			break
		}
		pins := make([]string, 0, len(n.Connections))
		for _, c := range n.Connections {
			pins = append(pins, c.ComponentRef+"."+c.PinRef)
		}
		fmt.Fprintf(&b, "%-10s %s\n", n.Name+":", strings.Join(pins, ", "))
	}
	return strings.TrimRight(b.String(), "\n")
}

// FindingsLines renders check.Run's output the way agni's `check` command does, as a string
// an example hands to a demokit step: a per-rule count summary, then each finding as
// "[severity] rule: subject (message)". Returns "no findings" for an empty slice. Findings
// keep check.Run's order (sorted by rule then subject); the summary is sorted by rule name so
// the output is stable regardless of map iteration order.
func FindingsLines(fs []check.Finding) string {
	if len(fs) == 0 {
		return "no findings"
	}
	byRule := map[string]int{}
	for _, f := range fs {
		byRule[f.Rule]++
	}
	rules := make([]string, 0, len(byRule))
	for r := range byRule {
		rules = append(rules, r)
	}
	sort.Strings(rules)

	var b strings.Builder
	b.WriteString("findings by rule:\n")
	for _, r := range rules {
		fmt.Fprintf(&b, "  %-22s %d\n", r, byRule[r])
	}
	b.WriteString("\n")
	for _, f := range fs {
		fmt.Fprintf(&b, "  [%s] %s: %s (%s)\n", f.Severity, f.Rule, f.Subject, f.Message)
	}
	fmt.Fprintf(&b, "\n%d finding(s) total", len(fs))
	return b.String()
}
