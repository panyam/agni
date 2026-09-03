// Package review runs a project's declared design-review checklist (a "manifest") against one
// design and reports, per checklist item, whether its check passed, failed, did not apply, or is not
// yet automated (WS3-050). The manifest is composition-as-config in the overlay — which checks, in
// which review areas — while the checks themselves (core rules, interface profiles, datalog queries)
// stay the engine's closed vocabulary; review only SELECTS from the composed catalog and compiles
// inline queries. This is the profiles-as-config pattern (WS3-045) generalized from one interface to
// a whole review.
package review

import (
	"fmt"
	"io"
	"strings"

	"github.com/panyam/agni/core/check"
	"gopkg.in/yaml.v3"
)

// Manifest is the review checklist: named review areas, each holding items. It is authored as YAML in
// the overlay; Load parses and validates it.
type Manifest struct {
	Name  string `yaml:"name"`
	Areas []Area `yaml:"areas"`
}

// Area is one review area (e.g. "CAN Interface") grouping related checklist items.
type Area struct {
	Name  string `yaml:"name"`
	Items []Item `yaml:"items"`
}

// Item is one checklist entry. Its parts play distinct roles: Title is the short human review LABEL
// shown in the compact report ("termination strategy"); Description is an optional longer free-text
// explanation, carried for a verbose or web report (the markdown table renders Title only for now);
// the embedded Binding is the machine REFERENCE to the check that verifies it, resolved against the
// catalog and run. ID names the item in the report. Note is an optional human hint shown for an item
// that did not fail — most usefully WHY an item is not automated ("needs a datasheet param rule") or
// a caveat on a shipped one ("presence-checked only"). An item with an empty Binding, or one bound to
// a rule that has not shipped, is tracked but not automated (a manual or not-yet-covered entry) —
// bind it to its intended future rule name and it flips to pass/fail automatically once that lands.
type Item struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Note        string `yaml:"note"`
	Binding     `yaml:",inline"`
}

// Binding selects the check that verifies an item. At most one field is set (they are mutually
// exclusive) — a catalog Rule by exact name ("profile/can-termination-missing"), a Tag (key=value)
// selecting a set of rules, a Profile name (sugar for the profile tag), an inline datalog Query, or a
// Present class-presence assertion (a part of a given component.class must exist on the design).
// Several items may share one Binding (the profile signal-missing rule covers every signal at once),
// so Title keeps them as distinct report rows. The yaml is inlined, so a manifest author writes
// rule/tag/profile/query directly on the item, flat.
type Binding struct {
	Rule    string          `yaml:"rule"`
	Tag     string          `yaml:"tag"`
	Profile string          `yaml:"profile"`
	Query   *QueryBinding   `yaml:"query"`
	Present *PresentBinding `yaml:"present"`
	Scope   ScopeBinding    `yaml:"scope"`
	// Requirement narrows a Profile binding to ONE of that profile's declared requirements, by
	// requirement type (WS3-115). Empty keeps the union semantics every manifest was written against:
	// `profile: X` means "every rule profile X compiles". Like Scope it NARROWS an existing selector
	// rather than being one, so it does not count toward the mutually-exclusive binding count, and it
	// requires Profile to be set.
	//
	// The point is that a profile's requirement list must be able to GROW. Under union semantics,
	// adding a requirement re-scores every item already bound to that profile: they all begin
	// reporting a defect none of them describes, which is the over-binding failure Scope addresses
	// for design-wide rules (WS3-058) arriving through the profile door instead. A selected item is
	// answered by its own requirement's rule and by nothing else, so the profile stays extensible.
	//
	// It is preferred over binding the generated rule by NAME because the profile's 3-valued presence
	// gate still applies (WS3-090): an absent interface reads not-applicable and a host-bound one
	// whose host is annotated nowhere reads not-automated, where a bare rule binding would read a
	// hollow pass. A requirement the profile does not declare — or one whose compiler does not apply,
	// like host-incomplete on a profile that binds no host — resolves to no rule and reads
	// not-automated, never pass.
	Requirement string `yaml:"requirement"`
	// AppliesToClass gates the item on device class (WS10-014): the item is computed-n/a when no
	// component carries any of these classes (the honest "no crystals here → n/a", automatable via the
	// device-class fact). Like Scope it is a GATE, not a selector, so it does not count toward the
	// mutually-exclusive binding count; it composes with a rule/tag/query binding. Values are
	// component.class names (crystal, ceramic_resonator, clock, ...), family tags included (WS10-015).
	AppliesToClass []string `yaml:"applies_to_class"`
}

// PresentBinding asserts that a class of component must EXIST on the design: the item PASSES when at
// least one component of Class is present and FAILS (with a single design-level finding) when none is.
// It is the "a part of this kind must be on the board" review primitive — a debug/programming
// connector (class test_connector), a test point, ... — and is distinct from the other bindings: an
// interface Profile checks a named bus's SHAPE (signals/pull-ups), and a presence-only profile makes
// an ABSENT known bus read not-applicable, NOT fail. A present: item is never not-applicable: the
// component-class tier exists on any netlist, so the presence question always has a pass/fail answer.
// Class is a component.class value (crystal, capacitor, test_connector, ...).
type PresentBinding struct {
	Class string `yaml:"class"`
}

// ScopeBinding narrows a rule/tag binding to one or more interfaces: the item runs the selected rule
// but reports only findings on nets that belong to the named profiles, and reads not-applicable when
// EVERY named interface is absent from the design. It makes a per-interface ask (e.g. "CAN ESD")
// reflect only its bus's nets instead of a broad design-wide rule's whole output (WS3-058). Scope is a
// FILTER, not a selector, so it does not count toward the mutually-exclusive binding count; it requires
// a rule or tag to filter. `profiles` is a list so one item can span several buses (e.g. one
// "external-interface ESD" ask over CAN+LIN+SGMII); the effective scope is the UNION of their nets.
type ScopeBinding struct {
	Profiles []string `yaml:"profiles"`
}

