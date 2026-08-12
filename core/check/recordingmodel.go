package check

import (
	"sort"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// RecordingModel wraps a Model and records which GATED tiers a rule actually reaches for while it
// evaluates, so a rule's declared Reads can be checked against its behaviour instead of trusted
// (WS3-122).
//
// It exists because only 15 of the shipped rules have a Spec twin holding their declaration to
// their body. For the rest the declaration is prose, and three gates trust it: a rule that reads
// datasheet params without declaring them RUNS where it should have been gated to not-applicable,
// and reports over a tier that was never attached. Nothing about that fails loudly, which is the
// same silence-reads-as-pass shape this catalog treats as the expensive failure.
//
// It records TIERS, not fact names, because that is the granularity the gates key on. See FactTier.
//
// # What an observation proves, and what it does not
//
// A recorded read is EVIDENCE: the rule called that accessor, so it depends on that tier. An
// ABSENT read is not evidence of anything, because a rule that early-returns on the fixture it was
// given never reaches its accessors. So a caller may soundly assert "everything read is declared"
// and may NOT assert "everything declared is read". The second reads as an over-declaration report
// to a human, never as a gate.
//
// It embeds Model, so accessors it does not override delegate unchanged and a Model method added
// later keeps compiling. Adding a method that reaches a gated tier means overriding it here too;
// that is the maintenance cost, and it is why the override set is annotated with its tier.
//
// Not safe for concurrent use. Rules run sequentially within one Run.
type RecordingModel struct {
	Model
	read map[FactTier]bool
}

// NewRecordingModel wraps m. A nil inner Model is allowed and behaves as the zero Model would:
// callers get whatever the embedded nil interface does, which is a panic on use. That is
// deliberate, since a recorder over no design records nothing and would silently report every rule
// as reading no tier.
func NewRecordingModel(m Model) *RecordingModel {
	return &RecordingModel{Model: m, read: map[FactTier]bool{}}
}

// Read returns the tiers observed since the last Reset, sorted for stable reporting.
func (r *RecordingModel) Read() []FactTier {
	out := make([]FactTier, 0, len(r.read))
	for t := range r.read {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Reset clears the observation set, so one wrapped Model can be reused across rules without a
// previous rule's reads being attributed to the next.
func (r *RecordingModel) Reset() { clear(r.read) }

func (r *RecordingModel) note(t FactTier) { r.read[t] = true }

// param tier: the seeded datasheet join. ComponentMPN is deliberately NOT here — it is the
// design-side part identity (a BomLine or an attribute), declared as component.mpn, and present
// with or without a seeded set.
func (r *RecordingModel) PartSpec(refDes string) *parampb.PartSpec {
	r.note(TierParam)
	return r.Model.PartSpec(refDes)
}

// board tier.
func (r *RecordingModel) BoardNets() []BoardNet {
	r.note(TierBoard)
	return r.Model.BoardNets()
}

// connectivity tier: everything sourced from a component's pins, which a placement that lost its
// symbol does not have.
func (r *RecordingModel) Pins() []PinInst {
	r.note(TierConnectivity)
	return r.Model.Pins()
}

func (r *RecordingModel) PinDir(refDes, pin string) ir.PinDirection {
	r.note(TierConnectivity)
	return r.Model.PinDir(refDes, pin)
}

func (r *RecordingModel) PinDeclared(refDes, pin string) bool {
	r.note(TierConnectivity)
	return r.Model.PinDeclared(refDes, pin)
}

func (r *RecordingModel) PinName(refDes, pin string) string {
	r.note(TierConnectivity)
	return r.Model.PinName(refDes, pin)
}

func (r *RecordingModel) PinConnected(refDes, pin string) bool {
	r.note(TierConnectivity)
	return r.Model.PinConnected(refDes, pin)
}

func (r *RecordingModel) PinRole(refDes, pin string) PinRole {
	r.note(TierConnectivity)
	return r.Model.PinRole(refDes, pin)
}

func (r *RecordingModel) PinNetName(refDes, pin string) string {
	r.note(TierConnectivity)
	return r.Model.PinNetName(refDes, pin)
}

func (r *RecordingModel) PinNetConflicts() []PinNetConflict {
	r.note(TierConnectivity)
	return r.Model.PinNetConflicts()
}

func (r *RecordingModel) IsConnected(refDes string) bool {
	r.note(TierConnectivity)
	return r.Model.IsConnected(refDes)
}
