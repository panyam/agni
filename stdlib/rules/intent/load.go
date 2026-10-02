package intent

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// A design's intent is a section of its descriptor (agni issue 824):
//
//	name: gateway
//	entry: gateway.edn
//	intent:
//	  modules:      [...]   # what the board must contain: a block by class or MPN, with an optional
//	                        # count, or a named block that must instantiate nets
//	  nets:         {...}   # facts about named nets: nominal voltage, peak current, protection,
//	                        # reset polarity, strap level, AC coupling
//	  sequences:    [...]   # power-up order
//	  strap_groups: [...]   # several strap nets read as one number
//	  io_map:       [...]   # which net lands on which pin
//	  margin_factor: 1.25   # headroom over a declared peak, optional
//
// Parse reads that shape whether it comes from a whole design.yaml or from a file holding only `name`
// and `intent:` (the --intent-path flag), so there is one schema. The six forms compile to exactly the
// rules the nine earlier ones did, under the same names, so a review checklist binding
// intent/voltage-domain-mismatch or intent/subsystem-<slug> is unaffected.

// designDoc is the part of a design descriptor Parse reads. The descriptor's other keys (entry,
// companions, symbols) are the project store's, so they are not decoded here.
type designDoc struct {
	Name   string    `yaml:"name"`
	Intent yaml.Node `yaml:"intent"`
}

// sectionDoc is the YAML wire shape of the intent section. It is a DTO separate from the domain type
// so Declaration carries no yaml tags and the file can be keyed by net while the domain stays plain.
type sectionDoc struct {
	Modules      []moduleDoc       `yaml:"modules"`
	Nets         map[string]netDoc `yaml:"nets"`
	Sequences    []sequenceDoc     `yaml:"sequences"`
	StrapGroups  []strapGroupDoc   `yaml:"strap_groups"`
	IOMap        []ioAssignmentDoc `yaml:"io_map"`
	MarginFactor float64           `yaml:"margin_factor"`
}

type ioAssignmentDoc struct {
	Net      string         `yaml:"net"`
	Device   string         `yaml:"device"`
	Pin      string         `yaml:"pin"`
	Function string         `yaml:"function"`
	To       *ioEndpointDoc `yaml:"to"`
}

type ioEndpointDoc struct {
	Device string `yaml:"device"`
	Pin    string `yaml:"pin"`
}

type sequenceDoc struct {
	Name     string             `yaml:"name"`
	Relation string             `yaml:"relation"`
	Order    []sequenceStageDoc `yaml:"order"`
}

type sequenceStageDoc struct {
	Rail   string `yaml:"rail"`
	Good   string `yaml:"good"`
	Enable string `yaml:"enable"`
}

// moduleDoc is one block the board must contain. Without nets it is a module, checked by class or MPN
// with an optional count. With nets it is a named block whose nets must all exist, and a class or MPN
// then names a component that must be present too (what was a subsystem and its source).
type moduleDoc struct {
	Name  string   `yaml:"name"`
	Class string   `yaml:"class"`
	MPN   string   `yaml:"mpn"`
	Count int      `yaml:"count"`
	Nets  []string `yaml:"nets"`
}

// netDoc is everything a design declares about one net. Each field compiles to the rule the earlier
// per-kind forms did: nominal (and domain) to voltage domains, peak to rail budgets, protect to
// protections, and reset, strap and ac_coupled to net properties.
type netDoc struct {
	Nominal   *float64 `yaml:"nominal"`
	Domain    string   `yaml:"domain"`
	Peak      *float64 `yaml:"peak"`
	Protect   []string `yaml:"protect"`
	Reset     string   `yaml:"reset"`
	Strap     string   `yaml:"strap"`
	MinOhms   float64  `yaml:"min_ohms"`
	MaxOhms   float64  `yaml:"max_ohms"`
	ACCoupled bool     `yaml:"ac_coupled"`
}

type strapGroupDoc struct {
	Name    string   `yaml:"name"`
	Device  string   `yaml:"device"`
	Nets    []string `yaml:"nets"`
	Value   int      `yaml:"value"`
	Bus     string   `yaml:"bus"`
	Default string   `yaml:"default"`
}

