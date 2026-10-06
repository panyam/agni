package check

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

// Catalog is the composed rule set the engine runs, one or more RuleSources merged under
// the namespace/collision policy (WS3-006). Rules come in source registration order, then
// each source's own order, so findings and ListRules are stable. Non-built-in rules are
// exposed as COPIES with the prefixed name and a stamped source tag. The source's own *Rule
// values are never mutated, so a suite can be registered into several catalogs.
type Catalog struct {
	rules      []*Rule
	byName     map[string]*Rule
	superseded []Supersession
}

// sourceNameRe is the source-name grammar. The prefix has to read cleanly inside a rule
// name and a CLI flag, so it is lowercase kebab only.
var sourceNameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// NewCatalog composes sources under the collision policy and returns an error at wiring time
// rather than letting one rule shadow another:
//   - only one anonymous source (the built-ins) may be registered;
//   - a named source must match [a-z0-9-]+ and be unique;
//   - rule names may not contain "/" (the separator belongs to the catalog);
//   - composed names must be unique. Prefixing already stops a named source shadowing a
//     built-in, and duplicates within or across sources are rejected.
func NewCatalog(sources ...RuleSource) (*Catalog, error) {
	c := &Catalog{byName: map[string]*Rule{}}
	if err := c.add(map[string]bool{}, sources...); err != nil {
		return nil, err
	}
	return c, nil
}

// add composes sources into c under the collision policy, recording each source name in seen so a
// duplicate source is rejected across calls as well as within one. NewCatalog and With both call it,
// so extending a catalog follows exactly the policy composing one does.
func (c *Catalog) add(seen map[string]bool, sources ...RuleSource) error {
	var declared []SupersedingSource
	for _, s := range sources {
		if sup, ok := s.(SupersedingSource); ok && len(sup.Supersedes()) > 0 {
			declared = append(declared, sup)
		}
		name := s.Name()
		if seen[name] {
			if name == "" {
				return fmt.Errorf("check: only one anonymous (built-in) source may be registered")
			}
			return fmt.Errorf("check: duplicate rule source %q", name)
		}
		seen[name] = true
		if name != "" && !sourceNameRe.MatchString(name) {
			return fmt.Errorf("check: source name %q must match [a-z0-9-]+", name)
		}
		for _, r := range s.Rules() {
			if strings.Contains(r.Name, "/") {
				return fmt.Errorf("check: rule name %q may not contain %q (the catalog's namespace separator)", r.Name, "/")
			}
			exposed := r
			if name != "" {
				cp := *r
				cp.Name = name + "/" + r.Name
				cp.Tags = maps.Clone(r.Tags)
				if cp.Tags == nil {
					cp.Tags = map[string]string{}
				}
				cp.Tags[KeySource] = name
				exposed = &cp
			}
			if _, dup := c.byName[exposed.Name]; dup {
				return fmt.Errorf("check: duplicate rule name %q after composition", exposed.Name)
			}
			c.byName[exposed.Name] = exposed
			c.rules = append(c.rules, exposed)
		}
	}
	c.applySupersessions(declared)
	return nil
}

// applySupersessions drops the rules the given sources supersede. It runs once, after every source in
// the composition has been added, so a source can supersede one added alongside it in the same call
// and not only one already present.
//
// A source's declaration never applies to its OWN rules (WS3-056). A profile overlay and the built-in
// profile it replaces carry identical tags (both stamp "profile": "SPI_NOR"), so matching on tags
// alone would drop the replacement too and leave the interface with no rules, which a report shows as
// a pass. Composition rejects duplicate source names, so exempting by name cannot exempt an unrelated
// source.
func (c *Catalog) applySupersessions(sources []SupersedingSource) {
	for _, s := range sources {
		var dropped []string
		keep := c.rules[:0:0]
		for _, r := range c.rules {
			if r.Tags[KeySource] != s.Name() && matchesAny(r, s.Supersedes()) {
				dropped = append(dropped, r.Name)
				delete(c.byName, r.Name)
				continue
			}
			keep = append(keep, r)
		}
		if len(dropped) == 0 {
			continue
		}
		c.rules = keep
		c.superseded = append(c.superseded, Supersession{By: s.Name(), Rules: dropped})
	}
}

// matchesAny reports whether r satisfies any of the given selections. An empty list matches nothing,
// so a source that declares no supersession supersedes nothing.
func matchesAny(r *Rule, fs []Facets) bool {
	for _, f := range fs {
		if matches(r.Name, f.Names) && matchesTags(r, f.Tags) {
			return true
		}
	}
	return false
}