// names returns the interfaces the scope names, de-duplicated, order-preserving.
func (s ScopeBinding) names() []string {
	var out []string
	seen := map[string]bool{}
	for _, n := range s.Profiles {
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// QueryBinding is an inline datalog check authored in the manifest (a house rule). Match is the
// datalog program (its goal must project Subject); Message is the finding template ({var} is replaced
// by the bound value). Kind defaults to "component" and Severity to "warning".
type QueryBinding struct {
	Match    string `yaml:"match"`
	Subject  string `yaml:"subject"`
	Kind     string `yaml:"kind"`
	Message  string `yaml:"message"`
	Severity string `yaml:"severity"`
	// ParamSymbol, when set, names the datasheet symbol this query checks (e.g. "IOUT"). It is an
	// opt-in that does not change the query logic: the finding gains a structured datasheet citation
	// (doc/page/section/confidence) resolved from the subject component's seeded spec, for the report.
	ParamSymbol string `yaml:"param_symbol"`
}

// count reports how many bindings are set (must be 0 or 1).
func (b Binding) count() int {
	n := 0
	for _, s := range []string{b.Rule, b.Tag, b.Profile} {
		if s != "" {
			n++
		}
	}
	if b.Query != nil {
		n++
	}
	if b.Present != nil {
		n++
	}
	return n
}

// Load reads a review manifest from r, parses the YAML, and validates it. It does NOT check that a
// bound rule name exists in the catalog — a rule that has not shipped yet is a legitimate "not
// automated" item, not an error (mirrors how a checklist item sits pending until its rule lands).
func Load(r io.Reader) (Manifest, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("review manifest: invalid YAML: %w", err)
	}
	if err := Validate(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Validate checks a manifest's structure: a name, at least one area, each area named, each item
// identified with at most one binding, each narrower (scope, requirement) paired with the selector it
// narrows, and each inline query well-formed (a malformed query is a teaching error up front, not a
// surprise at run).
//
// It is exported and separate from Load because a manifest does not always arrive as YAML (WS9-050):
// it travels as a request VALUE under CONSTRAINTS C22, so a browser form or a test may build one
// directly. Those manifests get held to the same rules, because the checks here are what make the
// mutually-exclusive bindings actually exclusive. Without it, an item carrying both a rule and a
// profile would resolve to whichever the runner happened to test for first, and score a real design
// against a check its author did not ask for.
func Validate(m Manifest) error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("review manifest: missing required field \"name\"")
	}
	if len(m.Areas) == 0 {
		return fmt.Errorf("review manifest %q: needs at least one area", m.Name)
	}
	for _, a := range m.Areas {
		if strings.TrimSpace(a.Name) == "" {
			return fmt.Errorf("review manifest %q: an area is missing its \"name\"", m.Name)
		}
		for _, it := range a.Items {
			if strings.TrimSpace(it.ID) == "" {
				return fmt.Errorf("review manifest %q, area %q: an item is missing its \"id\"", m.Name, a.Name)
			}
			if it.Binding.count() > 1 {
				return fmt.Errorf("review manifest item %q: declares more than one binding (rule/tag/profile/query are mutually exclusive)", it.ID)
			}
			if len(it.Binding.Scope.names()) > 0 && it.Binding.Rule == "" && it.Binding.Tag == "" {
				return fmt.Errorf("review manifest item %q: scope filters a rule/tag binding, so one must be set", it.ID)
			}
			if it.Binding.Requirement != "" && it.Binding.Profile == "" {
				return fmt.Errorf("review manifest item %q: requirement narrows a profile binding, so \"profile\" must be set", it.ID)
			}
			if it.Binding.Query != nil {
				if _, err := compileQuery(it); err != nil {
					return fmt.Errorf("review manifest item %q: %w", it.ID, err)
				}
			}
			if it.Binding.Present != nil && strings.TrimSpace(it.Binding.Present.Class) == "" {
				return fmt.Errorf("review manifest item %q: a present binding needs \"class\"", it.ID)
			}
		}
	}
	return nil
}

// compileQuery turns an item's inline QueryBinding into a check.Rule (shared by Load's validation and
// Run's resolution). Requires match/subject/message; kind defaults to component, severity to warning.
// Everything past that is the registered engine's business: this function never parses the query.
func compileQuery(it Item) (*check.Rule, error) {
	q := it.Binding.Query
	if strings.TrimSpace(q.Match) == "" || strings.TrimSpace(q.Subject) == "" || strings.TrimSpace(q.Message) == "" {
		return nil, fmt.Errorf("a query binding needs \"match\", \"subject\", and \"message\"")
	}
	kind := q.Kind
	switch kind {
	case "":
		kind = check.KindComponent
	case check.KindComponent, check.KindNet, check.KindPin:
	default:
		return nil, fmt.Errorf("query kind %q must be component, net, or pin", q.Kind)
	}
	sev := q.Severity
	if sev == "" {
		sev = "warning"
	}
	c, err := queryCompiler()
	if err != nil {
		return nil, err
	}
	return c.CompileQuery(QueryRequest{
		Rule: check.Rule{
			Name:     "review/" + it.ID,
			Severity: sev,
			Summary:  it.Title,
			Tags:     map[string]string{check.KeyCategory: check.CategoryConnectivity, check.KeyDistribution: check.DistOpen},
		},
		Query:       q.Match,
		Kind:        kind,
		Subject:     q.Subject,
		Message:     q.Message,
		ParamSymbol: q.ParamSymbol,
	})
}