// earlierForms names the forms the six replaced, and what each became, so a file written for the
// earlier layout fails with the spelling to use rather than reading as a design with no intent.
var earlierForms = map[string]string{
	"voltage_domains": "nets: {RAIL: {nominal: VOLTS, domain: NAME}}",
	"protections":     "nets: {RAIL: {protect: [ovp, discharge]}}",
	"net_properties":  "nets: {NET: {reset: low|high, strap: low|high, ac_coupled: true}}",
	"rail_budgets":    "nets: {RAIL: {peak: AMPS}}",
	"subsystems":      "modules: [{name: NAME, nets: [...], class or mpn for its source}]",
}

// intentKeys are the intent section's keys, for recognising a file that puts them at the top level.
var intentKeys = []string{"modules", "nets", "sequences", "strap_groups", "io_map", "margin_factor",
	"voltage_domains", "protections", "net_properties", "rail_budgets", "subsystems"}

// Parse reads a design's intent from its descriptor (or a file of the same shape) into a Declaration
// and validates it. The declaration takes the design's name. A malformed declaration fails at load,
// where the error can teach, rather than at run. Parse is WASM-clean (yaml only, no os); LoadFile adds
// the file read.
//
// Every form is checked for something it can evaluate: a module needs a class, an MPN or nets; a net
// needs at least one fact, and each fact a value its kind accepts; a sequence needs an order the
// netlist can be checked against; a strap group needs distinct nets that can encode its value; an
// io_map row needs a net, a device and a pin.
func Parse(b []byte) (Declaration, error) {
	var top map[string]yaml.Node
	if err := yaml.Unmarshal(b, &top); err != nil {
		return Declaration{}, fmt.Errorf("intent: invalid YAML: %w", err)
	}
	for _, k := range intentKeys {
		if _, ok := top[k]; ok {
			return Declaration{}, fmt.Errorf("intent: %q is at the top level; a design's intent now sits under \"intent:\" in its design.yaml (agni issue 824)", k)
		}
	}
	var doc designDoc
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return Declaration{}, fmt.Errorf("intent: invalid YAML: %w", err)
	}
	if strings.TrimSpace(doc.Name) == "" {
		return Declaration{}, fmt.Errorf("intent: missing required field \"name\"")
	}
	if doc.Intent.Kind == 0 {
		return Declaration{}, fmt.Errorf("intent %q: has no \"intent:\" section", doc.Name)
	}
	if doc.Intent.Kind == yaml.ScalarNode {
		return Declaration{}, fmt.Errorf("intent %q: \"intent:\" names a file (%q); intent is now declared inline in design.yaml (agni issue 824)", doc.Name, doc.Intent.Value)
	}
	if doc.Intent.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(doc.Intent.Content); i += 2 {
			k := doc.Intent.Content[i].Value
			if hint, ok := earlierForms[k]; ok {
				return Declaration{}, fmt.Errorf("intent %q: %q is no longer a form; write it as %s (agni issue 824)", doc.Name, k, hint)
			}
		}
	}
	var in sectionDoc
	var buf bytes.Buffer
	if err := yaml.NewEncoder(&buf).Encode(&doc.Intent); err != nil {
		return Declaration{}, fmt.Errorf("intent %q: %w", doc.Name, err)
	}
	dec := yaml.NewDecoder(&buf)
	dec.KnownFields(true) // a misspelled key is an error, not a fact that silently declares nothing
	if err := dec.Decode(&in); err != nil {
		return Declaration{}, fmt.Errorf("intent %q: %w", doc.Name, err)
	}
	return build(doc.Name, in)
}

