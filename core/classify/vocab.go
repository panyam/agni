package classify

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// VocabPatterns overrides one vocabulary: extra Patterns merged onto the built-in set, or the full set
// when Replace is true. It is the config shape a project supplies (via the --conventions lexicon block).
// It is the shared shape for both the class lexicon (here) and check's role lexicon (which aliases it).
type VocabPatterns struct {
	Patterns []string
	Replace  bool
}

// ClassVocab is the component-classification lexicon: per-class regex patterns matched against a part's
// text TOKENS to hint its ComponentClass. Like the RoleVocab naming lexicon (WS3-069), it exists so the
// classification vocabulary stops being frozen Go literals — a project extends it (an ESD-array MPN
// family, a house part-name convention) or tightens a fickle token via config (WS3-070).
//
// Patterns match a single token, case-insensitively; the built-in defaults are exact-anchored ("^tvs$")
// so classification is whole-token, never a substring (the pre-existing `tokenClasses` contract). A
// project may use looser patterns ("^pesd") to catch a part-number family.
type ClassVocab struct {
	patterns map[ComponentClass][]*regexp.Regexp
}

// DefaultClassVocab is the built-in classification lexicon: the historical tokenClasses map inverted to
// class -> exact-anchored token patterns, so the defaults reproduce the whole-token behavior exactly.
func DefaultClassVocab() *ClassVocab {
	byClass := map[ComponentClass][]string{}
	for tok, cl := range tokenClasses {
		byClass[cl] = append(byClass[cl], "^"+regexp.QuoteMeta(tok)+"$")
	}
	v := &ClassVocab{patterns: map[ComponentClass][]*regexp.Regexp{}}
	for cl, pats := range byClass {
		sort.Strings(pats) // deterministic order (tokenClasses map iteration is not)
		v.patterns[cl] = compileClassPatterns(pats)
	}
	return v
}

func compileClassPatterns(pats []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(pats))
	for i, p := range pats {
		out[i] = regexp.MustCompile("(?i)" + p)
	}
	return out
}

// HintsFor returns the set of classes whose patterns match any of the tokens (the classification hint set).
func (v *ClassVocab) HintsFor(tokens []string) map[ComponentClass]bool {
	hints := map[ComponentClass]bool{}
	for cl, pats := range v.patterns {
		for _, tok := range tokens {
			if matchesAny(tok, pats) {
				hints[cl] = true
				break
			}
		}
	}
	return hints
}

func matchesAny(tok string, pats []*regexp.Regexp) bool {
	for _, p := range pats {
		if p.MatchString(tok) {
			return true
		}
	}
	return false
}

// componentClassByName is the set of classes a config may override, keyed by their string value.
// ClassUnknown is deliberately absent (there is no vocabulary for "unknown").
var componentClassByName = map[string]ComponentClass{
	string(ClassResistor): ClassResistor, string(ClassCapacitor): ClassCapacitor,
	string(ClassInductor): ClassInductor, string(ClassFerrite): ClassFerrite,
	string(ClassDiode): ClassDiode, string(ClassLED): ClassLED, string(ClassTVS): ClassTVS,
	string(ClassFuse): ClassFuse, string(ClassConnector): ClassConnector,
	string(ClassTestConnector): ClassTestConnector,
	string(ClassTestPoint):     ClassTestPoint,
	string(ClassClock):         ClassClock, string(ClassOscillator): ClassOscillator,
	string(ClassCrystal): ClassCrystal, string(ClassCeramicResonator): ClassCeramicResonator,
	string(ClassIC): ClassIC, string(ClassTransistor): ClassTransistor,
}

