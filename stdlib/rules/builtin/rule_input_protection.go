package builtin

import (
	"fmt"
	"strconv"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// inputProtection flags a board power input a connector feeds with no fuse or TVS. See Detail.
var inputProtection = &check.Rule{
	Name:       "input-protection",
	Severity:   "warning",
	Summary:    "A connector feeds a power-input pin directly with no fuse or TVS in the path.",
	Impact:     "An unprotected power input passes every upstream fault into the board: a shorted load takes out the cable or supply instead of a fuse, and a hot-plug transient or miswired adapter reaches the regulator unclamped. A classic review finding with real field consequences.",
	Remedy:     "Put a fuse and a clamp in the path between the connector and the rail: a fuse for the sustained fault, a TVS for the transient.",
	Primitives: []string{"select", "traverse", "exists", "pin-role", "pattern", "reach"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net", "pin.electrical_type"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistPublicReference,
	},
	Detail:              ruleDoc("input-protection"),
	Eval:                inputProtectionVerdicts,
	StatesConsideredSet: true,
}

// inputProtectionVerdicts decides every connector net that actually feeds a power input, and the
// second half of that sentence is the part a bool could not express. `UnprotectedPowerReach` is false
// for a fused 12 V entry and equally false for a USB data pair, because in neither case did it find
// something to complain about. Passing both would put every signal pin on a connector into the list
// of things somebody checked for a fuse. `PowerPathProtection` reports the load COUNT for exactly
// this reason, and a connector net reaching no supply pin yields no verdict at all.
//
// The pass names the fuse or TVS it credits, so the evidence points at a part rather than asserting
// that one exists. The failure names the rail that is exposed, which is more useful than naming the
// connector again: on a board with a protected 5 V path and a bare 3V3 path off one connector, the
// finding is about the 3V3 path and the walk already knows which one it is.
func inputProtectionVerdicts(m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range m.Nets() {
		if m.IsGroundNet(n) {
			continue // the return path, not a power entry
		}
		hasConn := check.Exists(n.Connections, func(c *ir.Connection) bool {
			return m.HasClass(c.ComponentRef, check.ClassConnector)
		})
		if !hasConn {
			continue // no connector on it, so nothing enters the board here
		}
		report := check.PowerPathProtection(m, n)
		if report.Loads == 0 {
			continue // a connector net that feeds no supply pin is a signal, not a power entry
		}

		v := check.Verdict{Kind: check.KindNet, Subject: n.Name, NetID: n.GetId()}
		switch {
		case n.Attributes[netgraph.AttrExternal] == "true":
			v.Outcome = check.NotConsidered
			v.Reason = "the net continues onto a sheet this read did not open, so its fuse or clamp may be drawn outside it"
		case report.Unprotected != "":
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("the connector reaches power input %s with no fuse crossed and no clamp on the path", report.Unprotected),
				Terms: []check.WitnessTerm{
					{Label: "power entries reached", Value: strconv.Itoa(report.Loads)},
					{Label: "unprotected entry", Value: report.Unprotected},
				},
			}
			f := check.NetFinding("connector feeds a power input with no fuse or TVS in the path")(n)
			v.Finding = &f
		default:
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("%s guards the path from the connector to all %d power entries it reaches", report.Protector, report.Loads),
				Terms: []check.WitnessTerm{
					{Label: "power entries reached", Value: strconv.Itoa(report.Loads)},
					{Label: "protector", Value: report.Protector},
				},
			}
			v.Context = compContext(report.Protector, "fuse or clamp on the path")
		}
		out = append(out, v)
	}
	return out
}

// inputProtectionSpec is the rule's declarative twin (WS3-003). The guard clauses stay
// AST; the reach walk is one declared FFI shared with the Go Eval. The WS3-011
// vocabulary is new, so the Go side stays canonical (twin discipline:
// docsite/content/build/check-rule.md).
var inputProtectionSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "ground_name", Args: []check.Term{check.Fact{Name: "net.names"}}}}},
		check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "component.class"}, Op: "==", R: check.Lit{V: "connector"}}},
		check.IsTrue{T: check.Call{Fn: "unprotected_power_reach"}},
	}},
	Message: "connector feeds a power input with no fuse or TVS in the path",
}