// build validates the decoded section and converts it to a Declaration.
func build(name string, in sectionDoc) (Declaration, error) {
	if len(in.Modules) == 0 && len(in.Nets) == 0 && len(in.Sequences) == 0 && len(in.StrapGroups) == 0 && len(in.IOMap) == 0 {
		return Declaration{}, fmt.Errorf("intent %q: declares no modules, nets, sequences, strap_groups or io_map", name)
	}
	d := Declaration{Name: name}
	if err := buildModules(name, in.Modules, &d); err != nil {
		return Declaration{}, err
	}
	if err := buildNets(name, in.Nets, &d); err != nil {
		return Declaration{}, err
	}
	if err := buildStrapGroups(name, in.StrapGroups, &d); err != nil {
		return Declaration{}, err
	}
	seqSlugs := map[string]string{} // slug -> first name, to reject a rule-name collision
	for i, s := range in.Sequences {
		seq, err := parseSequence(name, i, s, seqSlugs)
		if err != nil {
			return Declaration{}, err
		}
		d.Sequences = append(d.Sequences, seq)
	}
	// margin_factor is optional and has no default (see Declaration.MarginFactor). Omitted, the margin
	// rule is never compiled. Declared, it must ask for headroom. A factor of 1 restates the capacity
	// rule and anything below 1 asks for a supply SMALLER than the budget, so both are author errors
	// caught here rather than a second rule that duplicates or inverts the first.
	if in.MarginFactor != 0 && in.MarginFactor <= 1 {
		return Declaration{}, fmt.Errorf("intent %q: \"margin_factor\" must be greater than 1 (got %g); omit it to leave the margin rule uncompiled", name, in.MarginFactor)
	}
	if in.MarginFactor != 0 && len(d.RailBudgets) == 0 {
		return Declaration{}, fmt.Errorf("intent %q: \"margin_factor\" is declared with no net declaring a \"peak\", so nothing applies it", name)
	}
	d.MarginFactor = in.MarginFactor
	for i, a := range in.IOMap {
		asg, err := parseIOAssignment(name, i, a)
		if err != nil {
			return Declaration{}, err
		}
		d.IOMap = append(d.IOMap, asg)
	}
	return d, nil
}

// buildModules splits the modules list into the two shapes it carries: a module checked by class or
// MPN with an optional count, and a named block whose nets must exist (a subsystem), whose class or
// MPN, when given, names a component that must be present too.
func buildModules(name string, ms []moduleDoc, d *Declaration) error {
	slugs := map[string]string{} // slug -> first name, to reject a rule-name collision
	for i, m := range ms {
		if strings.TrimSpace(m.Name) == "" {
			return fmt.Errorf("intent %q: module #%d is missing its \"name\"", name, i+1)
		}
		hasPart := strings.TrimSpace(m.Class) != "" || strings.TrimSpace(m.MPN) != ""
		if len(m.Nets) == 0 {
			if !hasPart {
				return fmt.Errorf("intent %q: module %q needs a \"class\", an \"mpn\" or \"nets\"", name, m.Name)
			}
			if m.Count < 0 {
				return fmt.Errorf("intent %q: module %q has a negative \"count\" %d", name, m.Name, m.Count)
			}
			d.Modules = append(d.Modules, Module{Name: m.Name, Class: m.Class, MPN: m.MPN, Count: m.Count})
			continue
		}
		// A block with nets is checked as a whole (one rule, intent/subsystem-<slug>), so a count on it
		// would compile to nothing.
		if m.Count != 0 {
			return fmt.Errorf("intent %q: module %q declares nets and a \"count\"; a block with nets is checked as one block, so declare the count on a module of its own", name, m.Name)
		}
		sl := slug(m.Name)
		if sl == "" {
			return fmt.Errorf("intent %q: module name %q has no alphanumeric characters to form a rule name", name, m.Name)
		}
		if first, dup := slugs[sl]; dup {
			return fmt.Errorf("intent %q: modules %q and %q slugify to the same rule name %q", name, first, m.Name, "intent/subsystem-"+sl)
		}
		slugs[sl] = m.Name
		sub := Subsystem{Name: m.Name, Nets: m.Nets}
		if hasPart {
			sub.Source = &Module{Name: m.Name + " source", Class: m.Class, MPN: m.MPN}
		}
		d.Subsystems = append(d.Subsystems, sub)
	}
	return nil
}

