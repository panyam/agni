package check

import "testing"

// The id must be INJECTIVE over subject tuples, and the delimiters are already inside real refs, so
// this is not a theoretical property.
//
// KindEndpoint's ref is literally "0,0" — a comma sitting in the position the tuple syntax uses as a
// separator. A net name comes from a source file and may carry anything. Without the escape,
// ("A,B") and ("A", "B") are one string and two different verdicts answer to one name.
func TestVerdictIDIsInjectiveOverTuples(t *testing.T) {
	tuples := [][]Entity{
		{{Kind: KindNet, Ref: "A,B"}},
		{{Kind: KindNet, Ref: "A"}, {Kind: KindNet, Ref: "B"}},
		{{Kind: KindEndpoint, Ref: "0,0"}},
		{{Kind: KindEndpoint, Ref: "0"}, {Kind: KindEndpoint, Ref: "0"}},
		{{Kind: KindNet, Ref: "X(Y)"}},
		{{Kind: KindNet, Ref: "X%2CY"}},
		{{Kind: KindNet, Ref: "X,Y"}},
		// THE CASE THAT ACTUALLY COLLIDES without the escape, and the reason the escape is not
		// decoration. A separator is always followed by "<kind>:", so an ordinary comma inside a ref
		// cannot be mistaken for one. A ref containing the whole sequence CAN: unescaped,
		// ("A,net:B") and ("A", "B") both render as `r:(net:A,net:B)`.
		{{Kind: KindNet, Ref: "A,net:B"}},
		// A ref that legitimately carries colons keeps them: kind is a closed vocabulary containing
		// none of the escaped characters, so the element stays unambiguous without escaping them.
		{{Kind: KindSymbol, Ref: "Library:Symbol"}},
	}
	seen := map[string][]Entity{}
	for _, es := range tuples {
		id := VerdictID(Verdict{Rule: "r", Subjects: es})
		if prev, dup := seen[id]; dup {
			t.Errorf("id %q collides:\n %+v\n %+v", id, prev, es)
		}
		seen[id] = es
	}
	if got := VerdictID(Verdict{Rule: "r", Subjects: tuples[len(tuples)-1]}); got != "r:(symbol:Library:Symbol)" {
		t.Errorf("a symbol ref must keep its colons: %q", got)
	}
}

// The case the whole change exists for, stated as one assertion. A high-side FET sits on its input
// rail and its output rail, so a part above breakdown on both is two answers about one ref-des. Under
// a one-entity id they were one name.
func TestVerdictIDSeparatesTheSamePartOnTwoRails(t *testing.T) {
	q1 := Entity{Kind: KindComponent, Ref: "Q1"}
	a := VerdictID(Verdict{Rule: "fet-vdss-below-switched-rail", Subjects: []Entity{q1, {Kind: KindNet, Ref: "+60V"}}})
	b := VerdictID(Verdict{Rule: "fet-vdss-below-switched-rail", Subjects: []Entity{q1, {Kind: KindNet, Ref: "+55V"}}})
	if a == b {
		t.Fatalf("both rails answer to %q, so the report links them to one row", a)
	}
	// And the rail is IN the name, so the id says which answer it is rather than only being unique.
	if want := "fet-vdss-below-switched-rail:(component:Q1,net:+60V)"; a != want {
		t.Errorf("id = %q, want %q", a, want)
	}
}

// A symmetric relation is canonicalised by the RULE, not here. copper-clearance orders its pair by
// name before building the verdict, because a framework that sorted every tuple would invert
// pin-tracking's subject-minus-reference claim. This pins that VerdictID preserves what it is given.
func TestVerdictIDPreservesTupleOrder(t *testing.T) {
	ab := VerdictID(Verdict{Rule: "r", Subjects: []Entity{{Kind: KindNet, Ref: "A"}, {Kind: KindNet, Ref: "B"}}})
	ba := VerdictID(Verdict{Rule: "r", Subjects: []Entity{{Kind: KindNet, Ref: "B"}, {Kind: KindNet, Ref: "A"}}})
	if ab == ba {
		t.Fatal("order is part of the identity for a directional relation; VerdictID must not sort")
	}
}
