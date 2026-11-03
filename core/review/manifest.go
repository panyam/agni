// Package review runs a project's design-review checklist against one design. The checklist is called
// a manifest. For each checklist item, the review reports whether its check passed, failed, did not
// apply, or is not yet automated (WS3-050). The manifest is composition-as-config. It says which
// checks run and in which review areas, and the checks themselves are still the engine's closed
// vocabulary. Review only SELECTS from the composed catalog and compiles inline queries. It applies
// the profiles-as-config pattern (WS3-045) to a whole review.
package review

import (
	"fmt"
	"io"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"gopkg.in/yaml.v3"
)

// Manifest is the review checklist. It holds named review areas, and each area holds items. Authors
// write it as YAML in the extension, and Load parses and validates it.
type Manifest struct {
	Name  string `yaml:"name"`
	Areas []Area `yaml:"areas"`
}

// Area is one review area (e.g. "CAN Interface") grouping related checklist items.
type Area struct {
	Name  string `yaml:"name"`
	Items []Item `yaml:"items"`
}

// Item is one checklist entry. ID names the item in the report. Title is the short human review LABEL
// ("termination strategy"). Description is optional longer free text. The wire carries it, and the
// markdown table does not render it. The embedded Binding is the machine REFERENCE to the check that
// verifies the item. Note is an optional hint shown for an item that did not fail. Most usefully it
// says WHY the item is not automated ("needs a datasheet param rule"), or it gives a caveat
// ("presence-checked only").
//
// The review tracks an item without automating it when its Binding is empty or names a rule that has
// not shipped. Bind it to its intended future rule name, and it flips to pass/fail once that rule
// ships.
type Item struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Note        string `yaml:"note"`
	Binding     `yaml:",inline"`
}

// Binding selects the check that verifies an item. At most one of Rule, Tag, Profile, Query and
// Present is set. Rule names a catalog rule exactly ("profile/can-termination-missing"). Tag is a
// key=value pair that selects a set of rules. Profile names a profile, as sugar for the profile tag.
// Query is an inline datalog query, and Present is a class-presence assertion. Several items may share
// one Binding, since the profile signal-missing rule covers every signal at once. Title keeps those
// items as distinct report rows. The yaml is inlined, so a manifest author writes
// rule/tag/profile/query directly on the item.
type Binding struct {
	Rule    string          `yaml:"rule"`
	Tag     string          `yaml:"tag"`
	Profile string          `yaml:"profile"`
	Query   *QueryBinding   `yaml:"query"`
	Present *PresentBinding `yaml:"present"`
	Scope   ScopeBinding    `yaml:"scope"`
	// Requirement narrows a Profile binding to ONE of that profile's declared requirements, by
	// requirement type (WS3-115). It requires Profile to be set. Empty keeps the union semantics, where
	// `profile: X` means "every rule profile X compiles". Like Scope it narrows rather than selects, so
	// it does not count toward the mutually-exclusive binding count.
	//
	// Imagine a profile's requirement list GROWS. Without Requirement, every item bound to the profile
	// would be re-scored against the new rules too. That is the WS3-058 over-binding failure, arriving
	// through the profile. Prefer Requirement over binding the generated rule by NAME. With Requirement
	// the profile's 3-valued presence gate still applies (WS3-090), and a bare rule binding reads a
	// hollow pass on an absent interface. Sometimes the profile does not declare the requirement, or the
	// requirement's compiler does not apply (host-incomplete on a profile with no host). Then it resolves
	// to no rule and reads not-automated, never pass. See
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
// connector (class test_connector) or a test point. Class is a component.class value. The item PASSES
// when at least one component of Class is present. It FAILS with a single design-level finding when
// none is. A presence-only profile makes an ABSENT bus read not-applicable. A present: item is never
// not-applicable, because the component-class tier exists on any netlist.
type PresentBinding struct {
	Class string `yaml:"class"`
}

// ScopeBinding narrows a rule/tag binding to one or more interfaces (WS3-058). The item runs the
// selected rule. It reports only the findings on nets that belong to the named profiles. It reads
// not-applicable when EVERY named interface is absent. So a per-interface ask ("CAN ESD") reflects only
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

// QueryBinding is an inline datalog check authored in the manifest, a house rule. Match is the
// datalog program, and its goal must project Subject. Message is the finding template, and the bound
// value replaces each {var} in it. Kind defaults to "component" and Severity to "warning".
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
	if err := ValidateStructure(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// ValidateOption adjusts what Validate checks a manifest against.
type ValidateOption func(*validateConfig)

type validateConfig struct{ vocabulary *facts.Registry }

// WithVocabulary checks each inline query against reg rather than the process default, so a query
// naming the run's own library (a project's lib/, or modules sent with the request) validates as it
// will run (agni issue 779). Pass the vocabulary the review will run with.
func WithVocabulary(reg *facts.Registry) ValidateOption {
	return func(c *validateConfig) { c.vocabulary = reg }
}

// Validate checks a manifest's structure with ValidateStructure. Then it compiles each inline query
// against the vocabulary the review will run with. A malformed or misspelled query then fails before
// anything runs, rather than reading as an empty answer.
//
// Validate is exported separately from Load for two reasons. A manifest travels as a request VALUE
// (C22, WS9-050), so a browser form or a test may build one directly. And only the caller knows which
// vocabulary the run will have. Load reads a manifest before any design or project is known, so it
// cannot know the vocabulary.
func Validate(m Manifest, opts ...ValidateOption) error {
	var c validateConfig
	for _, o := range opts {
		o(&c)
	}
	if err := ValidateStructure(m); err != nil {
		return err
	}
	for _, a := range m.Areas {
		for _, it := range a.Items {
			if it.Binding.Query == nil {
				continue
			}
			if _, err := compileQuery(it, c.vocabulary); err != nil {
				return fmt.Errorf("review manifest item %q: %w", it.ID, err)
			}
		}
	}
	return nil
}

// ValidateStructure checks what a manifest says, and compiles none of its queries. The manifest needs
// a name and at least one area, and each area needs a name. Each item needs an ID and at most one
// binding. A scope or a requirement needs the selector it narrows. Each inline query needs
// its required fields. Load calls ValidateStructure, and so does a caller describing a manifest with
// no run in view. Without this check, an item with both a rule and a profile would resolve to
// whichever one the runner tested for first.
func ValidateStructure(m Manifest) error {
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
			if q := it.Binding.Query; q != nil {
				if strings.TrimSpace(q.Match) == "" || strings.TrimSpace(q.Subject) == "" || strings.TrimSpace(q.Message) == "" {
					return fmt.Errorf("review manifest item %q: a query binding needs \"match\", \"subject\", and \"message\"", it.ID)
				}
			}
			if it.Binding.Present != nil && strings.TrimSpace(it.Binding.Present.Class) == "" {
				return fmt.Errorf("review manifest item %q: a present binding needs \"class\"", it.ID)
			}
		}
	}
	return nil
}

// compileQuery turns an item's inline QueryBinding into a check.Rule against vocab. A nil vocab means
// the process default. Validate and Run both call it. It requires match, subject and message. Kind
// defaults to component and severity to warning. It never parses the query. The registered compiler
// does that.
func compileQuery(it Item, vocab *facts.Registry) (*check.Rule, error) {
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
		Vocabulary:  vocab,
	})
}
