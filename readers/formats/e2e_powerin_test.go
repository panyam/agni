package formats

import (
	"os"
	"path/filepath"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// senseMonitor is a current monitor drawn the way KiCad's library draws the INA233: the shunt and bus
// sense pins are inputs, the supply is power_in, and one supply-named pin is left unspecified.
const senseMonitor = `(kicad_sch
	(lib_symbols
		(symbol "test:MON"
			(property "Reference" "U")
			(symbol "MON_1_1"
				(pin input line (name "Vbus") (number "1"))
				(pin input line (name "Vin+") (number "2"))
				(pin power_in line (name "VS") (number "3"))
				(pin unspecified line (name "VDD") (number "4")))))
	(symbol (lib_id "test:MON") (unit 1) (uuid "u1")
		(property "Reference" "U1") (property "Value" "MON")))`

// TestKiCadInputPinKeepsItsType reads a KiCad schematic through the real loader and requires a sense
// pin named like a supply to stay an input. Before agni issue 1241 the ingestion pass promoted Vbus and
// Vin+ to power_in by name, and every current monitor's sense nets read as undriven rails.
func TestKiCadInputPinKeepsItsType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mon.kicad_sch")
	if err := os.WriteFile(path, []byte(senseMonitor), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := (&Loader{}).ReadDesign(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]ir.PinDirection{
		"Vbus": ir.PinDirection_PIN_DIRECTION_INPUT,
		"Vin+": ir.PinDirection_PIN_DIRECTION_INPUT,
		"VS":   ir.PinDirection_PIN_DIRECTION_POWER_IN,
		"VDD":  ir.PinDirection_PIN_DIRECTION_POWER_IN, // unspecified is still filled
	}
	got := map[string]ir.PinDirection{}
	for _, lib := range d.GetLibraries() {
		for _, pt := range lib.GetParts() {
			for _, p := range pt.GetPins() {
				got[p.GetName()] = p.GetDirection()
			}
		}
	}
	if len(got) != len(want) {
		t.Fatalf("read pins %v, want the four of %v", got, want)
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("pin %s = %v, want %v", name, got[name], w)
		}
	}
}
