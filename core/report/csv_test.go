package report

import "testing"

// TestSanitizeCellEscapesFormulaCells covers the case that makes escaping non-optional: a spreadsheet
// executes a cell beginning with =, +, - or @ when the file is opened. Net names really do start
// with + (a rail named +3V3 on the shipped demo board), and a rule message is free prose.
func TestSanitizeCellEscapesFormulaCells(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{"equals", "=1+1", "'=1+1"},
		{"plus rail", "+3V3", "'+3V3"},
		{"minus", "-VBUS", "'-VBUS"},
		{"at", "@RESET", "'@RESET"},
		{"leading space then equals", " =cmd", "' =cmd"},
		{"leading tab then equals", "\t=cmd", "'\t=cmd"},
		{"ordinary net", "SCL", "SCL"},
		{"empty", "", ""},
		{"interior equals is harmless", "R1=10k", "R1=10k"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeCell(tc.in); got != tc.want {
				t.Errorf("SanitizeCell(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
