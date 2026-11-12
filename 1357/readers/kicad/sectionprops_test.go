package kicad

import (
	"strings"
	"testing"
)

const housePropsSch = `(kicad_sch
	(version 20250114)
	(generator "eeschema")
	(uuid "33333333-3333-3333-3333-333333333333")
	(paper "A4")
	(lib_symbols
		(symbol "test:J"
			(property "Reference" "J")
			(symbol "J_1_1"
				(pin passive line (at 0 3.81 270) (length 1.27) (name "~") (number "1")))))
	(symbol (lib_id "test:J") (at 50 50 0) (unit 1) (uuid "j1")
		(property "Reference" "J1" (at 47 49 0))
		(property "Value" "M2_SOCKET" (at 47 51 0))
		(property "Description" "Key M Card Edge Connector" (at 47 53 0))
		(property "Part Number" "1-2199230-6" (at 47 55 0))
		(property "DNP" "DNP" (at 47 57 0))
		(property "Notes" "" (at 47 59 0))))
`

// A house property goes on the section, and the lexicon's attribute keys read it there. It is kept
// off the component, because classification tokenizes every attribute value on a component (agni
// issue 1285).
func TestAHousePropertyRidesTheSection(t *testing.T) {
	d, err := ReadSchematic(strings.NewReader(housePropsSch), "house.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.GetComponents()) != 1 {
		t.Fatalf("%d components, want 1", len(d.GetComponents()))
	}
	c := d.GetComponents()[0]
	sec := c.GetSections()[0].GetAttributes()
	for k, want := range map[string]string{"Part Number": "1-2199230-6", "DNP": "DNP", "Description": "Key M Card Edge Connector", "Value": "M2_SOCKET"} {
		if got := sec[k]; got != want {
			t.Errorf("section %s = %q, want %q", k, got, want)
		}
	}
	for _, k := range []string{"Part Number", "DNP", "Description"} {
		if v, ok := c.GetAttributes()[k]; ok {
			t.Errorf("component carries %s = %q, which classification would read", k, v)
		}
	}
	if _, ok := sec["Notes"]; ok {
		t.Error("an empty property was kept")
	}
	if _, ok := sec["Reference"]; ok {
		t.Error("the reference was kept as an attribute")
	}
}
