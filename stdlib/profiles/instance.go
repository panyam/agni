package profiles

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// This file is the instance dimension of a profile (agni issues 1210, 1246). Without one, every net
// on the design that matches a signal counts toward one interface, so an SD card slot's SD2_CMD
// anchors the eMMC profile and an M.2 slot's M2_SMB_CLK becomes its clock. With one, each net the
// anchor matches is its own bus, and a signal belongs to that bus only when its net is named from the
// same stem.
//
// The datalog (busRules) and the Go twin (buses) must agree on membership, for the same reason
// matcher.go's two halves must: a review gate or coverage cell that disagrees with the rules scores
// a pass on a bus nothing checked.

// InstanceAnchorStem keys a bus on its anchor net's name less the anchor's suffix, so SD2_CMD is bus
// SD2 and its clock is SD2_CLK. A signal declared by suffix belongs to a bus when its net is exactly
// the stem followed by that suffix. A signal declared by glob or regex belongs when its net matches
// and starts with the stem followed by the anchor suffix's first character, the separator, so bus SD
// never takes SD2_DATA0.
const InstanceAnchorStem = "anchor_stem"

// validateInstance rejects an instance key the compiler cannot build: an unknown key, an anchor_stem
// profile whose anchor has no suffix to take off, and exclude signals with no instance to exclude, or
// that claim the anchor or a pull-up, neither of which means anything for a line that rules a bus out.
func validateInstance(p Profile) error {
	switch p.Instance {
	case "":
		if len(p.Excludes) > 0 {
			return fmt.Errorf("profile %q: excludes needs an instance key, since an excluding line rules out only the bus it belongs to; set instance: %s", p.Name, InstanceAnchorStem)
		}
	case InstanceAnchorStem:
		a := p.anchorSignal()
		if a == nil || a.Suffix == "" || a.Glob != "" || a.Regex != "" {
			return fmt.Errorf("profile %q: instance %s takes the bus's stem off the anchor's name, so the anchor must declare a suffix", p.Name, InstanceAnchorStem)
		}
	default:
		return fmt.Errorf("profile %q: unknown instance %q (known: %s)", p.Name, p.Instance, InstanceAnchorStem)
	}
	for _, s := range p.Excludes {
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("profile %q: an exclude signal needs a \"name\"", p.Name)
		}
		if s.Anchor || s.PullUp {
			return fmt.Errorf("profile %q: exclude signal %q cannot be an anchor or need a pull-up", p.Name, s.Name)
		}
		if err := validateSignalMatcher(s); err != nil {
			return fmt.Errorf("profile %q: %w", p.Name, err)
		}
	}
	return nil
}

// stemSeparator is the character a glob- or regex-declared signal's net must carry right after the
// stem, the first character of the anchor's suffix.
func (p Profile) stemSeparator() string {
	if sfx := p.anchorSuffix(); sfx != "" {
		return sfx[:1]
	}
	return ""
}

// onBus reports whether a net named name plays signal s on the bus with this stem. It is the Go twin
// of onBusBody.
func (p Profile) onBus(name, stem string, s Signal) bool {
	if !netMatchesSignal(name, s) {
		return false
	}
	if s.Glob == "" && s.Regex == "" && s.Suffix != "" {
		return name == stem+s.Suffix
	}
	return strings.HasPrefix(name, stem+p.stemSeparator())
}

// bus is one instance of an interface on a design: the anchor net that founds it, the wired nets
// each required signal has on it, and whether an exclude signal rules it out.
type bus struct {
	stem     string
	anchor   *ir.Net
	signals  map[string][]*ir.Net
	excluded bool
}

// inUse is the per-bus twin of the datalog in_use: two distinct required signals, the anchor always
// one of them, and no exclude signal.
func (b bus) inUse() bool { return !b.excluded && len(b.signals) >= 2 }

