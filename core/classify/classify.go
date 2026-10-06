package classify

import (
	"regexp"
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// prefixClasses is the built-in ref-des prefix table, the one DefaultClassVocab starts from and a
// project's conventions add to. "X" is absent because it means crystal in some house styles and
// terminal block in others, so it stays UNKNOWN unless part data or a project's prefixes resolve it.
var prefixClasses = map[string]ComponentClass{
	"R":    ClassResistor,
	"RN":   ClassResistor,
	"RT":   ClassThermistor,
	"C":    ClassCapacitor,
	"L":    ClassInductor,
	"FB":   ClassFerrite,
	"FER":  ClassFerrite,
	"D":    ClassDiode,
	"CR":   ClassDiode,
	"LED":  ClassLED,
	"TVS":  ClassTVS,
	"F":    ClassFuse,
	"FU":   ClassFuse,
	"J":    ClassConnector,
	"P":    ClassConnector,
	"CN":   ClassConnector,
	"CON":  ClassConnector,
	"TP":   ClassTestPoint,
	"Y":    ClassClock,
	"XTAL": ClassClock,
	"OSC":  ClassClock,
	"U":    ClassIC,
	"IC":   ClassIC,
	"Q":    ClassTransistor,
}

// tokenClasses maps a word appearing in part-type text (name, kind, Value attribute) to a
// class. Matching is on whole tokens, not substrings, so "shielded" does not read as an LED.
var tokenClasses = map[string]ComponentClass{
	"resistor":   ClassResistor,
	"capacitor":  ClassCapacitor,
	"inductor":   ClassInductor,
	"ferrite":    ClassFerrite,
	"bead":       ClassFerrite,
	"thermistor": ClassThermistor,
	"ntc":        ClassThermistor,
	"ptc":        ClassThermistor,
	"diode":      ClassDiode,
	"led":        ClassLED,
	"tvs":        ClassTVS,
	"esd":        ClassTVS,
	"zener":      ClassZener,
	"fuse":       ClassFuse,
	"connector":  ClassConnector,
	"conn":       ClassConnector,
	// A debug / test / edge-card / programming connector is a bench interface, refined out of the
	// connector base so protection rules skip it (WS3-066).
	"debug":       ClassTestConnector,
	"debugger":    ClassTestConnector,
	"jtag":        ClassTestConnector,
	"swd":         ClassTestConnector,
	"edge":        ClassTestConnector,
	"programming": ClassTestConnector,
	"programmer":  ClassTestConnector,
	"testpoint":   ClassTestPoint,
	// Clock sources (WS10-015). ALL clock tokens, "oscillator" included, mark CLOCK-FAMILY candidacy
	// and never a subtype. On real EDIF a whole vendor library is named "Oscillator" (its DXDB_LIBNAME
	// rides every crystal AND resonator in it) and the per-part "Oscillator Type?" label is swapped in
	// the field, so a token could only mis-subtype, and a wrong "oscillator" tag would HIDE a real
	// missing-load-cap finding on a crystal. The subtype comes from STRUCTURE (hasSupplyPin) or a
	// seeded datasheet device_class. "ceramic" is absent because it collides with ceramic capacitors.
	"crystal":    ClassClock,
	"xtal":       ClassClock,
	"resonator":  ClassClock,
	"oscillator": ClassClock,
}

// tvsFamilies are part-number families that are ESD or transient-voltage suppressors whatever their
// symbol says (agni issue 934). A board's ESD arrays often sit under an IC symbol (TPD4E05U06 as U16)
// and its TVS diodes under a plain diode with only the part number in their properties (PESD5Z5.0F),
// so neither the ref-des prefix nor a "tvs" word reaches them. A family is matched at the START of a
// token of the part's text, which includes its MPN property.
//
// Only a family match promotes an IC. The bare words "tvs" and "esd" keep refining a diode alone,
// because an IC's free-text description mentioning "ESD protection" says nothing about what it is.
var tvsFamilies = regexp.MustCompile(`^(tpd\d+e|pesd|esda|esd\d|usblc|rclamp|prtr|sp05|sm712|smaj|smbj|smcj|p6ke)`)

// isTVSFamily reports whether any token names a known suppressor family.
func isTVSFamily(tokens []string) bool {
	for _, t := range tokens {
		if tvsFamilies.MatchString(t) {
			return true
		}
	}
	return false
}

// Classify derives the component.class fact. The ref-des prefix convention gives the base
// class, and part-type data (the part's designator_prefix, then whole-token hints in its
// name/kind and the component's Value attribute) overrides or refines it. A token hint may
// refine a base class only within the same device family (diode -> led/tvs, inductor ->
// ferrite), so a resistor named "PULLUP_LED_EN" stays a resistor; with no base class the
// token decides outright. It is the pure, single-class derivation Stamp and check.Model share.
func Classify(c *ir.Component, pt *ir.PartType) ComponentClass {
	return ActiveLexicon().Classify(c, pt)
}

// Classify is the per-read form, the same derivation against THIS lexicon's vocabularies rather than
// the process globals (WS3-106).
func (l *Lexicon) Classify(c *ir.Component, pt *ir.PartType) ComponentClass {
	prefix := refDesPrefix(c.GetRefDes())
	// A declared prefix can arrive in placeholder form ("C?", "REF**", as the Mentor EDIF corpus
	// prints it), so the placeholder tail is trimmed before the lookup. Untrimmed, every part-typed
	// component on that corpus classified unknown. Placeholders are in
	// docsite/content/architecture/ingestion-and-ir.md#placeholder-designators.
	if p := strings.TrimRight(strings.ToUpper(pt.GetDesignatorPrefix()), "?*"); p != "" {
		prefix = p
	}
	base := l.class().ClassForPrefix(prefix)

	// Collect the SET of hints (WS3-070), not the first, so the generic "diode" in a "Tvs Diode"
	// description cannot shadow the "tvs" refinement whichever order they tokenize in.
	tokens := classTokens(pt, c)
	hints := l.class().HintsFor(tokens)

	// A suppressor's part number decides it on a diode, an IC or an unknown part.
	if (base == ClassDiode || base == ClassIC || base == ClassUnknown || base == "") && isTVSFamily(tokens) {
		return ClassTVS
	}

	// Clock family (WS10-015), scoped to clock candidates so the supply-pin signal never promotes an
	// arbitrary powered IC such as an MCU. A supply pin marks an ACTIVE oscillator. Without one the part
	// stays at the family, since tokens are family-only (see tokenClasses), and its subtype comes from a
	// seeded datasheet device_class (StampClassesFromSpecs).
	if base == ClassClock || hints[ClassClock] {
		if l.hasSupplyPin(pt) {
			return ClassOscillator
		}
		return ClassClock
	}

	switch base {
	case ClassDiode:
		if hints[ClassTVS] {
			return ClassTVS
		}
		if hints[ClassLED] {
			return ClassLED
		}
		if hints[ClassZener] {
			return ClassZener
		}
	case ClassConnector:
		if hints[ClassTestConnector] {
			return ClassTestConnector
		}
	case ClassInductor:
		if hints[ClassFerrite] {
			return ClassFerrite
		}
	case ClassUnknown, "":
		if cl := resolveHint(hints); cl != ClassUnknown {
			return cl
		}
		return ClassUnknown
	}
	return base
}

// hasSupplyPin reports whether a part type declares a power-supply pin by NAME (Vcc/Vdd/Vin/...), the
// structural mark of an ACTIVE clock oscillator, since a passive crystal or ceramic resonator has only
// signal terminals. It reads NAMES through the supply-pin lexicon and NOT the POWER_IN direction
// (WS10-015), because Stamp runs BEFORE StampPowerInPins and EDIF under-types a supply pin as INPUT.
// A part type with no declared pins yields false.
func (l *Lexicon) hasSupplyPin(pt *ir.PartType) bool {
	for _, p := range pt.GetPins() {
		if l.role().IsSupplyPin(p.GetName()) {
			return true
		}
	}
	return false
}

// hintPriority resolves a set of token hints on an UNKNOWN-prefix part to one class, most-specific
// first, so "Tvs Diode" on an unfamiliar prefix reads TVS rather than the generic diode.
var hintPriority = []ComponentClass{
	ClassTVS, ClassLED, ClassZener, ClassFerrite,
	// Clock subtypes rank above the ClassClock family (a subtype is more specific); the family ranks
	// above the leaf classes so a bare clock candidate resolves to clock, not further down the list.
	ClassOscillator, ClassCrystal, ClassCeramicResonator, ClassClock,
	ClassTestPoint, ClassTestConnector, ClassConnector,
	ClassFuse, ClassTransistor, ClassDiode, ClassThermistor, ClassResistor, ClassCapacitor, ClassInductor,
}

func resolveHint(hints map[ComponentClass]bool) ComponentClass {
	for _, cl := range hintPriority {
		if hints[cl] {
			return cl
		}
	}
	return ClassUnknown
}

// classTokens tokenizes the part name and kind plus EVERY component attribute value, because a
// part's TVS/ESD identity often lives in a Description, Part Label or library-name attribute rather
// than the Value (WS3-065). Attributes are read in sorted-key order so tokenization is deterministic.
// Tokens split on non-alphanumerics and on camelCase boundaries ("FerriteBead" -> ferrite, bead) and
// come back lowercased.
func classTokens(pt *ir.PartType, c *ir.Component) []string {
	parts := []string{pt.GetName(), pt.GetKind()}
	attrs := c.GetAttributes()
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, attrs[k])
	}
	text := strings.Join(parts, " ")
	var toks []string
	start := -1
	flush := func(end int) {
		if start >= 0 && end > start {
			toks = append(toks, strings.ToLower(text[start:end]))
		}
		start = -1
	}
	for i, r := range text {
		alnum := ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9')
		if !alnum {
			flush(i)
			continue
		}
		if 'A' <= r && r <= 'Z' && i > 0 && 'a' <= rune(text[i-1]) && rune(text[i-1]) <= 'z' {
			flush(i) // camelCase boundary
		}
		if start < 0 {
			start = i
		}
	}
	flush(len(text))
	return toks
}

// refDesPrefix is the leading run of letters of a ref-des, uppercased, with a leading '#'
// (KiCad power symbols, "#PWR01") skipped: "R12" -> "R", "RN3" -> "RN".
func refDesPrefix(ref string) string {
	ref = strings.TrimPrefix(ref, "#")
	i := 0
	for i < len(ref) {
		r := ref[i]
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			break
		}
		i++
	}
	return strings.ToUpper(ref[:i])
}
