package check

// Well-known Tag keys the built-in rules populate. Tags is open (any rule may add its own keys);
// these are the axes Agni's catalog and default group-by understand. Defining them as constants
// keeps Agni's own rules typo-safe without making Tags a closed schema.
const (
	KeyCategory     = "category"     // human organization bucket (provider-specific)
	KeyTier         = "tier"         // docs/19 expressiveness label: "P" | "R" | "A" | "X"
	KeyDistribution = "distribution" // how the rule's source may be shared (see Dist* values)
	KeySite         = "site"         // implementation site (docs/19 "Where a rule runs"); see Site* values
	KeySource       = "source"       // stamped by Catalog composition: the owning RuleSource's name (absent = built-in)
)

// Implementation-site tag values (Tags[KeySite]). Where a rule is actually evaluated: SiteCheck (the
// default, an analysis over the netlist IR) or SiteDiagnostic (the reader detects it at ingestion
// and records it in InputDiagnostics; the rule here only reports what the reader found, because the
// signal cannot be reconstructed from the normalized IR — docs/19). Absent means SiteCheck.
const (
	SiteCheck      = "check"
	SiteDiagnostic = "diagnostic"
)

// Category tag values used by the built-in rules (set as Tags[KeyCategory]). Not IR- or
// engine-meaningful; purely how this catalog groups its rules.
const (
	CategoryConnectivity = "connectivity" // ERC-style: pins, nets, drivers, connectivity
	CategoryNaming       = "naming"       // conventions and pairing over names
	CategoryPower        = "power"        // rails, decoupling, protection
	CategoryDatasheet    = "datasheet"    // derating and margin rules that join a parameter
	CategoryBoard        = "board"        // geometric DRC over the board tier (WS3-008)
	CategoryIntegrity    = "integrity"    // read-health tripwires: a firing means fix the read, not the design
)

// Distribution tag values: how a rule's source may be shared. Also gates catalog visibility, so
// the shareable build can surface open + public-reference rules while proprietary and customer
// suites load separately and out-of-repo.
const (
	DistOpen            = "open"             // derivable from open sources (KiCad DRC, gov specs)
	DistPublicReference = "public-reference" // encodes a public-referenced fact (IPC, vendor app notes)
	DistProprietary     = "proprietary"      // vendor/customer-locked; studied for coverage only
)

// Rules is the built-in rule set, in evaluation order. This is the single registry: adding a
// rule means adding one rule_*.go file and one line here. Run evaluates these; Tree groups
// them for display.
var Rules = []*Rule{
	singlePinNet,
	unconnectedComponent,
	unconnectedPin,
	danglingEndpoint,
	wireNoJunction,
	duplicateRefDes,
	outputOutputConflict,
	ncPinConnected,
	unspecifiedPinWithDriver,
	duplicateNetName,
	labelAliasConflict,
	powerTapConflict,
	testPointCoverage,
	floatingInput,
	powerInputNotDriven,
	decouplingPresent,
	bulkCap,
	crystalLoadCaps,
	resonatorRedundantLoadCaps,
	inputProtection,
	esdProtection,
	esdClampNotTVS,
	i2cPullUp,
	supplyExceedsAbsMax,
	railNominalOutOfRecommended,
	capVoltage,
	diffPairNaming,
	trackWidth,
	holeSize,
	annularWidth,
	copperClearance,
	ledPolarity,
	pinNetConflict,
	busNotModeled,
}

// Specs maps each Go-eval'd rule to its declarative twin: the same rule body as a Spec value
// (WS3-003, docs/19 "A rule is a value"). For these rules the Go Eval stays canonical — Run
// evaluates it, not the twin — while the twin is held to it by TestSpecParity (identical
// findings on every fixture) and TestSpecMetadata (the rule's hand-written Reads/Primitives
// equal the spec's derived ones). Flipping a rule to spec-canonical is replacing its Eval
// with its twin's, gated by the parity test — output-output-conflict was the first flip
// (rule_pin_matrix.go) and therefore no longer appears here. Spec-only rules (the matrix
// rows, and future rules on proven vocabulary per the docs/19 twin discipline) never do:
// their Eval IS the interpreter. Benchmarks comparing the two evaluation paths live in
// spec_bench_test.go and feed the WS3-004 fact-store decision.
var Specs = map[string]*Spec{
	"single-pin-net":         singlePinNetSpec,
	"unconnected-component":  unconnectedComponentSpec,
	"unconnected-pin":        unconnectedPinSpec,
	"wire-no-junction":       wireNoJunctionSpec,
	"dangling-endpoint":      danglingEndpointSpec,
	"duplicate-ref-des":      duplicateRefDesSpec,
	"floating-input":         floatingInputSpec,
	"power-input-not-driven": powerInputNotDrivenSpec,
	"decoupling-present":     decouplingPresentSpec,
	"bulk-cap":               bulkCapSpec,
	"input-protection":       inputProtectionSpec,
	"esd-protection":         esdProtectionSpec,
	"esd-clamp-not-tvs":      esdClampNotTVSSpec,
	"i2c-pull-up":            i2cPullUpSpec,
	"diff-pair-naming":       diffPairNamingSpec,
}
