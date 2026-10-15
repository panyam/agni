package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	rpt "github.com/panyam/agni/core/report"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/encoding/protojson"
)

// orderFixture is the design the order-sensitive tests run on. It has to produce ENOUGH findings
// for a reordering to be detectable. The three-finding EDIF fixture lets a shuffled writer agree
// with itself about one run in six, so a determinism test built on it passes most of the time and
// proves nothing. This one produces eleven.
const orderFixture = "testdata/conformance/showcase.fires.kicad_pro"

// runCheckCSV runs `check --format csv` over the EDIF conformance fixture and returns the parsed
// records, header included, so a test asserts against rows rather than substrings.
func runCheckCSV(t *testing.T, args ...string) ([][]string, string) {
	t.Helper()
	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(append([]string{"--format", "csv"}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check --format csv: %v", err)
	}
	recs, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v\n%s", err, out.String())
	}
	return recs, out.String()
}

func TestCheckCSVHeaderAndRows(t *testing.T) {
	recs, raw := runCheckCSV(t, "--rule", "single-pin-net", "testdata/conformance/fires.edn")

	if len(recs) < 2 {
		t.Fatalf("want a header and at least one finding, got %d records\n%s", len(recs), raw)
	}
	if got := recs[0]; !equalStrings(got, checkCSVColumns) {
		t.Errorf("header = %v, want %v", got, checkCSVColumns)
	}
	for i, rec := range recs {
		if len(rec) != len(checkCSVColumns) {
			t.Errorf("record %d has %d fields, want %d: %v", i, len(rec), len(checkCSVColumns), rec)
		}
	}
	row := recs[1]
	if row[2] != "single-pin-net" {
		t.Errorf("rule = %q, want single-pin-net", row[2])
	}
	if row[3] != "net" || row[4] != "STUB" {
		t.Errorf("subject = (%q, %q), want (net, STUB)", row[3], row[4])
	}
	if !strings.HasSuffix(row[8], "fires.edn") {
		t.Errorf("source_file = %q, want it to end in fires.edn", row[8])
	}
}

