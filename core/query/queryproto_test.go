package query

import (
	"reflect"
	"testing"
)

// fullQuery is a VALID datalog program touching every node and every field the wire form carries.
//
// Validity is a constraint, not a convenience: QueryFromProto runs ValidateRelations, so a program
// naming a relation that resolves to nothing fails on decode and never reaches the comparison. The
// program therefore defines its own relation (big_net) and otherwise names catalog ones.
//
// Value is the field set most worth covering. It carries four independent things (a string, an
// optional number, an absent marker, and a base unit), and absent in particular exists because an
// unstated field binding to the empty string made ordering comparisons silently pass.
func fullQuery() Query {
	num := 3.3
	return Query{
		Rules: []Rule{{
			Head: Atom{Relation: "big_net", Args: []Term{{Var: "n"}}},
			Body: Body{Literals: []Literal{
				{Pos: &Atom{Relation: "net.pin_count", Args: []Term{{Var: "n"}, {Var: "v"}}}},
				{Neg: &Atom{Relation: "component.class", Args: []Term{
					{Var: "c"}, {Const: &Value{S: "test_point"}},
				}}},
				{Compare: &Compare{
					Left:  Term{Var: "v"},
					Op:    ">",
					Right: Term{Const: &Value{Num: &num, BaseUnit: "V"}},
				}},
				// An absent constant, the variant that exists because "" compared as a number lied.
				{Compare: &Compare{
					Left:  Term{Var: "v"},
					Op:    "!=",
					Right: Term{Const: &Value{Absent: true}},
				}},
			}},
			Hops: 2,
		}},
		Goal:   Body{Literals: []Literal{{Pos: &Atom{Relation: "big_net", Args: []Term{{Var: "n"}}}}}},
		Select: []Term{{Var: "n"}, {Agg: &Aggregate{Func: "count", Var: "n"}}},
	}
}

// TestQueryProtoRoundTrip is the field-drift guard for the datalog half of the rule-definition
// contract (WS3-103), per C26. Nothing called QueryProto from a test before this.
//
// Going Query -> proto -> Query under deep equality means a conversion that forgets a field cannot
// pass. Asserting on the proto would not: a field the conversion never writes is absent from both
// sides of that comparison, so the two agree about nothing being there. This is the guard whose
// absence let Profile.HostClass ship broken in the profiles half of the same contract.
func TestQueryProtoRoundTrip(t *testing.T) {
	want := fullQuery()
	got, err := QueryFromProto(QueryProto(want))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip lost or changed a field:\n got %#v\nwant %#v", got, want)
	}
}
