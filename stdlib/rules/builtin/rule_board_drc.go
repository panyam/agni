package builtin

import (
	"fmt"
	"strconv"

	"github.com/panyam/agni/core/check"
)

// The first geometric DRC rules (WS3-008, the .kicad_dru class), over the board-geometry
// tier (Model.BoardNets). The three here are spec-first per-net threshold checks the AST
// expresses directly, and the first rules quantifying the board.nets entity set. The fourth
// (copper-clearance, rule_copper_clearance.go) is a pairwise spatial join the AST does not
// express yet. Per the WS3-008 overfitting guard, a geometry-query primitive must be evidenced
// by more than one rule before it earns an AST node, and this batch is that evidence.
//
// A board that declares its own minimums is checked against them (agni issue 933), because the
// declaration is the designer saying which fabrication process the board is routed for, and an HDI
// board at 0.1mm is a design for a finer fab rather than 1300 defects. A board declaring none is
// checked against the loosest common fabrication floors from the corpus JLCPCB capability rules
// (corpus/rules/kicad-dru/cimos-jlcpcb, MIT), which a mainstream fab cannot manufacture below at
// all. Every finding names which of the two it was measured against.
//
// Subjects are the owning net (KindNet), one finding per net per rule with the violation
// count in the message, because copper primitives have no stable identity. Per-violation
// locations need multi-location findings (OUT_OF_SCOPE.md).

// Fabrication-floor thresholds, nanometers, for a board that declares no minimum of its own.
const (
	minTrackWidthNm = 127_000 // 0.127mm (5mil) minimum trace width
	minHoleSizeNm   = 200_000 // 0.2mm minimum mechanical drill
	minAnnularNm    = 75_000  // 0.075mm minimum via annular ring
	minClearanceNm  = 127_000 // 0.127mm (5mil) minimum copper-to-copper spacing
)

// boardFloor names one of the board-wide minimums: the default floor, the name a finding gives it, and
// where a declaration sits in check.BoardRules.
type boardFloor struct {
	dflt     int64
	dfltDesc string
	declared func(check.BoardRules) int64
}

var boardFloors = map[string]boardFloor{
	"track":     {minTrackWidthNm, "0.127mm fabrication floor", func(r check.BoardRules) int64 { return r.TrackWidthNm }},
	"drill":     {minHoleSizeNm, "0.2mm floor", func(r check.BoardRules) int64 { return r.DrillNm }},
	"annular":   {minAnnularNm, "0.075mm floor", func(r check.BoardRules) int64 { return r.AnnularNm }},
	"clearance": {minClearanceNm, "0.127mm fabrication floor", func(r check.BoardRules) int64 { return r.ClearanceNm }},
}

// floorFor is the minimum a board's copper is held to for one quantity, and how a finding names it:
// the board's own declaration when it makes one, else the default floor.
func floorFor(m check.Model, which string) (int64, string) {
	f := boardFloors[which]
	if nm := f.declared(m.BoardRules()); nm > 0 {
		return nm, strconv.FormatFloat(float64(nm)/1e6, 'f', -1, 64) + "mm minimum the board declares"
	}
	return f.dflt, f.dfltDesc
}

// registerBoardFloors registers the Spec functions the threshold rules read their floor through. Each
// rule's initializer calls it, because a Spec validates its Calls when it binds, and registering again
// replaces a function with itself.
func registerBoardFloors() {
	which := func(args []any) string {
		s, _ := args[0].(string)
		if _, ok := boardFloors[s]; !ok {
			panic(fmt.Sprintf("board_floor: unknown floor %q", s))
		}
		return s
	}
	check.RegisterSpecFunc("board_floor", &check.SpecFunc{
		Reads: []string{"board.rules"},
		Fn: func(m check.Model, _ map[string]any, args []any) any {
			nm, _ := floorFor(m, which(args))
			return int(nm)
		},
	})
	check.RegisterSpecFunc("board_floor_desc", &check.SpecFunc{
		Reads: []string{"board.rules"},
		Fn: func(m check.Model, _ map[string]any, args []any) any {
			_, desc := floorFor(m, which(args))
			return desc
		},
	})
}

