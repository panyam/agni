package main

import (
	"html"
	"os"
	"strings"
	"testing"
)

// The verdict table is the bus sign-off grid agni issue 1240 asked for, one row per net
// i2c-pull-up considered with the route that passed it or the bound that failed it. The fixture has
// a pull-up one resistor away, one two resistors away behind a series part, and a bus net with
// none, and the csv is held whole so a change to any route's wording shows in the diff.
func TestPullUpAuditTableNamesEveryRoute(t *testing.T) {
	const design = "testdata/pull-up-audit/bus.edn"
	want, err := os.ReadFile("testdata/pull-up-audit/verdicts.csv.golden")
	if err != nil {
		t.Fatal(err)
	}
	if got := runCheck(t, "--verdicts", "--rule", "i2c-pull-up", "--format", "csv", design); got != string(want) {
		t.Errorf("verdict csv differs from the golden\n got:\n%s\nwant:\n%s", got, want)
	}

	page := runCheck(t, "--rule", "i2c-pull-up", "--format", "html", design)
	for _, s := range []string{
		"SDA reaches rail VCC_3V3 through R1",
		"SCL reaches rail VCC_3V3 along SCL -> [R2] -> BUF_NET -> [R3] -> VCC_3V3",
		"no rail is reachable from I2C2_SDA through a resistor within 3 hops",
	} {
		if !strings.Contains(page, html.EscapeString(s)) {
			t.Errorf("the html report must carry %q", s)
		}
	}
}
