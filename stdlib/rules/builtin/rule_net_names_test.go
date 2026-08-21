package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// anet builds a net carrying a collapsed alias list ("rank:name" entries, the
// netgraph.AttrAliases wire form) and one pin so it is not drawing noise.
func anet(name string, aliases ...string) *ir.Net {
	n := tnet(name, "U1.1")
	if len(aliases) > 0 {
		n.Attributes = map[string]string{"aliases": strings.Join(aliases, "\x1f")}
	}
	return n
}

func fireSubjects(t *testing.T, r *check.Rule, nets ...*ir.Net) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, f := range r.Findings(check.NewModel(&ir.Design{Nets: nets})) {
		got[check.EntityRef(f.Subject)] = f.Message
	}
	return got
}

// TestDuplicateNetName: two nets stating one name both fire; stub and empty names never
// collide; a unique name is quiet.
func TestDuplicateNetName(t *testing.T) {
	fs := duplicateNetName.Findings(check.NewModel(&ir.Design{Nets: []*ir.Net{
		tnet("VCC", "U1.1"), tnet("VCC", "U2.1"), // same explicit name, two nets -> both fire
		tnet("SIG", "U3.1"),                      // unique -> silent
		tnet("N$1", "U4.1"), tnet("N$1", "U5.1"), // stub names are per-net inventions -> silent
		tnet("", "U6.1"), tnet("", "U7.1"), // empty carries no claim -> silent
	}}))
	if len(fs) != 2 {
		t.Fatalf("fired %d times (%+v), want once per claiming VCC net", len(fs), fs)
	}
	for _, f := range fs {
		if check.EntityRef(f.Subject) != "VCC" || !strings.Contains(f.Message, "2 electrically distinct") {
			t.Errorf("finding %q: %q", f.Subject, f.Message)
		}
	}
}

// TestLabelAliasConflict: two sheet-scoped labels in ONE scope fire; the same leaf name
// in two different scopes is legitimate hierarchy aliasing; a design-wide name plus a
// local nickname is normal.
func TestLabelAliasConflict(t *testing.T) {
	got := fireSubjects(t, labelAliasConflict,
		anet("SIG_A", "1:SIG_A", "1:SIG_B"),                // two locals, root scope -> fires
		anet("/amp1/CTRL", "1:/amp1/CTRL", "1:/amp2/CTRL"), // one name per crossed sheet -> silent
		anet("/amp1/A", "1:/amp1/A", "1:/amp1/B"),          // two locals inside amp1 -> fires
		anet("VCC", "0:VCC", "1:VCC_NICK"),                 // rail + local nickname -> silent
		anet("PLAIN"),                                      // single name -> silent
	)
	want := map[string]string{"SIG_A": "SIG_A, SIG_B", "/amp1/A": "/amp1/A, /amp1/B"}
	if len(got) != len(want) {
		t.Fatalf("fired on %v, want %v", got, want)
	}
	for subj, names := range want {
		if !strings.Contains(got[subj], names) {
			t.Errorf("%s message = %q, want the clashing names %q", subj, got[subj], names)
		}
	}
}

// TestPowerTapConflict: two design-wide names on one net fire; one rail plus local
// aliases stays quiet.
func TestPowerTapConflict(t *testing.T) {
	got := fireSubjects(t, powerTapConflict,
		anet("+3V3", "0:+3V3", "0:+3.3V"),   // rival rails -> fires
		anet("VCC", "0:VCC", "1:PWR_LOCAL"), // rail + nickname -> silent
		anet("GND", "0:GND"),                // hmm: single alias entry never encodes, but be safe
		anet("SOLO"),
	)
	if len(got) != 1 || !strings.Contains(got["+3V3"], "+3.3V, +3V3") {
		t.Fatalf("fired on %v, want +3V3 with both rail names", got)
	}
}