// buildNets converts the per-net facts into the declaration's per-kind lists. Nets are taken in name
// order, so a declaration compiles the same way however the YAML map is written. Rails sharing a
// nominal voltage and a domain label form one voltage domain; without a label the domain is named by
// its voltage ("3.3V").
func buildNets(name string, nets map[string]netDoc, d *Declaration) error {
	names := make([]string, 0, len(nets))
	for n := range nets {
		names = append(names, n)
	}
	sort.Strings(names)
	domains := map[string]int{} // domain name -> index in d.VoltageDomains
	for _, net := range names {
		nd := nets[net]
		if strings.TrimSpace(net) == "" {
			return fmt.Errorf("intent %q: a net under \"nets\" has an empty name", name)
		}
		declared := false
		if nd.Nominal != nil {
			declared = true
			if *nd.Nominal <= 0 {
				return fmt.Errorf("intent %q: net %q needs a positive \"nominal\" voltage (got %g)", name, net, *nd.Nominal)
			}
			dom := strings.TrimSpace(nd.Domain)
			if dom == "" {
				dom = fmt.Sprintf("%gV", *nd.Nominal)
			}
			if i, ok := domains[dom]; ok {
				if d.VoltageDomains[i].Nominal != *nd.Nominal {
					return fmt.Errorf("intent %q: domain %q is declared at %gV and at %gV (net %q)", name, dom, d.VoltageDomains[i].Nominal, *nd.Nominal, net)
				}
				d.VoltageDomains[i].Rails = append(d.VoltageDomains[i].Rails, net)
			} else {
				domains[dom] = len(d.VoltageDomains)
				d.VoltageDomains = append(d.VoltageDomains, VoltageDomain{Name: dom, Nominal: *nd.Nominal, Rails: []string{net}})
			}
		} else if strings.TrimSpace(nd.Domain) != "" {
			return fmt.Errorf("intent %q: net %q names a \"domain\" with no \"nominal\" voltage", name, net)
		}
		if nd.Peak != nil {
			declared = true
			// A zero or negative peak is satisfied by every supply, so it would be a declaration that
			// can only ever pass. Rejecting it at load is the same discipline the value checks use.
			if *nd.Peak <= 0 {
				return fmt.Errorf("intent %q: net %q needs a positive \"peak\" current in amps (got %g)", name, net, *nd.Peak)
			}
			d.RailBudgets = append(d.RailBudgets, RailBudget{Rail: net, Peak: *nd.Peak})
		}
		seenKind := map[string]bool{}
		for _, k := range nd.Protect {
			declared = true
			if k != ProtectionOVP && k != ProtectionDischarge {
				return fmt.Errorf("intent %q: net %q has protection %q (want %q or %q)", name, net, k, ProtectionOVP, ProtectionDischarge)
			}
			if seenKind[k] {
				return fmt.Errorf("intent %q: net %q lists protection %q twice", name, net, k)
			}
			seenKind[k] = true
			d.Protections = append(d.Protections, Protection{Rail: net, Kind: k})
		}
		// reset and strap are assertions about a level, so an omitted or misspelled level is a load
		// error rather than a rule that silently never fires.
		if nd.Reset != "" {
			declared = true
			if nd.Reset != "low" && nd.Reset != "high" {
				return fmt.Errorf("intent %q: net %q has reset %q (want \"low\" or \"high\")", name, net, nd.Reset)
			}
			d.NetProperties = append(d.NetProperties, NetProperty{Net: net, Property: PropResetPolarity, Value: nd.Reset})
		}
		if nd.Strap != "" {
			declared = true
			if nd.Strap != "low" && nd.Strap != "high" {
				return fmt.Errorf("intent %q: net %q has strap %q (want \"low\" or \"high\")", name, net, nd.Strap)
			}
			if nd.MinOhms < 0 || nd.MaxOhms < 0 {
				return fmt.Errorf("intent %q: net %q has a negative resistance bound (min_ohms %g, max_ohms %g)", name, net, nd.MinOhms, nd.MaxOhms)
			}
			if nd.MinOhms > 0 && nd.MaxOhms > 0 && nd.MinOhms > nd.MaxOhms {
				return fmt.Errorf("intent %q: net %q has min_ohms %g above max_ohms %g, a band nothing can satisfy", name, net, nd.MinOhms, nd.MaxOhms)
			}
			d.NetProperties = append(d.NetProperties, NetProperty{Net: net, Property: PropStrap, Value: nd.Strap, MinOhms: nd.MinOhms, MaxOhms: nd.MaxOhms})
		} else if nd.MinOhms != 0 || nd.MaxOhms != 0 {
			// A band with no strap has no resistance to bound, and would compile to a check that can
			// never run, a declaration meaning nothing. Reject it rather than let it pass at review.
			return fmt.Errorf("intent %q: net %q declares min_ohms/max_ohms with no \"strap\"", name, net)
		}
		if nd.ACCoupled {
			declared = true
			d.NetProperties = append(d.NetProperties, NetProperty{Net: net, Property: PropACCoupled})
		}
		if !declared {
			return fmt.Errorf("intent %q: net %q declares nothing (nominal, peak, protect, reset, strap or ac_coupled)", name, net)
		}
	}
	return nil
}

