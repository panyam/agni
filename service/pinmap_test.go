package service

import (
	"strings"
	"testing"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
)

func pinVerdict(rule string, outcome checkspb.Outcome, ref, pin string, ctx map[string]*checkspb.Subject) *checkspb.Verdict {
	v := &checkspb.Verdict{Rule: rule, Outcome: outcome, Subjects: []*checkspb.Subject{{Kind: "pin", Ref: ref, Pin: pin}}}
	for role, s := range ctx {
		v.Context = append(v.Context, &checkspb.ContextSubject{Role: role, Subject: s})
	}
	return v
}

func netSubject(name string) *checkspb.Subject { return &checkspb.Subject{Kind: "net", Ref: name} }

// Two rows naming one pin are two rows, each joined to its own verdict by the declared net, and a row
// the map spells by function name joins through the "declared pin" context. A rule the run left out
// leaves its column empty rather than reading as a judgement.
func TestPinMapTableJoinsEachRowToItsOwnVerdicts(t *testing.T) {
	ioMap := []*configpb.PinAssignment{
		{Device: "U1", Pin: "PTA0", Net: "A"},
		{Device: "U1", Pin: "PTA0", Net: "B"},
		{Device: "U1", Pin: "7", Net: "C", Function: "SPI_CLK"},
	}
	pass, fail := checkspb.Outcome_OUTCOME_PASS, checkspb.Outcome_OUTCOME_FAIL
	verdicts := []*checkspb.Verdict{
		pinVerdict(ioMapPinRule, pass, "U1", "3", map[string]*checkspb.Subject{
			"declared net": netSubject("A"), "declared pin": {Kind: "pin", Ref: "U1", Pin: "PTA0"}, "on the pin": netSubject("A")}),
		pinVerdict(ioMapPinRule, fail, "U1", "3", map[string]*checkspb.Subject{
			"declared net": netSubject("B"), "declared pin": {Kind: "pin", Ref: "U1", Pin: "PTA0"}, "on the pin": netSubject("A")}),
		pinVerdict(ioMapPinRule, pass, "U1", "7", map[string]*checkspb.Subject{"declared net": netSubject("C"), "on the pin": netSubject("C")}),
		pinVerdict(ioMapFuncRule, fail, "U1", "7", map[string]*checkspb.Subject{"declared net": netSubject("C")}),
		{Rule: ioMapNetRule, Outcome: pass, Subjects: []*checkspb.Subject{netSubject("A")}},
		{Rule: ioMapNetRule, Outcome: pass, Subjects: []*checkspb.Subject{netSubject("B")}},
	}
	tb := PinMapTable(ioMap, verdicts)
	want := []string{
		"U1,PTA0,,A,A,,,pass,pass,,,pass",
		"U1,PTA0,,B,A,,,fail,pass,,,fail",
		"U1,7,SPI_CLK,C,C,,,pass,,,fail,fail",
	}
	if len(tb.GetRows()) != len(want) {
		t.Fatalf("%d rows, want %d", len(tb.GetRows()), len(want))
	}
	for i, w := range want {
		if got := strings.Join(tb.GetRows()[i].GetCells(), ","); got != w {
			t.Errorf("row %d:\n got %s\nwant %s", i, got, w)
		}
	}
	if PinMapTable(nil, verdicts) != nil {
		t.Error("a run carrying no map must give no table")
	}
}

// A pin in a verdict's context is named with its pin, as the subjects column names one, so a csv of
// an IO-map run says which pin a row declared (agni issue 1193).
func TestTheContextColumnNamesAPinWithItsPin(t *testing.T) {
	v := pinVerdict(ioMapPinRule, checkspb.Outcome_OUTCOME_PASS, "U1", "3", nil)
	v.Context = []*checkspb.ContextSubject{
		{Role: "declared net", Subject: netSubject("BLE_RX")},
		{Role: "declared pin", Subject: &checkspb.Subject{Kind: "pin", Ref: "U1", Pin: "PTB3"}},
	}
	tb := VerdictsTable([]*checkspb.Verdict{v}, nil)
	col := -1
	for i, c := range tb.GetColumns() {
		if c.GetName() == "context" {
			col = i
		}
	}
	if col < 0 || len(tb.GetRows()) != 1 {
		t.Fatalf("no context column or no row: %v", tb)
	}
	if got := tb.GetRows()[0].GetCells()[col]; got != "declared net=BLE_RX|declared pin=U1.PTB3" {
		t.Errorf("context = %q", got)
	}
}