// With returns a new catalog carrying every rule c already has, plus the rules of extra, composed
// under the same namespacing and collision policy. c is not modified.
//
// Use it to EXTEND a catalog you did not compose. A *Catalog holds composed rules, not its inputs, so
// rebuilding from the standard sources instead drops whatever else it carried (WS3-107).
//
// The base rules are carried across VERBATIM rather than re-composed, because they are already
// namespaced and a name like "profile-overlay/spi-nor-signal-missing" would be rejected for containing
// the separator. Only extra is namespaced, and its names are checked against everything already
// present, so an extension can never shadow a rule the base carried.
func (c *Catalog) With(extra ...RuleSource) (*Catalog, error) {
	out := &Catalog{
		rules:      append(make([]*Rule, 0, len(c.rules)), c.rules...),
		byName:     maps.Clone(c.byName),
		superseded: append(make([]Supersession, 0, len(c.superseded)), c.superseded...),
	}
	if out.byName == nil {
		out.byName = map[string]*Rule{}
	}
	// Seed the base's source names so re-adding one reports a duplicate source, as a single
	// composition would, rather than a per-rule collision.
	seen := map[string]bool{}
	for _, r := range c.rules {
		if src := r.Tags[KeySource]; src != "" {
			seen[src] = true
		}
	}
	if err := out.add(seen, extra...); err != nil {
		return nil, err
	}
	return out, nil
}

// DefaultCatalog is what the CLI and serve wire, the built-ins plus every source added via
// RegisterSource, in registration order. With no source registered it is the built-ins alone.
// It panics on a composition error, since the built-ins or a registered source failing the
// policy is a programming error the catalog tests catch first.
func DefaultCatalog() *Catalog {
	return CatalogWith()
}

// CatalogWith composes the built-ins, then every RegisterSource'd source, then the caller's
// extra sources, under the same collision policy. Every engine surface builds through it so
// registered sources are never dropped. The CLI passes its --conventions source here, and an
// embedder that wants explicit control instead of the global RegisterSource passes its suites
// as extras. It panics on a composition error, as DefaultCatalog does.
func CatalogWith(extra ...RuleSource) *Catalog {
	sources := make([]RuleSource, 0, 1+len(registeredSources)+len(extra))
	sources = append(sources, Builtins)
	sources = append(sources, registeredSources...)
	sources = append(sources, extra...)
	c, err := NewCatalog(sources...)
	if err != nil {
		panic("check: catalog failed composition: " + err.Error())
	}
	return c
}

// Rules returns the composed catalog in composition order. Callers must not mutate the
// returned rules; non-built-in entries are catalog-owned copies.
func (c *Catalog) Rules() []*Rule { return c.rules }

// Lookup returns the rule with the exact composed name (bare for built-ins,
// "source/name" for the rest), or nil.
func (c *Catalog) Lookup(name string) *Rule { return c.byName[name] }

// Filter selects over the composed catalog with the same Facets semantics as the
// package-level Filter. That includes the source tag, so "--tag source=<name>" selects one
// suite.
func (c *Catalog) Filter(f Facets) []*Rule { return Filter(c.rules, f) }

// Without returns a new catalog carrying every rule of c EXCEPT those matching f, the exclusion
// complement of Filter (WS3-056). c is not modified. The recorded supersessions carry across, and an
// exclusion here is not recorded as one.
//
// An empty Facets matches every rule, as in Filter, so Without(Facets{}) returns an EMPTY catalog
// rather than an unchanged one.
func (c *Catalog) Without(f Facets) *Catalog {
	out := &Catalog{
		rules:      make([]*Rule, 0, len(c.rules)),
		byName:     make(map[string]*Rule, len(c.byName)),
		superseded: append(make([]Supersession, 0, len(c.superseded)), c.superseded...),
	}
	for _, r := range c.rules {
		if matches(r.Name, f.Names) && matchesTags(r, f.Tags) {
			continue
		}
		out.rules = append(out.rules, r)
		out.byName[r.Name] = r
	}
	return out
}

// Superseded returns the supersessions applied when this catalog was composed, naming which rules were
// dropped, and which source replaced them. It is empty for a catalog composed from ordinary sources.
// A surface uses it to REPORT the suppression, since otherwise a report whose rules were removed reads
// as clean. Callers must not mutate the returned slice.
func (c *Catalog) Superseded() []Supersession { return c.superseded }

// Facets is a rule selection over the catalog. Names selects by exact rule Name; Tags selects by
// tag key -> acceptable values. An empty Facets selects every rule; within one tag key the listed
// values OR, while distinct constrained keys (and Names) intersect (a rule must match every
// constrained axis). The CLI (agni check) and the web service (CheckDesign subset) both select
// through it, so subsets mean the same on both. Any tag key works, including ones provider-supplied
// rules invent.
type Facets struct {
	Names []string
	Tags  map[string][]string
}

