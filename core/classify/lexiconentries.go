package classify

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
)

// LexiconEntry is one pattern, exclusion, ref-des prefix, part-number family or switch of an
// effective lexicon, with whether it came from the built-ins or from the convention laid over them
// (agni issue 1266).
type LexiconEntry struct {
	// Vocabulary is the list the entry sits in, spelled as the conventions YAML spells its path
	// ("net.rail", "pin.supply", "class.tvs").
	Vocabulary string
	// Kind is "pattern", "exclude", "prefix", "family" or "switch".
	Kind  string
	Value string
	// FromConvention is set on an entry the convention added, and clear on a built-in one.
	FromConvention bool
	// Replaced is set on a built-in entry the convention's `replace: true` took out.
	Replaced bool
}

// BuiltinLexicon returns the engine's built-in vocabularies in the conventions schema, the class
// tables included, so a reader sees every default a convention can add to or replace. It is a fresh
// value each call.
func BuiltinLexicon() *configpb.NamingLexicon {
	lx := defaultLexicon()
	lx.Class = map[string]*configpb.ClassVocab{}
	get := func(cl ComponentClass) *configpb.ClassVocab {
		v := lx.Class[string(cl)]
		if v == nil {
			v = &configpb.ClassVocab{}
			lx.Class[string(cl)] = v
		}
		return v
	}
	for tok, cl := range tokenClasses {
		v := get(cl)
		v.Patterns = append(v.Patterns, "^"+regexp.QuoteMeta(tok)+"$")
	}
	for p, cl := range prefixClasses {
		v := get(cl)
		v.Prefixes = append(v.Prefixes, p)
	}
	for cl, fams := range builtinFamilies {
		v := get(cl)
		v.Families = append(v.Families, fams...)
	}
	for _, v := range lx.Class {
		sort.Strings(v.Patterns)
		sort.Strings(v.Prefixes)
	}
	return lx
}

// namedVocab pairs a vocabulary's YAML path with its built-in and convention values.
type namedVocab struct {
	path       string
	base, over *configpb.VocabPatterns
}

func roleVocabs(base, over *configpb.NamingLexicon) []namedVocab {
	bn, bp, on, op := base.GetNet(), base.GetPin(), over.GetNet(), over.GetPin()
	return []namedVocab{
		{"net.rail", bn.GetRail(), on.GetRail()},
		{"net.ground", bn.GetGround(), on.GetGround()},
		{"net.feedback", bn.GetFeedback(), on.GetFeedback()},
		{"net.switching", bn.GetSwitching(), on.GetSwitching()},
		{"net.control", bn.GetControl(), on.GetControl()},
		{"net.gate_drive", bn.GetGateDrive(), on.GetGateDrive()},
		{"net.i2c", bn.GetI2C(), on.GetI2C()},
		{"pin.supply", bp.GetSupply(), op.GetSupply()},
		{"pin.gate", bp.GetGate(), op.GetGate()},
		{"pin.source", bp.GetSource(), op.GetSource()},
		{"pin.drain", bp.GetDrain(), op.GetDrain()},
		{"pin.exposed_pad", bp.GetExposedPad(), op.GetExposedPad()},
	}
}

// EffectiveEntries lists the lexicon a read with convention lexicon over uses, entry by entry, under
// the merge BuildRoleVocab and BuildClassVocab apply: a list's convention entries add to its built-in
// ones, and `replace: true` takes the built-in ones out (listed with Replaced) except a class's ref-des
// prefixes, which a replace keeps. A built-in prefix the convention lists under any class is Replaced,
// since the convention's wins. A nil over lists the built-ins alone.
//
// The order is net, pin, then class by name, each list's built-in entries before the convention's.
func EffectiveEntries(over *configpb.NamingLexicon) []LexiconEntry {
	base := BuiltinLexicon()
	var out []LexiconEntry
	add := func(vocab, kind string, vals []string, conv, replaced bool) {
		for _, v := range vals {
			out = append(out, LexiconEntry{Vocabulary: vocab, Kind: kind, Value: v, FromConvention: conv, Replaced: replaced})
		}
	}
	for i, nv := range roleVocabs(base, over) {
		// The rail-inference switch sits after the net vocabularies, where the YAML puts it.
		if i == 7 {
			addSwitch(&out, base, over)
		}
		replaced := nv.over.GetReplace()
		add(nv.path, "pattern", nv.base.GetPatterns(), false, replaced)
		add(nv.path, "exclude", nv.base.GetExclude(), false, replaced)
		add(nv.path, "pattern", nv.over.GetPatterns(), true, false)
		add(nv.path, "exclude", nv.over.GetExclude(), true, false)
	}
	// A convention's prefix wins over the built-in table, whichever class it names.
	claimed := map[string]bool{}
	for _, o := range over.GetClass() {
		for _, p := range o.GetPrefixes() {
			claimed[strings.ToUpper(p)] = true
		}
	}
	for _, name := range classNames(base, over) {
		b, o := base.GetClass()[name], over.GetClass()[name]
		path := "class." + name
		add(path, "pattern", b.GetPatterns(), false, o.GetReplace())
		add(path, "family", b.GetFamilies(), false, o.GetReplace())
		for _, p := range b.GetPrefixes() {
			add(path, "prefix", []string{p}, false, claimed[p])
		}
		add(path, "pattern", o.GetPatterns(), true, false)
		add(path, "family", o.GetFamilies(), true, false)
		add(path, "prefix", o.GetPrefixes(), true, false)
	}
	return out
}

