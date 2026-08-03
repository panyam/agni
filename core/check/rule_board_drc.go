package check

// The first geometric DRC rules (WS3-008, the .kicad_dru class), over the board-geometry
// tier (Model.BoardNets). Three are spec-first — per-net threshold checks the AST
// expresses directly, the first rules quantifying the board.nets entity set; the fourth
// (copper-clearance, rule_copper_clearance.go) is a pairwise spatial join the AST
// deliberately does not express yet: per the WS3-008 overfitting guard, a geometry-query
// primitive must be evidenced by more than one rule before it earns an AST node, and
// this batch is that evidence.
//
// Thresholds are the loosest common fabrication floors from the corpus JLCPCB capability
// rules (corpus/rules/kicad-dru/cimos-jlcpcb, MIT): a board violating them cannot be
// manufactured by a mainstream fab at all, so the defaults fire on real defects, not on
// tight-but-deliberate routing. Per-design thresholds (a .kicad_dru's own values) are
// rule parameterization — the WS3-006 registry's job; a re-thresholded rule is the same
// Spec with different Lits.
//
// Subjects are the owning net (KindNet), one finding per net per rule with the violation
// count in the message: copper primitives have no stable identity. Per-violation
// locations need multi-location findings (OUT_OF_SCOPE.md).

// Fabrication-floor thresholds, nanometers.
const (
	minTrackWidthNm = 127_000 // 0.127mm (5mil) minimum trace width
	minHoleSizeNm   = 200_000 // 0.2mm minimum mechanical drill
	minAnnularNm    = 75_000  // 0.075mm minimum via annular ring
	minClearanceNm  = 127_000 // 0.127mm (5mil) minimum copper-to-copper spacing
)

var trackWidth = (&Spec{
	Over: "board.nets",
	Let: map[string]Term{
		"thin": CountOf{Over: "bnet.segments", Where: Cmp{L: Fact{"segment.width"}, Op: "<", R: Lit{minTrackWidthNm}}},
	},
	Where:   Cmp{L: Var{"thin"}, Op: ">=", R: Lit{1}},
	Message: "net has {thin} track segment(s) narrower than the 0.127mm fabrication floor",
}).Rule(Rule{
	Name:     "track-width",
	Severity: "error",
	Summary:  "A routed track is narrower than the loosest common fabrication floor (0.127mm).",
	Impact:   "A trace below the fab's minimum width either fails DFM at order time or etches unreliably: opens, current-carrying failures, yield loss. A sub-floor trace is not tight routing; it is unmanufacturable by mainstream processes.",
	Tags: map[string]string{
		KeyCategory:     CategoryBoard,
		KeyTier:         "P",
		KeyDistribution: DistOpen,
	},
	Detail: ruleDoc("track-width"),
})

var holeSize = (&Spec{
	Over: "board.nets",
	Let: map[string]Term{
		"small": CountOf{Over: "bnet.vias", Where: Cmp{L: Fact{"via.drill"}, Op: "<", R: Lit{minHoleSizeNm}}},
	},
	Where:   Cmp{L: Var{"small"}, Op: ">=", R: Lit{1}},
	Message: "net has {small} via(s) drilled below the 0.2mm floor",
}).Rule(Rule{
	Name:     "hole-size",
	Severity: "error",
	Summary:  "A via's drill is smaller than the loosest common mechanical-drill floor (0.2mm).",
	Impact:   "A hole below the fab's minimum drill cannot be mechanically drilled: the order is rejected, or the via is silently upsized and clearances shift under you.",
	Tags: map[string]string{
		KeyCategory:     CategoryBoard,
		KeyTier:         "P",
		KeyDistribution: DistOpen,
	},
	Detail: ruleDoc("hole-size"),
})

var annularWidth = (&Spec{
	Over: "board.nets",
	Let: map[string]Term{
		"thin": CountOf{Over: "bnet.vias", Where: Cmp{L: Fact{"via.annular"}, Op: "<", R: Lit{minAnnularNm}}},
	},
	Where:   Cmp{L: Var{"thin"}, Op: ">=", R: Lit{1}},
	Message: "net has {thin} via(s) with an annular ring below the 0.075mm floor",
}).Rule(Rule{
	Name:     "annular-width",
	Severity: "error",
	Summary:  "A via's annular ring is thinner than the loosest common fabrication floor (0.075mm).",
	Impact:   "Drill wander eats the ring: a via with too little annulus breaks out of its pad on real tolerances, and the connection opens intermittently or fails outright.",
	Tags: map[string]string{
		KeyCategory:     CategoryBoard,
		KeyTier:         "P",
		KeyDistribution: DistOpen,
	},
	Detail: ruleDoc("annular-width"),
})
