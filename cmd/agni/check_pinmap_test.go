package main

import (
	"context"
	"html"
	"regexp"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	mcuIOMapFixture  = "testdata/github-issues/1188-mcu-io-map"
	phySignalFixture = "testdata/github-issues/1189-ic-signal-chains"
)

// pinMapRows runs a design's IO-map rules as `check --verdicts --format json` and tabulates the run, so
// the rows are the ones a csv or a workbook of the same run carries.
func pinMapRows(t *testing.T, design string, rules ...string) [][]string {
	t.Helper()
	args := []string{"--verdicts", "--format", "json"}
	for _, r := range rules {
		args = append(args, "--rule", r)
	}
	var run webapi.CheckDesignResponse
	if err := protojson.Unmarshal([]byte(runCheck(t, append(args, design)...)), &run); err != nil {
		t.Fatal(err)
	}
	resp, err := service.TableService{}.Tabulate(context.Background(), &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Check{Check: &run}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tb := range resp.GetTables() {
		if tb.GetName() == "pin_map" {
			var rows [][]string
			for _, r := range tb.GetRows() {
				rows = append(rows, r.GetCells())
			}
			return rows
		}
	}
	return nil
}

var ioMapRules = []string{"intent/io-map-pin-mismatch", "intent/io-map-net-absent", "intent/io-map-far-end"}

// The two tables agni issues 1188 and 1189 asked for, from the synthetic designs they shipped: each
// declared pin beside the net the netlist puts there, and each signal's chain to its partner.
func TestPinMapTableAnswersBothIssues(t *testing.T) {
	for _, tc := range []struct {
		design string
		want   [][]string
	}{
		{mcuIOMapFixture, [][]string{
			{"U1", "PTA0", "GPIO_LED", "LED_NET", "LED_NET", "", "", "pass", "pass", "not-considered", "pass"},
			{"U1", "PTA1", "UART_TX", "UART_TX_NET", "", "", "", "not-considered", "fail", "not-considered", "fail"},
		}},
		{phySignalFixture, [][]string{
			{"U2", "MDIO", "MDIO", "MDIO_NET", "MDIO_NET", "U3.MDIO", "MDIO_NET -> [R1] -> MDIO_OUT", "pass", "pass", "pass", "pass"},
			{"U2", "RST", "RESET", "RST_N", "RST_N", "U3.RST", "", "pass", "pass", "fail", "fail"},
		}},
	} {
		got := pinMapRows(t, tc.design, ioMapRules...)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: %d pin_map rows, want %d: %v", tc.design, len(got), len(tc.want), got)
		}
		for i := range tc.want {
			if strings.Join(got[i], ",") != strings.Join(tc.want[i], ",") {
				t.Errorf("%s row %d:\n got %v\nwant %v", tc.design, i, got[i], tc.want[i])
			}
		}
	}
}

// A run that selected none of the IO-map rules judged no row, so it carries no map and gives no table.
func TestPinMapTableNeedsAnIOMapRule(t *testing.T) {
	if rows := pinMapRows(t, mcuIOMapFixture, "single-pin-net"); rows != nil {
		t.Errorf("a run without an IO-map rule gave pin_map rows: %v", rows)
	}
}

var pinMapSection = regexp.MustCompile(`(?s)<section class="pin-map">.*?</section>`)

// sectionText is a pin-map section of the HTML report with the markup removed.
func sectionText(page string) []string {
	var out []string
	for _, s := range pinMapSection.FindAllString(page, -1) {
		s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, " ")
		out = append(out, strings.Join(strings.Fields(html.UnescapeString(s)), " "))
	}
	return out
}

func TestCheckHTMLHasASectionPerDevice(t *testing.T) {
	for _, tc := range []struct {
		design string
		want   []string
	}{
		{mcuIOMapFixture, []string{"Pins on U1", "fail PTA1 UART_TX UART_TX_NET (none)", "net fail"}},
		{phySignalFixture, []string{"Pins on U2", "pass MDIO MDIO MDIO_NET MDIO_NET U3.MDIO MDIO_NET -> [R1] -> MDIO_OUT", "fail RST RESET RST_N RST_N U3.RST"}},
	} {
		secs := sectionText(runCheck(t, "--format", "html", tc.design))
		if len(secs) != 1 {
			t.Fatalf("%s: %d pin-map sections, want 1", tc.design, len(secs))
		}
		for _, w := range tc.want {
			if !strings.Contains(secs[0], w) {
				t.Errorf("%s: the section lacks %q:\n%s", tc.design, w, secs[0])
			}
		}
	}
}
