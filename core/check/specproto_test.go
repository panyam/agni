package check

import (
	"reflect"
	"testing"
)

// fullSpec is a VALID spec that uses every Term and Expr node in the closed vocabulary, and sets
// every field on each. Validity is a constraint here rather than an inconvenience: SpecFromProto
// validates what it decodes, so a fixture using a fact or an FFI name this build does not know would
// fail on validation and never reach the comparison the test exists to make.
//
// Every node appears because the guard is only as wide as its fixture. A node type is protected by
// the panic in termProto/exprProto, which fires the first time anything encodes an unhandled node. A
// FIELD on an existing node has no such protection, and that is what this covers.
func fullSpec() Spec {
	return Spec{
		Over:    "nets",
		Message: "every node, once",
		// Scope must be a DISTINGUISHABLE non-zero value, not a copy of Where: a fixture whose two
		// expression fields are equal round-trips cleanly through a converter that assigns one to the
		// other, which is the drift this guard exists to catch (C26's note on sparse fixtures).
		Scope: IsTrue{T: Fact{Name: "net.attr.global"}},
		Let: map[string]Term{
			// CountOf: both fields, and a nested Expr to prove the recursion carries.
			"caps": CountOf{
				Over:  "net.connections",
				Where: Cmp{L: Fact{Name: "component.class"}, Op: "==", R: Lit{V: "capacitor"}},
			},
		},
		Where: And{Xs: []Expr{
			// Cmp with a Var on the left, so the Let binding above is actually referenced.
			Cmp{L: Var{Name: "caps"}, Op: ">", R: Lit{V: 0}},
			Or{Xs: []Expr{
				IsTrue{T: Fact{Name: "net.attr.global"}},
				// Call with both Fn and a non-empty Args.
				IsTrue{T: Call{Fn: "rail_name", Args: []Term{Fact{Name: "net.names"}}}},
			}},
			Not{X: Match{T: Fact{Name: "net.name_leaf"}, Pattern: "^TP_"}},
			In{T: Fact{Name: "net.name_leaf"}, Set: []string{"VCC", "GND"}},
			ExistsIn{Over: "net.connections", Where: IsTrue{T: Fact{Name: "conn.virtual"}}},
		}},
	}
}

// TestSpecProtoRoundTrip is the field-drift guard for the Spec half of the rule-definition contract
// (WS3-103), per C26. Nothing called SpecProto from a test before this.
//
// The node vocabulary was already protected: termProto and exprProto panic on a type they have no
// case for, so adding a node and forgetting its wire form fails loudly the first time it is encoded.
// A FIELD added to an existing node is the gap, because a field the converter does not copy simply
// is not copied and nothing complains. That is how Profile.HostClass was lost, and this is the same
// guard applied here before it happens rather than after.
func TestSpecProtoRoundTrip(t *testing.T) {
	want := fullSpec()
	got, err := SpecFromProto(SpecProto(want))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip lost or changed a field:\n got %#v\nwant %#v", got, want)
	}
}

// TestRuleMetaProtoRoundTrip is the same C26 field-drift guard for the OTHER converter pair in this
// file. RuleMetaProto/RuleMetaFromProto had none, which is the asymmetry C26 exists to close: a rule's
// prose and gates cross stdlib/ruledef through these two functions, and a field the converter never
// learned is absent from both sides of any assertion made on the proto. That is exactly how
// Profile.HostClass was lost.
//
// The fixture sets every carried field to a distinguishable non-zero value, which is the load-bearing
// part of the guard: a field left at its zero value round-trips cleanly through a conversion that
// drops it entirely. Reads and Primitives are deliberately absent from the wire form (they are derived
// from a definition's body by its compiler), so they stay zero on both sides here.
func TestRuleMetaProtoRoundTrip(t *testing.T) {
	want := Rule{
		Name:               "every-field-set",
		Severity:           "warning",
		Summary:            "a one-line summary",
		Impact:             "what goes wrong when it is violated",
		Remedy:             "what to do about it",
		Detail:             "## long-form\n\nmarkdown",
		Tags:               map[string]string{KeyCategory: CategoryConnectivity, KeyTier: "R"},
		OptionalReads:      []string{"param.abs_max_supply"},
		RequiresCapability: []Capability{CapTypesPowerOut, CapNetClass},
	}
	got := RuleMetaFromProto(RuleMetaProto(want))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip lost or changed a field:\n got %#v\nwant %#v", got, want)
	}
}

// TestSpecLitIntNormalizes documents an ASYMMETRY that is deliberate, so nobody "fixes" it into a
// bug. litProto encodes both int and int64, because a hand-written Go spec may carry either, while
// litFromProto always decodes to int. The evaluator speaks int and nothing else in this package
// handles int64 (`case int64` appears only in specproto.go), so decoding to int is decoding to the
// type the rest of the engine can actually evaluate.
//
// The consequence worth knowing: an int64 literal does NOT survive a round trip unchanged, it comes
// back as int. That is a narrowing on paper and a correction in practice, which is why the guard
// above uses int, the type every real spec uses.
func TestSpecLitIntNormalizes(t *testing.T) {
	s := Spec{Over: "nets", Where: Cmp{L: Fact{Name: "net.pin_count"}, Op: ">", R: Lit{V: int64(3)}}}
	got, err := SpecFromProto(SpecProto(s))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	lit := got.Where.(Cmp).R.(Lit)
	if _, isInt := lit.V.(int); !isInt {
		t.Errorf("int64 literal decoded as %T, want int (the type the evaluator handles)", lit.V)
	}
	if lit.V != 3 {
		t.Errorf("int64 literal decoded to %v, want 3", lit.V)
	}
}

// TestScopeContributesDerivedReads pins an invariant that is easy to break and silent when broken:
// moving a clause from Where to Scope must not change what the rule DECLARES it reads.
//
// DerivedReads feeds check.Available, which gates a rule to not-applicable when a tier it reads is
// absent. A fact consulted only in Scope and missing from Reads would leave a rule claiming not to
// need a tier it does need, so it would run against a design that cannot answer it. Scope is a
// relabelling of clauses, not a change to a rule's dependencies.
func TestScopeContributesDerivedReads(t *testing.T) {
	scoped := Spec{
		Over:    "nets",
		Scope:   IsTrue{T: Fact{Name: "net.attr.external"}},
		Where:   Cmp{L: Fact{Name: "net.pin_count"}, Op: "<", R: Lit{V: 2}},
		Message: "x",
	}
	// net.attr.external declares its dependency as "net.attributes": DerivedReads speaks the reads
	// vocabulary, not the fact names.
	var hasScopeFact bool
	for _, r := range scoped.DerivedReads() {
		if r == "net.attributes" {
			hasScopeFact = true
		}
	}
	if !hasScopeFact {
		t.Errorf("a fact read only in Scope is missing from DerivedReads: %v", scoped.DerivedReads())
	}

	// The whole point, stated directly: the same clauses split either way declare the same reads.
	merged := Spec{
		Over: "nets",
		Where: And{Xs: []Expr{
			IsTrue{T: Fact{Name: "net.attr.external"}},
			Cmp{L: Fact{Name: "net.pin_count"}, Op: "<", R: Lit{V: 2}},
		}},
		Message: "x",
	}
	if !reflect.DeepEqual(scoped.DerivedReads(), merged.DerivedReads()) {
		t.Errorf("splitting Where into Scope changed the declared reads\n scoped: %v\n merged: %v",
			scoped.DerivedReads(), merged.DerivedReads())
	}
}