// buses returns every bus of an instance-keyed profile on the model, in net order. A net with no
// connection belongs to none, as wiredNet selects in the datalog.
func (p Profile) buses(m check.Model) []bus {
	a := p.anchorSignal()
	if a == nil {
		return nil
	}
	var out []bus
	for _, n := range m.Nets() {
		if len(n.GetConnections()) == 0 || !netMatchesSignal(n.GetName(), *a) {
			continue
		}
		out = append(out, bus{stem: strings.TrimSuffix(n.GetName(), a.Suffix), anchor: n, signals: map[string][]*ir.Net{}})
	}
	if len(out) == 0 {
		return nil
	}
	for _, n := range m.Nets() {
		if len(n.GetConnections()) == 0 {
			continue
		}
		for i := range out {
			b := &out[i]
			for _, s := range p.Signals {
				if p.onBus(n.GetName(), b.stem, s) {
					b.signals[s.Name] = append(b.signals[s.Name], n)
				}
			}
			for _, s := range p.Excludes {
				if p.onBus(n.GetName(), b.stem, s) {
					b.excluded = true
				}
			}
		}
	}
	return out
}

// busesInUse returns the buses the rules judge.
func (p Profile) busesInUse(m check.Model) []bus {
	var out []bus
	for _, b := range p.buses(m) {
		if b.inUse() {
			out = append(out, b)
		}
	}
	return out
}

// busNets returns the nets playing any of signals on a bus in use, keyed by name.
func (p Profile) busNets(m check.Model, signals []Signal) map[string]bool {
	out := map[string]bool{}
	for _, b := range p.busesInUse(m) {
		for _, s := range signals {
			for _, n := range b.signals[s.Name] {
				out[n.GetName()] = true
			}
		}
	}
	return out
}

// busRules derives the instance-keyed presence relations every requirement of such a profile reads.
//
//	anchor_bus(a, s)    anchor net a founds the bus with stem s
//	on_bus(s, sig, n)   net n plays required signal sig on bus s
//	has_signal(s, sig)  bus s carries sig
//	excluded(s)         an exclude signal's net is on bus s
//	in_use(s)           bus s carries two distinct signals, its anchor among them, and is not excluded
func (p Profile) busRules() []query.Rule {
	a := p.anchorSignal()
	anchorBody := append(wiredNet(query.V("a")), netMatch(query.V("a"), *a)...)
	anchorBody = append(anchorBody, query.Pos(query.Rel("str.concat", query.V("s"), query.Str(a.Suffix), query.V("a"))))
	rules := []query.Rule{query.Def(query.Rel("anchor_bus", query.V("a"), query.V("s")), anchorBody...)}
	for _, s := range p.Signals {
		rules = append(rules,
			query.Def(query.Rel("on_bus", query.V("s"), query.Str(s.Name), query.V("n")), p.onBusBody(s)...),
			query.Def(query.Rel("has_signal", query.V("s"), query.Str(s.Name)),
				query.Pos(query.Rel("on_bus", query.V("s"), query.Str(s.Name), query.V("n")))))
	}
	for _, s := range p.Excludes {
		rules = append(rules, query.Def(query.Rel("excluded", query.V("s")), p.onBusBody(s)...))
	}
	inUse := []query.Literal{
		query.Pos(query.Rel("has_signal", query.V("s"), query.Str(a.Name))),
		query.Pos(query.Rel("has_signal", query.V("s"), query.V("x"))),
		query.Pos(query.Rel("has_signal", query.V("s"), query.V("y"))),
		query.Cmp(query.V("x"), "!=", query.V("y")),
	}
	if len(p.Excludes) > 0 {
		inUse = append(inUse, query.Neg(query.Rel("excluded", query.V("s"))))
	}
	return append(rules, query.Def(query.Rel("in_use", query.V("s")), inUse...))
}

// onBusBody binds n to the nets playing signal s on the bus with stem s, the datalog twin of onBus.
// A suffix-declared signal's net is built from the stem, so the body looks up one name per bus rather
// than matching every net on the design.
func (p Profile) onBusBody(s Signal) []query.Literal {
	body := []query.Literal{query.Pos(query.Rel("anchor_bus", query.V("a"), query.V("s")))}
	if s.Glob == "" && s.Regex == "" && s.Suffix != "" {
		body = append(body, query.Pos(query.Rel("str.concat", query.V("s"), query.Str(s.Suffix), query.V("n"))))
		body = append(body, wiredNet(query.V("n"))...)
		return append(body, netMatch(query.V("n"), s)...)
	}
	body = append(body, query.Pos(query.Rel("str.concat", query.V("s"), query.Str(p.stemSeparator()), query.V("head"))))
	body = append(body, wiredNet(query.V("n"))...)
	body = append(body, netMatch(query.V("n"), s)...)
	return append(body, query.Pos(query.Rel("str.prefix", query.V("n"), query.V("head"))))
}
