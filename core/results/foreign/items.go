package foreign

import (
	"regexp"
	"strings"
)

// A foreign checker names the entity a violation is about in FREE TEXT. KiCad writes "Pad 1 [VCC] of
// R1 on B.Cu" or "Symbol U1 Pin 1 [VIN, Power input, Line]", with no structured ref_des or net field
// in its JSON, so attaching a violation to our model means parsing those strings.
//
// This is ONE table with a form matrix behind it. A shape that is not in the table must fall through
// to the residue and be reported, never be half-matched, because a wrong join attaches a real
// violation to an innocent part. See
// docsite/content/architecture/checks-contract.md#importing-another-tools-results.

// itemRef is what a description yielded. Any field may be empty; RefDes and Net both empty means no
// joinable entity, a normal outcome (a wire's description carries only orientation and length).
type itemRef struct {
	RefDes string
	Pin    string
	Net    string
}

func (r itemRef) empty() bool { return r.RefDes == "" && r.Net == "" }

// itemPattern is one description shape. The named capture groups are the join keys; a group the shape
// does not have is simply absent.
type itemPattern struct {
	name string
	re   *regexp.Regexp
}

// itemPatterns is ordered and the first match wins, so a more specific shape must precede a more
// general one. "Symbol U1 Pin 1 [...]" has to be tried before "Symbol U1 [...]", and the pad shapes before the
// generic "<graphic> of <ref> on <layer>".
//
// Every entry was derived from real kicad-cli output over the repo's board and schematic fixtures,
// not from documentation, since the descriptions are UI strings with no stability guarantee.
var itemPatterns = []itemPattern{
	// Board: pads. Two spellings, one with a layer suffix and one (the pad-stack forms) without.
	{"pad", regexp.MustCompile(`^Pad (?P<pin>\S+) \[(?P<net>[^\]]*)\] of (?P<ref>\S+) on `)},
	{"pad", regexp.MustCompile(`^(?:PTH|NPTH|SMD) pad (?P<pin>\S+) \[(?P<net>[^\]]*)\] of (?P<ref>\S+)`)},
	// Board: copper carrying a net but belonging to no part.
	{"track", regexp.MustCompile(`^(?:Track|Arc) \[(?P<net>[^\]]*)\] on `)},
	{"via", regexp.MustCompile(`^Via \[(?P<net>[^\]]*)\]`)},
	{"zone", regexp.MustCompile(`^Zone \[(?P<net>[^\]]*)\]`)},
	{"footprint", regexp.MustCompile(`^Footprint (?P<ref>\S+)`)},
	// Schematic.
	{"symbol-pin", regexp.MustCompile(`^Symbol (?P<ref>\S+) Pin (?P<pin>\S+) \[`)},
	{"symbol", regexp.MustCompile(`^Symbol (?P<ref>\S+) \[`)},
	{"label", regexp.MustCompile(`^(?:Local |Global |Hierarchical )?Label '(?P<net>[^']+)'`)},
	// Fields and footprint graphics both name their owning part; they are last so the shapes above
	// that carry a pin or a net are preferred.
	{"field", regexp.MustCompile(`^\S+ field of (?P<ref>\S+)$`)},
	{"graphic", regexp.MustCompile(`^\S+ of (?P<ref>\S+) on `)},
}

// parseItem extracts the join keys from one item description, returning an empty ref when no shape
// matches. The caller reports what it could not parse; this never guesses.
func parseItem(desc string) itemRef {
	for _, p := range itemPatterns {
		m := p.re.FindStringSubmatch(desc)
		if m == nil {
			continue
		}
		var r itemRef
		for i, g := range p.re.SubexpNames() {
			switch g {
			case "ref":
				r.RefDes = m[i]
			case "pin":
				r.Pin = m[i]
			case "net":
				r.Net = m[i]
			}
		}
		// KiCad spells "on no net" as the literal "<no net>", which would otherwise join every
		// unconnected pad to one invented net.
		if r.Net == "<no net>" || r.Net == "" {
			r.Net = ""
		}
		r.RefDes = strings.TrimSpace(r.RefDes)
		return r
	}
	return itemRef{}
}

// residueClass buckets an unjoinable description so the summary reports classes rather than one-off
// strings, separating benign residue (board outline geometry) from a gap worth closing (a
// part-bearing shape the table does not know).
func residueClass(desc string) string {
	switch {
	case strings.Contains(desc, " Wire,"), strings.HasPrefix(desc, "Wire"):
		return "a schematic wire, whose description carries only orientation and length"
	case strings.Contains(desc, "Edge.Cuts"):
		return "board outline geometry, which belongs to no component or net"
	case strings.Contains(desc, "Silkscreen"), strings.Contains(desc, "Fab"), strings.Contains(desc, "Courtyard"):
		return "free board graphics or text, which belong to no component or net"
	case desc == "":
		return "a violation the source reported with no items at all"
	default:
		return "an entity shape the import does not recognize"
	}
}
