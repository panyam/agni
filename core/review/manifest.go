// Package review runs a project's declared design-review checklist (a "manifest") against one
// design and reports, per checklist item, whether its check passed, failed, did not apply, or is not
// yet automated (WS3-050). The manifest is composition-as-config (which checks, in which review
// areas), while the checks themselves stay the engine's closed vocabulary. Review only SELECTS from
// the composed catalog and compiles inline queries, the profiles-as-config pattern (WS3-045) applied
// to a whole review.
package review

import (
	"fmt"
	"io"
	"strings"

	"github.com/panyam/agni/core/check"
	"gopkg.in/yaml.v3"
)

// Manifest is the review checklist: named review areas, each holding items. It is authored as YAML in
// the extension; Load parses and validates it.
type Manifest struct {
	Name  string `yaml:"name"`
	Areas []Area `yaml:"areas"`
}

// Area is one review area (e.g. "CAN Interface") grouping related checklist items.
type Area struct {
	Name  string `yaml:"name"`
	Items []Item `yaml:"items"`
}

// Item is one checklist entry. Title is the short human review LABEL ("termination strategy");
// Description is optional longer free text, carried on the wire but not rendered by the markdown
// table; the embedded Binding is the machine REFERENCE to the check that verifies it. ID names the item
// in the report. Note is an optional hint shown for an item that did not fail, most usefully WHY it is
// not automated ("needs a datasheet param rule") or a caveat ("presence-checked only").
//
// An item with an empty Binding, or one bound to a rule that has not shipped, is tracked but not
// automated. Bind it to its intended future rule name and it flips to pass/fail once that lands.
type Item struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Note        string `yaml:"note"`
	Binding     `yaml:",inline"`
}

// Binding selects the check that verifies an item. At most one of these is set: a catalog Rule by
// exact name ("profile/can-termination-missing"), a Tag (key=value) selecting a set of rules, a
// Profile name (sugar for the profile tag), an inline datalog Query, or a Present class-presence
// assertion. Several items may share one Binding (the profile signal-missing rule covers every signal
// at once), so Title keeps them as distinct report rows. The yaml is inlined, so a manifest author
// writes rule/tag/profile/query directly on the item.
type Binding struct {
	Rule    string          `yaml:"rule"`
	Tag     string          `yaml:"tag"`
	Profile string          `yaml:"profile"`
	Query   *QueryBinding   `yaml:"query"`
	Present *PresentBinding `yaml:"present"`
	Scope   ScopeBinding    `yaml:"scope"`
	// Requirement narrows a Profile binding to ONE of that profile's declared requirements, by
	// requirement type (WS3-115), and requires Profile to be set. Empty keeps the union semantics,
	// `profile: X` meaning "every rule profile X compiles". Like Scope it narrows rather than selects, so
	// it does not count toward the mutually-exclusive binding count.
	//
	// It lets a profile's requirement list GROW without re-scoring every item bound to it (the WS3-058
	// over-binding failure arriving through the profile door). Prefer it over binding the generated rule
	// by NAME, because the profile's 3-valued presence gate still applies (WS3-090) where a bare rule
	// binding reads a hollow pass on an absent interface. A requirement the profile does not declare, or
	// whose compiler does not apply (host-incomplete on a profile with no host), resolves to no rule and
	// reads not-automated, never pass. See
	// docsite/content/architecture/checks-contract.md#the-other-half-rule-definitions.
	Requirement string `yaml:"requirement"`
	// AppliesToClass gates the item on device class (WS10-014). The item is computed-n/a when no
	// component carries any of these classes ("no crystals here, so n/a"). Like Scope it is a GATE, not a
	// selector, so it does not count toward the mutually-exclusive binding count, and it composes with a
	// rule/tag/query binding. Values are component.class names (crystal, ceramic_resonator, clock, ...),
	// family tags included (WS10-015).
	AppliesToClass []string `yaml:"applies_to_class"`
}

// PresentBinding asserts that a class of component must EXIST on the design, such as a debug
// connector (class test_connector) or a test point. The item PASSES when at least one component of
// Class is present and FAILS with a single design-level finding when none is. Unlike a presence-only
// profile, which makes an ABSENT bus read not-applicable, a present: item is never not-applicable,
// because the component-class tier exists on any netlist. Class is a component.class value.
type PresentBinding struct {
	Class string `yaml:"class"`
}

// ScopeBinding narrows a rule/tag binding to one or more interfaces (WS3-058). The item runs the
// selected rule but reports only findings on nets belonging to the named profiles, and reads
// not-applicable when EVERY named interface is absent, so a per-interface ask ("CAN ESD") reflects only
// its bus's nets. Scope is a FILTER, not a selector, so it does not count toward the mutually-exclusive
// binding count, and it requires a rule or tag. The effective scope is the UNION of the named
// profiles' nets, so one item can span several buses.
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
	// ParamSymbol, when set, names the datasheet symbol this query checks (e.g. "IOUT"). It leaves the
	// query logic alone and gives the finding a structured datasheet citation resolved from the subject
	// component's seeded spec.
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
// bound rule name exists in the catalog, since a rule that has not shipped yet is a legitimate
// not-automated item.
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
// narrows, and each inline query well-formed, so a malformed query fails up front rather than at run.
//
// It is exported separately from Load because a manifest travels as a request VALUE (C22, WS9-050),
// so a browser form or a test may build one directly. Without it, an item carrying both a rule and a
// profile would resolve to whichever the runner tested for first.
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

// compileQuery turns an item's inline QueryBinding into a check.Rule, for both Load's validation and
// Run's resolution. Requires match/subject/message; kind defaults to component, severity to warning.
// It never parses the query, which is the registered compiler's business.
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
