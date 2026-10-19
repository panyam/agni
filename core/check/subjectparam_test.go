package check

import (
	"encoding/json"
	"os"
	"testing"
)

// The spelling is shared with the viewer through the twin fixture, so a change here that the
// viewer's reader does not make fails one of the two suites.
func TestSubjectParamMatchesTheTwin(t *testing.T) {
	b, err := os.ReadFile("testdata/subject_param_twin.json")
	if err != nil {
		t.Fatal(err)
	}
	var twin struct {
		Cases []struct{ Param, Kind, Ref, Pin string }
		Bad   []string
	}
	if err := json.Unmarshal(b, &twin); err != nil {
		t.Fatal(err)
	}
	if len(twin.Cases) < 5 || len(twin.Bad) < 5 {
		t.Fatalf("the twin holds %d cases and %d bad ones, too few to prove anything", len(twin.Cases), len(twin.Bad))
	}
	for _, c := range twin.Cases {
		e, err := ParseSubjectParam(c.Param)
		if err != nil {
			t.Errorf("ParseSubjectParam(%q): %v", c.Param, err)
			continue
		}
		if e.Kind != c.Kind || e.Ref != c.Ref || e.Pin != c.Pin {
			t.Errorf("ParseSubjectParam(%q) = %s %q %q, want %s %q %q", c.Param, e.Kind, e.Ref, e.Pin, c.Kind, c.Ref, c.Pin)
		}
		if got := SubjectParam(Entity{Kind: c.Kind, Ref: c.Ref, Pin: c.Pin}); got != c.Param {
			t.Errorf("SubjectParam(%s %q %q) = %q, want %q", c.Kind, c.Ref, c.Pin, got, c.Param)
		}
	}
	for _, s := range twin.Bad {
		if _, err := ParseSubjectParam(s); err == nil {
			t.Errorf("ParseSubjectParam(%q) read a subject, want it refused", s)
		}
	}
	if got := SubjectParams([]Entity{BusEntity("D[7:0]"), NetNameEntity("SDA"), PinEntity("U1", "")}); len(got) != 1 || got[0] != "net:SDA" {
		t.Errorf("SubjectParams kept %v, want only the net a scope covers", got)
	}
}