// buildStrapGroups validates and converts the strap groups.
func buildStrapGroups(name string, gs []strapGroupDoc, d *Declaration) error {
	groupSlugs := map[string]string{}
	for i, g := range gs {
		if strings.TrimSpace(g.Name) == "" {
			return fmt.Errorf("intent %q: strap_group #%d is missing its \"name\"", name, i+1)
		}
		if len(g.Nets) == 0 {
			return fmt.Errorf("intent %q: strap_group %q lists no \"nets\"; a group with no bits encodes nothing to check", name, g.Name)
		}
		// The declared value has to be representable in the bits declared, or the group can never
		// encode it and the rule would fail on every design including a correct one. That is an
		// authoring error, so it is rejected here rather than reported as a design finding.
		if max := 1<<len(g.Nets) - 1; g.Value < 0 || g.Value > max {
			return fmt.Errorf("intent %q: strap_group %q declares value %d, which %d net(s) cannot encode (range 0..%d)", name, g.Name, g.Value, len(g.Nets), max)
		}
		if g.Default != "" && g.Default != "low" && g.Default != "high" {
			return fmt.Errorf("intent %q: strap_group %q has default %q (want \"low\", \"high\", or omitted)", name, g.Name, g.Default)
		}
		seenNet := map[string]bool{}
		for _, n := range g.Nets {
			if seenNet[n] {
				return fmt.Errorf("intent %q: strap_group %q lists net %q twice; one net cannot be two bits of the same number", name, g.Name, n)
			}
			seenNet[n] = true
		}
		sl := slug(g.Name)
		if sl == "" {
			return fmt.Errorf("intent %q: strap_group name %q has no alphanumeric characters to form a rule name", name, g.Name)
		}
		if first, dup := groupSlugs[sl]; dup {
			return fmt.Errorf("intent %q: strap_groups %q and %q slugify to the same rule name %q", name, first, g.Name, "intent/strap-group-"+sl)
		}
		groupSlugs[sl] = g.Name
		d.StrapGroups = append(d.StrapGroups, StrapGroup{
			Name: g.Name, Device: g.Device, Nets: g.Nets, Value: g.Value, Bus: g.Bus, Default: g.Default,
		})
	}
	return nil
}

// parseIOAssignment validates one declared pin-map row.
//
// The three required fields are required because a row missing any of them states nothing checkable.
// Without a net there is no assignment, without a device there is nothing to look the pin up on, and
// without a pin the row is a net-presence claim that io-map-net-absent already makes for every row.
//
// A far end must be COMPLETE or absent. A `to` naming a device and no pin is the shape that would
// otherwise reach the rule as a half-question, and the rule would have to invent a reading of it.
// Rejecting it at load is the same discipline the rail-budget and strap-group checks above use.
func parseIOAssignment(declName string, i int, a ioAssignmentDoc) (IOAssignment, error) {
	row := fmt.Sprintf("io_map #%d", i+1)
	if strings.TrimSpace(a.Net) == "" {
		return IOAssignment{}, fmt.Errorf("intent %q: %s is missing its \"net\"", declName, row)
	}
	row = fmt.Sprintf("io_map row for net %q", a.Net)
	if strings.TrimSpace(a.Device) == "" {
		return IOAssignment{}, fmt.Errorf("intent %q: %s is missing its \"device\"", declName, row)
	}
	if strings.TrimSpace(a.Pin) == "" {
		return IOAssignment{}, fmt.Errorf("intent %q: %s is missing its \"pin\"", declName, row)
	}
	asg := IOAssignment{
		Net: strings.TrimSpace(a.Net), Device: strings.TrimSpace(a.Device),
		Pin: strings.TrimSpace(a.Pin), Function: strings.TrimSpace(a.Function),
	}
	if a.To != nil {
		if strings.TrimSpace(a.To.Device) == "" || strings.TrimSpace(a.To.Pin) == "" {
			return IOAssignment{}, fmt.Errorf("intent %q: %s declares a \"to\" with %s; a far end needs both a device and a pin",
				declName, row, missingHalf(a.To))
		}
		asg.To = &IOEndpoint{Device: strings.TrimSpace(a.To.Device), Pin: strings.TrimSpace(a.To.Pin)}
	}
	return asg, nil
}

