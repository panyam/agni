package check

import "testing"

// The id is what makes a CLI row addressable, so what it is INSENSITIVE to matters as much as what
// it is built from.
func TestVerdictIDShape(t *testing.T) {
	for _, c := range []struct {
		name, want string
		v          Verdict
	}{
		{"pin joins ref-des and designator", "pin-exceeds-abs-max:(pin:U12.7)",
			Verdict{Subjects: []Entity{{Kind: KindPin, Ref: "U12", Pin: "7"}}, Rule: "pin-exceeds-abs-max"}},
		{"net is its name", "i2c-pull-up:(net:SDA)",
			Verdict{Subjects: []Entity{{Kind: KindNet, Ref: "SDA"}}, Rule: "i2c-pull-up"}},
		{"component is its ref-des", "decoupling-present:(component:U3)",
			Verdict{Subjects: []Entity{{Kind: KindComponent, Ref: "U3"}}, Rule: "decoupling-present"}},
		{"a colon in the ref survives, since the ref is the remainder", "symbol-unresolved:(symbol:Library:Symbol)",
			Verdict{Subjects: []Entity{{Kind: KindSymbol, Ref: "Library:Symbol"}}, Rule: "symbol-unresolved"}},
	} {
		if got := VerdictID(c.v); got != c.want {
			t.Errorf("%s: want %q, got %q", c.name, c.want, got)
		}
	}
}

// THE PROPERTY THE LINK RESTS ON. A verdict id must not move when the ANSWER moves, or a link filed
// against a passing check breaks the moment it starts failing, which is exactly when someone wants
// to follow it.
func TestVerdictIDIgnoresTheOutcome(t *testing.T) {
	base := Verdict{Subjects: []Entity{{Kind: KindPin, Ref: "U12", Pin: "7"}}, Rule: "pin-exceeds-abs-max"}

	ids := map[string]bool{}
	for _, o := range []Outcome{Pass, Fail, NoLimit, NotConsidered} {
		v := base
		v.Outcome = o
		ids[VerdictID(v)] = true
	}
	if len(ids) != 1 {
		t.Errorf("the id must survive a flip in the answer, got %d distinct ids: %v", len(ids), ids)
	}
}

// Nor may it move when the PROSE moves. A reworded message is a cosmetic change and must not
// invalidate every link ever filed against the rule.
func TestVerdictIDIgnoresProseAndEvidence(t *testing.T) {
	base := Verdict{Subjects: []Entity{{Kind: KindNet, Ref: "SDA"}}, Rule: "i2c-pull-up", Outcome: Pass}
	plain := VerdictID(base)

	withProof := base
	withProof.Witness = &Witness{
		Statement: "SDA reaches rail +3V3 through R1",
		Terms:     []WitnessTerm{{Label: "hop limit", Value: "3"}},
	}
	withProof.Context = []ContextSubject{{Entity: Entity{Kind: KindComponent, Ref: "R1"}, Role: "pull-up"}}
	withProof.Reason = "some reason"
	withProof.Subjects[0].NetID = "82ddd812ce0e"

	if got := VerdictID(withProof); got != plain {
		t.Errorf("evidence is not identity: %q became %q", plain, got)
	}
}

// Two rules asking about one subject are two different questions and must not collide, and neither
// must one rule asking about two subjects.
func TestVerdictIDSeparatesRulesAndSubjects(t *testing.T) {
	seen := map[string]string{}
	for _, v := range []Verdict{
		{Subjects: []Entity{{Kind: KindPin, Ref: "U12", Pin: "7"}}, Rule: "pin-exceeds-abs-max"},
		{Subjects: []Entity{{Kind: KindPin, Ref: "U12", Pin: "7"}}, Rule: "pin-out-of-recommended"},
		{Subjects: []Entity{{Kind: KindPin, Ref: "U12", Pin: "14"}}, Rule: "pin-exceeds-abs-max"},
		// A net and a component may legitimately share a name, which is why Kind stays in the key.
		{Subjects: []Entity{{Kind: KindNet, Ref: "U3"}}, Rule: "r"},
		{Subjects: []Entity{{Kind: KindComponent, Ref: "U3"}}, Rule: "r"},
	} {
		id := VerdictID(v)
		if prev, dup := seen[id]; dup {
			t.Errorf("id %q collides: %s and %s", id, prev, SubjectRefs(v))
		}
		seen[id] = SubjectRefs(v)
	}
}
