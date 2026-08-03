// Package intake produces a SANITIZED, deterministic summary of a design — the factual skeleton the
// /design-intake onboarding workflow builds on (WS3-091). Its confidentiality guarantee is STRUCTURAL:
// the Skeleton type has no field that can hold a net name or a net-to-net connection, so an intake
// summary cannot leak the confidential parts of a design (net names, topology, layout). It carries only
// counts, device classes, part identities (the AVL/BOM view), anomaly kinds + ref-des, and nominal
// voltages. That flips the C16 boundary from "the agent chose not to paste a net name" to "the tool has
// no field to express one." The /design-intake skill runs this first, then layers judgment on top.
package intake

import (
	"fmt"
	"sort"
	"strings"

	"github.com/panyam/agni/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Skeleton is the sanitized, deterministic intake summary. Every field is a COUNT, a device CLASS, a
// part identity (ref-des / MPN / manufacturer / value), an anomaly kind + ref-des, or a nominal VOLTAGE.
// There is deliberately no field for a net name, a connection, or any topology (CONSTRAINTS C16); the
// boundary is enforced by this type, not by discipline. Same design + model => same Skeleton.
type Skeleton struct {
	Components    int            `json:"components"`
	Sections      int            `json:"sections"`
	Nets          int            `json:"nets"`
	ClassCount    map[string]int `json:"class_count"`              // device class (incl. family tags, so a TVS counts in both tvs and diode) -> ref-des count
	Unclassified  int            `json:"unclassified"`             // components the classifier could not place
	PartTypes     []PartTypeRow  `json:"part_types"`               // BOM grouped by (mpn, mfr, value, class), by count desc — the default view
	Parts         []PartRow      `json:"parts,omitempty"`          // per-component AVL, sorted by ref-des; emitted only with --parts full (else omitted to keep the summary small)
	RailNominals  []float64      `json:"rail_nominals"`            // distinct name-derived rail nominals; the NET NAMES are dropped here
	Anomalies     []Anomaly      `json:"anomalies,omitempty"`      // read/design issues, by kind; never a net name
	DatasheetGaps []string       `json:"datasheet_gaps,omitempty"` // MPNs on the board with no seeded PartSpec (needs a params tier)
	HasParams     bool           `json:"has_params"`               // whether MPN / datasheet-gap columns are populated
}

// PartRow is the AVL/BOM view of one component — part identity only, all safe to cross the boundary.
// MPN is empty unless a datasheet params tier is attached to the model.
type PartRow struct {
	RefDes       string `json:"ref_des"`
	MPN          string `json:"mpn,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Value        string `json:"value,omitempty"`
	Class        string `json:"class,omitempty"`
}

// PartTypeRow is one distinct part type in the BOM — a (MPN, manufacturer, value, class) tuple and how
// many components share it. Collapses jellybean passives (1000+ identical Murata caps into one row) while
// leaving significant parts (distinct MPNs) one per row; a manufacturer-name variant ("Murata" vs
// "MURATA") surfaces as two rows, which is the intended AVL-hygiene signal.
type PartTypeRow struct {
	Count        int    `json:"count"`
	MPN          string `json:"mpn,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Value        string `json:"value,omitempty"`
	Class        string `json:"class,omitempty"`
}

// Anomaly is a detected read/design issue, reported as a kind + count + the ref-des it touches. It
// carries NO net name by construction: a pin-net conflict names the pin's COMPONENT, not the nets.
type Anomaly struct {
	Kind   string   `json:"kind"`
	Count  int      `json:"count"`
	RefDes []string `json:"ref_des,omitempty"`
}

// Build computes the sanitized intake Skeleton from a loaded model. Counts come from the model's
// classified component set (mirroring the component.class relation); rail nominals from the name-derived
// net.nominal_voltage fact, projected to the VOLTAGE only so the net name never enters the result.
func Build(m check.Model) *Skeleton {
	s := &Skeleton{ClassCount: map[string]int{}, HasParams: m.HasParams()}
	for _, c := range m.Components() {
		s.Components++
		s.Sections += len(c.GetSections())
		classes := m.Classes(c.RefDes)
		if len(classes) == 0 {
			s.Unclassified++
		}
		for _, cl := range classes {
			s.ClassCount[string(cl)]++
		}
		mpn := m.ComponentMPN(c.RefDes)
		s.Parts = append(s.Parts, PartRow{
			RefDes:       c.RefDes,
			MPN:          mpn,
			Manufacturer: attr(c, "Manufacturer", "manufacturer"),
			Value:        attr(c, "Value", "value"),
			Class:        string(m.ComponentClass(c.RefDes)),
		})
		if m.HasParams() && mpn != "" && m.PartSpec(c.RefDes) == nil {
			s.DatasheetGaps = append(s.DatasheetGaps, mpn)
		}
	}
	s.Nets = len(m.Nets())
	sort.Slice(s.Parts, func(i, j int) bool { return s.Parts[i].RefDes < s.Parts[j].RefDes })

	// BOM by distinct part TYPE: collapse the per-component rows by (mpn, mfr, value, class). Jellybean
	// passives (1000+ identical caps) become one row; distinct-MPN parts stay one per row; a manufacturer
	// spelling variant becomes a separate row (the AVL-hygiene signal). Sorted by count desc.
	typeIdx := map[[4]string]int{}
	for _, p := range s.Parts {
		key := [4]string{p.MPN, p.Manufacturer, p.Value, p.Class}
		if i, ok := typeIdx[key]; ok {
			s.PartTypes[i].Count++
		} else {
			typeIdx[key] = len(s.PartTypes)
			s.PartTypes = append(s.PartTypes, PartTypeRow{Count: 1, MPN: p.MPN, Manufacturer: p.Manufacturer, Value: p.Value, Class: p.Class})
		}
	}
	sort.Slice(s.PartTypes, func(i, j int) bool {
		a, b := s.PartTypes[i], s.PartTypes[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Class != b.Class {
			return a.Class < b.Class
		}
		if a.MPN != b.MPN {
			return a.MPN < b.MPN
		}
		return a.Manufacturer < b.Manufacturer
	})

	// Rail nominals: the name-derived nominal of each rail net, kept as the VOLTAGE only (Num), distinct.
	seen := map[float64]bool{}
	for _, f := range check.Facts(m) {
		if f.Relation == check.RelNetNominalVoltage && f.Num != nil && !seen[*f.Num] {
			seen[*f.Num] = true
			s.RailNominals = append(s.RailNominals, *f.Num)
		}
	}
	sort.Float64s(s.RailNominals)

	// Anomalies — kind + count + ref-des, never a net name.
	if col := collisionRefDes(m); len(col) > 0 {
		s.Anomalies = append(s.Anomalies, Anomaly{Kind: "ref_des_collision", Count: len(col), RefDes: col})
	}
	if pnc := m.PinNetConflicts(); len(pnc) > 0 {
		seenRef := map[string]bool{}
		var refs []string
		for _, p := range pnc {
			if !seenRef[p.RefDes] {
				seenRef[p.RefDes] = true
				refs = append(refs, p.RefDes)
			}
		}
		sort.Strings(refs)
		s.Anomalies = append(s.Anomalies, Anomaly{Kind: "pin_net_conflict", Count: len(pnc), RefDes: refs})
	}
	if b := m.UnmodeledBuses(); len(b) > 0 {
		s.Anomalies = append(s.Anomalies, Anomaly{Kind: "unmodeled_bus", Count: len(b)})
	}
	sort.Strings(s.DatasheetGaps)
	return s
}

func attr(c *ir.Component, keys ...string) string {
	for _, k := range keys {
		if v := c.GetAttributes()[k]; v != "" {
			return v
		}
	}
	return ""
}

func collisionRefDes(m check.Model) []string {
	var out []string
	for _, c := range m.RefDesCollisions() {
		out = append(out, c.GetRefDes())
	}
	sort.Strings(out)
	return out
}

// Markdown renders the Skeleton as the intake.md sections. Deterministic and sanitized-by-construction:
// it can only print what the Skeleton holds, and the Skeleton holds no net name. full selects the parts
// view: false (default) prints the BOM by distinct type, true prints the per-component AVL.
func Markdown(s *Skeleton, full bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Aggregates\n- Components: %d | Sections: %d | Nets: %d\n\n", s.Components, s.Sections, s.Nets)

	fmt.Fprintf(&b, "## Class summary (query-derived)\n| Class | Ref count |\n|-------|-----------|\n")
	type cc struct {
		name string
		n    int
	}
	var cs []cc
	for k, v := range s.ClassCount {
		cs = append(cs, cc{k, v})
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].n != cs[j].n {
			return cs[i].n > cs[j].n
		}
		return cs[i].name < cs[j].name
	})
	for _, c := range cs {
		fmt.Fprintf(&b, "| %s | %d |\n", c.name, c.n)
	}
	fmt.Fprintf(&b, "| unclassified | %d |\n\n", s.Unclassified)

	fmt.Fprintf(&b, "## Rails (nominal only; net names withheld)\n")
	for _, v := range s.RailNominals {
		fmt.Fprintf(&b, "- %gV\n", v)
	}
	b.WriteString("\n")

	if len(s.Anomalies) > 0 {
		fmt.Fprintf(&b, "## Anomalies\n")
		for _, a := range s.Anomalies {
			fmt.Fprintf(&b, "- %s: %d", a.Kind, a.Count)
			if len(a.RefDes) > 0 {
				fmt.Fprintf(&b, " (%s)", strings.Join(a.RefDes, ", "))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(s.DatasheetGaps) > 0 {
		fmt.Fprintf(&b, "## Datasheet gaps (MPN on board, no seeded spec)\n")
		for _, mpn := range s.DatasheetGaps {
			fmt.Fprintf(&b, "- %s\n", mpn)
		}
		b.WriteString("\n")
	}

	if full {
		fmt.Fprintf(&b, "## Parts (AVL / BOM view, per-component)\n| Ref | MPN | Manufacturer | Value | Class |\n|-----|-----|--------------|-------|-------|\n")
		for _, p := range s.Parts {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", p.RefDes, p.MPN, p.Manufacturer, p.Value, p.Class)
		}
	} else {
		fmt.Fprintf(&b, "## Parts (BOM by distinct type; --parts full for the per-component AVL)\n| Count | MPN | Manufacturer | Value | Class |\n|-------|-----|--------------|-------|-------|\n")
		for _, t := range s.PartTypes {
			fmt.Fprintf(&b, "| %d | %s | %s | %s | %s |\n", t.Count, t.MPN, t.Manufacturer, t.Value, t.Class)
		}
	}
	return b.String()
}
