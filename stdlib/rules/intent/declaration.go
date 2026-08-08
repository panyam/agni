// Package intent checks a loaded design against a DESIGN-INTENT declaration: an external, authored
// statement of what a design is SUPPOSED to contain — which modules, which voltage domains — that the
// netlist is checked against. It is the hardest-to-automate tier of the review classification:
// the netlist says what IS wired, but "all required modules present" and "voltage domains identified"
// compare the schematic against an intended architecture that lives OUTSIDE the schematic, so they
// cannot be decided from the netlist alone.
//
// The load-bearing guard is that the intent is declared INDEPENDENTLY of the netlist and never derived
// from it: each rule iterates the DECLARATION and probes the netlist for each expectation, so a missing
// module or a rail on the wrong voltage domain FAILS. A rule that enumerated its expectations from the
// design would always pass (circular) — the exact silent-false-pass the empty-set-is-silent discipline
// exists to prevent. There is therefore no built-in intent (unlike profiles, which ship generic
// SPI_NOR/CAN): every intent is design-specific and is loaded only via --intent-path, so a design run
// with no declaration leaves these items not-automated rather than silently passing.
//
// The mechanism (this package, the rules, the --intent-path flag) is shareable engine code; a specific
// design's declaration is customer-boundary data authored in the overlay (the C16 datasheet posture
// generalized). Compile turns a Declaration into check rules the same way profiles.Compile turns a
// Profile into rules; the CLI splices them into the catalog via check.CatalogWith.
package intent

// Declaration is one design's intended architecture: the expected modules, voltage domains, and
// subsystems (clock/reset/power tree). It is authored as YAML in the overlay and parsed by Load/Parse;
// it is never derived from a netlist.
type Declaration struct {
	// Name identifies the declaration in findings and reports (e.g. "Automotive ECU design intent").
	Name string
	// Modules is the set of functional blocks the design is required to contain. moduleMissingRule
	// fails once per declared module absent from the design.
	Modules []Module
	// VoltageDomains is the set of power domains the design declares, each pinning named rails to a
	// nominal voltage. voltageDomainRule fails when a declared rail is absent, or present but its
	// name-derived nominal disagrees with the declared domain (a rail on the wrong domain).
	VoltageDomains []VoltageDomain
	// Subsystems is the set of named architectural subsystems the design must instantiate (a clock
	// tree, a reset scheme, the power tree). Each compiles to its OWN rule (one per subsystem, keyed by
	// its Name slug) so distinct review items — "clock architecture consistency", "reset architecture
	// consistency" — bind to their own subsystem and report independently, unlike Modules which share
	// one rule. A subsystem fails when its declared source component is absent or any of its declared
	// nets is missing.
	Subsystems []Subsystem
	// Protections is the set of rail-protection requirements: a named rail that must carry a protection
	// device (OVP clamp, discharge path). Each KIND compiles to its own rule (intent/protection-<kind>)
	// so distinct review items — "OV protection", "rail discharge" — bind and report independently. A
	// protection fails when the declared rail carries no device of the required kind. It is keyed on the
	// declared NET NAME (not the rail-role heuristic, which misses names like VBATT01 that carry no
	// voltage token), so an input rail the customer names explicitly is checkable.
	Protections []Protection
	// NetProperties is the set of declared PROPERTIES of named nets — not their existence, which the
	// forms above cover, but an assertion about what a net IS (a reset is active-low, a link is
	// AC-coupled). Each KIND compiles to its own rule (intent/property-<kind>) so distinct review
	// items bind and report independently, the same reason Protections split by kind. A property
	// fails when the design's structure CONTRADICTS the declaration.
	NetProperties []NetProperty
}

// Property kinds.
const (
	// PropResetPolarity asserts a reset net's asserted level. Value is "low" or "high".
	PropResetPolarity = "reset-polarity"
	// PropACCoupled asserts a net is AC-coupled: a series capacitor carries it, rather than DC.
	PropACCoupled = "ac-coupled"
	// PropStrap asserts the level a boot/config strap net is intended to latch at reset. Value is
	// "low" or "high".
	PropStrap = "strap"
)

