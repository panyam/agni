package classify

import (
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// PartIndex indexes a design's part types by (library, part) name so a component section resolves to
// its PartType. It holds a qualified "library/part" key and a loose "/part" key, and the loose key
// matches when a section omits the library ref. It also holds a fallback alias on the part's native id
// (see below). Stamp and check.NewModel both use it, so both resolve parts identically.
func PartIndex(d *ir.Design) map[string]*ir.PartType {
	parts := map[string]*ir.PartType{}
	for _, lib := range d.GetLibraries() {
		for _, p := range lib.GetParts() {
			parts[lib.GetName()+"/"+p.GetName()] = p
			parts["/"+p.GetName()] = p
		}
	}
	// Fallback alias on the source's NATIVE ID (WS1-045). A section may reference its part by the id the
	// source uses, which can DIFFER from the PartType's display name. An EDIF cell `(rename ID "Display")`
	// is keyed above by Display while the instance's cellRef names the ID, and the two coincide only for
	// the OrCAD `(rename &<num> "<num>")` shape. Without this alias a cell whose Display differs from its
	// ID never resolves and its pins are silently dropped. The id is `&`-stripped to match the reader's
	// section-PartRef normalization, and GUARDED so a real display-name key always wins a collision.
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

// FirstPart walks a component's sections in order and returns the first PartType one of them resolves
// to in an index from PartIndex. It returns nil when no section resolves (a component with no known
// part type). The class derivation reads this part type for its name/kind and designator prefix.
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
// once at ingestion (WS3-071). It recomputes and overwrites the set, so a re-stamp is safe.
// formats.DeriveDesign sets its place in the pass order. It is the process-level form of
// (*Lexicon).Stamp (WS3-106).
func Stamp(d *ir.Design) { ActiveLexicon().Stamp(d) }

// MostSpecific picks the most-specific class from a device_classes set using the classifier's
// specificity order. A refined subtype like tvs beats its diode family tag. So a Model exposing a
// single component.class gives the same answer as the set widens with family tags (WS3-071). An empty
// set is ClassUnknown, and a class outside the token-hint priority (ClassIC) still resolves.
//
// It is the head of BySpecificity rather than its own walk. The model wants one answer and the drawing
// wants the list, and sharing the ranking means the two cannot rank a set differently (agni issue 710).
func MostSpecific(classes []string) ComponentClass {
	ranked := BySpecificity(classes)
	if len(ranked) == 0 {
		return ClassUnknown
	}
	return ComponentClass(ranked[0])
}

// BySpecificity orders a device_classes set most-specific first, dropping the unknown marker and the
// empty string. A ranked class comes before an unranked one, and two unranked classes keep the order
// the evidence tiers wrote. The convention tier stamps first, so a datasheet class missing from the
// table sorts behind the keyword-derived one rather than displacing it.
//
// When only the ingestion pass has written the set, it is already in this order, because ClassesOf
// writes the specific class and then its family. So this reorders nothing until a second evidence
// tier contributes.
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

// ClassNames returns a component's device-class names, dropping each tag's evidence. It is
// for a consumer that asks only what a component IS; a consumer weighing how well it is known reads
// the tags.
func ClassNames(c *ir.Component) []string {
	out := make([]string, 0, len(c.GetDeviceClasses()))
	for _, t := range c.GetDeviceClasses() {
		out = append(out, t.GetClass())
	}
	return out
}

// TagsOf expands a single derived class into device_classes TAGS attributed to one source. It returns
// the specific class plus its family tag, which is the ClassesOf set with its evidence attached.
func TagsOf(cl ComponentClass, src ir.ClassSource) []*ir.ComponentClassTag {
	names := ClassesOf(cl)
	out := make([]*ir.ComponentClassTag, 0, len(names))
	for _, n := range names {
		out = append(out, &ir.ComponentClassTag{Class: n, Source: src})
	}
	return out
}

// AddClassTag adds one class to a component's device_classes set, attributed to src. A class already
// present is not duplicated. When src is stronger, the tag's recorded source is UPGRADED, and the
// upgrade changes provenance and never membership. The empty and unknown classes are not facts and
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

// classFamily maps a specific class to its SUBTYPE family parent. A consumer checks for the parent's
// tag to test family membership. Only genuine "is-a" subtypes are listed. A TVS is-a diode, an LED
// is-a diode, and a ferrite bead is-a inductor. ClassTestConnector is DELIBERATELY absent, since it
// was split OUT of connector (WS3-066) so that protection rules quantifying over connector exclude a
// bench interface. Cross-family electrical groupings (passive, pass-element) are NOT families and are
// still Go predicates (isPassiveClass, passClass).
var classFamily = map[ComponentClass]ComponentClass{
	ClassTVS:     ClassDiode,
	ClassLED:     ClassDiode,
	ClassZener:   ClassDiode,
	ClassFerrite: ClassInductor,
	// A thermistor is a two-terminal resistor for every topological question and is not one for
	// anything temperature-related. ClassFerrite makes the same split against ClassInductor.
	ClassThermistor: ClassResistor,
	// Clock sources (WS10-015). The family is ClassClock and NOT ClassCrystal, because an oscillator
	// contains a crystal without being one, so HasClass(crystal) must be false for it. A family-level
	// rule quantifies over every clock source while a subtype rule branches (crystal-load-caps excludes
	// oscillator and resonator).
	ClassOscillator:       ClassClock,
	ClassCrystal:          ClassClock,
	ClassCeramicResonator: ClassClock,
	// An eFuse is a current-limiting switch (agni issue 1248). It keeps its own tag, since a datasheet
	// has said "efuse" since WS10-013 and a query may ask for it by that name.
	ClassEfuse: ClassCurrentLimitSwitch,
}

// ClassEfuse is the eFuse tag that comes from a datasheet's device_class. A project's config extends
// the component class vocabulary, and ClassEfuse is not in that vocabulary, because a project has no
// keyword that could recognise an eFuse. It exists here so an eFuse also gets ClassCurrentLimitSwitch.
const ClassEfuse ComponentClass = "efuse"

// ClassesOf expands a single derived class into its device_classes SET, the specific class plus its
// family tag. A consumer then tests membership (tvs in classes, any diode-family in classes) rather
// than equality. ClassUnknown yields the empty set, since "unknown" is the absence of a class fact,
// not a tag to record.
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
// set by hand, such as a test IR or a host that classifies a component itself. TagsOf is the derived
// form, which expands one class into its family; this is the literal one, and it adds no family tag.
func Tags(names ...string) []*ir.ComponentClassTag {
	out := make([]*ir.ComponentClassTag, 0, len(names))
	for _, n := range names {
		out = append(out, &ir.ComponentClassTag{Class: n, Source: ir.ClassSource_CLASS_SOURCE_CONVENTION})
	}
	return out
}

// HasClassTags reports whether any component in the design has a device-class tag. A consumer uses it
// to tell a classified design from a hand-authored IR that never went through the classify pass. It
// is a DESIGN-level question because a single component with no tags is an ordinary
// unclassified part, and only the absence across the whole design says the pass never ran.
func HasClassTags(d *ir.Design) bool {
	for _, c := range d.GetComponents() {
		if len(c.GetDeviceClasses()) > 0 {
			return true
		}
	}
	return false
}