// floorLets are the Let bindings a threshold rule compares against and names in its message.
func floorLets(which string, extra map[string]check.Term) map[string]check.Term {
	extra["floor"] = check.Call{Fn: "board_floor", Args: []check.Term{check.Lit{V: which}}}
	extra["floor_desc"] = check.Call{Fn: "board_floor_desc", Args: []check.Term{check.Lit{V: which}}}
	return extra
}

var trackWidth = func() *check.Rule {
	registerBoardFloors()
	return (&check.Spec{
		Over: "board.nets",
		Let: floorLets("track", map[string]check.Term{
			"thin": check.CountOf{Over: "bnet.segments", Where: check.Cmp{L: check.Fact{Name: "segment.width"}, Op: "<", R: check.Var{Name: "floor"}}},
		}),
		Where:   check.Cmp{L: check.Var{Name: "thin"}, Op: ">=", R: check.Lit{V: 1}},
		Message: "net has {thin} track segment(s) narrower than the {floor_desc}",
	}).Rule(check.Rule{
		Name:     "track-width",
		Severity: "error",
		Summary:  "A routed track is narrower than the minimum the board declares, or the loosest common fabrication floor (0.127mm) when it declares none.",
		Impact:   "A trace below the fab's minimum width either fails DFM at order time or etches unreliably: opens, current-carrying failures, yield loss. A sub-floor trace is not tight routing; it is unmanufacturable by mainstream processes.",
		Remedy:   "Widen the track to the fab's minimum, or move the board to a process quoted for the width you need. Below the floor a trace will not etch reliably at mainstream yields.",
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryBoard,
			check.KeyTier:         "P",
			check.KeyDistribution: check.DistOpen,
		},
		Detail: ruleDoc("track-width"),
	})
}()

var holeSize = func() *check.Rule {
	registerBoardFloors()
	return (&check.Spec{
		Over: "board.nets",
		Let: floorLets("drill", map[string]check.Term{
			"small": check.CountOf{Over: "bnet.vias", Where: check.Cmp{L: check.Fact{Name: "via.drill"}, Op: "<", R: check.Var{Name: "floor"}}},
		}),
		Where:   check.Cmp{L: check.Var{Name: "small"}, Op: ">=", R: check.Lit{V: 1}},
		Message: "net has {small} via(s) drilled below the {floor_desc}",
	}).Rule(check.Rule{
		Name:     "hole-size",
		Severity: "error",
		Summary:  "A via's drill is smaller than the minimum the board declares, or the loosest common mechanical-drill floor (0.2mm) when it declares none.",
		Impact:   "A hole below the fab's minimum drill cannot be mechanically drilled: the order is rejected, or the via is silently upsized and clearances shift under you.",
		Remedy:   "Enlarge the drill to the fab's minimum. Left as it is, the order is either rejected or the via is silently upsized, and the clearances around it move with it.",
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryBoard,
			check.KeyTier:         "P",
			check.KeyDistribution: check.DistOpen,
		},
		Detail: ruleDoc("hole-size"),
	})
}()

var annularWidth = func() *check.Rule {
	registerBoardFloors()
	return (&check.Spec{
		Over: "board.nets",
		Let: floorLets("annular", map[string]check.Term{
			"thin": check.CountOf{Over: "bnet.vias", Where: check.Cmp{L: check.Fact{Name: "via.annular"}, Op: "<", R: check.Var{Name: "floor"}}},
		}),
		Where:   check.Cmp{L: check.Var{Name: "thin"}, Op: ">=", R: check.Lit{V: 1}},
		Message: "net has {thin} via(s) with an annular ring below the {floor_desc}",
	}).Rule(check.Rule{
		Name:     "annular-width",
		Severity: "error",
		Summary:  "A via's annular ring is thinner than the minimum the board declares, or the loosest common fabrication floor (0.075mm) when it declares none.",
		Impact:   "Drill wander eats the ring: a via with too little annulus breaks out of its pad on real tolerances, and the connection opens intermittently or fails outright.",
		Remedy:   "Enlarge the pad or reduce the drill until the annular ring clears the fab's floor with tolerance left over for drill wander.",
		Tags: map[string]string{
			check.KeyCategory:     check.CategoryBoard,
			check.KeyTier:         "P",
			check.KeyDistribution: check.DistOpen,
		},
		Detail: ruleDoc("annular-width"),
	})
}()
