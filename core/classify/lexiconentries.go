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
	// Kind is "pattern", "exclude", "prefix", "exclude_prefix", "family", "pair", "unit", "key" or
	// "switch", "key" being one attribute key or do-not-populate value under "attribute.<list>". A
	// pair's Value is its two suffixes joined by "=" (see DiffPairEntry), and a unit's is the unit a
	// bare number on the class means, under the vocabulary "value_unit.<class>".
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
	lx.ValueUnit = map[string]string{}
	for cl, u := range DefaultValueVocab().bareUnit {
		lx.ValueUnit[string(cl)] = u
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
		{"net.no_connect", bn.GetNoConnect(), on.GetNoConnect()},
		{"net.voltage", bn.GetVoltage(), on.GetVoltage()},
		{"pin.supply", bp.GetSupply(), op.GetSupply()},
		{"pin.gate", bp.GetGate(), op.GetGate()},
		{"pin.source", bp.GetSource(), op.GetSource()},
		{"pin.drain", bp.GetDrain(), op.GetDrain()},
		{"pin.exposed_pad", bp.GetExposedPad(), op.GetExposedPad()},
		{"pin.anode", bp.GetAnode(), op.GetAnode()},
		{"pin.cathode", bp.GetCathode(), op.GetCathode()},
		{"pin.feedback", bp.GetFeedback(), op.GetFeedback()},
		{"pin.switching", bp.GetSwitching(), op.GetSwitching()},
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
		// The suffix pairs and the rail-inference switch close the net vocabularies.
		if i == 9 {
			add("net.diff_pair", "pair", diffPairEntries(base.GetNet().GetDiffPair()), false, over.GetNet().GetDiffPair().GetReplace())
			add("net.diff_pair", "pair", diffPairEntries(over.GetNet().GetDiffPair()), true, false)
			addSwitch(&out, base, over)
		}
		replaced := nv.over.GetReplace()
		add(nv.path, "pattern", nv.base.GetPatterns(), false, replaced)
		add(nv.path, "exclude", nv.base.GetExclude(), false, replaced)
		add(nv.path, "pattern", nv.over.GetPatterns(), true, false)
		add(nv.path, "exclude", nv.over.GetExclude(), true, false)
	}
	for _, ak := range attributeLists(base, over) {
		add(ak.path, "key", ak.base.GetKeys(), false, ak.over.GetReplace())
		add(ak.path, "key", ak.over.GetKeys(), true, false)
	}
	// A convention's prefix wins over the built-in table, whichever class it names, and one it
	// excludes is gone.
	claimed := map[string]bool{}
	for _, o := range over.GetClass() {
		for _, p := range o.GetPrefixes() {
			claimed[strings.ToUpper(p)] = true
		}
		for _, p := range o.GetExcludePrefixes() {
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
		add(path, "exclude_prefix", o.GetExcludePrefixes(), true, false)
	}
	for _, name := range unitClasses(base, over) {
		path := "value_unit." + name
		_, overridden := over.GetValueUnit()[name]
		if u, ok := base.GetValueUnit()[name]; ok {
			add(path, "unit", []string{u}, false, overridden)
		}
		if overridden {
			add(path, "unit", []string{over.GetValueUnit()[name]}, true, false)
		}
	}
	return out
}

// DiffPairEntry spells a suffix pair as a LexiconEntry's value. ParseDiffPairEntry reads it back.
func DiffPairEntry(p *configpb.DiffSuffix) string { return p.GetPositive() + "=" + p.GetNegative() }

// ParseDiffPairEntry reads a value DiffPairEntry wrote.
func ParseDiffPairEntry(s string) *configpb.DiffSuffix {
	pos, neg, _ := strings.Cut(s, "=")
	return &configpb.DiffSuffix{Positive: pos, Negative: neg}
}

func diffPairEntries(v *configpb.DiffPairVocab) []string {
	out := make([]string, 0, len(v.GetPairs()))
	for _, p := range v.GetPairs() {
		out = append(out, DiffPairEntry(p))
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

// attributeList pairs an attribute list's YAML path with its built-in and convention values.
type attributeList struct {
	path       string
	base, over *configpb.AttributeKeys
}

func attributeLists(base, over *configpb.NamingLexicon) []attributeList {
	ba, oa := base.GetAttribute(), over.GetAttribute()
	return []attributeList{
		{"attribute.mpn", ba.GetMpn(), oa.GetMpn()},
		{"attribute.value", ba.GetValue(), oa.GetValue()},
		{"attribute.dnp", ba.GetDnp(), oa.GetDnp()},
		{"attribute.dnp_values", ba.GetDnpValues(), oa.GetDnpValues()},
	}
}

func unitClasses(base, over *configpb.NamingLexicon) []string {
	seen := map[string]bool{}
	var names []string
	for _, m := range []map[string]string{base.GetValueUnit(), over.GetValueUnit()} {
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
	out.Net.DiffPair = &configpb.DiffPairVocab{Replace: true}
	out.ValueUnit = map[string]string{}
	keys := map[string]*configpb.AttributeKeys{}
	for _, ak := range attributeLists(nil, nil) {
		keys[ak.path] = &configpb.AttributeKeys{Replace: true}
	}
	out.Attribute = &configpb.AttributeVocab{
		Mpn: keys["attribute.mpn"], Value: keys["attribute.value"], Dnp: keys["attribute.dnp"], DnpValues: keys["attribute.dnp_values"],
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
		if e.Kind == "key" {
			keys[e.Vocabulary].Keys = append(keys[e.Vocabulary].Keys, e.Value)
			continue
		}
		if e.Kind == "unit" {
			out.ValueUnit[strings.TrimPrefix(e.Vocabulary, "value_unit.")] = e.Value
			continue
		}
		if e.Kind == "pair" {
			out.Net.DiffPair.Pairs = append(out.Net.DiffPair.Pairs, ParseDiffPairEntry(e.Value))
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
			case "exclude_prefix":
				c.ExcludePrefixes = append(c.ExcludePrefixes, e.Value)
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
	n.NoConnect, n.Voltage = lists["net.no_connect"], lists["net.voltage"]
	p.Supply, p.Gate, p.Source, p.Drain, p.ExposedPad = lists["pin.supply"], lists["pin.gate"], lists["pin.source"], lists["pin.drain"], lists["pin.exposed_pad"]
	p.Anode, p.Cathode, p.Feedback, p.Switching = lists["pin.anode"], lists["pin.cathode"], lists["pin.feedback"], lists["pin.switching"]
	out.Class = classes
	return out
}

// RoleLeaf is the part of a net name the net vocabularies match: the last segment of a hierarchical
// name, so /Power/VCC matches as VCC.
func RoleLeaf(name string) string { return roleLeaf(name) }
