package builtin

import "github.com/panyam/agni/core/check"

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

var trackWidth = (&check.Spec{
	Over: "board.nets",
	Let: map[string]check.Term{
		"thin": check.CountOf{Over: "bnet.segments", Where: check.Cmp{L: check.Fact{Name: "segment.width"}, Op: "<", R: check.Lit{V: minTrackWidthNm}}},
	},
	Where:   check.Cmp{L: check.Var{Name: "thin"}, Op: ">=", R: check.Lit{V: 1}},
	Message: "net has {thin} track segment(s) narrower than the 0.127mm fabrication floor",
}).Rule(check.Rule{
	Name:     "track-width",
	Severity: "error",
	Summary:  "A routed track is narrower than the loosest common fabrication floor (0.127mm).",
	Impact:   "A trace below the fab's minimum width either fails DFM at order time or etches unreliably: opens, current-carrying failures, yield loss. A sub-floor trace is not tight routing; it is unmanufacturable by mainstream processes.",
	Remedy:   "Widen the track to the fab's minimum, or move the board to a process quoted for the width you need. Below the floor a trace will not etch reliably at mainstream yields.",
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryBoard,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("track-width"),
})

var holeSize = (&check.Spec{
	Over: "board.nets",
	Let: map[string]check.Term{
		"small": check.CountOf{Over: "bnet.vias", Where: check.Cmp{L: check.Fact{Name: "via.drill"}, Op: "<", R: check.Lit{V: minHoleSizeNm}}},
	},
	Where:   check.Cmp{L: check.Var{Name: "small"}, Op: ">=", R: check.Lit{V: 1}},
	Message: "net has {small} via(s) drilled below the 0.2mm floor",
}).Rule(check.Rule{
	Name:     "hole-size",
	Severity: "error",
	Summary:  "A via's drill is smaller than the loosest common mechanical-drill floor (0.2mm).",
	Impact:   "A hole below the fab's minimum drill cannot be mechanically drilled: the order is rejected, or the via is silently upsized and clearances shift under you.",
	Remedy:   "Enlarge the drill to the fab's minimum. Left as it is, the order is either rejected or the via is silently upsized, and the clearances around it move with it.",
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryBoard,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("hole-size"),
})

var annularWidth = (&check.Spec{
	Over: "board.nets",
	Let: map[string]check.Term{
		"thin": check.CountOf{Over: "bnet.vias", Where: check.Cmp{L: check.Fact{Name: "via.annular"}, Op: "<", R: check.Lit{V: minAnnularNm}}},
	},
	Where:   check.Cmp{L: check.Var{Name: "thin"}, Op: ">=", R: check.Lit{V: 1}},
	Message: "net has {thin} via(s) with an annular ring below the 0.075mm floor",
}).Rule(check.Rule{
	Name:     "annular-width",
	Severity: "error",
	Summary:  "A via's annular ring is thinner than the loosest common fabrication floor (0.075mm).",
	Impact:   "Drill wander eats the ring: a via with too little annulus breaks out of its pad on real tolerances, and the connection opens intermittently or fails outright.",
	Remedy:   "Enlarge the pad or reduce the drill until the annular ring clears the fab's floor with tolerance left over for drill wander.",
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryBoard,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("annular-width"),
})
