package classify

import (
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// PartIndex indexes a design's part types by (library, part) name so a component section resolves to
// its PartType. It carries both the qualified "library/part" key and a loose "/part" key that matches
// when a section omits the library ref, PLUS a fallback alias on the part's native id (see below). It is
// shared by Stamp and check.NewModel so both resolve parts identically.
func PartIndex(d *ir.Design) map[string]*ir.PartType {
	parts := map[string]*ir.PartType{}
	for _, lib := range d.GetLibraries() {
		for _, p := range lib.GetParts() {
			parts[lib.GetName()+"/"+p.GetName()] = p
			parts["/"+p.GetName()] = p
		}
	}
	// Fallback alias on the source's NATIVE ID (WS1-045): a section may reference its part by the id the
	// source uses, which can DIFFER from the PartType's display name. An EDIF cell `(rename ID "Display")`
	// is keyed above by Display, but the instance's cellRef names the ID — and the two coincide only for
	// the OrCAD `(rename &<num> "<num>")` shape the reader's `&`-strip was built for; a cell whose Display
	// differs from its ID (a real oscillator cell: `MC2016Z50.0000C1ZYSH` vs id `MC2016Z500560000C1ZYSH`)
	// never resolved, so the part's pins were silently dropped. Add the `&`-stripped id (matching the
	// reader's section-PartRef normalization) as a fallback, GUARDED so a real display-name key always
	// wins a collision. Harmless for formats whose native id is empty or equals the name (a no-op).
	for _, lib := range d.GetLibraries() {
		for _, p := range lib.GetParts() {
			id := strings.TrimPrefix(p.GetProv().GetNativeId(), "&")
			if id == "" || id == p.GetName() {
				continue
			}
			for _, k := range []string{lib.GetName() + "/" + id, "/" + id} {
				if _, ok := parts[k]; !ok {
					parts[k] = p
				}
			}
		}
	}
	return parts
}

// FirstPart resolves the first PartType any of a component's sections references, using an index from
// PartIndex; nil when no section resolves (a component with no known part type). It is the part-type
// signal the class derivation reads (name/kind, designator prefix).
func FirstPart(index map[string]*ir.PartType, c *ir.Component) *ir.PartType {
	for _, s := range c.GetSections() {
		if p := index[s.GetLibraryRef()+"/"+s.GetPartRef()]; p != nil {
			return p
		}
		if p := index["/"+s.GetPartRef()]; p != nil {
			return p
		}
	}
	return nil
}

// Stamp runs the classification pass over a read design, filling each component's device_classes SET
// once at ingestion (WS3-071). The loader calls it after readers finish, so every format is classified
// by the same cross-format conventions and check reads a normalized data fact. Idempotent: it recomputes
// and overwrites the set, so a re-stamp after a re-read is safe.
// It classifies against the PROCESS-level lexicon; a read that carries its own conventions calls
// (*Lexicon).Stamp instead, so two designs in one process can be stamped differently (WS3-106).
func Stamp(d *ir.Design) { ActiveLexicon().Stamp(d) }

// MostSpecific picks the most-specific class from a device_classes set using the classifier's
// specificity order (a refined subtype like tvs beats its diode family tag), so a Model exposing a
// single component.class stays stable as the set widens with family tags (WS3-071). An empty set is
// ClassUnknown, and a class outside the token-hint priority (ClassIC) still resolves.
//
// It is the head of BySpecificity rather than its own walk, so nothing can rank a set one way for a
// consumer that wants one answer and another way for a consumer that wants the list. The drawing and
// the model each want one of those, and they used to disagree (agni issue 710).
func MostSpecific(classes []string) ComponentClass {
	ranked := BySpecificity(classes)
	if len(ranked) == 0 {
		return ClassUnknown
	}
	return ComponentClass(ranked[0])
}

// BySpecificity orders a device_classes set most-specific first, dropping the unknown marker and the
// empty string. A class the specificity table ranks comes before one it does not, and two unranked
// classes keep the order the set had, which is the order the evidence tiers wrote them in: the
// convention tier stamps first, so a datasheet class the table does not know stays behind the
// keyword-derived one rather than displacing it.
//
// The set the ingestion pass alone produces is already in this order (ClassesOf writes the specific
// class then its family), so this reorders nothing until a second evidence tier contributes.
func BySpecificity(classes []string) []string {
	rank := func(c string) int {
		for i, cl := range hintPriority {
			if ComponentClass(c) == cl {
				return i
			}
		}
		return len(hintPriority)
	}
	out := make([]string, 0, len(classes))
	for _, c := range classes {
		if c != "" && ComponentClass(c) != ClassUnknown {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i]) < rank(out[j]) })
	return out
}

