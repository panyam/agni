package classify

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestStampPowerInPins: an under-typed supply-named input pin is promoted to POWER_IN; a confident
// direction, a supply OUTPUT name (VOUT), a polarized-cap "+" terminal, a net-form name ("3V3"),
// ground, and a plain signal are all left untouched.
func TestStampPowerInPins(t *testing.T) {
	const (
		IN     = ir.PinDirection_PIN_DIRECTION_INPUT
		INOUT  = ir.PinDirection_PIN_DIRECTION_INOUT
		UNSPEC = ir.PinDirection_PIN_DIRECTION_UNSPECIFIED
		OUT    = ir.PinDirection_PIN_DIRECTION_OUTPUT
		POWER  = ir.PinDirection_PIN_DIRECTION_POWER_IN
	)
	cases := []struct {
		name string
		in   ir.PinDirection
		want ir.PinDirection
	}{
		{"VDD", IN, POWER},      // canonical supply input
		{"VCC_3V3", IN, POWER},  // VCC prefix with a suffix
		{"VDDA", IN, POWER},     // VDD prefix (analog supply)
		{"VIN", INOUT, POWER},   // inout is under-typed too
		{"VBAT", UNSPEC, POWER}, // unspecified is under-typed too
		{"VOUT", IN, IN},        // supply OUTPUT name: NOT in the input vocab
		{"+", IN, IN},           // polarized-cap terminal, excluded
		{"3V3", IN, IN},         // net-form name, excluded
		{"GND", IN, IN},         // ground, not a power supply
		{"IO1", IN, IN},         // plain signal
		{"VDD", OUT, OUT},       // confident OUTPUT is never overwritten
		{"VDD", POWER, POWER},   // already typed (KiCad-style): no-op
	}
	for _, c := range cases {
		d := &ir.Design{Libraries: []*ir.PartLibrary{{Parts: []*ir.PartType{{
			Name: "P", Pins: []*ir.Pin{{Name: c.name, Designator: "1", Direction: c.in}},
		}}}}}
		StampPowerInPins(d)
		if got := d.Libraries[0].Parts[0].Pins[0].Direction; got != c.want {
			t.Errorf("pin %q dir %v -> %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

// TestStampPowerInPinsHonorsSupplyPinVocab: the supply-pin vocabulary is config-overridable via the
// same lexicon as the net roles (WS3-069), so a project whose parts name supply pins with a house
// prefix ("PWR_") extends it and the stamp promotes those pins — no engine change, no frozen literal.
func TestStampPowerInPinsHonorsSupplyPinVocab(t *testing.T) {
	defer SetActiveRoleVocab(nil)
	v, err := BuildRoleVocab(VocabPatterns{}, VocabPatterns{}, VocabPatterns{}, VocabPatterns{Patterns: []string{`^PWR_`}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	SetActiveRoleVocab(v)
	d := &ir.Design{Libraries: []*ir.PartLibrary{{Parts: []*ir.PartType{{
		Name: "P", Pins: []*ir.Pin{{Name: "PWR_3V3", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_INPUT}},
	}}}}}
	StampPowerInPins(d)
	if got := d.Libraries[0].Parts[0].Pins[0].Direction; got != ir.PinDirection_PIN_DIRECTION_POWER_IN {
		t.Errorf("PWR_3V3 under the extended supply-pin vocab: got %v, want POWER_IN", got)
	}
}

// TestStampPowerInPinsIdempotent: a re-stamp (a re-read) does not change an already-promoted pin.
func TestStampPowerInPinsIdempotent(t *testing.T) {
	d := &ir.Design{Libraries: []*ir.PartLibrary{{Parts: []*ir.PartType{{
		Name: "P", Pins: []*ir.Pin{{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_INPUT}},
	}}}}}
	StampPowerInPins(d)
	StampPowerInPins(d)
	if got := d.Libraries[0].Parts[0].Pins[0].Direction; got != ir.PinDirection_PIN_DIRECTION_POWER_IN {
		t.Errorf("re-stamp dir = %v, want POWER_IN", got)
	}
}
