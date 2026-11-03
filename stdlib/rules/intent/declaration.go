// Package intent checks a loaded design against a DESIGN-INTENT declaration, an authored statement of
// what a design is SUPPOSED to contain. "All required modules present" and "voltage domains
// identified" compare the schematic against an architecture that lives OUTSIDE it, so the netlist
// alone cannot decide them.
//
// The intent is declared INDEPENDENTLY of the netlist and never derived from it. Each rule iterates
// the DECLARATION and probes the netlist for each expectation, so a missing module or a rail on the
// wrong domain FAILS, where a rule enumerating its expectations from the design would always pass.
// So there is no built-in intent (unlike profiles, which ship generic SPI_NOR/CAN). Every intent is
// loaded via --intent-path, and a design run with no declaration leaves these items not-automated.
//
// Anything a reviewer signs off separately gets its own rule (WS3-058). Which forms compile one rule
// per entry, one per kind, or to fixed names is in
// docsite/content/guide/design-intent.md#one-rule-per-declared-thing.
//
// The mechanism is engine code, and a specific design's declaration is private data authored in the
// extension (the C16 datasheet posture). Compile turns a Declaration into check rules the way
// profiles.Compile turns a Profile into rules, and the CLI splices them in via check.CatalogWith.
package intent

// Declaration is one design's intended architecture: the expected modules, voltage domains, and
// subsystems (clock/reset/power tree). It is authored as YAML in the extension and parsed by Load/Parse;
// it is never derived from a netlist.
type Declaration struct {
	// Name identifies the declaration in findings and reports (e.g. "Automotive design intent").
	Name string
	// Modules lists the functional blocks that the design must contain. moduleMissingRule fails once
	// for each declared module the design lacks.
	Modules []Module
	// VoltageDomains lists the declared power domains. Each one pins named rails to a nominal voltage.
	// voltageDomainRule fails when a declared rail is absent. It also fails when the rail is present
	// and its name-derived nominal disagrees with the declared domain (a rail on the wrong domain).
	VoltageDomains []VoltageDomain
	// Subsystems lists the named architectural subsystems that the design must instantiate, such as a
	// clock tree, a reset scheme or the power tree. Each compiles to one rule, intent/subsystem-<slug>.
	// A subsystem fails when its declared source component is absent or any of its declared nets is
	// missing.
	Subsystems []Subsystem
	// Protections lists the rails that must have a protection device (OVP clamp, discharge path). Each
	// is keyed on the declared NET NAME (see Protection). Each KIND compiles to one rule,
	// intent/protection-<kind>.
	Protections []Protection
	// NetProperties asserts what a named net IS rather than that it exists. A reset is active-low, or a
	// link is AC-coupled. Each KIND compiles to one rule, intent/property-<kind>. A property fails when
	// the design's structure CONTRADICTS the declaration.
	NetProperties []NetProperty
	// RailBudgets is the declared CURRENT demand of named rails (WS3-095). A netlist records
	// connectivity and not current, so the demand has to be declared. It compiles to
	// intent/rail-current-capacity.
	RailBudgets []RailBudget
	// Sequences is the declared power-up ORDER of groups of rails (WS3-092). It is the third intent
	// mechanism, after presence and property. Each compiles to one rule, intent/sequence-<slug>. A
	// sequence declares a gating chain, and it fails when that chain is not in the design or runs the
	// other way round.
	Sequences []Sequence
	// StrapGroups lists the strap GROUPS. Each group is several nets read as one binary number
	// (WS3-120, see StrapGroup). Each group compiles to one rule, intent/strap-group-<slug>, and one
	// cross-group collision rule covers all of them.
	StrapGroups []StrapGroup
	// IOMap is the design's declared PIN ASSIGNMENT. It says which net connects to which pin of which
	// device, and optionally what sits at the net's far end. The schematic is drawn FROM it, so any
	// board with a large MCU or SoC has one.
	//
	// It compiles to four fixed-name rules whatever the map's length (agni issue 517). That is the
	// documented exception to one rule per declared thing. Three rules ask whether the design kept
	// the map's promises (pin mismatch, net absent, far end wrong). io-map-coverage reports the nets
	// that the map never mentions. See compile.go.
	IOMap []IOAssignment
	// Exposures lists the declared exposure of each named component, sorted by ref-des. The model
	// reads the same declaration to decide which connectors the exposure rules check (see
	// check.WithIntent). So only one rule compiles from it, intent/exposure-declared, and that rule
	// only confirms that each declaration names a connector the design has.
	Exposures []Exposure
	// MarginFactor is the house headroom policy. A rail's supply must be rated for the rail's peak
	// budget times this factor, so 1.2 means 20% headroom. It compiles intent/rail-current-margin and
	// has NO DEFAULT. A default would put one company's policy in a rule literal, and WS3-069 moved
	// naming vocabularies out of rule literals. A default would also let a bound item PASS against a
	// number nobody declared. With no factor the margin rule is not compiled, so a bound item reads
	// needs-design-intent.
	MarginFactor float64
	// PullUpReach lists the pull-up reach that each named net declares, sorted by net name (agni issue
	// 820). The model reads the same declaration (see check.WithIntent). So the pull-up walk from that
	// net uses the declared reach rather than check.PullUpReachHops. Only one rule compiles from it,
	// intent/pull-up-reach-declared, and that rule only confirms that each declaration names a net the
	// design has.
	PullUpReach []PullUpReach
}

