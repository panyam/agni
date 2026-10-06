package kicad

import (
	"encoding/json"
	"io"
	"strconv"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// ConstraintKindBoardRules is the ir.Constraint.kind tag for the board-wide minimums a KiCad project
// declares, the fabrication process the designer routed the board for (agni issue 933). There is at
// most one per design, and its params are millimetres keyed by KiCad's own names.
const ConstraintKindBoardRules = "board_rules"

// boardRuleKeys are the board.design_settings.rules entries agni checks copper against, in KiCad's
// spelling. Millimetres, as KiCad writes every PCB scalar in a .kicad_pro.
var boardRuleKeys = []string{"min_track_width", "min_clearance", "min_through_hole_diameter", "min_via_annular_width"}

// ParseBoardRules reads board.design_settings.rules from a KiCad .kicad_pro: the minimum track width,
// clearance, drill and annular ring the board is designed to. A key the project leaves out, or sets to
// zero (KiCad's "no constraint"), is absent from the result, so a consumer can tell an undeclared
// minimum from a declared one. A malformed project, or one declaring none, yields an empty map rather
// than failing the read, as ParseNetClassDefs does.
func ParseBoardRules(r io.Reader) map[string]float64 {
	var doc struct {
		Board struct {
			DesignSettings struct {
				Rules map[string]any `json:"rules"`
			} `json:"design_settings"`
		} `json:"board"`
	}
	out := map[string]float64{}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return out
	}
	for _, k := range boardRuleKeys {
		if v, ok := doc.Board.DesignSettings.Rules[k].(float64); ok && v > 0 {
			out[k] = v
		}
	}
	return out
}

// AnnotateBoardRules stamps a project's board-wide minimums onto ir.Design as one Constraint of kind
// board_rules, and does nothing when the project declares none. Params are strings because
// ir.Constraint says they are; the model parses them.
func AnnotateBoardRules(d *ir.Design, rules map[string]float64) {
	if d == nil || len(rules) == 0 {
		return
	}
	params := map[string]string{}
	for k, v := range rules {
		params[k] = strconv.FormatFloat(v, 'g', -1, 64)
	}
	d.Constraints = append(d.Constraints, &ir.Constraint{Name: "board", Kind: ConstraintKindBoardRules, Params: params})
}