func addSwitch(out *[]LexiconEntry, base, over *configpb.NamingLexicon) {
	on := base.GetNet().GetInferRailFromSupplyPins()
	conv := over.GetNet() != nil && over.GetNet().InferRailFromSupplyPins != nil
	if conv {
		on = over.GetNet().GetInferRailFromSupplyPins()
	}
	*out = append(*out, LexiconEntry{
		Vocabulary: "net", Kind: "switch", Value: fmt.Sprintf("infer_rail_from_supply_pins=%t", on), FromConvention: conv,
	})
}

func classNames(base, over *configpb.NamingLexicon) []string {
	seen := map[string]bool{}
	var names []string
	for _, m := range []map[string]*configpb.ClassVocab{base.GetClass(), over.GetClass()} {
		for n := range m {
			if !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
	}
	sort.Strings(names)
	return names
}

// EffectiveLexicon states the lexicon a read with convention lexicon over uses as one convention
// lexicon, every list written out whole with `replace: true`, so a convention carrying it reads a
// design exactly as the merge does. The rail-inference switch is always stated.
func EffectiveLexicon(over *configpb.NamingLexicon) *configpb.NamingLexicon {
	out := &configpb.NamingLexicon{Net: &configpb.NetNameVocab{}, Pin: &configpb.PinNameVocab{}, Class: map[string]*configpb.ClassVocab{}}
	// Every list starts stated and empty, since a list left out of a convention keeps its built-ins,
	// which would undo a replace that emptied it.
	lists := map[string]*configpb.VocabPatterns{}
	for _, nv := range roleVocabs(nil, nil) {
		lists[nv.path] = &configpb.VocabPatterns{Replace: true}
	}
	classes := map[string]*configpb.ClassVocab{}
	for _, name := range classNames(BuiltinLexicon(), over) {
		classes[name] = &configpb.ClassVocab{Replace: true}
	}
	for _, e := range EffectiveEntries(over) {
		if e.Replaced {
			continue
		}
		if e.Kind == "switch" {
			on := e.Value == "infer_rail_from_supply_pins=true"
			out.Net.InferRailFromSupplyPins = &on
			continue
		}
		if len(e.Vocabulary) > 6 && e.Vocabulary[:6] == "class." {
			c := classes[e.Vocabulary[6:]]
			switch e.Kind {
			case "pattern":
				c.Patterns = append(c.Patterns, e.Value)
			case "family":
				c.Families = append(c.Families, e.Value)
			case "prefix":
				c.Prefixes = append(c.Prefixes, e.Value)
			}
			continue
		}
		v := lists[e.Vocabulary]
		if e.Kind == "exclude" {
			v.Exclude = append(v.Exclude, e.Value)
		} else {
			v.Patterns = append(v.Patterns, e.Value)
		}
	}
	n, p := out.Net, out.Pin
	n.Rail, n.Ground, n.Feedback = lists["net.rail"], lists["net.ground"], lists["net.feedback"]
	n.Switching, n.Control, n.GateDrive, n.I2C = lists["net.switching"], lists["net.control"], lists["net.gate_drive"], lists["net.i2c"]
	p.Supply, p.Gate, p.Source, p.Drain, p.ExposedPad = lists["pin.supply"], lists["pin.gate"], lists["pin.source"], lists["pin.drain"], lists["pin.exposed_pad"]
	out.Class = classes
	return out
}

// RoleLeaf is the part of a net name the net vocabularies match: the last segment of a hierarchical
// name, so /Power/VCC matches as VCC.
func RoleLeaf(name string) string { return roleLeaf(name) }
