package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/panyam/agni/examples/common"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// TestMain clears AGNI_EXAMPLE_DESIGN, for the same reason examples/common does. Every test here
// asserts what the walkthrough says about its BUNDLED fixture, and the variable exists to replace that
// fixture with someone's own board.
func TestMain(m *testing.M) {
	os.Unsetenv(common.DesignPathEnv)
	os.Exit(m.Run())
}

func openDefault(t *testing.T) *session {
	t.Helper()
	s, err := open(common.AskPath("design", defaultDesign).Path())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// The walkthrough says the design's convention adds one rail pattern to the built-ins.
func TestTheDesignAddsOneRailPattern(t *testing.T) {
	s := openDefault(t)
	r, err := s.lex.GetLexicon(context.Background(), &webapi.GetLexiconRequest{Snapshot: s.snap})
	if err != nil {
		t.Fatal(err)
	}
	got := lexiconLines(r)
	for _, want := range []string{"  1 from design (sensor-house)\n", "_[0-9]+V[0-9]$", "^(VCC|VDD|VEE|VBUS|VIN|VOUT|VBAT|VSUP)"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in:\n%s", want, got)
		}
	}
}

// The walkthrough says SENSOR_3V3 is a rail because of the design's pattern and no other.
func TestSensorRailComesFromTheDesign(t *testing.T) {
	s := openDefault(t)
	r, err := s.lex.ExplainNet(context.Background(), &webapi.ExplainNetRequest{Snapshot: s.snap, Net: defaultNet})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.GetRoles()) != 1 || len(r.GetMatches()) != 1 || r.GetMatches()[0].GetSource() != webapi.LexiconSource_LEXICON_SOURCE_DESIGN {
		t.Errorf("want one role from one design pattern, got:\n%s", explainLines(r))
	}
}

// The walkthrough says that without the convention SENSOR_3V3 stops being a rail and both bus lines
// fail i2c-pull-up.
func TestWithoutTheConventionTheBusFails(t *testing.T) {
	s := openDefault(t)
	r, err := s.lex.CompareLexicons(context.Background(), &webapi.CompareLexiconsRequest{Snapshot: s.snap, Conventions: withoutLexicon})
	if err != nil {
		t.Fatal(err)
	}
	want := "role_changes: 1\n  SENSOR_3V3 | rail (convention) | \n" +
		"finding_changes: 2\n" +
		"  appeared | i2c-pull-up | net | SCL | I2C net has no pull-up resistor to a rail\n" +
		"  appeared | i2c-pull-up | net | SDA | I2C net has no pull-up resistor to a rail\n"
	if got := changeLines(r); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
