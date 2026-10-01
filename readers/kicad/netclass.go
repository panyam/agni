package kicad

import (
	"encoding/json"
	"io"
	"math"
	"sort"
	"strconv"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// NetClassRules is the net-class assignment a KiCad project file carries in its net_settings, as name
// patterns plus explicit per-net assignments. This is the ONLY source that states a net's class, since
// neither the .kicad_sch nor the .kicad_pcb records membership, so ir.Net.net_classes is populated
// from here (WS1-037). A project with no rules leaves every net unclassed.
//
// BOTH SOURCES CONTRIBUTE. KiCad unions a net's explicit assignments with every matching pattern, so
// these two fields are inputs to one union rather than a lookup with a fallback (WS1-050).
type NetClassRules struct {
	assignments map[string][]string // exact net name -> classes (netclass_assignments)
	patterns    []netClassPattern   // every matching pattern applies (netclass_patterns)
}

type netClassPattern struct {
	class   string
	pattern string
}

// NetClassDef is one class's DECLARED routing constraints, the values the project says nets in this
// class are supposed to be routed at. Membership (who is in the class) is the other half, above.
//
// Priority orders the cascade when a net is in several classes, and LOWER WINS. KiCad sorts
// constituents ascending and pins the DEFAULT class at max-int, so it fills only what no other class
// supplied. The cascade is PER FIELD, not per class. A net in a high-priority class that declares only
// a clearance still takes its track width from the next class down, so this type carries per-field
// presence rather than plain zero values.
//
// Absent and zero are different. A class that declares no track width leaves the field for a
// lower-priority class to fill, where a class declaring 0 states a (nonsensical) value. KiCad writes
// each key only when the class HasX(), so absence is expressible in the source and has to survive the
// read.
type NetClassDef struct {
	Name string
	// IsDefault marks the class KiCad applies to EVERY net, not only nets assigned to it. It is a
	// magic NAME in the format ("Default"), so the reader recognizes it rather than the core, because
	// identifying it is format knowledge (C9 left-shift).
	//
	// It fills any constraint the net's own classes left unstated, and a net in no class at all takes
	// its values outright. A cascade that only walked a net's own memberships would under-report on
	// both counts.
	IsDefault bool
	Priority  int
	// Millimetres, and only the PCB scalars, which KiCad writes via pcbIUScale.IUTomm(). The
	// schematic ones (wire_width, bus_width) are MILS and are left out.
	Clearance   *float64
	TrackWidth  *float64
	ViaDiameter *float64
	ViaDrill    *float64
}

// ParseNetClasses reads net_settings.netclass_{assignments,patterns} from a KiCad .kicad_pro
// (JSON). A malformed or netclass-free project yields empty rules, never an error, because net-class
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
// rule matches. Membership is a UNION, matching KiCad's own resolution. The explicit assignment does
// NOT short-circuit pattern matching, and pattern matching does not stop at the first hit, so a net
// named by an assignment and matched by two patterns belongs to all three classes.
//
// Sorted for determinism only, since the assignment map is a Go map and a set has no source order.
// This is NOT KiCad's precedence order, which is a per-class `priority` on the class DEFINITIONS
// (WS3-111). Precedence decides which class's clearance or track width wins, never who is a member,
// so nothing here reads it.
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

// ParseNetClassDefs reads net_settings.classes[] from a KiCad .kicad_pro. These are the per-class
// routing constraints saying what a class's nets are SUPPOSED to be routed at, the declared half of
// the declared-vs-actual comparison (WS3-111). As with ParseNetClasses, a malformed or class-free
// project yields nothing rather than failing the read.
//
// The default class arrives in this same array, named "Default", with its priority forced to max-int
// so it cascades last. It is FLAGGED rather than merely ranked, because KiCad applies it to every net,
// filling whatever the net's own classes left unstated, and hands it to a net with no classes outright.
func ParseNetClassDefs(r io.Reader) []NetClassDef {
	var doc struct {
		NetSettings struct {
			Classes []struct {
				Name        string   `json:"name"`
				Priority    *int     `json:"priority"`
				Clearance   *float64 `json:"clearance"`
				TrackWidth  *float64 `json:"track_width"`
				ViaDiameter *float64 `json:"via_diameter"`
				ViaDrill    *float64 `json:"via_drill"`
			} `json:"classes"`
		} `json:"net_settings"`
	}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil
	}
	var out []NetClassDef
	for _, c := range doc.NetSettings.Classes {
		if c.Name == "" {
			continue
		}
		d := NetClassDef{
			Name:        c.Name,
			IsDefault:   c.Name == defaultClassName,
			Priority:    defaultPriority,
			Clearance:   c.Clearance,
			TrackWidth:  c.TrackWidth,
			ViaDiameter: c.ViaDiameter,
			ViaDrill:    c.ViaDrill,
		}
		if c.Priority != nil {
			d.Priority = *c.Priority
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// defaultClassName is the magic class name KiCad gives the fallback net class. Every net inherits
// its unstated constraints, whether or not the net is assigned to it.
const defaultClassName = "Default"

// defaultPriority puts a class with no stated priority last in the cascade, beside the Default class.
// It matches the max-int KiCad pins Default at, so an unstated priority never silently outranks a
// class that actually declared one.
const defaultPriority = math.MaxInt32

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

// ConstraintKindNetClass is the ir.Constraint.kind tag for a net-class definition. The IR node is a
// thin carrier (name + kind + params, "the rules DSL owns the semantics"), so the class name rides
// `name`, the scalars ride `params`, and any consumer keys off this kind rather than guessing from the
// shape of the params map.
const ConstraintKindNetClass = "netclass"

// AnnotateNetClassDefs stamps a KiCad project's per-class routing constraints onto ir.Design as
// Constraint nodes (WS3-111), the first reader to populate that node.
//
// Params are strings because ir.Constraint says they are. The IR carries the declaration and the query
// layer parses and compares. Only stated scalars are written, so a consumer can tell "this class
// declares no track width" (the field cascades to a lower-priority class) from "this class declares
// zero".
func AnnotateNetClassDefs(d *ir.Design, defs []NetClassDef) {
	if d == nil || len(defs) == 0 {
		return
	}
	for _, def := range defs {
		params := map[string]string{"priority": strconv.Itoa(def.Priority)}
		if def.IsDefault {
			params["is_default"] = "true"
		}
		for key, v := range map[string]*float64{
			"clearance":    def.Clearance,
			"track_width":  def.TrackWidth,
			"via_diameter": def.ViaDiameter,
			"via_drill":    def.ViaDrill,
		} {
			if v != nil {
				params[key] = strconv.FormatFloat(*v, 'g', -1, 64)
			}
		}
		d.Constraints = append(d.Constraints, &ir.Constraint{
			Name:   def.Name,
			Kind:   ConstraintKindNetClass,
			Params: params,
		})
	}
}

// wildcardMatch reports whether the whole of s matches a KiCad net-class pattern, where * is
// any run (including empty) and ? is exactly one character. Unlike path.Match, * crosses '/',
// because KiCad net names embed sheet paths and a pattern like "/PWR/*" is meant to span them.
// Classic two-pointer glob with backtracking, byte-wise (net names are ASCII).
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
