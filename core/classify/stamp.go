package classify

import (
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
func MostSpecific(classes []string) ComponentClass {
	set := map[ComponentClass]bool{}
	for _, c := range classes {
		set[ComponentClass(c)] = true
	}
	for _, cl := range hintPriority {
		if set[cl] {
			return cl
		}
	}
	for _, c := range classes {
		if ComponentClass(c) != ClassUnknown {
			return ComponentClass(c)
		}
	}
	return ClassUnknown
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
