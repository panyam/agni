package intent

import (
	"fmt"
	"io"
	"sort"
	"strings"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/internal/yamlpb"
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
// and `intent:` (the --intent-path flag). The section's schema is the configpb.DesignIntent message
// (CONSTRAINTS C26), which YAML only spells, so FromProto is the one place a declaration is validated
// whether it arrived as a file or as a message. The six forms compile to exactly the
// rules the nine earlier ones did, under the same names, so a review checklist binding
// intent/voltage-domain-mismatch or intent/subsystem-<slug> is unaffected.

// designDoc is the part of a design descriptor Parse reads. The descriptor's other keys (entry,
// companions, symbols) are the project store's, so they are not decoded here.
type designDoc struct {
	Name   string    `yaml:"name"`
	Intent yaml.Node `yaml:"intent"`
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
	in, err := DecodeSection(&doc.Intent)
	if err != nil {
		return Declaration{}, fmt.Errorf("intent %q: %w", doc.Name, err)
	}
	return FromProto(doc.Name, in)
}

// DecodeSection binds a design descriptor's intent section to its proto, refusing the earlier
// layouts with the spelling to use instead. It checks SHAPE only (keys, types, the earlier forms);
// FromProto is what validates the declarations, so a store can read a descriptor without compiling
// rules from it.
func DecodeSection(n *yaml.Node) (*configpb.DesignIntent, error) {
	if n.Kind == yaml.ScalarNode {
		return nil, fmt.Errorf("intent names a file (%q); a design's intent is now declared inline under intent: in its design.yaml (agni issue 824)", n.Value)
	}
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i].Value
			if hint, ok := earlierForms[k]; ok {
				return nil, fmt.Errorf("%q is no longer a form; write it as %s (agni issue 824)", k, hint)
			}
		}
	}
	in := &configpb.DesignIntent{}
	// A misspelled key is an error naming its line, not a fact that silently declares nothing.
	if err := yamlpb.Decode(n, in); err != nil {
		return nil, err
	}
	return in, nil
}