func missingHalf(e *ioEndpointDoc) string {
	switch {
	case strings.TrimSpace(e.Device) == "" && strings.TrimSpace(e.Pin) == "":
		return "neither a device nor a pin"
	case strings.TrimSpace(e.Device) == "":
		return "no device"
	default:
		return "no pin"
	}
}

// parseSequence validates one declared sequence and converts it. It is split out of Parse because it
// carries the one validation in this file that is about EVALUABILITY rather than shape. A sequence
// with no adjacent good/enable pair compiles to a rule with nothing to judge, and a rule that can only
// ever pass is author error. It is caught at load (WS3-099), where the message can teach, rather than
// at run as a verdict nobody can trace back to the declaration.
//
// The teaching matters more here than elsewhere, because the case it rejects is a real and correct
// board, one whose rail order lives in a PMIC's configuration or in firmware. Saying so in the error
// stops an author from inventing net names to satisfy the schema.
func parseSequence(declName string, i int, s sequenceDoc, slugs map[string]string) (Sequence, error) {
	if strings.TrimSpace(s.Name) == "" {
		return Sequence{}, fmt.Errorf("intent %q: sequence #%d is missing its \"name\"", declName, i+1)
	}
	if s.Relation != SequenceEnableGated {
		return Sequence{}, fmt.Errorf("intent %q: sequence %q has relation %q (want %q, the only ordering a netlist evidences)", declName, s.Name, s.Relation, SequenceEnableGated)
	}
	sl := slug(s.Name)
	if sl == "" {
		return Sequence{}, fmt.Errorf("intent %q: sequence name %q has no alphanumeric characters to form a rule name", declName, s.Name)
	}
	if first, dup := slugs[sl]; dup {
		return Sequence{}, fmt.Errorf("intent %q: sequences %q and %q slugify to the same rule name %q", declName, first, s.Name, "intent/sequence-"+sl)
	}
	slugs[sl] = s.Name
	if len(s.Order) < 2 {
		return Sequence{}, fmt.Errorf("intent %q: sequence %q needs at least two stages in \"order\" (an order of one has nothing to come before)", declName, s.Name)
	}
	seq := Sequence{Name: s.Name, Relation: s.Relation}
	rails := map[string]bool{}
	for j, st := range s.Order {
		if strings.TrimSpace(st.Rail) == "" {
			return Sequence{}, fmt.Errorf("intent %q: sequence %q stage #%d is missing its \"rail\"", declName, s.Name, j+1)
		}
		if rails[st.Rail] {
			return Sequence{}, fmt.Errorf("intent %q: sequence %q lists rail %q twice, so its position in the order is ambiguous", declName, s.Name, st.Rail)
		}
		rails[st.Rail] = true
		seq.Order = append(seq.Order, SequenceStage{Rail: st.Rail, Good: st.Good, Enable: st.Enable})
	}
	if !hasGatingPair(seq) {
		return Sequence{}, fmt.Errorf("intent %q: sequence %q declares no adjacent \"good\" -> \"enable\" pair, so nothing in the netlist can be checked against it. "+
			"A rail order enforced inside a PMIC or by firmware leaves no trace in a netlist; record it as a review note rather than as a sequence", declName, s.Name)
	}
	return seq, nil
}

// hasGatingPair reports whether any adjacent pair of stages supplies both handles the enable-gated
// relation reads. It is the compiles-to-something test, and the same predicate Compile uses, so a
// sequence that loads always yields a rule with at least one link to judge.
func hasGatingPair(s Sequence) bool {
	for i := 0; i+1 < len(s.Order); i++ {
		if s.Order[i].Good != "" && s.Order[i+1].Enable != "" {
			return true
		}
	}
	return false
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
