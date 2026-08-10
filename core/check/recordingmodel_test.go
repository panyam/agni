package check

import (
	"testing"

	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestTierOf(t *testing.T) {
	for fact, want := range map[string]FactTier{
		"param.cap_rated_voltage": TierParam,
		"param(mpn, max_voltage)": TierParam,
		"component.device_class":  TierParam, // relation name, no "param" prefix
		"component.esd_rated":     TierParam,
		"board.track_width":       TierBoard,
		"pin.electrical_type":     TierConnectivity,
		"pin.role":                TierConnectivity,
		"on_net":                  TierConnectivity,
		"net.names":               "",
		"component.class":         "",
		"wire.junction":           "",
	} {
		if got := TierOf(fact); got != want {
			t.Errorf("TierOf(%q) = %q, want %q", fact, got, want)
		}
	}
}

// An OPTIONAL read still declares its tier. Available treats optional reads as non-gating, which is
// a different question from whether the author declared the rule touches the tier at all, and
// conflating the two made the audit report esd-protection for reading a param tier its Reads names.
func TestDeclaredTiersIncludesOptionalReads(t *testing.T) {
	r := &Rule{
		Reads:         []string{"net.names", "param.esd_rating"},
		OptionalReads: []string{"param.esd_rating"},
	}
	if !DeclaredTiers(r)[TierParam] {
		t.Error("an optional param read is still a declared param tier")
	}
}

// TestRecordingModelNotesEveryGatedAccessor is what keeps the audit from going blind.
//
// The audit reports green both when every rule declares honestly and when the recorder silently
// stopped noticing. Dropping one note() call from RecordingModel passes the whole audit suite,
// because a rule reading that accessor then looks like a rule that never touched it. So the
// recorder's own completeness is pinned here, accessor by accessor, rather than resting on the
// audit that depends on it.
func TestRecordingModelNotesEveryGatedAccessor(t *testing.T) {
	for name, tc := range map[string]struct {
		call func(m Model)
		want FactTier
	}{
		"PartSpec":        {func(m Model) { m.PartSpec("U1") }, TierParam},
		"BoardNets":       {func(m Model) { m.BoardNets() }, TierBoard},
		"Pins":            {func(m Model) { m.Pins() }, TierConnectivity},
		"PinDir":          {func(m Model) { m.PinDir("U1", "1") }, TierConnectivity},
		"PinDeclared":     {func(m Model) { m.PinDeclared("U1", "1") }, TierConnectivity},
		"PinConnected":    {func(m Model) { m.PinConnected("U1", "1") }, TierConnectivity},
		"PinRole":         {func(m Model) { m.PinRole("U1", "1") }, TierConnectivity},
		"PinNetName":      {func(m Model) { m.PinNetName("U1", "1") }, TierConnectivity},
		"PinNetConflicts": {func(m Model) { m.PinNetConflicts() }, TierConnectivity},
		"IsConnected":     {func(m Model) { m.IsConnected("U1") }, TierConnectivity},
	} {
		rec := NewRecordingModel(NewModelWithParams(&ir.Design{}, nil, param.ParamSet{}))
		tc.call(rec)
		got := rec.Read()
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("%s recorded %v, want exactly [%s]", name, got, tc.want)
		}
	}
}

// An accessor outside the gated tiers must record NOTHING, or every rule would look like it reads
// every tier and the audit would assert nothing while still passing.
func TestRecordingModelIgnoresUngatedAccessors(t *testing.T) {
	rec := NewRecordingModel(NewModelWithParams(&ir.Design{}, nil, param.ParamSet{}))
	rec.Nets()
	rec.Components()
	rec.HasNetName("VCC")
	rec.ComponentMPN("U1")
	if got := rec.Read(); len(got) != 0 {
		t.Errorf("ungated accessors recorded %v, want none", got)
	}
}

func TestRecordingModelReset(t *testing.T) {
	rec := NewRecordingModel(NewModelWithParams(&ir.Design{}, nil, param.ParamSet{}))
	rec.Pins()
	if len(rec.Read()) != 1 {
		t.Fatalf("setup: want one tier, got %v", rec.Read())
	}
	rec.Reset()
	if got := rec.Read(); len(got) != 0 {
		t.Errorf("after Reset, got %v, want none; a stale observation is attributed to the next rule", got)
	}
}