// deviceClassAliases maps common datasheet device_class spellings to a canonical ComponentClass, keyed
// by the alnum-lowercased form (so "ceramic resonator", "Ceramic-Resonator", and "CERAMICRESONATOR" all
// hit one key). It is the datasheet-path analogue of the keyword lexicon (WS10-015): a seeded
// device_class is a free-form vendor string ("SPXO", "ceramic resonator") that must resolve to the same
// canonical class the keyword path produces, or it lands bare and misses its family tag. Only genuine
// synonyms belong here; a string already equal to a canonical class name needs no entry (identity).
var deviceClassAliases = map[string]ComponentClass{
	"ceramicresonator": ClassCeramicResonator,
	"resonator":        ClassCeramicResonator,
	"quartzcrystal":    ClassCrystal,
	"xtal":             ClassCrystal,
	"spxo":             ClassOscillator, // simple packaged crystal oscillator
	"xo":               ClassOscillator,
	"tcxo":             ClassOscillator, // temperature-compensated
	"vcxo":             ClassOscillator, // voltage-controlled
	"activeoscillator": ClassOscillator,
	"clocksource":      ClassClock,
}

// NormalizeDeviceClass maps a datasheet device_class string to a canonical ComponentClass (WS10-015).
// It alnum-lowercases the input and looks it up in the alias table, then falls back to the canonical
// class-name set (so "efuse", "ldo", "crystal" pass through as themselves), then to the raw string cast
// for an unknown-but-meaningful value (identity, the pass-through the additive enrichment relies on).
// An empty string yields ClassUnknown. This is what the datasheet enrichment runs before ClassesOf, so
// a vendor spelling reaches the same family tag the keyword path would.
func NormalizeDeviceClass(s string) ComponentClass {
	key := alnumLower(s)
	if key == "" {
		return ClassUnknown
	}
	if cl, ok := deviceClassAliases[key]; ok {
		return cl
	}
	if cl, ok := componentClassByName[key]; ok {
		return cl
	}
	return ComponentClass(s)
}

// alnumLower reduces a string to its lowercase alphanumeric characters, collapsing spelling variants of
// a device_class ("ceramic resonator", "Ceramic-Resonator") to one alias key.
func alnumLower(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ParseComponentClass maps a class NAME (its string value, e.g. "tvs" or "test_point") to the class,
// reporting whether it is a known, overridable class. It is the validation a config loader uses so an
// unknown class name in a lexicon block is a teaching error, not a silently ignored key.
func ParseComponentClass(name string) (ComponentClass, bool) {
	cl, ok := componentClassByName[name]
	return cl, ok
}

var activeClassVocab = DefaultClassVocab()

// SetActiveClassVocab replaces the process-level classification lexicon (the CLI calls this after
// loading a project's --conventions lexicon block). Passing nil restores the defaults. It must run
// before ingestion (ReadDesign), since the classify pass stamps device_classes with the active vocab.
func SetActiveClassVocab(v *ClassVocab) {
	if v == nil {
		v = DefaultClassVocab()
	}
	activeClassVocab = v
}

// ActiveClassVocab returns the classification lexicon currently in effect.
func ActiveClassVocab() *ClassVocab { return activeClassVocab }

// BuildClassVocab applies per-class pattern overrides onto DefaultClassVocab, compiling and VALIDATING
// each pattern (config is operator input, so a bad regex is a returned error). An empty override leaves
// that class at its default; Replace drops the built-in patterns for that class. Overrides are keyed by
// the class's string value (e.g. "tvs"); an unknown class name is an error.
func BuildClassVocab(overrides map[ComponentClass]VocabPatterns) (*ClassVocab, error) {
	def := DefaultClassVocab()
	v := &ClassVocab{patterns: map[ComponentClass][]*regexp.Regexp{}}
	for cl, pats := range def.patterns {
		v.patterns[cl] = append([]*regexp.Regexp{}, pats...)
	}
	for cl, o := range overrides {
		var base []*regexp.Regexp
		if !o.Replace {
			base = v.patterns[cl]
		}
		for _, p := range o.Patterns {
			re, err := regexp.Compile("(?i)" + p)
			if err != nil {
				return nil, fmt.Errorf("class %q pattern %q: %w", cl, p, err)
			}
			base = append(base, re)
		}
		v.patterns[cl] = base
	}
	return v, nil
}
