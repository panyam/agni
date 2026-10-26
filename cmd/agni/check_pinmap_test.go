package main

import (
	"context"
	"html"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	mcuIOMapFixture    = "testdata/github-issues/1188-mcu-io-map"
	phySignalFixture   = "testdata/github-issues/1189-ic-signal-chains"
	pinFunctionFixture = "testdata/pin-functions"
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

var ioMapRules = []string{"intent/io-map-pin-mismatch", "intent/io-map-net-absent", "intent/io-map-far-end", "intent/io-map-function"}

// The two tables agni issues 1188 and 1189 asked for, from the synthetic designs they shipped: each
// declared pin beside the net the netlist puts there, and each signal's chain to its partner.
func TestPinMapTableAnswersBothIssues(t *testing.T) {
	for _, tc := range []struct {
		design string
		want   [][]string
	}{
		{mcuIOMapFixture, [][]string{
			{"U1", "PTA0", "GPIO_LED", "LED_NET", "LED_NET", "", "", "pass", "pass", "not-considered", "", "pass"},
			{"U1", "PTA1", "UART_TX", "UART_TX_NET", "", "", "", "not-considered", "fail", "not-considered", "", "fail"},
		}},
		{phySignalFixture, [][]string{
			{"U2", "MDIO", "MDIO", "MDIO_NET", "MDIO_NET", "U3.MDIO", "MDIO_NET -> [R1] -> MDIO_OUT", "pass", "pass", "pass", "", "pass"},
			{"U2", "RST", "RESET", "RST_N", "RST_N", "U3.RST", "", "pass", "pass", "fail", "", "fail"},
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
		design   string
		sections int
		want     []string // in the first section
	}{
		{mcuIOMapFixture, 1, []string{"Pins on U1", "fail PTA1 UART_TX UART_TX_NET (none)", "net fail"}},
		{phySignalFixture, 1, []string{"Pins on U2", "pass MDIO MDIO MDIO_NET MDIO_NET U3.MDIO MDIO_NET -> [R1] -> MDIO_OUT", "fail RST RESET RST_N RST_N U3.RST"}},
		{pinFunctionFixture, 2, []string{"Pins on U1", "fail PTB5 LPI2C1_SDA SENSOR_SCL SENSOR_SCL U2.SCL", "function fail"}},
	} {
		secs := sectionText(runCheck(t, "--format", "html", tc.design))
		if len(secs) != tc.sections {
			t.Fatalf("%s: %d pin-map sections, want %d", tc.design, len(secs), tc.sections)
		}
		for _, w := range tc.want {
			if !strings.Contains(secs[0], w) {
				t.Errorf("%s: the section lacks %q:\n%s", tc.design, w, secs[0])
			}
		}
	}
}

// A declared function checked against the datasheet's alternate-function table (agni issue 667): a
// legal function passes, SDA on the pin that offers SCL fails, and a part with no datasheet entry is
// not-considered rather than passed.
func TestPinMapChecksDeclaredFunctionsAgainstTheDatasheet(t *testing.T) {
	got := pinMapRows(t, pinFunctionFixture, ioMapRules...)
	want := [][]string{
		{"U1", "PTB3", "LPUART1_RX", "BLE_RX", "BLE_RX", "", "", "pass", "pass", "not-considered", "pass", "pass"},
		{"U1", "PTB4", "LPI2C1_SDA", "SENSOR_SDA", "SENSOR_SDA", "U2.SDA", "SENSOR_SDA", "pass", "pass", "pass", "pass", "pass"},
		{"U1", "PTB5", "LPI2C1_SDA", "SENSOR_SCL", "SENSOR_SCL", "U2.SCL", "SENSOR_SCL", "pass", "pass", "pass", "fail", "fail"},
		{"U2", "SDA", "I2C_SDA", "SENSOR_SDA", "SENSOR_SDA", "", "", "pass", "pass", "not-considered", "not-considered", "pass"},
	}
	if len(got) != len(want) {
		t.Fatalf("%d pin_map rows, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if strings.Join(got[i], ",") != strings.Join(want[i], ",") {
			t.Errorf("row %d:\n got %v\nwant %v", i, got[i], want[i])
		}
	}
}

// `check --table pin_map` writes the run's pin_map table as csv, the same rows Tabulate serves, so the
// mission's exercise needs no JSON filter.
func TestCheckTableWritesThePinMapAsCSV(t *testing.T) {
	recs, _ := runCheckCSV(t, "--verdicts", "--table", "pin_map", pinFunctionFixture)
	if len(recs) != 5 || strings.Join(recs[0], ",") != strings.Join(columnNames(service.PinMapColumns), ",") {
		t.Fatalf("want the pin_map header and 4 rows, got %v", recs)
	}
	if got := strings.Join(recs[3], ","); got != "U1,PTB5,LPI2C1_SDA,SENSOR_SCL,SENSOR_SCL,U2.SCL,SENSOR_SCL,pass,pass,pass,fail,fail" {
		t.Errorf("PTB5 row: %s", got)
	}
	for _, tc := range []struct{ args []string }{
		{[]string{"--verdicts", "--format", "csv", "--table", "nope", pinFunctionFixture}},
		{[]string{"--format", "csv", "--table", "pin_map", pinFunctionFixture}},
		{[]string{"--verdicts", "--format", "json", "--table", "pin_map", pinFunctionFixture}},
	} {
		cmd := checkCmd()
		cmd.SetOut(io.Discard)
		cmd.SetArgs(tc.args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("%v: want an error", tc.args)
		}
	}
}
