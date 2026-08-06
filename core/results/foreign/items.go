package foreign

import (
	"regexp"
	"strings"
)

// A foreign checker names the entity a violation is about in FREE TEXT. KiCad writes "Pad 1 [VCC] of
// R1 on B.Cu", "Track [GND] on F.Cu, length 1.0000 mm", "Symbol U1 Pin 1 [VIN, Power input, Line]".
// There is no structured ref_des or net field anywhere in its JSON, so attaching a violation to our
// model means parsing those strings.
//
// That makes this table the load-bearing part of the import, and it is kept as ONE table with a form
// matrix behind it, the same discipline the EDIF name grammar earned: a shape that is not in the table
// must fall through to the residue and be reported, never be half-matched into the wrong entity. A
// wrong join is worse than no join — it attaches a real violation to an innocent part.

// itemRef is what a description yielded. Any field may be empty; all empty means the description named
// no entity we can join to, which is a normal outcome rather than a failure (a wire's description
// carries only its orientation and length).
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

// itemPatterns is ordered: the first match wins, so a more specific shape must precede a more general
// one. "Symbol U1 Pin 1 [...]" has to be tried before "Symbol U1 [...]", and the pad shapes before the
// generic "<graphic> of <ref> on <layer>".
//
// Every entry here was derived from real kicad-cli output over the repo's board and schematic
// fixtures, not from the file-format documentation: the descriptions are UI strings with no stability
// guarantee, so evidence is the only honest source.
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
		// KiCad spells "this pad is on no net" as the literal net name "<no net>". Carrying it through
		// would invent a net by that name and join every unconnected pad on the board to it.
		if r.Net == "<no net>" || r.Net == "" {
			r.Net = ""
		}
		r.RefDes = strings.TrimSpace(r.RefDes)
		return r
	}
	return itemRef{}
}

// residueClass buckets an unjoinable description so the summary reports classes rather than a wall of
// one-off strings. It answers "what KIND of thing did we fail to attach", which is what tells a benign
// residue (board outline geometry has no entity to name) from a gap worth closing (a part-bearing
// shape the table does not know).
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
