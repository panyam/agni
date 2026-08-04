package check

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

// Catalog is the composed rule set the engine runs: one or more RuleSources merged under
// the namespace/collision policy (WS3-006). Composition order is source registration
// order, rules in each source's own order — deterministic, so findings and ListRules are
// stable. Non-built-in rules are exposed as COPIES with the prefixed name and a stamped
// source tag; the source's own *Rule values are never mutated, so a suite can be
// registered into several catalogs safely.
type Catalog struct {
	rules  []*Rule
	byName map[string]*Rule
}

// sourceNameRe is the source-name grammar: the prefix must read cleanly inside a rule
// name and a CLI flag, so it is lowercase kebab only.
var sourceNameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// NewCatalog composes sources under the collision policy, failing loudly at composition
// (wiring time, where an error is actionable) rather than shadowing silently:
//   - only one anonymous source (the built-ins) may be registered;
//   - a named source must match [a-z0-9-]+ and be unique;
//   - rule names may not contain "/" (the separator belongs to the catalog);
//   - the composed name set must be collision-free — a customer rule cannot shadow a
//     built-in because prefixing makes that impossible by construction, and duplicate
//     names within or across sources are rejected.
func NewCatalog(sources ...RuleSource) (*Catalog, error) {
	c := &Catalog{byName: map[string]*Rule{}}
	seen := map[string]bool{}
	for _, s := range sources {
		name := s.Name()
		if seen[name] {
			if name == "" {
				return nil, fmt.Errorf("check: only one anonymous (built-in) source may be registered")
			}
			return nil, fmt.Errorf("check: duplicate rule source %q", name)
		}
		seen[name] = true
		if name != "" && !sourceNameRe.MatchString(name) {
			return nil, fmt.Errorf("check: source name %q must match [a-z0-9-]+", name)
		}
		for _, r := range s.Rules() {
			if strings.Contains(r.Name, "/") {
				return nil, fmt.Errorf("check: rule name %q may not contain %q (the catalog's namespace separator)", r.Name, "/")
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
				return nil, fmt.Errorf("check: duplicate rule name %q after composition", exposed.Name)
			}
			c.byName[exposed.Name] = exposed
			c.rules = append(c.rules, exposed)
		}
	}
	return c, nil
}

// DefaultCatalog is what the CLI and serve wire: the built-ins plus every source added via
// RegisterSource, in registration order. With no source registered it is the built-ins alone.
// It panics on a composition error, because the built-in set (or a registered source that
// slipped RegisterSource's checks) failing the policy is a programming error the catalog tests
// catch first.
func DefaultCatalog() *Catalog {
	return CatalogWith()
}

// CatalogWith composes the standard catalog — the built-ins, then every RegisterSource'd
// source — followed by the caller's extra sources, under the same collision policy. It is the
// one builder the engine surfaces use so registered sources are never dropped: the CLI passes
// its --conventions source here, and an embedder that wants explicit control (rather than the
// global RegisterSource) composes its suites as extras. It panics on a composition error, for
// the same reason DefaultCatalog does.
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
// package-level Filter — including the source tag, so "--tag source=tesla" selects one
// suite with no extra machinery.
func (c *Catalog) Filter(f Facets) []*Rule { return Filter(c.rules, f) }

// Facets is a rule selection over the catalog. Names selects by exact rule Name; Tags selects by
// tag key -> acceptable values. An empty Facets selects every rule; within one tag key the listed
// values OR, while distinct constrained keys (and Names) intersect (a rule must match every
// constrained axis). This is the one selection primitive the CLI (agni check) and the web service
// (CheckDesign subset) share, so subset semantics stay identical across both surfaces, and it works
// for any tag key including ones provider-supplied rules invent.
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

// Available reports whether r can produce meaningful findings over the model right now, and if not,
// a short reason a UI can show. It derives from Reads (the rule's declared fact dependencies) rather
// than a stored track label: a rule is unavailable when it reads a fact whose provider layer is not
// present for this design. The datasheet parameter layer (WS10) is absent unless seeded (the
// param(...) fact); a board.* rule is listed only for a board-carrying source format (m.SourceFormat).
// m may be nil for the design-less catalog listing, where a board rule is available (the tier exists
// in the engine); topology facts are always available.
// boardFormats are the ir.Design source formats that can carry a board-geometry sidecar. A
// board.* rule is listed available for these (one entry per producer: kicad-pcb, ipc-2581);
// the per-design authoritative gate remains the Model's board tier.
var boardFormats = map[string]bool{
	"kicad-pcb": true,
	"ipc-2581":  true,
}

// paramTierRelations are datasheet-tier fact relations whose names do NOT start with "param" (so the
// prefix test above misses them), but which are just as absent without a seeded set. Available gates a
// rule reading one of them to not-applicable without --params, so a review item bound to it reads
// not-applicable rather than a hollow pass. component.device_class joins PartSpec.device_class (WS10-013);
// component.esd_rated joins the datasheet ESD rating (WS3-076), the same silent-without-seed posture.
// The names are literals (not the RelComponentDeviceClass / RelEsdRated consts) because those consts
// are stdlib/relations content now (issue 10) and check cannot import relations — relations imports
// check. A relation NAME is a stable contract string; check gating on it by literal is sound.
var paramTierRelations = map[string]bool{
	"component.device_class": true, // RelComponentDeviceClass (WS10-013)
	"component.esd_rated":    true, // RelEsdRated (WS3-076)
}

func Available(r *Rule, m Model) (ok bool, reason string) {
	for _, fact := range r.Reads {
		if slices.Contains(r.OptionalReads, fact) {
			// A read the rule consults only to EXEMPT findings (esd-protection crediting an
			// IC's ESD rating): its tier being absent does not make the rule inapplicable, so
			// it never gates. See Rule.OptionalReads.
			continue
		}
		if (strings.HasPrefix(fact, "param") || paramTierRelations[fact]) && (m == nil || !m.HasParams()) {
			// The params tier is a per-run injection (a seeded corpus via `check
			// --params` / NewModelWithParams), not a property of the design. It is absent
			// for a bare design (no --params) and for the catalog listing (m == nil), so a
			// datasheet rule is not-applicable there — this label tells a catalog UI, and
			// the review runner, why. But when a params tier IS attached (m.HasParams()),
			// the rule is applicable and must run: the earlier unconditional gate here made
			// every datasheet rule read not-applicable in a review even WITH --params (the
			// review runner treats Available as an authoritative per-run gate, not just a
			// listing hint), so a seeded datasheet ask could never pass or fail. Mirrors the
			// board branch below, which is likewise model-aware.
			return false, "needs a seeded datasheet parameter set (check --params)"
		}
		if strings.HasPrefix(fact, "board.") && m != nil && !m.HasBoard() && !boardFormats[m.SourceFormat()] {
			// The board tier is per-artifact: a geometric rule can only run when the design
			// carries board geometry. HasBoard is the authoritative gate — a board tier was
			// actually attached (a board-format file's sidecar, or a separate export passed
			// via `agni review --board-path`, WS3-089), so a netlist entry whose SourceFormat
			// is not a board format still ungates once a board is attached. The SourceFormat
			// term is the coarse catalog-listing fallback: a board-capable format may ship a
			// geometry-less export, whose empty tier keeps the rules silent by construction.
			// With no design in hand (m == nil, the catalog listing) the rule is available:
			// the tier exists in the engine.
			return false, "design carries no board geometry (WS1-006 sidecar)"
		}
	}
	return true, ""
}