// PullUpReach is one net's declared pull-up reach. The walk from the net may cross that many series
// resistors on its way to the pull-up's rail. With it the designer says that the bus's series network
// is sized for that reach. An isolation resistor or a level-shifting network is such a network. The
// default reach of 3 assumes the network is not sized that way.
type PullUpReach struct {
	// Net is the exact net name the reach is declared on.
	Net string
	// Hops is the declared reach, always positive (Load refuses anything else).
	Hops int
}

// StrapGroup is several strap nets read together as one binary number, such as a device's address or
// mode select spread across pins (WS3-120). A per-net declaration cannot say "these three nets are
// one number". Without a group neither the encoded VALUE nor a collision between two devices can be
// expressed.
//
// PARTIAL EVIDENCE is the hard case. A strap pin usually has an internal pull, and the datasheet asks
// for an external resistor only in the NON-DEFAULT state. So a 3-bit address commonly has resistors
// on one or two bits. Default supplies the level of the other pins, since the netlist does not record
// it. With no Default an unbiased bit makes the value UNDECIDABLE, and the group reports inconclusive.
// Assuming zero would fabricate an address, and a fabricated address can fabricate a COLLISION
// between two parts that are fine.
type StrapGroup struct {
	// Name labels the group ("PHYAD", "boot mode"). It slugifies into the rule name
	// intent/strap-group-<slug>, and a review item binds to that name. It must slugify uniquely (Load
	// validates).
	Name string
	// Device is the ref-des the group configures (U12). The nets hold the evidence, so Device is not
	// matched against the design. It only names the part in findings, so a reviewer knows which page
	// to open.
	Device string
	// Nets are the group's strap nets in MSB-FIRST bit order. The declaration owns the order because a
	// netlist never states which pin is the high bit, and inferring it from names like PHYAD2 would put
	// a naming heuristic back in a rule literal.
	Nets []string
	// Value is the number the group is meant to encode, decoded MSB-first from Nets.
	Value int
	// Bus scopes collision detection. Two groups sharing a Bus must not encode the same Value. Empty
	// opts the group out. A mode select that is not an address on any shared bus wants that.
	Bus string
	// Default is the level an UNBIASED pin in this group takes from the part's internal pull, one of
	// "low", "high", or empty for "the netlist cannot tell, so do not guess". See the type doc.
	Default string
}

// RailBudget is one rail's declared peak current demand (WS3-095), in amps.
//
// It has no typical-draw field because no rule reads one, and an author who filled it in would
// believe the engine checked it. Add one when a rule consumes it.
type RailBudget struct {
	// Rail is the exact net name of the budgeted rail. It is matched literally, like Protection.Rail,
	// so a rail is still checkable when the rail-role heuristic does not recognize it.
	Rail string
	// Peak is the maximum current in amps the rail is expected to draw. Load requires it to be > 0,
	// since a zero budget is satisfied by everything and would pass silently.
	Peak float64
}

// SequenceEnableGated names the only sequence relation that a netlist can show evidence of. Each
// stage after the first is held off, because the previous stage's power-good signal drives its
// enable. Parse rejects any other value rather than accept a declaration nothing checks.
//
// It is a named value rather than a flag, so a second kind of relation can be added later. A
// sequencer part stepping rails from its own configuration is one such structure, and an explicit
// delay element is another. Each would be a different query.
const SequenceEnableGated = "enable-gated"