// Filter returns the rules matching f, preserving the input order. An empty Facets returns rules
// unchanged; each constrained axis narrows by membership, and the axes intersect.
func Filter(rules []*Rule, f Facets) []*Rule {
	out := make([]*Rule, 0, len(rules))
	for _, r := range rules {
		if !matches(r.Name, f.Names) {
			continue
		}
		if matchesTags(r, f.Tags) {
			out = append(out, r)
		}
	}
	return out
}

// matchesTags reports whether r satisfies every constrained tag key in want.
func matchesTags(r *Rule, want map[string][]string) bool {
	for key, values := range want {
		if !matches(r.Tags[key], values) {
			return false
		}
	}
	return true
}

// matches reports whether v is in want, treating an empty want as "no constraint" (always true).
func matches(v string, want []string) bool {
	return len(want) == 0 || slices.Contains(want, v)
}

// boardFormats are the ir.Design source formats that can carry a board-geometry sidecar, one entry
// per producer. The per-design authoritative gate is still the Model's board tier.
var boardFormats = map[string]bool{
	"kicad-pcb": true,
	"ipc-2581":  true,
}

// NoBoardReason is the reason Available gives a rule that reads the board tier when the run attached
// no board. A caller saying "no board was read" compares against it rather than a copy of the text
// (agni issue 848).
const NoBoardReason = "design carries no board geometry (WS1-006 sidecar)"

// Available reports whether r can produce meaningful findings over m, and if not, a short reason a
// UI can show. It derives from r.Reads, so a rule is unavailable when it reads a fact whose tier is
// absent for this design. m is nil for the design-less catalog listing, where the param tier is
// absent and board and capability rules are available. The review runner treats the result as an
// authoritative per-run gate, not only a listing hint.
func Available(r *Rule, m Model) (ok bool, reason string) {
	for _, fact := range r.Reads {
		if slices.Contains(r.OptionalReads, fact) {
			// Read only to EXEMPT findings (esd-protection crediting an IC's ESD rating), so an
			// absent tier never gates. See Rule.OptionalReads.
			continue
		}
		if TierOf(fact) == TierParam && (m == nil || !m.HasParams()) {
			// The params tier is a per-run injection (check --params, WithParamProvider), not a
			// property of the design. When one IS attached the rule must run, or a seeded
			// datasheet ask in a review could never pass or fail.
			return false, "needs a seeded datasheet parameter set"
		}
		if TierOf(fact) == TierBoard && m != nil && !m.HasBoard() && !boardFormats[m.SourceFormat()] {
			// HasBoard is the authoritative gate, true once any board tier is attached (a
			// board-format sidecar, or `agni review --board-path`, WS3-089), so a netlist entry
			// ungates too. SourceFormat is the coarse fallback. A board-capable format may ship a
			// geometry-less export, and its empty tier keeps the rules silent.
			return false, NoBoardReason
		}
	}
	// Source-format capability gate (WS3-096). A rule inferring a defect from the ABSENCE of a
	// construct the format cannot express returns no findings, which a review cannot tell from a
	// clean pass, so the declared requirement is checked here to report not-applicable instead.
	// See docsite/content/architecture/rules-and-checks.md#source-format-capabilities.
	for _, c := range r.RequiresCapability {
		if m != nil && !capabilityMet(c, m) {
			return false, capabilityReason(c)
		}
	}
	return true, ""
}

// capabilityMet reports whether the model's source format supplies capability c. An unrecognized
// capability does not gate, as an unrecognized Read does not, so a typo shows up as a rule that never
// gates rather than one silently suppressed.
func capabilityMet(c Capability, m Model) bool {
	switch c {
	case CapTypesPowerOut:
		return m.FormatTypesPowerOut()
	case CapNoConnectChannel:
		return m.HasNoConnectChannel()
	case CapRefDesCollisions:
		return m.SuppliesDiagnostic(string(CapRefDesCollisions))
	case CapNetClass:
		return m.HasNetClasses()
	case CapNetClassDefs:
		return len(m.NetClassDefs()) > 0
	case CapJunctionTaps:
		return m.SuppliesDiagnostic(string(CapJunctionTaps))
	}
	return true
}

// capabilityReason is the short not-applicable reason a UI and the review report show for an unmet
// capability c.
func capabilityReason(c Capability) string {
	switch c {
	case CapTypesPowerOut:
		return "source format does not type power-output pins (driver absence is not conclusive here)"
	case CapNoConnectChannel:
		return "source format cannot express intentional no-connect"
	case CapRefDesCollisions:
		return "this format's reader does not detect ref-des collisions (nothing was checked)"
	case CapNetClass:
		return "design carries no net-class assignments (only a KiCad project file supplies them)"
	case CapNetClassDefs:
		return "design declares no net-class definitions, so there is no declared limit to compare against"
	case CapJunctionTaps:
		return "this format's reader does not examine wire geometry, so no T-tap was checked"
	}
	return "source format lacks a capability this rule requires"
}
