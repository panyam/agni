package kicad

import (
	"encoding/json"
	"io"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// NetClassRules is the net-class assignment a KiCad project file carries in its
// net_settings: ordered name patterns plus (legacy) explicit net-name assignments. This is
// the ONLY source that states a net's class — neither the .kicad_sch nor the .kicad_pcb
// records membership — so ir.Net.net_class is populated from here (WS1-037). A project with
// no rules leaves every net_class empty, the prior behavior.
type NetClassRules struct {
	assignments map[string]string // exact net name -> class (legacy netclass_assignments)
	patterns    []netClassPattern // ordered; first match wins (netclass_patterns)
}

type netClassPattern struct {
	class   string
	pattern string
}

// ParseNetClasses reads net_settings.netclass_{assignments,patterns} from a KiCad .kicad_pro
// (JSON). A malformed or netclass-free project yields empty rules, never an error: net-class
// is advisory metadata whose absence must not fail a project read. netclass_assignments is
// decoded loosely (its value may be a bare class string or a one-element array across KiCad
// versions) so a shape variation there never discards the patterns.
func ParseNetClasses(r io.Reader) *NetClassRules {
	rules := &NetClassRules{}
	var doc struct {
		NetSettings struct {
			Assignments map[string]json.RawMessage `json:"netclass_assignments"`
			Patterns    []struct {
				NetClass string `json:"netclass"`
				Pattern  string `json:"pattern"`
			} `json:"netclass_patterns"`
		} `json:"net_settings"`
	}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return rules
	}
	for name, raw := range doc.NetSettings.Assignments {
		if c := firstClass(raw); c != "" {
			if rules.assignments == nil {
				rules.assignments = map[string]string{}
			}
			rules.assignments[name] = c
		}
	}
	for _, p := range doc.NetSettings.Patterns {
		if p.Pattern != "" && p.NetClass != "" {
			rules.patterns = append(rules.patterns, netClassPattern{class: p.NetClass, pattern: p.Pattern})
		}
	}
	return rules
}

// firstClass coerces a netclass_assignments value to a class name, accepting both the bare
// string form ("HS") and the one-element array form (["HS"]) different KiCad versions emit.
func firstClass(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		return arr[0]
	}
	return ""
}

// ClassOf returns the net class assigned to netName, or "" when no rule matches. An exact
// assignment wins; otherwise the first pattern in file order whose wildcard matches the whole
// name wins, matching KiCad's ordered netclass_patterns evaluation.
func (r *NetClassRules) ClassOf(netName string) string {
	if r == nil {
		return ""
	}
	if c, ok := r.assignments[netName]; ok {
		return c
	}
	for _, p := range r.patterns {
		if wildcardMatch(p.pattern, netName) {
			return p.class
		}
	}
	return ""
}

// AnnotateNetClasses stamps ir.Net.net_class from a KiCad project's net-class rules, leaving a
// net with no matching rule untouched. It runs after the project merge because the rules live
// in the .kicad_pro, not in the schematic or board file the readers consume.
func AnnotateNetClasses(d *ir.Design, rules *NetClassRules) {
	if d == nil || rules == nil {
		return
	}
	for _, n := range d.Nets {
		if c := rules.ClassOf(n.Name); c != "" {
			n.NetClass = c
		}
	}
}

// wildcardMatch reports whether the whole of s matches a KiCad net-class pattern, where * is
// any run (including empty) and ? is exactly one character. Unlike path.Match there is no
// separator * cannot cross: KiCad net names embed '/' (sheet paths), and a pattern like
// "/PWR/*" is meant to span it. Classic two-pointer glob with backtracking, byte-wise (net
// names are ASCII).
func wildcardMatch(pattern, s string) bool {
	pi, si := 0, 0
	star, mark := -1, 0
	for si < len(s) {
		switch {
		case pi < len(pattern) && (pattern[pi] == '?' || pattern[pi] == s[si]):
			pi++
			si++
		case pi < len(pattern) && pattern[pi] == '*':
			star, mark = pi, si
			pi++
		case star != -1:
			pi = star + 1
			mark++
			si = mark
		default:
			return false
		}
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}
