package check

import (
	"fmt"
	"regexp"

	"github.com/panyam/agni/core/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// This file credits what a part carries inside, as its datasheet states it. An RTC's oscillator
// pins integrate load capacitance, a level translator's ports integrate pull-ups, and a USB power
// switch integrates a current limit and reverse blocking (agni issues 1247, 1248, 1298). Each is a
// fact about the part that the board never shows, so a rule that only reads the board fails a
// correct design. Each credit cites the datasheet row it relied on. A rule that fails without one
// names the part it could not read (UnreadIC), so a reader knows that --params might settle the
// finding.

// rowMatch names a kind of datasheet row by its printed symbol, reduced through alnumUpper, or by its
// printed name, in one base unit.
type rowMatch struct {
	symbols map[string]bool
	name    *regexp.Regexp
	unit    string
}

func (rm rowMatch) find(rows []*parampb.Parameter) (base, raw *parampb.Parameter) {
	for _, row := range rows {
		if !rm.symbols[alnumUpper(row.GetSymbol())] && !rm.name.MatchString(row.GetName()) {
			continue
		}
		if q, ok := param.InBaseUnit(row); ok && q.Unit == rm.unit {
			return q, row
		}
	}
	return nil, nil
}

var (
	// NXP's PCF8523 prints CL(itg) for the 7 pF or 12.5 pF it selects on OSCI and OSCO.
	integratedLoadCap = rowMatch{
		symbols: map[string]bool{"CLITG": true, "CLINT": true, "CINT": true},
		name:    regexp.MustCompile(`(?i)\b(integrated|internal)\s+(load\s+)?capacit`),
		unit:    "F",
	}
	integratedPullUp = rowMatch{
		symbols: map[string]bool{"RPU": true, "RPULLUP": true, "RPUINT": true},
		name:    regexp.MustCompile(`(?i)pull-?up`),
		unit:    param.UnitOhm,
	}
	// TI's TPD3S014 prints IOS for its current limit.
	currentLimit = rowMatch{
		symbols: map[string]bool{"IOS": true, "ILIM": true, "ILIMIT": true, "ICL": true, "IOCP": true},
		name:    regexp.MustCompile(`(?i)current\s+limit`),
		unit:    "A",
	}
	// TI's TPD3S014 prints IREV, reverse leakage current with its input unpowered.
	reverseCurrent = rowMatch{
		symbols: map[string]bool{"IREV": true, "IRCB": true, "IREVLKG": true},
		name:    regexp.MustCompile(`(?i)reverse\s+(leakage\s+)?current`),
		unit:    "A",
	}
)

// pinRows resolves ref's pin on a net to its datasheet pin and returns the rows bound to that pin.
// A part-wide row is not among them, since a credit about one pin must not reach every pin, and a
// spec with no pin data yields nothing. The pin resolves through param.ResolvePin, which refuses an
// ambiguous or conflicting pin rather than guessing.
func pinRows(m Model, ref, pinRef string) (*parampb.PartSpec, *parampb.Pin, []*parampb.Parameter) {
	spec := m.PartSpec(ref)
	if spec == nil || len(spec.GetPins()) == 0 {
		return nil, nil, nil
	}
	pkg := ""
	if p := param.PackageForMPN(spec, m.ComponentMPN(ref)); p != nil {
		pkg = p.GetId()
	}
	pin, err := param.ResolvePin(spec, m.PinName(ref, pinRef), pinRef, pkg)
	if err != nil {
		return nil, nil, nil
	}
	return spec, pin, param.PinParameters(spec, pin.GetId())
}

// pinCredit returns the first fitted part on net n, other than skip, whose datasheet binds a row
// matching rm to the part's pin on n. It also returns that pin and the row.
func pinCredit(m Model, n *ir.Net, skip string, rm rowMatch) (ref string, pin *parampb.Pin, spec *parampb.PartSpec, base, raw *parampb.Parameter) {
	for _, c := range n.GetConnections() {
		ref := c.GetComponentRef()
		if ref == skip || !m.IsFitted(ref) {
			continue
		}
		spec, pin, rows := pinRows(m, ref, c.GetPinRef())
		if base, raw := rm.find(rows); base != nil {
			return ref, pin, spec, base, raw
		}
	}
	return "", nil, nil, nil, nil
}

// rowValue is a row's typical value, else its maximum, else its minimum, in its base unit.
func rowValue(q *parampb.Parameter) float64 {
	v := q.GetValue()
	switch {
	case v.Typ != nil:
		return v.GetTyp()
	case v.Max != nil:
		return v.GetMax()
	}
	return v.GetMin()
}

func cite(w *Witness, spec *parampb.PartSpec, raw *parampb.Parameter) *Witness {
	if cit := DatasheetCitationOf(spec, raw); cit != nil {
		w.Datasheet = []*DatasheetCitation{cit}
	}
	return w
}

// IntegratedLoadCapCredit returns the part on net n, other than skip (the crystal), whose datasheet
// binds an integrated load capacitance to the part's pin on n, with a witness stating the value and
// citing the row. A crystal terminal on such a pin needs no capacitor on the board, since the
// oscillator has its own (agni issue 1247). Returns "" and nil when nothing qualifies, as on every
// design read without --params.
func IntegratedLoadCapCredit(m Model, n *ir.Net, skip string) (string, *Witness) {
	ref, pin, spec, q, raw := pinCredit(m, n, skip, integratedLoadCap)
	if ref == "" || rowValue(q) <= 0 {
		return "", nil
	}
	pf := fmtQty(rowValue(q)*1e12, "pF")
	return ref, cite(&Witness{
		Statement: fmt.Sprintf("%s integrates %s of load capacitance on its %s pin, per its datasheet", ref, pf, pin.GetName()),
		Terms: []WitnessTerm{
			{Label: "oscillator part", Value: ref},
			{Label: "integrated load capacitance", Value: pf},
		},
	}, spec, raw)
}

// IntegratedPullUpCredit returns the part on net n whose datasheet binds an internal pull-up to the
// part's pin on n, with a witness stating the value and citing the row, as a level translator's ports
// or an SD host's command line do (agni issue 1298). The pull-up walk credits it as a pull-up to the
// part's own supply, which the board does not draw. Returns "" and nil without a datasheet.
func IntegratedPullUpCredit(m Model, n *ir.Net) (string, *Witness) {
	ref, pin, spec, q, raw := pinCredit(m, n, "", integratedPullUp)
	if ref == "" || rowValue(q) <= 0 {
		return "", nil
	}
	kohm := fmtQty(rowValue(q)/1e3, "kΩ")
	return ref, cite(&Witness{
		Statement: fmt.Sprintf("%s pulls its %s pin up internally through %s, per its datasheet", ref, pin.GetName(), kohm),
		Terms: []WitnessTerm{
			{Label: "pulled up by", Value: ref},
			{Label: "integrated pull-up", Value: kohm},
		},
	}, spec, raw)
}

// switchLoad returns the current-limiting switch that is the only power load on target, or "" when
// target feeds anything else. target is the net that a power-entry walk reached from a connector. A
// switch on the
// net does not excuse a regulator beside it, so every fitted power-input pin there must belong to one
// switch.
func switchLoad(m Model, target *ir.Net) string {
	sw := ""
	for _, c := range target.GetConnections() {
		ref := c.GetComponentRef()
		if IsVirtualRef(ref) || !m.IsFitted(ref) || ConnDir(m, c) != ir.PinDirection_PIN_DIRECTION_POWER_IN {
			continue
		}
		if !m.HasClass(ref, ClassCurrentLimitSwitch) || (sw != "" && sw != ref) {
			return ""
		}
		sw = ref
	}
	return sw
}

// CurrentLimitCredit returns the current-limiting switch that is the only power load on target,
// with a witness. That switch is the board's overcurrent protection on that entry. input-protection
// otherwise asks a fuse for that job (agni issue 1248). The part's class comes from its datasheet
// (ClassCurrentLimitSwitch), since a regulator limits its current too and is no protection against
// its own failure, and the witness cites the datasheet's current limit when it states one.
func CurrentLimitCredit(m Model, target *ir.Net) (string, *Witness) {
	sw := switchLoad(m, target)
	if sw == "" {
		return "", nil
	}
	w := &Witness{
		Statement: fmt.Sprintf("%s is a current-limiting power switch, per its datasheet", sw),
		Terms:     []WitnessTerm{{Label: "current-limiting switch", Value: sw}},
	}
	spec := m.PartSpec(sw)
	if q, raw := currentLimit.find(spec.GetParameters()); q != nil && rowValue(q) > 0 {
		amps := fmtQty(rowValue(q), "A")
		w.Statement = fmt.Sprintf("%s is a current-limiting power switch that limits at %s, per its datasheet", sw, amps)
		w.Terms = append(w.Terms, WitnessTerm{Label: "current limit", Value: amps})
		cite(w, spec, raw)
	}
	return sw, w
}

// ReverseBlockingCredit returns the current-limiting switch that is the only power load on target,
// with a witness citing the row. The switch's datasheet must specify a reverse current, and a
// switch states one only when it blocks reverse current. It answers backfeed and not reverse
// polarity, and the witness says so (agni issue 1248).
func ReverseBlockingCredit(m Model, target *ir.Net) (string, *Witness) {
	sw := switchLoad(m, target)
	if sw == "" {
		return "", nil
	}
	spec := m.PartSpec(sw)
	q, raw := reverseCurrent.find(spec.GetParameters())
	if q == nil {
		return "", nil
	}
	ua := fmtQty(q.GetValue().GetMax()*1e6, "µA")
	return sw, cite(&Witness{
		Statement: fmt.Sprintf("%s blocks reverse current, at most %s, per its datasheet, which stops backfeed but not a reversed supply", sw, ua),
		Terms: []WitnessTerm{
			{Label: "blocking element", Value: sw},
			{Label: "reverse current", Value: ua},
		},
	}, spec, raw)
}

// UnreadIC returns the first fitted IC on net n, other than skip, that the run has no datasheet for.
// The IC may integrate the thing a rule looked for on the board, so a rule reporting the absence says
// it is unverified rather than settled. "" when every IC on the net was read, or none is there.
func UnreadIC(m Model, n *ir.Net, skip string) string {
	for _, c := range n.GetConnections() {
		ref := c.GetComponentRef()
		if ref == skip || !m.IsFitted(ref) || !m.HasClass(ref, ClassIC) {
			continue
		}
		if m.PartSpec(ref) == nil {
			return ref
		}
	}
	return ""
}