// NetProperty is one declared property of one net (WS3-088). Kinds (validated at load):
// "reset-polarity" with Value "low" or "high"; "ac-coupled", which takes no Value; "strap" with
// Value "low" or "high".
//
// WHAT THESE RULES CAN AND CANNOT CONCLUDE, because the kinds differ and the difference decides
// what a passing item means:
//
//   - ac-coupled is DECIDABLE from the netlist. A series capacitor is either on the net or it is not,
//     so absent means the declaration is unmet and the rule fails.
//   - reset-polarity is only PARTLY decidable. A netlist states polarity nowhere; the evidence is a
//     bias resistor, and a reset driven by a supervisor with an internal pull carries none. So the
//     rule fires on a CONTRADICTION (declared low, biased low) and is SILENT where the design shows
//     nothing either way. Silence there means "no contradiction found", NOT "polarity confirmed",
//     stated here, in the rule's doc, and in the finding vocabulary because a review item bound to it
//     inherits that limit.
//   - strap (WS3-086) reads the SAME evidence as reset-polarity and asks the INVERTED question, so
//     it is worth being explicit that they are not the same rule wearing two names. reset-polarity's
//     Value is the level that ASSERTS reset, so bias TOWARD it is the defect and bias away from it is
//     correct: the rule can only ever catch one of the two ways a reset line goes wrong. strap's Value
//     is the level the pin should LATCH, so bias toward it is correct and bias away from it is the
//     defect, in EITHER direction.
//     Absent bias is still silent, and for the same cause: strap pins carry internal pulls, and the
//     standard datasheet instruction is to fit an external resistor only for the non-default state. A
//     design declaring the default level with no resistor on the net is correct and common, so firing
//     there would report a non-defect on the majority of real straps.
//
// The engine has no per-subject not-applicable: an outcome is per review ITEM and follows whether the
// rule fired. So the honest options for the undecidable case were to stay silent (this) or to fail a
// declaration the design merely does not evidence, which would report a non-defect. Reporting
// unverifiable declarations is a separate, useful check and deliberately not this one.
type NetProperty struct {
	// Net is the exact net name the property is declared on.
	Net string
	// Property is the kind (PropResetPolarity, PropACCoupled, PropStrap).
	Property string
	// Value qualifies the kind: "low"/"high" for reset-polarity and strap, empty for ac-coupled.
	Value string
}

// Module is one required functional block, matched to a design component by device CLASS (the primary
// path — device_classes are stamped at ingestion, so a class match works on any netlist with no
// --params) or by exact MPN (secondary — the MPN map is built only when the model is loaded with
// --params, so an MPN-only module is unmatched without it). At least one of Class/MPN is set (Load
// validates this). A module matches when ANY design component satisfies its criterion.
type Module struct {
	// Name is the human label shown in a "declared module <Name> not present" finding.
	Name string
	// Class is a device-class value (soc, can_transceiver, regulator, ...); matched with
	// Model.HasClass, so a family tag (diode) matches its specific classes (tvs, led) too. Empty to
	// match by MPN only.
	Class string
	// MPN is an exact manufacturer part number; matched case-sensitively against Model.ComponentMPN.
	// Empty to match by Class only. Requires a params-built model to resolve (see the Module doc).
	MPN string
	// Count is the EXACT number of components the design is expected to contain for this module's
	// criterion (e.g. 2 CAN transceivers, 4 Wi-Fi radios). 0 means unspecified — no count check
	// (moduleCountRule skips it), so declaring a module without a count keeps the presence-only
	// behavior. When > 0, moduleCountRule fails if the actual matching-component count differs (too
	// few OR too many), a distinct ask from module-missing (which only asks "at least one"). Counting
	// by MPN requires a params-built model, same as the MPN match path.
	Count int
}

// VoltageDomain is one declared power domain: a nominal voltage and the rail net names that must sit on
// it. It encodes the design intent "these rails are the 3.3V domain" so the check can flag a rail that
// is absent or whose name declares a different voltage.
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

// Subsystem is one named architectural block the design must instantiate, evidenced by a source
// component and/or a set of nets that must all exist. It generalizes the "declare an intended
// subsystem, check the design realizes it" shape across the clock tree, the reset scheme, and the
// power tree. At least one of Source/Nets is set (Load validates this). Each Subsystem compiles to its
// own rule (subsystemRule), so an item bound to it reports only that subsystem.
type Subsystem struct {
	// Name is the subsystem label (e.g. "main clock", "reset", "power tree"); it slugifies into the
	// rule name (intent/subsystem-<slug>) that a review item binds to, and appears in findings. Names
	// must slugify uniquely within a declaration (Load validates this).
	Name string
	// Source is an optional required component for the subsystem (the clock's crystal, the reset
	// supervisor), matched exactly like a Module (by class or MPN). nil to check nets only.
	Source *Module
	// Nets are net names the subsystem requires; each must exist on the design or the subsystem fails.
	// Empty to check the source only.
	Nets []string
}

// Protection is one rail-protection requirement: the named rail net must carry a protection device of
// the given kind. Kinds (validated at load): "ovp" — a TVS or zener clamps the rail; "discharge" — a
// resistor bridges the rail to a ground net (a bleeder). It is keyed on the exact Rail net name, so a
// customer-named input rail (VBATT01, VB1_PWR_AO) is checkable even when the rail-role heuristic does
// not recognize it. Each kind compiles to its own rule; a declared protection with no matching device
// on the rail fails.
type Protection struct {
	// Rail is the exact net name that must be protected.
	Rail string
	// Kind is the protection type: "ovp" or "discharge".
	Kind string
}
