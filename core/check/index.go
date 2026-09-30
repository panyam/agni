package check

// Well-known Tag keys the built-in rules populate. Tags is open, so any rule may add its own keys;
// these are the axes the catalog and default group-by understand.
const (
	KeyCategory     = "category"     // human organization bucket (provider-specific)
	KeyTier         = "tier"         // expressiveness tier (docsite/content/architecture/rules-and-checks.md#expressiveness-tiers): "P" | "R" | "A" | "X"
	KeyDistribution = "distribution" // how the rule's source may be shared (see Dist* values)
	KeySite         = "site"         // where the rule is evaluated; see Site* values
	KeySource       = "source"       // the owning RuleSource's name, stamped by Catalog composition (absent = built-in)
)

// Implementation-site tag values (Tags[KeySite]). SiteCheck, the default when absent, is an analysis
// over the netlist IR. SiteDiagnostic means the reader detects the problem at ingestion and records
// it in InputDiagnostics, and the rule only reports what the reader found, because the signal cannot
// be reconstructed from the normalized IR. See
// docsite/content/architecture/rules-and-checks.md#where-a-rule-runs-input-diagnostics-versus-analysis-checks.
const (
	SiteCheck      = "check"
	SiteDiagnostic = "diagnostic"
)

// Category tag values used by the built-in rules (set as Tags[KeyCategory]). The engine attaches no
// meaning to them; they only group the catalog.
const (
	CategoryConnectivity = "connectivity" // ERC-style: pins, nets, drivers, connectivity
	CategoryNaming       = "naming"       // conventions and pairing over names
	CategoryPower        = "power"        // rails, decoupling, protection
	CategoryDatasheet    = "datasheet"    // derating and margin rules that join a parameter
	CategoryBoard        = "board"        // geometric DRC over the board tier (WS3-008)
	CategoryIntegrity    = "integrity"    // read-health tripwires: a firing means fix the read, not the design
)

// Distribution tag values say how a rule's source may be shared. They also gate catalog visibility, so
// the shareable build surfaces open and public-reference rules while proprietary suites load
// separately, out of repo.
const (
	DistOpen            = "open"             // derivable from open sources (KiCad DRC, gov specs)
	DistPublicReference = "public-reference" // encodes a public-referenced fact (IPC, vendor app notes)
	DistProprietary     = "proprietary"      // vendor/customer-locked; studied for coverage only
)

// The built-in rules live in stdlib/rules/builtin, which installs them as the anonymous built-in
// source via RegisterBuiltins at init (#9). A program gets the standard catalog by blank-importing
// that package. See source.go (Builtins, RegisterBuiltins) and catalog.go (CatalogWith).