// ClassNames returns a component's device-class names, dropping the evidence each tag carries. It is
// for a consumer that asks only what a component IS; a consumer weighing how well it is known reads
// the tags.
func ClassNames(c *ir.Component) []string {
	out := make([]string, 0, len(c.GetDeviceClasses()))
	for _, t := range c.GetDeviceClasses() {
		out = append(out, t.GetClass())
	}
	return out
}

// TagsOf expands a single derived class into device_classes TAGS attributed to one source: the
// specific class plus its family tag, the ClassesOf set with its evidence attached.
func TagsOf(cl ComponentClass, src ir.ClassSource) []*ir.ComponentClassTag {
	names := ClassesOf(cl)
	out := make([]*ir.ComponentClassTag, 0, len(names))
	for _, n := range names {
		out = append(out, &ir.ComponentClassTag{Class: n, Source: src})
	}
	return out
}

// AddClassTag adds one class to a component's device_classes set, attributed to src. A class already
// present is not duplicated; its recorded source is UPGRADED when src is stronger, which is a note
// about provenance and never a change to membership. The empty and unknown classes are not facts and
// are dropped. It is the component twin of AddNetRole, with the same additive-only contract.
func AddClassTag(c *ir.Component, class string, src ir.ClassSource) {
	if class == "" || ComponentClass(class) == ClassUnknown {
		return
	}
	for _, t := range c.GetDeviceClasses() {
		if t.GetClass() == class {
			if src > t.GetSource() {
				t.Source = src
			}
			return
		}
	}
	c.DeviceClasses = append(c.DeviceClasses, &ir.ComponentClassTag{Class: class, Source: src})
}

// classFamily maps a specific class to its SUBTYPE family parent, the tag a consumer checks for
// family membership. Only genuine "is-a" subtypes are listed: a TVS is-a diode, an LED is-a diode, a
// ferrite bead is-a inductor. ClassTestConnector is DELIBERATELY absent — it was split OUT of connector
// (WS3-066) precisely so protection rules that quantify over connector exclude a bench interface, so it
// carries no connector family tag. Cross-family electrical groupings (passive, pass-element) are NOT
// families and stay Go predicates (isPassiveClass, passClass); they are not single-tag memberships.
var classFamily = map[ComponentClass]ComponentClass{
	ClassTVS:     ClassDiode,
	ClassLED:     ClassDiode,
	ClassZener:   ClassDiode,
	ClassFerrite: ClassInductor,
	// A thermistor is a two-terminal resistor for every topological question and is not one for
	// anything temperature-related, which is the split ClassFerrite makes against ClassInductor.
	ClassThermistor: ClassResistor,
	// Clock sources (WS10-015). The family is ClassClock, deliberately NOT ClassCrystal: an oscillator
	// is-NOT-a crystal (it contains one), so a family-level clock rule must not read HasClass(crystal)
	// true for it. All three carry the clock family tag so a family-level rule quantifies over every
	// clock source while a subtype-specific rule branches (crystal-load-caps excludes oscillator/resonator).
	ClassOscillator:       ClassClock,
	ClassCrystal:          ClassClock,
	ClassCeramicResonator: ClassClock,
}

// ClassesOf expands a single derived class into its device_classes SET: the specific class plus its
// family tag, so a consumer does membership (tvs in classes, any diode-family in classes) rather than
// equality. ClassUnknown yields the empty set, since "unknown" is the absence of a class fact, not a
// tag to carry.
func ClassesOf(cl ComponentClass) []string {
	if cl == ClassUnknown {
		return nil
	}
	out := []string{string(cl)}
	if fam, ok := classFamily[cl]; ok {
		out = append(out, string(fam))
	}
	return out
}

// Tags builds CONVENTION-tier tags from class names verbatim, for a caller writing a device_classes
// set by hand: a test IR, or a host that classifies a component itself. TagsOf is the derived form,
// which expands one class into its family; this is the literal one, and it adds no family tag.
func Tags(names ...string) []*ir.ComponentClassTag {
	out := make([]*ir.ComponentClassTag, 0, len(names))
	for _, n := range names {
		out = append(out, &ir.ComponentClassTag{Class: n, Source: ir.ClassSource_CLASS_SOURCE_CONVENTION})
	}
	return out
}

// HasClassTags reports whether any component in the design carries a device-class tag, which is how
// a consumer tells a design the classify pass has seen from a hand-authored IR that never went
// through it. It is a DESIGN-level question on purpose: a single component with no tags is an
// ordinary unclassified part, and only the absence across the whole design says the pass never ran.
func HasClassTags(d *ir.Design) bool {
	for _, c := range d.GetComponents() {
		if len(c.GetDeviceClasses()) > 0 {
			return true
		}
	}
	return false
}
