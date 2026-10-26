package intent

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/ident"
	"github.com/panyam/agni/core/param"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// ioMapFunctionRule: the function a map row selects is not one the part's datasheet lists for that
// pin (agni issue 667). It is a fifth rule rather than a clause of io-map-pin-mismatch because the
// fix differs: a wrong net is a schematic edit, and an illegal function is a pin-mux plan that
// cannot be built on this pin at all.
func ioMapFunctionRule(d Declaration) *check.Rule {
	return &check.Rule{
		Name:     RuleIOMapFunction,
		Severity: "error",
		Summary:  "a function the design intent's IO map selects is not one the part's datasheet lists for that pin",
		Detail:   intentDoc(RuleIOMapFunction),
		Impact: "the pin-mux configuration the map describes cannot be written. The part has no mux " +
			"setting that puts that peripheral on that pin, so the signal never reaches it, and the board " +
			"needs a rework or the firmware a different pin.",
		Remedy:              intentRemedy(RuleIOMapFunction),
		Reads:               []string{"component.pin", "pin.name", "component.mpn", "param.pin"},
		Tags:                intentTags(),
		Eval:                func(ctx context.Context, m check.Model) []check.Verdict { return ioMapFunctionVerdicts(m, d) },
		StatesConsideredSet: true,
	}
}

// ioMapFunctionVerdicts decides every map row. A row declaring no function, and a row whose part
// has no alternate-function table to read, are not-considered with the step that stopped them, so
// the denominator says how many declared functions were actually compared.
func ioMapFunctionVerdicts(m check.Model, d Declaration) []check.Verdict {
	out := make([]check.Verdict, 0, len(d.IOMap))
	for _, a := range d.IOMap {
		res := resolvePin(m, a.Device, a.Pin)
		v := check.Verdict{Subjects: []check.Entity{res.subject(a.Device)}, Context: rowContext(a, res)}
		if a.Function == "" {
			v.Outcome, v.Reason = check.NotConsidered, "this row "+phraseNoFunction
			out = append(out, v)
			continue
		}
		if res.outcome != check.Pass {
			res.decide(&v)
			out = append(out, v)
			continue
		}
		spec := m.PartSpec(a.Device)
		specPin, why := specPinFor(m, spec, a.Device, res.designator)
		if specPin == nil {
			v.Outcome, v.Reason = check.NotConsidered, why
			out = append(out, v)
			continue
		}
		if len(specPin.GetAlternates()) == 0 {
			v.Outcome, v.Reason = check.NotConsidered, fmt.Sprintf(
				"the datasheet entry for %s "+phraseNoAlternates+" for pin %s, so no pin-mux table was read for it",
				a.Device, specPin.GetName())
			out = append(out, v)
			continue
		}
		var listed []string
		best := ident.Result{Match: ident.None}
		for _, alt := range specPin.GetAlternates() {
			listed = append(listed, alt.GetName())
			if r := ident.Compare(a.Function, alt.GetName()); check.MatchRank(r.Match) > check.MatchRank(best.Match) {
				best = r
			}
		}
		cite := []*check.DatasheetCitation{check.DatasheetCitationOfProv(spec, specPin.GetProv())}
		terms := []check.WitnessTerm{
			{Label: "declared function", Value: a.Function},
			{Label: "datasheet pin", Value: specPin.GetName()},
			{Label: "functions listed", Value: strings.Join(listed, ", ")},
		}
		if best.Match != ident.None {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("the datasheet lists %q on %s pin %s%s",
					best.B, a.Device, res.label(), functionNote(best)),
				Terms: terms, Datasheet: cite,
			}
			out = append(out, v)
			continue
		}
		v.Outcome = check.Fail
		v.Witness = &check.Witness{
			Statement: fmt.Sprintf("the datasheet does not list %q on %s pin %s, which can be %s",
				a.Function, a.Device, res.label(), strings.Join(listed, ", ")),
			Terms: terms, Datasheet: cite,
		}
		v.Finding = &check.Finding{
			Subject: check.PinEntity(a.Device, res.designator),
			Message: fmt.Sprintf("the IO map selects %q on %s pin %s, and the part's datasheet lists no such function there; it lists %s",
				a.Function, a.Device, res.label(), strings.Join(listed, ", ")),
			Prov:          check.ComponentProv(m, a.Device),
			DatasheetProv: cite,
		}
		out = append(out, v)
	}
	return out
}

