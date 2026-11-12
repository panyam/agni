package service

import (
	"github.com/panyam/agni/core/check"
	rpt "github.com/panyam/agni/core/report"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/stdlib/rules/intent"
)

// The pin map lays out a design intent's IO map one row per declared pin. Each row is joined to the
// conclusions of the four per-row IO-map rules (agni issues 1188, 1189, 667). The rules are kept
// apart, because a reviewer acts on each answer differently. A reviewer signing off the map wants
// this reading, with the declaration beside the netlist and the datasheet.

var (
	ioMapPinRule    = intent.SourceName + "/" + intent.RuleIOMapPin
	ioMapNetRule    = intent.SourceName + "/" + intent.RuleIOMapNetAbsent
	ioMapFarEndRule = intent.SourceName + "/" + intent.RuleIOMapFarEnd
	ioMapCoverRule  = intent.SourceName + "/" + intent.RuleIOMapCoverage
	ioMapFuncRule   = intent.SourceName + "/" + intent.RuleIOMapFunction
)

// PinMapColumns is the pin_map table's header. pin is the map's spelling. wired_net is the resolved
// pin's net, empty when the pin resolved to no net or did not resolve. The four _check columns are
// each rule's outcome. outcome is the row's own. It is fail when any rule failed, then inconclusive,
// then pass, and not-considered when no rule could judge the row.
var PinMapColumns = []webapi.TableColumn{
	{Name: "device", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "pin", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "function"},
	{Name: "net", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "wired_net", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "far_end", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
	{Name: "route"},
	{Name: "pin_check"},
	{Name: "net_check"},
	{Name: "far_end_check"},
	{Name: "function_check"},
	{Name: "outcome"},
}

// ranIOMap reports whether a run's rules include one that the pin map reads. Only such a run carries
// its map. A run selecting other rules carries none, since its table would hold rows nothing judged.
func ranIOMap(rules []*check.Rule) bool {
	for _, r := range rules {
		switch r.Name {
		case ioMapPinRule, ioMapNetRule, ioMapFarEndRule, ioMapCoverRule, ioMapFuncRule:
			return true
		}
	}
	return false
}

// PinMapTable returns one row per declared pin, in the map's order, each joined to its verdicts. A
// rule left out of the run leaves its column empty. PinMapTable returns nil when the run carries no
// map.
func PinMapTable(ioMap []*configpb.PinAssignment, verdicts []*checkspb.Verdict) *webapi.Table {
	if len(ioMap) == 0 {
		return nil
	}
	t := &webapi.Table{Name: "pin_map", Columns: columns(PinMapColumns)}
	for _, a := range ioMap {
		pinV := rowVerdict(verdicts, ioMapPinRule, a)
		farV := rowVerdict(verdicts, ioMapFarEndRule, a)
		fnV := rowVerdict(verdicts, ioMapFuncRule, a)
		netV := netVerdict(verdicts, a.GetNet())
		wired := contextRef(pinV, intent.RoleOnThePin)
		var farEnd string
		if to := a.GetTo(); to != nil {
			farEnd = to.GetDevice() + "." + to.GetPin()
		}
		var route string
		for _, term := range farV.GetWitness().GetTerms() {
			if term.GetLabel() == "route" {
				route = term.GetValue()
			}
		}
		t.Rows = append(t.Rows, &webapi.TableRow{Cells: []string{
			a.GetDevice(),
			a.GetPin(),
			a.GetFunction(),
			a.GetNet(),
			wired,
			farEnd,
			route,
			outcomeCell(pinV),
			outcomeCell(netV),
			outcomeCell(farV),
			outcomeCell(fnV),
			rowOutcome(pinV, netV, farV, fnV),
		}})
	}
	return t
}

// rowVerdict finds a per-row rule's verdict on one map row. The row's key is its device, its pin and
// its net. The subject names the device and the pin. Once the lookup succeeded, the pin is the
// design's designator, and the "declared pin" context holds the map's spelling when the two differ.
// The "declared net" context tells apart two rows naming one pin.
func rowVerdict(verdicts []*checkspb.Verdict, rule string, a *configpb.PinAssignment) *checkspb.Verdict {
	for _, v := range verdicts {
		if v.GetRule() != rule || len(v.GetSubjects()) == 0 || contextRef(v, intent.RoleDeclaredNet) != a.GetNet() {
			continue
		}
		s := v.GetSubjects()[0]
		if s.GetRef() != a.GetDevice() {
			continue
		}
		if s.GetPin() == a.GetPin() || contextRef(v, intent.RoleDeclaredPin) == a.GetDevice()+"."+a.GetPin() {
			return v
		}
	}
	return nil
}

// netVerdict finds io-map-net-absent's verdict on a declared net. That rule gives one verdict per net
// rather than one per row, so two rows naming one net share it.
func netVerdict(verdicts []*checkspb.Verdict, net string) *checkspb.Verdict {
	for _, v := range verdicts {
		if v.GetRule() == ioMapNetRule && len(v.GetSubjects()) > 0 && v.GetSubjects()[0].GetRef() == net {
			return v
		}
	}
	return nil
}

// contextRef returns the entity that a verdict's context names under a role, in EntityRef's spelling.
func contextRef(v *checkspb.Verdict, role string) string {
	for _, c := range v.GetContext() {
		if c.GetRole() == role {
			return subjectRef(c.GetSubject())
		}
	}
	return ""
}

// outcomeCell is a verdict's outcome word, empty when the run holds no verdict for the row.
func outcomeCell(v *checkspb.Verdict) string {
	if v == nil {
		return ""
	}
	return OutcomeWord(v.GetOutcome())
}

// rowOutcome returns the row's own outcome. That is the worst outcome any rule decided, or
// not-considered when no rule decided anything. A row one rule passed and another could not judge
// passes, since each rule's column says which judged it.
func rowOutcome(vs ...*checkspb.Verdict) string {
	rank := map[checkspb.Outcome]int{
		checkspb.Outcome_OUTCOME_FAIL:         3,
		checkspb.Outcome_OUTCOME_INCONCLUSIVE: 2,
		checkspb.Outcome_OUTCOME_PASS:         1,
	}
	worst := checkspb.Outcome_OUTCOME_NOT_CONSIDERED
	for _, v := range vs {
		if v != nil && rank[v.GetOutcome()] > rank[worst] {
			worst = v.GetOutcome()
		}
	}
	return OutcomeWord(worst)
}

// PinMapSections groups the pin_map table by device, in the order each device first appears. The
// HTML report lays it out the same way ("Pins on U1"). It reads the table rather than the run, so
// the page and every csv of the run encode one projection (C35).
func PinMapSections(t *webapi.Table) []rpt.PinMap {
	if t == nil {
		return nil
	}
	var out []rpt.PinMap
	at := map[string]int{}
	for _, r := range t.GetRows() {
		c := r.GetCells()
		i, ok := at[c[0]]
		if !ok {
			i = len(out)
			at[c[0]] = i
			out = append(out, rpt.PinMap{Device: c[0]})
		}
		out[i].Rows = append(out[i].Rows, rpt.PinMapRow{
			Pin: c[1], Function: c[2], Net: c[3], WiredNet: c[4], FarEnd: c[5], Route: c[6],
			PinCheck: check.Outcome(c[7]), NetCheck: check.Outcome(c[8]), FarEndCheck: check.Outcome(c[9]),
			FunctionCheck: check.Outcome(c[10]), Outcome: check.Outcome(c[11]),
		})
	}
	return out
}
