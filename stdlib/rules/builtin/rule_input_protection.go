package builtin

import (
	"context"
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
	Reads:      []string{"component.class", "component.device_class", "component.dnp", "net.attributes", "net.names", "on_net", "param.max", "pin.electrical_type"},
	// A current-limiting switch is credited from its datasheet. A run without a corpus has no
	// datasheet, so the rule still judges every entry then (agni issue 1248).
	OptionalReads: []string{"component.device_class", "param.max"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistPublicReference,
	},
	Detail:              ruleDoc("input-protection"),
	Eval:                inputProtectionVerdicts,
	Scoping:             check.ScopeSubjects,
	StatesConsideredSet: true,
}

// inputProtectionVerdicts decides every connector net that actually feeds a power input. A bool
// could not express that half. `UnprotectedPowerReach` is false for a fused 12 V entry and equally
// false for a USB data pair, because in neither case did it find something to complain about.
// Passing both would put every signal pin on a connector into the list of things somebody checked
// for a fuse. `PowerPathProtection` reports the load COUNT for exactly this reason, and a connector
// net reaching no supply pin yields no verdict at all.
//
// The pass names the fuse or TVS it credits, so the evidence points at a part rather than asserting
// that one exists. A net the board drives itself is NotConsidered, since its connector is an outlet
// (check.BoardSource, agni issue 1016). The failure names the exposed rail rather than the
// connector again. Take a board with a protected 5 V path and a bare 3V3 path off one connector. The
// finding is about the 3V3 path, and the walk already knows which one it is.
func inputProtectionVerdicts(ctx context.Context, m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range check.ScopedNets(ctx, m) {
		if m.IsGroundNet(n) {
			continue // the return path, not a power entry
		}
		hasConn := check.Exists(n.Connections, func(c *ir.Connection) bool {
			return m.ExposedConnector(c.ComponentRef)
		})
		if !hasConn {
			continue // no connector on it, so nothing enters the board here
		}
		report := check.PowerPathProtection(m, n)
		if report.Loads == 0 {
			continue // a connector net that feeds no supply pin is a signal, not a power entry
		}

		v := check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindNet, Ref: n.Name, NetID: n.GetId()}}}
		src := check.BoardSource(m, n)
		switch {
		case n.Attributes[netgraph.AttrExternal] == "true":
			v.Outcome = check.NotConsidered
			v.Reason = "the net continues onto a sheet this read did not open, so its fuse or clamp may be drawn outside it"
		case src != "":
			v.Outcome = check.NotConsidered
			v.Reason = "the board drives this net itself, from " + src + ", so its connector is an outlet rather than a power entry"
		case report.Unprotected != "":
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("the connector reaches power input %s with no fuse crossed and no clamp on the path", report.Unprotected),
				Terms: []check.WitnessTerm{
					{Label: "power entries reached", Value: strconv.Itoa(report.Loads)},
					{Label: "unprotected entry", Value: report.Unprotected},
				},
			}
			if report.Unread != "" {
				// The entry's IC may be a current-limiting switch, which only its datasheet says.
				v.Witness.Statement += fmt.Sprintf(", and %s's datasheet was not read, so whether it limits its current is unverified", report.Unread)
				v.Witness.Terms = append(v.Witness.Terms, check.WitnessTerm{Label: "unverified part", Value: report.Unread})
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
			if w := report.ProtectorWitness; w != nil {
				entries := "the power entry the connector reaches"
				if report.Loads > 1 {
					entries = fmt.Sprintf("all %d power entries the connector reaches", report.Loads)
				}
				v.Witness.Statement = w.Statement + ", and it guards " + entries
				v.Witness.Terms = append(v.Witness.Terms, w.Terms...)
				v.Witness.Datasheet = w.Datasheet
				v.Context = compContext(report.Protector, "current-limiting switch")
			}
		}
		out = append(out, v)
	}
	return out
}

// inputProtectionSpec is the rule's declarative twin (WS3-003). The guard clauses are still AST,
// and the reach walk is one declared FFI shared with the Go Eval. The WS3-011 vocabulary is new, so
// the Go side is still canonical (see docsite/content/build/check-rule.md for the twin discipline).
var inputProtectionSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "ground_name", Args: []check.Term{check.Fact{Name: "net.names"}}}}},
		check.ExistsIn{Over: "net.connections", Where: check.IsTrue{T: check.Fact{Name: "component.exposed_connector"}}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.board_driven"}}},
		check.IsTrue{T: check.Call{Fn: "unprotected_power_reach"}},
	}},
	Message: "connector feeds a power input with no fuse or TVS in the path",
}