// TestCheckCSVMatchesJSON is the anti-drift check. The two machine-readable formats must report the
// same findings for one run. Without it the csv writer can quietly fall behind the json one and
// nothing fails.
func TestCheckCSVMatchesJSON(t *testing.T) {
	recs, _ := runCheckCSV(t, orderFixture)

	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json", orderFixture})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check --format json: %v", err)
	}
	var got struct {
		Findings []struct {
			Rule    string `json:"rule"`
			Subject struct {
				Ref string `json:"ref"`
			} `json:"subject"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json output did not parse: %v", err)
	}

	if len(recs)-1 != len(got.Findings) {
		t.Fatalf("csv has %d findings, json has %d", len(recs)-1, len(got.Findings))
	}
	for i, f := range got.Findings {
		row := recs[i+1]
		// The csv escapes a leading formula character, so compare against the escaped form rather
		// than asserting the two are byte-equal and special-casing it away.
		if row[2] != f.Rule || row[4] != rpt.SanitizeCell(f.Subject.Ref) {
			t.Errorf("row %d = (%q, %q), json = (%q, %q)", i, row[2], row[4], f.Rule, f.Subject.Ref)
		}
	}
}

func TestCheckCSVIsDeterministic(t *testing.T) {
	_, first := runCheckCSV(t, orderFixture)
	_, second := runCheckCSV(t, orderFixture)
	if first != second {
		t.Error("two runs over the same design produced different bytes, so two exports cannot be diffed")
	}
}

// TestCheckCSVEscapesARealNetName is the end-to-end half of the escaping guarantee. The unit test
// above can be satisfied by a helper nothing calls, so this one drives the whole command over a
// fixture that really carries a rail named +3V3 and asserts the emitted cell is escaped. The rail is a
// power symbol's, because KiCad names a root-sheet label "/+3V3", which needs no escaping.
func TestCheckCSVEscapesARealNetName(t *testing.T) {
	recs, raw := runCheckCSV(t, "--rule", "decoupling-present", "testdata/conformance/showcase.fires.kicad_sch")

	for _, rec := range recs[1:] {
		if rec[4] == "'+3V3" {
			return
		}
		if rec[4] == "+3V3" {
			t.Fatalf("net +3V3 emitted unescaped; a spreadsheet evaluates it as a formula on open\n%s", raw)
		}
	}
	t.Fatalf("fixture produced no +3V3 finding, so this test no longer guards anything\n%s", raw)
}

// TestCheckCSVQuotesAwkwardMessages covers the three characters that break a hand-rolled writer.
// All three are legal in a rule message, and a reader that cannot parse them back gets a silently
// truncated table rather than an error.
func TestCheckCSVQuotesAwkwardMessages(t *testing.T) {
	findings := []*checkspb.Finding{{
		Rule:     "synthetic",
		Severity: "error",
		Subject:  &checkspb.Subject{Kind: "net", Ref: "N1"},
		Message:  "has a comma, a \"quote\" and a\nnewline",
		Provenance: &ir.Provenance{
			SourceFile: "a.edn",
		},
	}}

	var buf bytes.Buffer
	if err := writeCheckCSV(&buf, findings); err != nil {
		t.Fatalf("writeCheckCSV: %v", err)
	}
	recs, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v\n%s", err, buf.String())
	}
	if len(recs) != 2 {
		t.Fatalf("want header and one row, got %d records: %v", len(recs), recs)
	}
	if got, want := recs[1][7], "has a comma, a \"quote\" and a\nnewline"; got != want {
		t.Errorf("message round-tripped as %q, want %q", got, want)
	}
}

// TestCheckCSVContextCell pins the flattening of the repeated context field, which is ordered
// because a role may repeat within one finding (issue 349).
func TestCheckCSVContextCell(t *testing.T) {
	findings := []*checkspb.Finding{{
		Rule:    "synthetic",
		Subject: &checkspb.Subject{Kind: "component", Ref: "Y1"},
		Context: []*checkspb.ContextSubject{
			{Role: "terminal", Subject: &checkspb.Subject{Kind: "net", Ref: "XIN"}},
			{Role: "terminal", Subject: &checkspb.Subject{Kind: "net", Ref: "XOUT"}},
		},
	}}
	var buf bytes.Buffer
	if err := writeCheckCSV(&buf, findings); err != nil {
		t.Fatalf("writeCheckCSV: %v", err)
	}
	recs, err := csv.NewReader(strings.NewReader(buf.String())).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v", err)
	}
	if got, want := recs[1][10], "terminal=XIN|terminal=XOUT"; got != want {
		t.Errorf("context = %q, want %q", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestTheEncodeRPCWritesTheCLIsCSV covers agni issue 956: a check run tabulated and encoded through
// the TableService, as the viewer saves one, is byte for byte what `check --format csv` writes.
func TestTheEncodeRPCWritesTheCLIsCSV(t *testing.T) {
	run := func(args ...string) []byte {
		t.Helper()
		cmd := checkCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(append(args, orderFixture))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("check %v: %v", args, err)
		}
		return out.Bytes()
	}
	cli := run("--format", "csv")
	var resp webapi.CheckDesignResponse
	if err := protojson.Unmarshal(run("--verdicts", "--format", "json"), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.GetFindings()) == 0 {
		t.Fatal("the fixture has no findings, so two header rows would compare equal")
	}
	var ts service.TableService
	tab, err := ts.Tabulate(context.Background(), &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Check{Check: &resp}})
	if err != nil {
		t.Fatal(err)
	}
	enc, err := ts.Encode(context.Background(), &webapi.EncodeTablesRequest{Tables: tab.GetTables()[:1], Encoding: webapi.TableEncoding_TABLE_ENCODING_CSV})
	if err != nil {
		t.Fatal(err)
	}
	if got := enc.GetFiles()[0]; got.GetName() != "findings" || !bytes.Equal(got.GetContent(), cli) {
		t.Errorf("Encode wrote %s:\n%s\ncheck --format csv wrote:\n%s", got.GetName(), got.GetContent(), cli)
	}
}
