package foreign

import (
	"strings"
	"testing"
)

// TestItemDescriptionRoundTrip pins that the writer and the parser agree on the item separator. It
// also pins that a message stored before the separator changed still parses, because an imported
// results file keeps the message it was written with (#1875).
func TestItemDescriptionRoundTrip(t *testing.T) {
	const violation = "Clearance violation (netclass 'Default' clearance 0.2000 mm; actual 0.1500 mm)"
	const item = "Track [VCC] on F.Cu, length 10.8000 mm"

	written := findings(kicadViolation{
		Type:        "clearance",
		Description: violation,
		Items: []struct {
			Description string `json:"description"`
			UUID        string `json:"uuid"`
		}{{Description: item, UUID: "u1"}},
	}, RulePrefixDRC, "drc.json")
	if len(written) != 1 {
		t.Fatalf("findings wrote %d findings for one item, want 1", len(written))
	}
	msg := written[0].GetMessage()
	if strings.Contains(msg, "—") {
		t.Errorf("message %q still carries an em-dash", msg)
	}
	if !strings.HasPrefix(msg, violation) {
		t.Errorf("message %q does not start with the violation description", msg)
	}

	cases := []struct{ name, msg, want string }{
		{"written now", msg, item},
		{"new form", violation + " (item: " + item + ")", item},
		{"old form", violation + " — " + item, item},
		{"item with parentheses", "Pin conflict (item: Pin 2 (Passive) of R1)", "Pin 2 (Passive) of R1"},
		{"no item", violation, ""},
		{"no item, ends in a parenthesis", "Silkscreen clipped (board edge)", ""},
	}
	for _, c := range cases {
		if got := itemDescription(c.msg); got != c.want {
			t.Errorf("%s: itemDescription(%q) = %q, want %q", c.name, c.msg, got, c.want)
		}
	}
}
