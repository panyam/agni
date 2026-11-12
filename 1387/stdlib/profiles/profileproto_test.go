package profiles

import (
	"reflect"
	"testing"
)

// fullProfile is a profile with EVERY field set to a distinguishable non-zero value, including the
// ones a conversion is most likely to forget. A drift guard is only as good as its fixture. A field
// left at its zero value round-trips even through a conversion that drops it. Zero in equals zero
// out, and the comparison cannot tell the difference.
func fullProfile() Profile {
	return Profile{
		Name:        "acme-bus",
		HostAttrKey: "interface",
		HostAttrVal: "ACME",
		HostClass:   "crystal",
		Signals: []Signal{{
			Name:   "TXD",
			Prefix: "ACME_",
			Suffix: "_TX",
			Glob:   "ACME_*_TX",
			Regex:  "^ACME_.*_TX$",
			PullUp: true,
			Anchor: true,
		}},
		Requirements: []Requirement{{
			Type:   "termination",
			Params: map[string]string{"a": "_P", "b": "_N"},
		}},
		Instance: InstanceAnchorStem,
		Excludes: []Signal{{Name: "DET", Prefix: "ACME_", Suffix: "_DET", Glob: "ACME_*_DET", Regex: "_DET$"}},
	}
}

// TestProfileProtoRoundTrip is the field-drift guard for the interface-profile half of the
// rule-definition contract (WS3-103). This converter had no such test before.
//
// Going Profile -> proto -> Profile and requiring deep equality means a conversion that forgets a
// field cannot pass, because the returned struct differs from the one that went in. Asserting on
// the proto instead would not catch it. The conversion never writes the field, so the field is
// absent from both sides of that comparison, and the two agree about nothing being there.
//
// service's manifest conversion has had the same guard since it was written
// (TestManifestProtoRoundTrip). Profiles went without one, and HostClass (WS3-044) was silently
// dropped in consequence. A profile binding its host by datasheet device class lost the binding
// crossing stdlib/ruledef, so no host was found and the host path was silent. Silence is also the
// documented behaviour when no params are seeded, so nobody could see the failure.
func TestProfileProtoRoundTrip(t *testing.T) {
	want := fullProfile()
	got := ProfileFromProto(ProfileProto(want))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip lost or changed a field:\n got %+v\nwant %+v", got, want)
	}
}

// TestProfileProtoCarriesClassOnlyHost pins the case the drop actually broke, a profile whose ONLY
// host binding is a device class. HasHost gates every host-dependent requirement, so losing
// HostClass turns a profile that has a host into one that does not, and the requirement compiles to
// nothing instead of erroring.
func TestProfileProtoCarriesClassOnlyHost(t *testing.T) {
	p := Profile{
		Name:      "xtal-load",
		HostClass: "crystal",
		Signals:   []Signal{{Name: "XIN", Suffix: "_XIN", Anchor: true}},
	}
	if !p.HasHost() {
		t.Fatal("fixture is wrong: a class-only binding must count as a host")
	}
	if got := ProfileFromProto(ProfileProto(p)); !got.HasHost() {
		t.Errorf("a class-only host binding did not survive the wire form: HostClass = %q", got.HostClass)
	}
}