// specPinFor resolves a design pin onto its datasheet pin through param.ResolvePin, the one
// implementation of the name-then-number precedence. A nil pin comes with the reason the
// comparison could not be made.
func specPinFor(m check.Model, spec *parampb.PartSpec, refDes, designator string) (*parampb.Pin, string) {
	if spec == nil {
		mpn := m.ComponentMPN(refDes)
		if mpn == "" {
			return nil, fmt.Sprintf("%s "+phraseNoPartNumber+", so no datasheet entry can be joined to it", refDes)
		}
		return nil, fmt.Sprintf("the datasheet corpus "+phraseNoEntry+" %s's part %s", refDes, mpn)
	}
	pkg := ""
	if p := param.PackageForMPN(spec, m.ComponentMPN(refDes)); p != nil {
		pkg = p.GetId()
	}
	pin, err := param.ResolvePin(spec, m.PinName(refDes, designator), designator, pkg)
	switch {
	case err == nil:
		return pin, ""
	case errors.Is(err, param.ErrNoPinData):
		return nil, fmt.Sprintf("the datasheet entry for %s's part %s "+phraseNoPins, refDes, spec.GetMpn())
	default:
		return nil, fmt.Sprintf("%s pin %s "+phraseSpecPinUnresolved+" of the datasheet entry for %s: %v",
			refDes, designator, spec.GetMpn(), err)
	}
}

// functionNote says what was assumed to match the declared function to the datasheet's, empty when
// they agreed outright. A loose match passes, and the note is how a reader sees it was inferred.
func functionNote(r ident.Result) string {
	if r.Match == ident.Exact && r.Note == "" {
		return ""
	}
	if r.Note == "" {
		return " (" + string(r.Match) + " match)"
	}
	return " (" + string(r.Match) + " match: " + r.Note + ")"
}

// The phrases io-map-function's not-considered reasons are built from, one per step that can stop
// it. FunctionReasonOf reads a reason back to its kind through these same constants, so a reworded
// reason keeps its kind, and a report sent from a board we never see can count why without carrying
// the sentence, which names parts (agni issue 1196).
const (
	phraseNoFunction        = "declares no function"
	phraseNoComponent       = "carries no component"
	phraseNoPinList         = "supplied no pin list"
	phraseNoPartNumber      = "has no part number"
	phraseNoEntry           = "has no entry for"
	phraseNoPins            = "declares no pins"
	phraseSpecPinUnresolved = "does not resolve to one pin"
	phraseNoAlternates      = "lists no alternate functions"
)

// FunctionReason is the step that stopped io-map-function on a row it reports not-considered.
type FunctionReason string

// The steps, in the order the rule takes them.
const (
	FunctionReasonNotDeclared     FunctionReason = "not-declared"
	FunctionReasonPinUnresolved   FunctionReason = "pin-unresolved"
	FunctionReasonNoPartNumber    FunctionReason = "no-part-number"
	FunctionReasonNoDatasheet     FunctionReason = "no-datasheet"
	FunctionReasonNoDatasheetPins FunctionReason = "no-datasheet-pins"
	FunctionReasonSpecPinUnknown  FunctionReason = "spec-pin-unresolved"
	FunctionReasonNoPinMux        FunctionReason = "no-pin-mux"
	FunctionReasonOther           FunctionReason = "other"
)

// FunctionReasonOf names the step a not-considered io-map-function reason came from, and
// FunctionReasonOther for text this rule did not write.
func FunctionReasonOf(reason string) FunctionReason {
	for _, c := range []struct {
		phrase string
		kind   FunctionReason
	}{
		{phraseNoFunction, FunctionReasonNotDeclared},
		{phraseNoComponent, FunctionReasonPinUnresolved},
		{phraseNoPinList, FunctionReasonPinUnresolved},
		{phraseNoPartNumber, FunctionReasonNoPartNumber},
		{phraseNoEntry, FunctionReasonNoDatasheet},
		{phraseNoPins, FunctionReasonNoDatasheetPins},
		{phraseSpecPinUnresolved, FunctionReasonSpecPinUnknown},
		{phraseNoAlternates, FunctionReasonNoPinMux},
	} {
		if strings.Contains(reason, c.phrase) {
			return c.kind
		}
	}
	return FunctionReasonOther
}

// inferredMatch is the note an IO-map witness carries when a pin name or a function matched only
// after normalizing or dropping tokens (pinMatch.note, functionNote).
var inferredMatch = regexp.MustCompile(`\((normalized|fuzzy) (pin-name )?match`)

// InferredMatch reports whether an IO-map witness statement says a match was inferred, so a report
// can count those without keeping the statement.
func InferredMatch(statement string) bool { return inferredMatch.MatchString(statement) }
