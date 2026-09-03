package service

import (
	"reflect"
	"sort"
	"testing"

	"github.com/panyam/agni/core/check"
)

// fullVerdict is the C26 fixture: EVERY wire-carried field set to a distinguishable non-zero value.
//
// The fixture is the load-bearing part of the guard. A field left at its zero value round-trips
// cleanly through a conversion that drops it, because zero in is zero out, so a sparse fixture
// reports success while covering nothing.
func fullVerdict() check.Verdict {
	// A TWO-entity tuple, deliberately. A repeated field round-trips at arity one for any converter
	// that drops everything past the first element, so a one-subject fixture would pass on the bug
	// this change exists to make impossible.
	return check.Verdict{Subjects: []check.Entity{
		{Kind: check.KindPin, Ref: "U12", Pin: "7", NetID: "82ddd812ce0e"},
		{Kind: check.KindNet, Ref: "+5V", NetID: "9de85e683cf7"},
	}, Rule: "pin-exceeds-abs-max", Outcome: check.Inconclusive, Reason: "pin could not be resolved to a datasheet terminal", Witness: &check.Witness{
		Statement: "3.3 V is within the absolute maximum of 3.6 V",
		Terms: []check.WitnessTerm{
			{Label: "nominal", Value: "3.3 V"},
			{Label: "absolute maximum", Value: "3.6 V"},
		},
		Datasheet: []*check.DatasheetCitation{{
			Doc: "ACME-XLAT Rev A", DocRef: "ds", Page: 4, Section: "Absolute Maximum Ratings",
			Method: "hand", Confidence: 0.9, Verification: "verified", VerifiedRevision: "Rev A",
		}},
	}, Context: []check.ContextSubject{
		{Entity: check.Entity{Kind: check.KindNet, Ref: "+3V3", NetID: "9de85e683cf7"}, Role: "rail"},
		{Entity: check.Entity{Kind: check.KindPin, Ref: "U9", Pin: "2"}, Role: "source"},
	}}
}

// C26: the twin and its converter carry a deep-equality round-trip test.
func TestVerdictProtoRoundTrip(t *testing.T) {
	want := fullVerdict()
	if got := VerdictFromProto(VerdictProto(want)); !reflect.DeepEqual(got, want) {
		t.Errorf("round trip lost or changed a field\n got: %+v\nwant: %+v", got, want)
	}
}

// Every outcome must survive the enum crossing. An outcome that mapped to UNSPECIFIED and back to ""
// would turn a considered subject into an unreadable one, and the default arm of the mapping is
// deliberately UNSPECIFIED rather than PASS so such a bug cannot read as "fine".
func TestEveryOutcomeRoundTrips(t *testing.T) {
	for _, o := range []check.Outcome{check.Pass, check.Fail, check.NoLimit, check.NotConsidered, check.Inconclusive} {
		v := fullVerdict()
		v.Outcome = o
		if got := VerdictFromProto(VerdictProto(v)).Outcome; got != o {
			t.Errorf("outcome %q came back as %q", o, got)
		}
	}
}

// The id is DERIVED, so an inbound one is not authority. Trusting it would let a producer rename a
// verdict by asserting a different name, and the whole point of deriving it is that two sides compute
// the same name without negotiating.
func TestVerdictIDIsDerivedNotTrusted(t *testing.T) {
	p := VerdictProto(fullVerdict())
	if p.GetId() != check.VerdictID(fullVerdict()) {
		t.Errorf("outbound id must be the derived one, got %q", p.GetId())
	}
	p.Id = "attacker-chosen:net:whatever"
	if got := check.VerdictID(VerdictFromProto(p)); got != check.VerdictID(fullVerdict()) {
		t.Errorf("an inbound id must not survive into identity, got %q", got)
	}
}

// THE GUARD C26 ACTUALLY ASKS FOR, since the round trip above can only test fields it knows about.
// A new field on check.Verdict that the converter never learned is absent from both sides of every
// assertion made on the proto, which is exactly how naming.Lexicon and Profile.HostClass each shipped
// a silently dropped field. This fails when the struct grows, so the next person has to SAY whether
// the field belongs on the wire.
func TestVerdictFieldCensus(t *testing.T) {
	// Finding is knowingly absent from the wire: a failing verdict's finding travels in
	// CheckDesignResponse.findings, and sending it here too would put one defect on the wire twice.
	wire := []string{"Context", "Outcome", "Reason", "Rule", "Subjects", "Witness"}
	goOnly := []string{"Finding"}

	var got []string
	rt := reflect.TypeOf(check.Verdict{})
	for i := 0; i < rt.NumField(); i++ {
		got = append(got, rt.Field(i).Name)
	}
	sort.Strings(got)
	want := append(append([]string{}, wire...), goOnly...)
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("check.Verdict's fields changed.\n got: %v\nwant: %v\n\n"+
			"A new field is not covered by TestVerdictProtoRoundTrip until it is in the fixture. "+
			"Decide whether it belongs on the wire: if it does, add it to the proto, both converters "+
			"and fullVerdict(), then list it in `wire` here. If it does not, list it in `goOnly` and "+
			"say why.", got, want)
	}
}
