package check

import (
	"fmt"
	"regexp"

	"github.com/panyam/agni/core/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// integratedLoadCapSymbols are the printed symbols of an integrated load capacitance row, reduced
// through alnumUpper. NXP's PCF8523 prints CL(itg) for the 7 pF or 12.5 pF it selects on OSCI and
// OSCO.
var integratedLoadCapSymbols = map[string]bool{"CLITG": true, "CLINT": true, "CINT": true}

// integratedLoadCapName catches a row whose symbol is unfamiliar but whose name says what it is.
var integratedLoadCapName = regexp.MustCompile(`(?i)\b(integrated|internal)\s+(load\s+)?capacit`)

// IntegratedLoadCapCredit returns the part on net n, other than skip (the crystal), whose datasheet
// binds an integrated load capacitance to the pin it has on n, with a witness stating the value and
// citing the row. A crystal terminal on such a pin needs no capacitor on the board, since the
// oscillator carries its own (agni issue 1247).
//
// The row must be bound to that PIN. A part-wide row would credit every pin of an MCU whose
// integrated capacitors sit on one oscillator only, so a spec with no pin data credits nothing. The
// pin resolves through param.ResolvePin, which refuses an ambiguous or conflicting pin rather than
// guessing. Returns "" and nil when nothing qualifies, which is every design read without --params.
func IntegratedLoadCapCredit(m Model, n *ir.Net, skip string) (string, *Witness) {
	for _, c := range n.GetConnections() {
		ref := c.GetComponentRef()
		if ref == skip || !m.IsFitted(ref) {
			continue
		}
		spec := m.PartSpec(ref)
		if spec == nil || len(spec.GetPins()) == 0 {
			continue
		}
		pkg := ""
		if p := param.PackageForMPN(spec, m.ComponentMPN(ref)); p != nil {
			pkg = p.GetId()
		}
		pin, err := param.ResolvePin(spec, m.PinName(ref, c.GetPinRef()), c.GetPinRef(), pkg)
		if err != nil {
			continue
		}
		for _, row := range param.PinParameters(spec, pin.GetId()) {
			if !integratedLoadCapSymbols[alnumUpper(row.GetSymbol())] && !integratedLoadCapName.MatchString(row.GetName()) {
				continue
			}
			q, ok := param.InBaseUnit(row)
			if !ok || q.Unit != "F" {
				continue
			}
			v := q.GetValue()
			farads := v.GetTyp()
			if farads <= 0 {
				farads = v.GetMax()
			}
			if farads <= 0 {
				continue
			}
			pf := fmtQty(farads*1e12, "pF")
			w := &Witness{
				Statement: fmt.Sprintf("%s integrates %s of load capacitance on its %s pin, per its datasheet", ref, pf, pin.GetName()),
				Terms: []WitnessTerm{
					{Label: "oscillator part", Value: ref},
					{Label: "integrated load capacitance", Value: pf},
				},
			}
			if cit := DatasheetCitationOf(spec, row); cit != nil {
				w.Datasheet = []*DatasheetCitation{cit}
			}
			return ref, w
		}
	}
	return "", nil
}

// UnreadOscillator returns the first fitted IC on net n, other than skip, that the run has no
// datasheet for. Its pin may integrate the load capacitance a crystal terminal lacks, so a rule
// reporting that terminal says the absence is unverified rather than settled. "" when every IC on the
// net was read, or none is there.
func UnreadOscillator(m Model, n *ir.Net, skip string) string {
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
