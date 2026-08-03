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
