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

// The built-in rule set and its declarative-twin map used to live here as `var Rules` and
// `var Specs`. They moved to package stdlib/rules/builtin (phase 2b of the package reorg), which
// installs them as the anonymous built-in source via check.RegisterBuiltins at init. The engine
// core no longer owns any rules: a program gets the standard catalog by blank-importing
// stdlib/rules/builtin, exactly as it registers an overlay suite. See source.go (Builtins,
// RegisterBuiltins) and catalog.go (CatalogWith).
