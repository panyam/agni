package query

import (
	"reflect"
	"slices"
	"testing"

	"github.com/panyam/jaala/ns"
)

// fullQuery is a VALID datalog program touching every node and every field the wire form carries.
//
// Validity is a constraint, not a convenience. QueryFromProto runs ValidateRelations, so a program
// naming a relation that resolves to nothing fails on decode and never reaches the comparison. The
// program therefore defines its own relation (big_net) and otherwise names catalog ones.
//
// Aggregate.Distinct and Query.Having (agni issue 613) are here for the same reason, and so are the
// body aggregate, the head types and paging, which the wire form dropped without an error until it
// carried them. TestQueryProtoKnowsEveryEngineField is what notices the next field.
//
// Value is the field set most worth covering. It carries four independent things (a string, an
// optional number, an absent marker, and a base unit), and absent in particular exists because an
// unstated field binding to the empty string made ordering comparisons silently pass.
func fullQuery() Query {
	num := 3.3
	two := 2.0
	return Query{
		Rules: []Rule{{
			Head: Atom{Relation: "big_net", Args: []Term{{Var: "n"}}},
			Body: Body{Literals: []Literal{
				{Pos: &Atom{Relation: "net.pin_count", Args: []Term{{Var: "n"}, {Var: "v"}}, Fact: "f"}},
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
				// An expression, nested, with a call and a negation, since each is a different Op.
				{Compare: &Compare{
					Left: Term{Var: "h"},
					Op:   "=",
					Right: Term{Expr: &Expr{Op: "*", Args: []Term{
						{Expr: &Expr{Op: "call", Func: "half", Args: []Term{{Var: "v"}}}},
						{Expr: &Expr{Op: "neg", Args: []Term{{Const: &Value{Num: &num}}}}},
					}}},
				}},
				{Agg: &BodyAggregate{
					Result: "k",
					Agg:    Aggregate{Func: "count", Var: "p", Distinct: true},
					Body: Body{Literals: []Literal{
						{Pos: &Atom{Relation: "pin.net", Args: []Term{{Var: "r"}, {Var: "p"}, {Var: "n"}}}},
					}},
				}},
				{Opt: &Body{Literals: []Literal{
					{Pos: &Atom{Relation: "component.mpn", Args: []Term{{Var: "r"}, {Var: "m"}}}},
				}}},
			}},
			Hops: 2,
			HeadTypes: []ns.ArgType{
				{Kind: "net", KindFrom: "k", Owner: "o", Type: ns.TypeNumber, Unit: "V", Domain: []string{"a", "b"}},
			},
		}},
		Goal:   Body{Literals: []Literal{{Pos: &Atom{Relation: "big_net", Args: []Term{{Var: "n"}}}}}},
		Select: []Term{{Var: "n"}, {Agg: &Aggregate{Func: "count", Var: "n"}}, {Agg: &Aggregate{Func: "list", Var: "n", Distinct: true}}},
		// A having, whose left term is an aggregate. It shares DatalogCompare with a body comparison,
		// so this also asserts the two decode back to their own halves of the query rather than one
		// leaking into the other.
		Having: []Compare{{
			Left:  Term{Agg: &Aggregate{Func: "count", Var: "n", Distinct: true}},
			Op:    ">=",
			Right: Term{Const: &Value{S: "2", Num: &two}},
		}},
		OrderBy: []Order{{Term: Term{Var: "n"}, Desc: true}},
		After:   []Value{{S: "GND"}},
		Limit:   50,
		Offset:  10,
	}
}

// TestQueryProtoRoundTrip is the field-drift guard for the datalog half of the rule-definition
// contract (WS3-103), per C26. Nothing called QueryProto from a test before this.
//
// Going Query -> proto -> Query under deep equality means a conversion that forgets a field cannot
// pass. Asserting on the proto would not, because a field the conversion never writes is absent
// from both sides of that comparison, so the two agree about nothing being there. This is the guard
// whose absence let Profile.HostClass ship broken in the profiles half of the same contract.
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

// TestQueryProtoKnowsEveryEngineField fails when the engine adds a field to its query IR that this
// ledger, and so the wire form, has not been taught. The round trip cannot notice one, since fullQuery
// leaves an unknown field zero on both sides. Adding a name here means carrying it in QueryProto and
// filling it in fullQuery.
func TestQueryProtoKnowsEveryEngineField(t *testing.T) {
	carried := map[reflect.Type][]string{
		reflect.TypeFor[Query]():         {"Rules", "Goal", "Select", "Having", "OrderBy", "After", "Limit", "Offset"},
		reflect.TypeFor[Rule]():          {"Head", "Body", "Hops", "HeadTypes"},
		reflect.TypeFor[Body]():          {"Literals"},
		reflect.TypeFor[Literal]():       {"Pos", "Neg", "Compare", "Agg", "Opt"},
		reflect.TypeFor[BodyAggregate](): {"Result", "Agg", "Body"},
		reflect.TypeFor[Atom]():          {"Relation", "Args", "Fact"},
		reflect.TypeFor[Compare]():       {"Left", "Op", "Right"},
		reflect.TypeFor[Term]():          {"Var", "Const", "Agg", "Expr"},
		reflect.TypeFor[Expr]():          {"Op", "Func", "Args"},
		reflect.TypeFor[Aggregate]():     {"Func", "Var", "Distinct"},
		reflect.TypeFor[Order]():         {"Term", "Desc"},
		reflect.TypeFor[Value]():         {"S", "Num", "Absent", "BaseUnit"},
		reflect.TypeFor[ns.ArgType]():    {"Kind", "KindFrom", "Owner", "Type", "Unit", "Domain"},
	}
	for typ, names := range carried {
		var exported []string
		for f := range typ.Fields() {
			if f.IsExported() {
				exported = append(exported, f.Name)
			}
		}
		for _, f := range exported {
			if !slices.Contains(names, f) {
				t.Errorf("%s.%s is not carried by the wire form", typ.Name(), f)
			}
		}
		for _, n := range names {
			if !slices.Contains(exported, n) {
				t.Errorf("%s.%s is in the ledger but the engine has no such field", typ.Name(), n)
			}
		}
	}
}
