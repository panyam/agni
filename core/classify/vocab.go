package classify

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/panyam/agni/core/model"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
)

// ClassVocab is the component-classification lexicon: per-class regex patterns matched against a part's
// text TOKENS to hint its ComponentClass, and the ref-des prefix table that gives a part its base class.
// Like the RoleVocab naming lexicon (WS3-069), it exists so the classification vocabulary stops being
// frozen Go literals — a project extends it (an ESD-array MPN family, a house part-name convention, a
// TH prefix for its thermistors) or tightens a fickle token via config (WS3-070, agni issue 677).
//
// Patterns match a single token, case-insensitively; the built-in defaults are exact-anchored ("^tvs$")
// so classification is whole-token, never a substring (the pre-existing `tokenClasses` contract). A
// project may use looser patterns ("^pesd") to catch a part-number family.
type ClassVocab struct {
	patterns map[ComponentClass][]*regexp.Regexp
	prefixes map[string]ComponentClass
}

// DefaultClassVocab is the built-in classification lexicon: the historical tokenClasses map inverted to
// class -> exact-anchored token patterns, so the defaults reproduce the whole-token behavior exactly.
func DefaultClassVocab() *ClassVocab {
	byClass := map[ComponentClass][]string{}
	for tok, cl := range tokenClasses {
		byClass[cl] = append(byClass[cl], "^"+regexp.QuoteMeta(tok)+"$")
	}
	v := &ClassVocab{patterns: map[ComponentClass][]*regexp.Regexp{}, prefixes: map[string]ComponentClass{}}
	for p, cl := range prefixClasses {
		v.prefixes[p] = cl
	}
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

// ClassForPrefix is the base class a ref-des letter prefix ("R", "TP") conventionally marks, or
// ClassUnknown when neither the built-in table nor the project names it. The prefix is expected
// uppercased, as refDesPrefix returns it.
func (v *ClassVocab) ClassForPrefix(prefix string) ComponentClass {
	if cl, ok := v.prefixes[prefix]; ok {
		return cl
	}
	return ClassUnknown
}

func matchesAny(tok string, pats []*regexp.Regexp) bool {
	for _, p := range pats {
		if p.MatchString(tok) {
			return true
		}
	}
	return false
}

// componentClassByName is the set of classes a config may override, keyed by their string value. It is
// derived from model.ComponentClasses, which already leaves ClassUnknown out (there is no vocabulary
// for "unknown"). It used to be a hand-kept literal, and it lacked thermistor, zener and
// ideal_diode_controller, so a project extending any of the three was refused (agni issue 677).
var componentClassByName = func() map[string]ComponentClass {
	m := map[string]ComponentClass{}
	for _, cl := range model.ComponentClasses() {
		m[string(cl)] = cl
	}
	return m
}()

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
	// Ideal-diode / ORing / power-mux controllers (agni items behind reverse-polarity and
	// reverse-current review asks). Vendors spell this family many ways and none of them is
	// structurally recognisable, which is the whole reason the class is datasheet-driven.
	"idealdiodecontroller":      ClassIdealDiodeController,
	"idealdiode":                ClassIdealDiodeController,
	"oringcontroller":           ClassIdealDiodeController,
	"oringfetcontroller":        ClassIdealDiodeController,
	"orcontroller":              ClassIdealDiodeController,
	"powermux":                  ClassIdealDiodeController,
	"powerpathcontroller":       ClassIdealDiodeController,
	"reversepolaritycontroller": ClassIdealDiodeController,
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

// BuildClassVocab applies per-class overrides onto DefaultClassVocab, compiling and VALIDATING each
// pattern and prefix (config is operator input, so a bad one is a returned error). An empty override
// leaves that class at its default; Replace drops the built-in patterns for that class and leaves its
// prefixes alone. A prefix is added to the built-in table and wins over it. Overrides are keyed by the
// class; the caller has already refused a name the engine does not know (ParseComponentClass).
func BuildClassVocab(overrides map[ComponentClass]*configpb.ClassVocab) (*ClassVocab, error) {
	def := DefaultClassVocab()
	v := &ClassVocab{patterns: map[ComponentClass][]*regexp.Regexp{}, prefixes: def.prefixes}
	for cl, pats := range def.patterns {
		v.patterns[cl] = append([]*regexp.Regexp{}, pats...)
	}
	// Sorted so which of two colliding classes an error names does not depend on map order.
	classes := make([]ComponentClass, 0, len(overrides))
	for cl := range overrides {
		classes = append(classes, cl)
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i] < classes[j] })
	claimed := map[string]ComponentClass{}
	for _, cl := range classes {
		o := overrides[cl]
		var base []*regexp.Regexp
		if !o.GetReplace() {
			base = v.patterns[cl]
		}
		for _, p := range o.GetPatterns() {
			re, err := regexp.Compile("(?i)" + p)
			if err != nil {
				return nil, fmt.Errorf("class %q pattern %q: %w", cl, p, err)
			}
			base = append(base, re)
		}
		v.patterns[cl] = base
		for _, raw := range o.GetPrefixes() {
			p := strings.ToUpper(raw)
			if !isLetters(p) {
				return nil, fmt.Errorf("class %q prefix %q: a ref-des prefix is a run of letters, so this one can never match", cl, raw)
			}
			if other, ok := claimed[p]; ok && other != cl {
				return nil, fmt.Errorf("prefix %q is listed under both class %q and class %q", raw, other, cl)
			}
			claimed[p] = cl
			v.prefixes[p] = cl
		}
	}
	return v, nil
}

func isLetters(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}
