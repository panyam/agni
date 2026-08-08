package kicad

import (
	"encoding/json"
	"io"
	"sort"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// NetClassRules is the net-class assignment a KiCad project file carries in its net_settings: name
// patterns plus explicit per-net assignments. This is the ONLY source that states a net's class —
// neither the .kicad_sch nor the .kicad_pcb records membership — so ir.Net.net_classes is populated
// from here (WS1-037). A project with no rules leaves every net unclassed.
//
// BOTH SOURCES CONTRIBUTE, and neither is exclusive. KiCad resolves a net's membership by unioning
// its explicit assignments with every matching pattern into a set, so these two fields are inputs to
// one union, not a lookup with a fallback (WS1-050).
type NetClassRules struct {
	assignments map[string][]string // exact net name -> classes (netclass_assignments)
	patterns    []netClassPattern   // every matching pattern applies (netclass_patterns)
}

type netClassPattern struct {
	class   string
	pattern string
}

// ParseNetClasses reads net_settings.netclass_{assignments,patterns} from a KiCad .kicad_pro
// (JSON). A malformed or netclass-free project yields empty rules, never an error: net-class
// is advisory metadata whose absence must not fail a project read. netclass_assignments values are
// decoded as json.RawMessage because the shape varies (an array of classes today, a bare string in
// older projects), and a shape variation on one net must never discard the patterns.
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
		if cs := assignedClasses(raw); len(cs) > 0 {
			if rules.assignments == nil {
				rules.assignments = map[string][]string{}
			}
			rules.assignments[name] = cs
		}
	}
	for _, p := range doc.NetSettings.Patterns {
		if p.Pattern != "" && p.NetClass != "" {
			rules.patterns = append(rules.patterns, netClassPattern{class: p.NetClass, pattern: p.Pattern})
		}
	}
	return rules
}

// assignedClasses coerces a netclass_assignments value to its class names. KiCad writes an ARRAY
// per net (`m_netClassLabelAssignments` is a `map<netname, set<netclass>>`, serialized by iterating
// the set), so the array form is the multi-class form, not a version quirk. The bare-string form is
// accepted because older projects carry it.
func assignedClasses(raw json.RawMessage) []string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if s == "" {
			return nil
		}
		return []string{s}
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil {
		return arr
	}
	return nil
}

// ClassesOf returns every net class netName belongs to, sorted and deduplicated, or nil when no
// rule matches. Membership is a UNION, matching KiCad's own resolution: the explicit assignment
// does NOT short-circuit pattern matching, and pattern matching does not stop at the first hit, so
// a net named by an assignment and matched by two patterns belongs to all three classes.
//
// Sorted for determinism only. The assignment map is a Go map, so read order is random, and a
// stable IR is worth more than a source order a set never had anyway. This is NOT KiCad's
// precedence order, which is a per-class `priority` living with the class DEFINITIONS (WS3-111);
// nothing here reads those, and nothing here needs to, because precedence only decides which
// class's clearance or track width wins, never who is a member.
func (r *NetClassRules) ClassesOf(netName string) []string {
	if r == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, c := range r.assignments[netName] {
		seen[c] = true
	}
	for _, p := range r.patterns {
		if wildcardMatch(p.pattern, netName) {
			seen[p.class] = true
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// AnnotateNetClasses stamps ir.Net.net_classes from a KiCad project's net-class rules, leaving a
// net with no matching rule untouched. It runs after the project merge because the rules live
// in the .kicad_pro, not in the schematic or board file the readers consume.
func AnnotateNetClasses(d *ir.Design, rules *NetClassRules) {
	if d == nil || rules == nil {
		return
	}
	for _, n := range d.Nets {
		if cs := rules.ClassesOf(n.Name); len(cs) > 0 {
			n.NetClasses = cs
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
