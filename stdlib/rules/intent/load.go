package intent

import (
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// declarationDoc is the YAML wire shape of a Declaration. It is a DTO separate from the domain type so
// Declaration carries no yaml tags and the file can nest (voltage_domains: [{name, nominal, rails}])
// where the domain struct stays plain. A customer authors one of these in their overlay.
type declarationDoc struct {
	Name           string           `yaml:"name"`
	Modules        []moduleDoc      `yaml:"modules"`
	VoltageDomains []vDomainDoc     `yaml:"voltage_domains"`
	Subsystems     []subsystemDoc   `yaml:"subsystems"`
	Protections    []protectionDoc  `yaml:"protections"`
	NetProperties  []netPropertyDoc `yaml:"net_properties"`
	RailBudgets    []railBudgetDoc  `yaml:"rail_budgets"`
	MarginFactor   float64          `yaml:"margin_factor"`
}

type moduleDoc struct {
	Name  string `yaml:"name"`
	Class string `yaml:"class"`
	MPN   string `yaml:"mpn"`
	Count int    `yaml:"count"`
}

type vDomainDoc struct {
	Name    string   `yaml:"name"`
	Nominal float64  `yaml:"nominal"`
	Rails   []string `yaml:"rails"`
}

type subsystemDoc struct {
	Name   string     `yaml:"name"`
	Source *moduleDoc `yaml:"source"`
	Nets   []string   `yaml:"nets"`
}

type protectionDoc struct {
	Rail string `yaml:"rail"`
	Kind string `yaml:"kind"`
}

type railBudgetDoc struct {
	Rail string  `yaml:"rail"`
	Peak float64 `yaml:"peak"`
}

type netPropertyDoc struct {
	Net      string `yaml:"net"`
	Property string `yaml:"property"`
	Value    string `yaml:"value"`
}

// Parse reads a YAML intent declaration into a Declaration and validates its structure: a name is
// present, and the declaration is not empty (at least one module, voltage domain, subsystem, or
// protection). Each module needs a class or an mpn (matching nothing otherwise), each voltage domain
// needs a name, a positive nominal, and at least one rail, each subsystem needs a name (slugifying
// uniquely) plus a source or at least one net, and each protection needs a rail and a known kind (ovp
// or discharge), each rail_budget needs a rail and a positive peak with no rail budgeted twice, and a
// declared margin_factor must exceed 1 and have budgets to apply to.
// A malformed declaration is a teaching error at load, not a surprise at run. Parse is
// WASM-clean (yaml only, no os); LoadFile adds the file read.
func Parse(b []byte) (Declaration, error) {
	var doc declarationDoc
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return Declaration{}, fmt.Errorf("intent: invalid YAML: %w", err)
	}
	if strings.TrimSpace(doc.Name) == "" {
		return Declaration{}, fmt.Errorf("intent: missing required field \"name\"")
	}
	if len(doc.Modules) == 0 && len(doc.VoltageDomains) == 0 && len(doc.Subsystems) == 0 && len(doc.Protections) == 0 &&
		len(doc.NetProperties) == 0 && len(doc.RailBudgets) == 0 {
		return Declaration{}, fmt.Errorf("intent %q: declares no modules, voltage_domains, subsystems, protections, net_properties, or rail_budgets", doc.Name)
	}
	d := Declaration{Name: doc.Name}
	for i, m := range doc.Modules {
		if strings.TrimSpace(m.Name) == "" {
			return Declaration{}, fmt.Errorf("intent %q: module #%d is missing its \"name\"", doc.Name, i+1)
		}
		if strings.TrimSpace(m.Class) == "" && strings.TrimSpace(m.MPN) == "" {
			return Declaration{}, fmt.Errorf("intent %q: module %q needs a \"class\" or an \"mpn\"", doc.Name, m.Name)
		}
		if m.Count < 0 {
			return Declaration{}, fmt.Errorf("intent %q: module %q has a negative \"count\" %d", doc.Name, m.Name, m.Count)
		}
		d.Modules = append(d.Modules, Module{Name: m.Name, Class: m.Class, MPN: m.MPN, Count: m.Count})
	}
	for i, v := range doc.VoltageDomains {
		if strings.TrimSpace(v.Name) == "" {
			return Declaration{}, fmt.Errorf("intent %q: voltage domain #%d is missing its \"name\"", doc.Name, i+1)
		}
		if v.Nominal <= 0 {
			return Declaration{}, fmt.Errorf("intent %q: voltage domain %q needs a positive \"nominal\"", doc.Name, v.Name)
		}
		if len(v.Rails) == 0 {
			return Declaration{}, fmt.Errorf("intent %q: voltage domain %q needs at least one rail", doc.Name, v.Name)
		}
		d.VoltageDomains = append(d.VoltageDomains, VoltageDomain{Name: v.Name, Nominal: v.Nominal, Rails: v.Rails})
	}
	slugs := map[string]string{} // slug -> first name, to reject a rule-name collision
	for i, s := range doc.Subsystems {
		if strings.TrimSpace(s.Name) == "" {
			return Declaration{}, fmt.Errorf("intent %q: subsystem #%d is missing its \"name\"", doc.Name, i+1)
		}
		if s.Source == nil && len(s.Nets) == 0 {
			return Declaration{}, fmt.Errorf("intent %q: subsystem %q needs a \"source\" or at least one \"nets\" entry", doc.Name, s.Name)
		}
		sl := slug(s.Name)
		if sl == "" {
			return Declaration{}, fmt.Errorf("intent %q: subsystem name %q has no alphanumeric characters to form a rule name", doc.Name, s.Name)
		}
		if first, dup := slugs[sl]; dup {
			return Declaration{}, fmt.Errorf("intent %q: subsystems %q and %q slugify to the same rule name %q", doc.Name, first, s.Name, "intent/subsystem-"+sl)
		}
		slugs[sl] = s.Name
		sub := Subsystem{Name: s.Name, Nets: s.Nets}
		if s.Source != nil {
			if strings.TrimSpace(s.Source.Class) == "" && strings.TrimSpace(s.Source.MPN) == "" {
				return Declaration{}, fmt.Errorf("intent %q: subsystem %q source needs a \"class\" or an \"mpn\"", doc.Name, s.Name)
			}
			sub.Source = &Module{Name: s.Name + " source", Class: s.Source.Class, MPN: s.Source.MPN}
		}
		d.Subsystems = append(d.Subsystems, sub)
	}
	for i, p := range doc.Protections {
		if strings.TrimSpace(p.Rail) == "" {
			return Declaration{}, fmt.Errorf("intent %q: protection #%d is missing its \"rail\"", doc.Name, i+1)
		}
		if p.Kind != ProtectionOVP && p.Kind != ProtectionDischarge {
			return Declaration{}, fmt.Errorf("intent %q: protection for rail %q has kind %q (want %q or %q)", doc.Name, p.Rail, p.Kind, ProtectionOVP, ProtectionDischarge)
		}
		d.Protections = append(d.Protections, Protection{Rail: p.Rail, Kind: p.Kind})
	}
	for i, np := range doc.NetProperties {
		if strings.TrimSpace(np.Net) == "" {
			return Declaration{}, fmt.Errorf("intent %q: net_property #%d is missing its \"net\"", doc.Name, i+1)
		}
		switch np.Property {
		case PropACCoupled:
			if strings.TrimSpace(np.Value) != "" {
				return Declaration{}, fmt.Errorf("intent %q: net_property %q kind %q takes no \"value\" (got %q)", doc.Name, np.Net, np.Property, np.Value)
			}
		case PropResetPolarity, PropStrap:
			// The value is the assertion. Without it the rule has nothing to contradict, so an
			// omitted or misspelled level is a load error rather than a rule that silently never fires.
			if np.Value != "low" && np.Value != "high" {
				return Declaration{}, fmt.Errorf("intent %q: net_property %q kind %q needs \"value\" of \"low\" or \"high\" (got %q)", doc.Name, np.Net, np.Property, np.Value)
			}
		default:
			return Declaration{}, fmt.Errorf("intent %q: net_property %q has property %q (want %q or %q)", doc.Name, np.Net, np.Property, PropResetPolarity, PropACCoupled)
		}
		d.NetProperties = append(d.NetProperties, NetProperty{Net: np.Net, Property: np.Property, Value: np.Value})
	}
	rails := map[string]bool{} // one budget per rail, so two declarations cannot both fire on one net
	for i, rb := range doc.RailBudgets {
		if strings.TrimSpace(rb.Rail) == "" {
			return Declaration{}, fmt.Errorf("intent %q: rail_budget #%d is missing its \"rail\"", doc.Name, i+1)
		}
		// A zero or negative peak is satisfied by every supply, so it would be a declaration that can
		// only ever pass. Rejecting it at load is the same discipline the "value" checks above use.
		if rb.Peak <= 0 {
			return Declaration{}, fmt.Errorf("intent %q: rail_budget %q needs a positive \"peak\" current in amps (got %g)", doc.Name, rb.Rail, rb.Peak)
		}
		if rails[rb.Rail] {
			return Declaration{}, fmt.Errorf("intent %q: rail %q has more than one rail_budget", doc.Name, rb.Rail)
		}
		rails[rb.Rail] = true
		d.RailBudgets = append(d.RailBudgets, RailBudget{Rail: rb.Rail, Peak: rb.Peak})
	}
	// margin_factor is optional and has no default (see Declaration.MarginFactor). Omitted, the margin
	// rule is never compiled. Declared, it must ask for headroom: a factor of 1 restates the capacity
	// rule and anything below 1 asks for a supply SMALLER than the budget, so both are author errors
	// caught here rather than a second rule that duplicates or inverts the first.
	if doc.MarginFactor != 0 && doc.MarginFactor <= 1 {
		return Declaration{}, fmt.Errorf("intent %q: \"margin_factor\" must be greater than 1 (got %g); omit it to leave the margin rule uncompiled", doc.Name, doc.MarginFactor)
	}
	if doc.MarginFactor != 0 && len(doc.RailBudgets) == 0 {
		return Declaration{}, fmt.Errorf("intent %q: \"margin_factor\" is declared with no rail_budgets, so nothing applies it", doc.Name)
	}
	d.MarginFactor = doc.MarginFactor
	return d, nil
}

// Load reads a YAML intent declaration from r and parses+validates it (Parse). It is the io.Reader
// entry point; LoadFile wraps it with the os file read for the --intent-path flag.
func Load(r io.Reader) (Declaration, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return Declaration{}, err
	}
	return Parse(b)
}