// FromProto validates a design's declared intent and compiles it into the Declaration the rules
// read, under the design's name. It is the whole of validation, so a DesignIntent that arrived on the
// wire is held to exactly what a file is. A declaration stating nothing is an error, because a design
// that declares intent and checks nothing would read as covered.
func FromProto(name string, in *configpb.DesignIntent) (Declaration, error) {
	if len(in.GetModules()) == 0 && len(in.GetNets()) == 0 && len(in.GetSequences()) == 0 && len(in.GetStrapGroups()) == 0 && len(in.GetIoMap()) == 0 {
		return Declaration{}, fmt.Errorf("intent %q: declares no modules, nets, sequences, strap_groups or io_map", name)
	}
	d := Declaration{Name: name}
	if err := buildModules(name, in.GetModules(), &d); err != nil {
		return Declaration{}, err
	}
	if err := buildNets(name, in.GetNets(), &d); err != nil {
		return Declaration{}, err
	}
	if err := buildStrapGroups(name, in.GetStrapGroups(), &d); err != nil {
		return Declaration{}, err
	}
	seqSlugs := map[string]string{} // slug -> first name, to reject a rule-name collision
	for i, s := range in.GetSequences() {
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
	mf := in.GetMarginFactor()
	if mf != 0 && mf <= 1 {
		return Declaration{}, fmt.Errorf("intent %q: \"margin_factor\" must be greater than 1 (got %g); omit it to leave the margin rule uncompiled", name, mf)
	}
	if mf != 0 && len(d.RailBudgets) == 0 {
		return Declaration{}, fmt.Errorf("intent %q: \"margin_factor\" is declared with no net declaring a \"peak\", so nothing applies it", name)
	}
	d.MarginFactor = mf
	for i, a := range in.GetIoMap() {
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
func buildModules(name string, ms []*configpb.IntentModule, d *Declaration) error {
	slugs := map[string]string{} // slug -> first name, to reject a rule-name collision
	for i, m := range ms {
		if strings.TrimSpace(m.GetName()) == "" {
			return fmt.Errorf("intent %q: module #%d is missing its \"name\"", name, i+1)
		}
		hasPart := strings.TrimSpace(m.GetClass()) != "" || strings.TrimSpace(m.GetMpn()) != ""
		if len(m.GetNets()) == 0 {
			if !hasPart {
				return fmt.Errorf("intent %q: module %q needs a \"class\", an \"mpn\" or \"nets\"", name, m.GetName())
			}
			if m.GetCount() < 0 {
				return fmt.Errorf("intent %q: module %q has a negative \"count\" %d", name, m.GetName(), m.GetCount())
			}
			d.Modules = append(d.Modules, Module{Name: m.GetName(), Class: m.GetClass(), MPN: m.GetMpn(), Count: int(m.GetCount())})
			continue
		}
		// A block with nets is checked as a whole (one rule, intent/subsystem-<slug>), so a count on it
		// would compile to nothing.
		if m.GetCount() != 0 {
			return fmt.Errorf("intent %q: module %q declares nets and a \"count\"; a block with nets is checked as one block, so declare the count on a module of its own", name, m.GetName())
		}
		sl := slug(m.GetName())
		if sl == "" {
			return fmt.Errorf("intent %q: module name %q has no alphanumeric characters to form a rule name", name, m.GetName())
		}
		if first, dup := slugs[sl]; dup {
			return fmt.Errorf("intent %q: modules %q and %q slugify to the same rule name %q", name, first, m.GetName(), "intent/subsystem-"+sl)
		}
		slugs[sl] = m.GetName()
		sub := Subsystem{Name: m.GetName(), Nets: m.GetNets()}
		if hasPart {
			sub.Source = &Module{Name: m.GetName() + " source", Class: m.GetClass(), MPN: m.GetMpn()}
		}
		d.Subsystems = append(d.Subsystems, sub)
	}
	return nil
}

// buildNets converts the per-net facts into the declaration's per-kind lists. Nets are taken in name
// order, so a declaration compiles the same way however the YAML map is written. Rails sharing a
// nominal voltage and a domain label form one voltage domain; without a label the domain is named by
// its voltage ("3.3V").
func buildNets(name string, nets map[string]*configpb.NetIntent, d *Declaration) error {
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
			dom := strings.TrimSpace(nd.GetDomain())
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
		} else if strings.TrimSpace(nd.GetDomain()) != "" {
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
		for _, k := range nd.GetProtect() {
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
		if nd.GetReset_() != "" {
			declared = true
			if nd.GetReset_() != "low" && nd.GetReset_() != "high" {
				return fmt.Errorf("intent %q: net %q has reset %q (want \"low\" or \"high\")", name, net, nd.GetReset_())
			}
			d.NetProperties = append(d.NetProperties, NetProperty{Net: net, Property: PropResetPolarity, Value: nd.GetReset_()})
		}
		if nd.GetStrap() != "" {
			declared = true
			if nd.GetStrap() != "low" && nd.GetStrap() != "high" {
				return fmt.Errorf("intent %q: net %q has strap %q (want \"low\" or \"high\")", name, net, nd.GetStrap())
			}
			if nd.GetMinOhms() < 0 || nd.GetMaxOhms() < 0 {
				return fmt.Errorf("intent %q: net %q has a negative resistance bound (min_ohms %g, max_ohms %g)", name, net, nd.GetMinOhms(), nd.GetMaxOhms())
			}
			if nd.GetMinOhms() > 0 && nd.GetMaxOhms() > 0 && nd.GetMinOhms() > nd.GetMaxOhms() {
				return fmt.Errorf("intent %q: net %q has min_ohms %g above max_ohms %g, a band nothing can satisfy", name, net, nd.GetMinOhms(), nd.GetMaxOhms())
			}
			d.NetProperties = append(d.NetProperties, NetProperty{Net: net, Property: PropStrap, Value: nd.GetStrap(), MinOhms: nd.GetMinOhms(), MaxOhms: nd.GetMaxOhms()})
		} else if nd.GetMinOhms() != 0 || nd.GetMaxOhms() != 0 {
			// A band with no strap has no resistance to bound, and would compile to a check that can
			// never run, a declaration meaning nothing. Reject it rather than let it pass at review.
			return fmt.Errorf("intent %q: net %q declares min_ohms/max_ohms with no \"strap\"", name, net)
		}
		if nd.GetAcCoupled() {
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
func buildStrapGroups(name string, gs []*configpb.StrapGroup, d *Declaration) error {
	groupSlugs := map[string]string{}
	for i, g := range gs {
		if strings.TrimSpace(g.GetName()) == "" {
			return fmt.Errorf("intent %q: strap_group #%d is missing its \"name\"", name, i+1)
		}
		if len(g.GetNets()) == 0 {
			return fmt.Errorf("intent %q: strap_group %q lists no \"nets\"; a group with no bits encodes nothing to check", name, g.GetName())
		}
		// The declared value has to be representable in the bits declared, or the group can never
		// encode it and the rule would fail on every design including a correct one. That is an
		// authoring error, so it is rejected here rather than reported as a design finding.
		if max := 1<<len(g.GetNets()) - 1; g.GetValue() < 0 || int(g.GetValue()) > max {
			return fmt.Errorf("intent %q: strap_group %q declares value %d, which %d net(s) cannot encode (range 0..%d)", name, g.GetName(), g.GetValue(), len(g.GetNets()), max)
		}
		if g.GetDefault() != "" && g.GetDefault() != "low" && g.GetDefault() != "high" {
			return fmt.Errorf("intent %q: strap_group %q has default %q (want \"low\", \"high\", or omitted)", name, g.GetName(), g.GetDefault())
		}
		seenNet := map[string]bool{}
		for _, n := range g.GetNets() {
			if seenNet[n] {
				return fmt.Errorf("intent %q: strap_group %q lists net %q twice; one net cannot be two bits of the same number", name, g.GetName(), n)
			}
			seenNet[n] = true
		}
		sl := slug(g.GetName())
		if sl == "" {
			return fmt.Errorf("intent %q: strap_group name %q has no alphanumeric characters to form a rule name", name, g.GetName())
		}
		if first, dup := groupSlugs[sl]; dup {
			return fmt.Errorf("intent %q: strap_groups %q and %q slugify to the same rule name %q", name, first, g.GetName(), "intent/strap-group-"+sl)
		}
		groupSlugs[sl] = g.GetName()
		d.StrapGroups = append(d.StrapGroups, StrapGroup{
			Name: g.GetName(), Device: g.GetDevice(), Nets: g.GetNets(), Value: int(g.GetValue()), Bus: g.GetBus(), Default: g.GetDefault(),
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
func parseIOAssignment(declName string, i int, a *configpb.PinAssignment) (IOAssignment, error) {
	row := fmt.Sprintf("io_map #%d", i+1)
	if strings.TrimSpace(a.GetNet()) == "" {
		return IOAssignment{}, fmt.Errorf("intent %q: %s is missing its \"net\"", declName, row)
	}
	row = fmt.Sprintf("io_map row for net %q", a.GetNet())
	if strings.TrimSpace(a.GetDevice()) == "" {
		return IOAssignment{}, fmt.Errorf("intent %q: %s is missing its \"device\"", declName, row)
	}
	if strings.TrimSpace(a.GetPin()) == "" {
		return IOAssignment{}, fmt.Errorf("intent %q: %s is missing its \"pin\"", declName, row)
	}
	asg := IOAssignment{
		Net: strings.TrimSpace(a.GetNet()), Device: strings.TrimSpace(a.GetDevice()),
		Pin: strings.TrimSpace(a.GetPin()), Function: strings.TrimSpace(a.GetFunction()),
	}
	if to := a.GetTo(); to != nil {
		if strings.TrimSpace(to.GetDevice()) == "" || strings.TrimSpace(to.GetPin()) == "" {
			return IOAssignment{}, fmt.Errorf("intent %q: %s declares a \"to\" with %s; a far end needs both a device and a pin",
				declName, row, missingHalf(to))
		}
		asg.To = &IOEndpoint{Device: strings.TrimSpace(to.GetDevice()), Pin: strings.TrimSpace(to.GetPin())}
	}
	return asg, nil
}

func missingHalf(e *configpb.PinEndpoint) string {
	switch {
	case strings.TrimSpace(e.GetDevice()) == "" && strings.TrimSpace(e.GetPin()) == "":
		return "neither a device nor a pin"
	case strings.TrimSpace(e.GetDevice()) == "":
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
func parseSequence(declName string, i int, s *configpb.PowerSequence, slugs map[string]string) (Sequence, error) {
	if strings.TrimSpace(s.GetName()) == "" {
		return Sequence{}, fmt.Errorf("intent %q: sequence #%d is missing its \"name\"", declName, i+1)
	}
	if s.GetRelation() != SequenceEnableGated {
		return Sequence{}, fmt.Errorf("intent %q: sequence %q has relation %q (want %q, the only ordering a netlist evidences)", declName, s.GetName(), s.GetRelation(), SequenceEnableGated)
	}
	sl := slug(s.GetName())
	if sl == "" {
		return Sequence{}, fmt.Errorf("intent %q: sequence name %q has no alphanumeric characters to form a rule name", declName, s.GetName())
	}
	if first, dup := slugs[sl]; dup {
		return Sequence{}, fmt.Errorf("intent %q: sequences %q and %q slugify to the same rule name %q", declName, first, s.GetName(), "intent/sequence-"+sl)
	}
	slugs[sl] = s.GetName()
	if len(s.GetOrder()) < 2 {
		return Sequence{}, fmt.Errorf("intent %q: sequence %q needs at least two stages in \"order\" (an order of one has nothing to come before)", declName, s.GetName())
	}
	seq := Sequence{Name: s.GetName(), Relation: s.GetRelation()}
	rails := map[string]bool{}
	for j, st := range s.GetOrder() {
		if strings.TrimSpace(st.GetRail()) == "" {
			return Sequence{}, fmt.Errorf("intent %q: sequence %q stage #%d is missing its \"rail\"", declName, s.GetName(), j+1)
		}
		if rails[st.GetRail()] {
			return Sequence{}, fmt.Errorf("intent %q: sequence %q lists rail %q twice, so its position in the order is ambiguous", declName, s.GetName(), st.GetRail())
		}
		rails[st.GetRail()] = true
		seq.Order = append(seq.Order, SequenceStage{Rail: st.GetRail(), Good: st.GetGood(), Enable: st.GetEnable()})
	}
	if !hasGatingPair(seq) {
		return Sequence{}, fmt.Errorf("intent %q: sequence %q declares no adjacent \"good\" -> \"enable\" pair, so nothing in the netlist can be checked against it. "+
			"A rail order enforced inside a PMIC or by firmware leaves no trace in a netlist; record it as a review note rather than as a sequence", declName, s.GetName())
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