// Sequence is one declared power-up ordering. It lists the stages in the order they must come up,
// and names the relation the design is claimed to enforce it with.
//
// A pass here means less than "sequencing correct". A netlist holds no order. An order leaves only
// one trace in connectivity, the declared gating chain. So a silent rule means every declared link
// was found in the design. It does not mean the board powers up in that sequence. The rule doc
// repeats this for the reviewer.
//
// A board that sequences inside a PMIC or in firmware has no chain to name, so it declares no
// sequence and its review items read needs-design-intent. Parse rejects a sequence with no gating
// handle (see load.go), since its rule could only ever pass.
type Sequence struct {
	// Name is the sequence label ("SoC power tree", "modem rails"), and findings show it. It slugifies
	// into the rule name intent/sequence-<slug>, and a review item binds to that name. Names must
	// slugify uniquely within a declaration (Load validates this).
	Name string
	// Relation is how the order is claimed to be enforced. SequenceEnableGated is the only value
	// today; Load rejects the rest.
	Relation string
	// Order is the stages, earliest first. There must be at least two. At least one adjacent pair
	// must have the handles that the relation reads (Load validates both).
	Order []SequenceStage
}

// SequenceStage is one step of a declared power-up order. It names a rail, a net that signals the
// rail is up, and a net that holds it off. The check reads the two handles, and the rail names the
// stage.
type SequenceStage struct {
	// Rail is the stage's rail net name. It identifies the stage in findings and in the declaration.
	// A rail missing from the design is NOT a finding here. The voltage-domain and subsystem forms own
	// that presence question. Reporting it twice would put one defect under two review items.
	Rail string
	// Good is the net that signals this stage is up (a regulator's power-good output, a supervisor's
	// output). Empty when the stage gates nothing after it.
	//
	// Unlike Rail, a declared Good missing from the design IS a finding. The declaration depends on it
	// as evidence, and without it the chain enforces nothing.
	Good string
	// Enable is the net that holds this stage off until it is driven (a regulator's EN pin net, a
	// load switch's control, a peripheral's reset-release line). Empty for the first stage, or for
	// any stage nothing gates. A declared Enable missing from the design is a finding, for the same
	// reason as Good.
	Enable string
}

// Property kinds.
const (
	// PropResetPolarity asserts a reset net's asserted level. Value is "low" or "high".
	PropResetPolarity = "reset-polarity"
	// PropACCoupled asserts a net is AC-coupled through a series capacitor.
	PropACCoupled = "ac-coupled"
	// PropStrap asserts the level a boot/config strap net is intended to latch at reset. Value is
	// "low" or "high".
	PropStrap = "strap"
)

// NetProperty is one declared property of one net (WS3-088). Load validates the kind.
// "reset-polarity" takes Value "low" or "high", "ac-coupled" takes no Value, and "strap" takes Value
// "low" or "high".
//
// Each kind lets its rule conclude something different, and that decides what a passing item means.
//
//   - ac-coupled is DECIDABLE from the netlist. A series capacitor is on the net or it is not, so
//     absent means the declaration is unmet and the rule fails.
//   - reset-polarity is only PARTLY decidable. A netlist states polarity nowhere, the evidence is a
//     bias resistor, and a reset driven by a supervisor with an internal pull carries none. So the
//     rule fires on a CONTRADICTION (declared low, biased low) and reports INCONCLUSIVE where the
//     design shows nothing either way (agni issue 74).
//   - strap (WS3-086) reads the SAME evidence as reset-polarity and asks the INVERTED question.
//     reset-polarity's Value is the level that ASSERTS reset, so bias toward it is the defect and
//     the rule catches only that one of a reset line's two failures. strap's Value is the level the
//     pin should LATCH, so bias away from it is the defect, in EITHER direction. Absent bias is
//     silent, because strap pins carry internal pulls and the datasheet asks for an external resistor
//     only in the non-default state. Firing there would flag the majority of real straps.
type NetProperty struct {
	// Net is the exact net name the property is declared on.
	Net string
	// Property is the kind (PropResetPolarity, PropACCoupled, PropStrap).
	Property string
	// Value qualifies the kind: "low"/"high" for reset-polarity and strap, empty for ac-coupled.
	Value string
	// MinOhms / MaxOhms bound the acceptable resistance of a STRAP's pull resistor, in ohms. Both are
	// optional and independent. Declare one to bound that side only, or neither to check direction
	// alone. When the resistor's value cannot be compared to the band, the rule reports inconclusive
	// (agni issue 1281). Every other property kind ignores them.
	//
	// The band is declared, not built in. A strap resistor must be strong enough to hold against
	// leakage and weak enough not to fight an active driver, and both ends depend on the part. A CMOS
	// input with nanoamp leakage is happy past 100k. A driver has to override some straps, and those
	// want a few hundred ohms. A universal band from the engine would fire on correct boards, and the
	// review-integrity rule forbids that.
	MinOhms float64
	MaxOhms float64
}

