package builtin

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/formats"
)

// TestPourClearanceFindsKiCadsZonePairs holds copper-clearance on pours to KiCad's own DRC (agni
// 1410). Jetson's KiCad oracle (hack/seed-triage) lists every clearance violation KiCad found,
// track against zone among them. The .kicad_pcb keeps no fill, so the rule reads the ODB++ job
// KiCad exported from the same board, whose surfaces are the fill KiCad computed. Every
// track-to-zone pair KiCad measured under agni's 0.127mm floor must then be a failing pair here.
// KiCad measures against its own rules, mostly far wider than the floor, so a pair it measured
// above the floor says nothing about this rule and is left out. ODB++ writes a net name's spaces as
// underscores, so names are compared that way. It reads an 85 MB job, so it runs in
// `make odb-oracle` rather than the gate.
func TestPourClearanceFindsKiCadsZonePairs(t *testing.T) {
	if os.Getenv("AGNI_ODB_ORACLE") == "" {
		t.Skip("not the default suite: run `make odb-oracle`, which fetches the export corpus first")
	}
	const job = "../../../tools/samples/odb/kicad/jetson-agx-thor-baseboard.odb.tgz"
	const oracle = "../../../hack/seed-triage/jetson-agx-thor-baseboard.kicad.oracle"
	f, err := os.Open(oracle)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	actual := regexp.MustCompile(`actual ([0-9.]+) mm`)
	want := map[[2]string]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		col := strings.Split(sc.Text(), "\t")
		// A track against a zone: one item a Track, the other a Zone.
		if len(col) < 7 || col[0] != "drc" || col[1] != "clearance" || !strings.HasPrefix(col[5], "Track [") || !strings.HasPrefix(col[6], "Zone [") {
			continue
		}
		m := actual.FindStringSubmatch(col[4])
		if m == nil {
			continue
		}
		if mm, _ := strconv.ParseFloat(m[1], 64); mm >= 0.127-0.0005 {
			continue
		}
		nets := strings.Split(col[3], "|")
		a, b := strings.ReplaceAll(strings.TrimPrefix(nets[0], "net:"), " ", "_"), strings.ReplaceAll(strings.TrimPrefix(nets[1], "net:"), " ", "_")
		if a > b {
			a, b = b, a
		}
		want[[2]string{a, b}] = true
	}
	if len(want) < 10 {
		t.Fatalf("the oracle lists %d track-to-zone pairs under the floor, too few to mean anything", len(want))
	}
	g, err := (&formats.Loader{}).BoardGeometry(job)
	if err != nil {
		t.Fatalf("%v; run `make odb-oracle`", err)
	}
	failing := map[[2]string]bool{}
	for _, v := range copperClearanceVerdicts(context.Background(), check.NewModel(&ir.Design{}, check.WithBoard(g))) {
		if v.Outcome == check.Fail {
			failing[[2]string{v.Subjects[0].Ref, v.Subjects[1].Ref}] = true
		}
	}
	for p := range want {
		if !failing[p] {
			t.Errorf("KiCad measures track %s against zone %s under the floor, and the rule finds no failing pair", p[0], p[1])
		}
	}
	t.Logf("all %d of KiCad's track-to-zone pairs under the floor fail here too", len(want))
}
