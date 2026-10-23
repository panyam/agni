package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
)

// probedOneCLI asks the library member with a `not` in it, over the tutorial gateway, where C1's GND
// end carries no test point.
const probedOneCLI = `component.probed_one(?r, ?p, ?u) => ?r, ?u`

// --witnesses prints, under each row, the member's rule with its body facts and the `not` that held
// (agni issue 794), in every format that renders a document.
func TestQueryWitnessesRenderUnderEachRow(t *testing.T) {
	freshWorkspace(t)
	design := gatewayDesign
	for _, tc := range []struct {
		format string
		want   []string
	}{
		{"text", []string{
			"\n    component.probed_one(C1, PMIC_CORE_3V3, GND)  via component.probed_one(?r, ?probed, ?unprobed) :- ",
			"\n        component.net(TP1, PMIC_CORE_3V3)  [designs/gateway/gateway.edn:PMIC_CORE_3V3]\n",
			"\n      not net.has_test_point(GND)  (absent)\n",
		}},
		{"markdown", []string{"**How each row holds**", "Row 1, C1, GND:", "  not net.has_test_point(GND)  (absent)"}},
		{"html", []string{`<tr class="how"><td colspan="3"><pre>component.probed_one(C1, PMIC_CORE_3V3, GND)`, "not net.has_test_point(GND)  (absent)"}},
	} {
		t.Run(tc.format, func(t *testing.T) {
			out := runQuery(t, design, probedOneCLI, "--witnesses", "--format", tc.format)
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("--witnesses --format %s missing %q:\n%s", tc.format, w, out)
				}
			}
			// Without the flag the same view carries no tree.
			if plain := runQuery(t, design, probedOneCLI, "--format", tc.format); strings.Contains(plain, "net.has_test_point(GND)") {
				t.Errorf("--format %s without --witnesses prints a witness:\n%s", tc.format, plain)
			}
		})
	}
}

// json is the wire message with each row's witnesses, as C31 has it.
func TestQueryWitnessesJSONIsTheWireMessage(t *testing.T) {
	freshWorkspace(t)
	out := runQuery(t, gatewayDesign, probedOneCLI, "--witnesses", "--format", "json")
	var resp webapi.RunQueryResponse
	if err := protojson.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("not protojson of webapi.RunQueryResponse: %v", err)
	}
	if len(resp.GetRows()) == 0 {
		t.Fatal("no rows")
	}
	for _, r := range resp.GetRows() {
		ws := r.GetWitnesses()
		if len(ws) != 1 || ws[0].GetRelation() != "component.probed_one" || len(ws[0].GetChildren()) != 3 || !ws[0].GetChildren()[2].GetNegated() {
			t.Errorf("row %v witnesses %v, want component.probed_one with its three body facts, the last negated", r.GetCells(), ws)
		}
	}
}

// A csv's first row is its header and every row a row of the table, so --witnesses is refused there
// rather than dropped.
func TestQueryWitnessesRefuseCSV(t *testing.T) {
	cmd := queryCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{gatewayDesign, probedOneCLI, "--witnesses", "--format", "csv"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "a csv holds the table alone") {
		t.Errorf("--witnesses --format csv: err = %v, want the refusal", err)
	}
}

// A set asks every query for witnesses, and each section prints its trees.
func TestQuerySetWitnesses(t *testing.T) {
	freshWorkspace(t)
	set := writeFile(t, "set.yaml", "title: Probes\nqueries:\n  - name: one end\n    query: '"+probedOneCLI+"'\n")
	out := runQuery(t, gatewayDesign, "--set", set, "--witnesses")
	if !strings.Contains(out, "== one end ==") || !strings.Contains(out, "not net.has_test_point(GND)  (absent)") {
		t.Errorf("the set's section carries no witness tree:\n%s", out)
	}
}