// Module is one required functional block, matched to a design component by device CLASS or by exact
// MPN. Both match on any netlist with no --params, because device_classes are stamped at ingestion
// and every model joins the design's MPNs (agni issue 748). At least one of Class/MPN is set (Load
// validates this). A module matches when ANY design component satisfies its criterion.
type Module struct {
	// Name is the human label shown in a "declared module <Name> not present" finding.
	Name string
	// Class is a device-class value (soc, can_transceiver, regulator, ...); matched with
	// Model.HasClass, so a family tag (diode) matches its specific classes (tvs, led) too. Empty to
	// match by MPN only.
	Class string
	// MPN is an exact manufacturer part number; matched case-sensitively against Model.ComponentMPN.
	// Empty to match by Class only.
	MPN string
	// Count is the EXACT number of components expected to match this module's criterion (e.g. 2 CAN
	// transceivers). 0 means unspecified, and moduleCountRule skips it. When > 0, moduleCountRule
	// fails if the matching count differs (too few OR too many), where module-missing asks only "at
	// least one".
	Count int
}

// VoltageDomain is one declared power domain. It gives a nominal voltage and the rail net names that
// must sit on it. The check flags a declared rail when the rail is absent or its name declares a
// different voltage.
type VoltageDomain struct {
	// Name is the human label for the domain (e.g. "io_3v3", "core"), shown in findings.
	Name string
	// Nominal is the domain's nominal voltage in volts (e.g. 3.3). A declared rail whose
	// name-derived nominal differs from this is flagged.
	Nominal float64
	// Rails are the net names the design is expected to carry for this domain (e.g. ["3V3",
	// "VDD_IO"]). Each is probed independently: absent -> finding; present but off-nominal -> finding.
	Rails []string
}

// Subsystem is one named architectural block that the design must instantiate, such as the clock
// tree, the reset scheme or the power tree. Its evidence is a source component, a set of nets that
// must all exist, or both. At least one of Source/Nets is set (Load validates this).
type Subsystem struct {
	// Name is the subsystem label (e.g. "main clock", "reset", "power tree"), and findings show it. It
	// slugifies into the rule name intent/subsystem-<slug>, and a review item binds to that name.
	// Names must slugify uniquely within a declaration (Load validates this).
	Name string
	// Source is an optional required component for the subsystem (the clock's crystal, the reset
	// supervisor), matched exactly like a Module (by class or MPN). nil to check nets only.
	Source *Module
	// Nets are net names the subsystem requires; each must exist on the design or the subsystem fails.
	// Empty to check the source only.
	Nets []string
}

// Protection requires the named rail net to carry a protection device of the given kind, and fails
// when no matching device is on the rail. Load validates the kind. "ovp" means a TVS or zener clamps
// the rail, and "discharge" means a resistor bridges the rail to a ground net (a bleeder). It is
// keyed on the exact Rail net name. So an input rail named with no voltage token (VIN_MAIN, VSYS_AO)
// is checkable, even though the rail-role heuristic does not recognize it.
type Protection struct {
	// Rail is the exact net name that must be protected.
	Rail string
	// Kind is the protection type: "ovp" or "discharge".
	Kind string
}

// IOAssignment is one row of a declared pin map. It says this net connects to this pin of this
// device.
//
// Pin may be spelled either way. A map uses the functional NAME a datasheet and a firmware header use
// ("PTC11"), a netlist answers in the package DESIGNATOR ("41"), and both resolve through core/ident
// so the two spellings of one pin agree.
type IOAssignment struct {
	// Net is the net the map places on this pin, named as the design names it.
	Net string
	// Device is the ref-des the pin belongs to.
	Device string
	// Pin is the pin, spelled as a package designator or as the part type's functional name.
	Pin string
	// Function is the peripheral function selected on this pin ("ADC0_S17"). io-map-function checks
	// it against the alternate functions the part's datasheet lists for the pin (agni issue 667), and
	// reports the row not-considered when the part has no such table in the corpus.
	Function string
	// To is the far end this net is declared to reach, or nil when the row does not say. Real maps
	// fill it sparsely (about a third of rows in the one we measured), so the far-end rule reports its
	// denominator; see docsite/content/guide/design-intent.md#the-nine-forms.
	To *IOEndpoint
}

// IOEndpoint names one end of a declared connection.
type IOEndpoint struct {
	Device string
	Pin    string
}

// ExposureInternal and ExposureExternal are the two values of a component's exposure. Internal marks
// a connector joining this board to another inside the same product, and the exposure rules leave it
// alone. External is the default and needs no declaration.
const (
	ExposureInternal = "internal"
	ExposureExternal = "external"
)

// Exposure is one component's declared exposure (agni issue 831). Whether a connector faces the field
// is a fact about one product rather than about the part, since the same mezzanine connector is the
// module socket on one carrier and the cable entry on another.
type Exposure struct {
	// Ref is the component's ref-des as the design spells it.
	Ref string
	// Exposure is ExposureInternal or ExposureExternal.
	Exposure string
}
