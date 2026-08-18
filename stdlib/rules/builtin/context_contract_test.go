package builtin

import (
	"strings"
	"testing"
	"unicode"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// The contract every Finding.Context entry has to satisfy, asserted over whatever the catalog
// actually fires rather than rule by rule (agni issue 349, sweep agni issue 361).
//
// Written as a property over the whole catalog on purpose. The per-rule tests below assert that a
// given rule DOES carry context; this asserts that no rule carries it WRONGLY, and it keeps holding
// for rules converted later without anybody remembering to add a check. The failure modes it catches
// are the four the authoring convention warns about, and all four produce a chip that looks fine and
// misbehaves when clicked.
func assertContextContract(t *testing.T, fs []check.Finding) {
	t.Helper()
	for _, f := range fs {
		for i, c := range f.Context {
			where := f.Rule + " context[" + itoa(i) + "]"
			if c.Subject == "" {
				t.Errorf("%s: empty subject; a chip with no entity navigates nowhere", where)
			}
			if c.Role == "" {
				t.Errorf("%s: empty role; the chip renders with no label", where)
			}
			if strings.ToLower(c.Role) != c.Role {
				t.Errorf("%s: role %q is not lower-case", where, c.Role)
			}
			for _, r := range c.Role {
				if unicode.IsSpace(r) {
					t.Errorf("%s: role %q contains a space; a role is a short noun, not a description", where, c.Role)
				}
			}
			if c.Kind == "" {
				t.Errorf("%s: empty kind; the client cannot decide how to highlight it", where)
			}
			// The two that actually bite. A context repeating the subject gives the reader a chip
			// that navigates to where they already are; a context the message does not name is a chip
			// with no explanation, and usually means the wrong variable was declared.
			if c.Subject == f.Subject && c.Pin == "" {
				t.Errorf("%s: repeats the finding's own subject %q", where, f.Subject)
			}
			// What the message has to NAME depends on the kind. A pin context on a component-subject
			// finding shares the subject's ref des, and the message names the pin DESIGNATOR ("pin 1"),
			// not the ref des, because the ref des is already the subject. Checking the ref there would
			// fail on correct rules, which is what the first version of this helper did.
			named := c.Subject
			if c.Kind == check.KindPin && c.Pin != "" {
				named = c.Pin
			}
			if !strings.Contains(f.Message, named) {
				t.Errorf("%s: entity %q is not named in the message %q", where, named, f.Message)
			}
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// ctxOf indexes a finding's context by role. Only for tests whose roles ARE unique; the collision
// cases assert over the list directly, because sharing a role is the point there.
func ctxOf(f check.Finding) map[string]check.ContextSubject {
	out := map[string]check.ContextSubject{}
	for _, c := range f.Context {
		out[c.Role] = c
	}
	return out
}

// Every rule converted in the agni issue 361 sweep, asserted through the fixture its own test already
// builds. Each case says which entity the message names that the subject is not, because that is the
// question the conversion answers and the thing a later refactor could silently drop.
func TestSweptRulesCarryTheEntitiesTheyName(t *testing.T) {
	t.Run("fet-vdss-below-rail names the rail", func(t *testing.T) {
		m := check.NewModelWithParams(fetDesign("+60V", ""), nil, param.ParamSet{"ACME-FET": fetSpec("ACME-FET", 50)})
		fs := fetVdssBelowRail.Eval(m)
		if len(fs) != 1 {
			t.Fatalf("want 1 finding, got %d", len(fs))
		}
		assertContextContract(t, fs)
		c := ctxOf(fs[0])["rail"]
		if c.Subject != "+60V" || c.Kind != check.KindNet {
			t.Errorf("rail context = %+v, want the +60V net", c)
		}
	})

	t.Run("supply-exceeds-abs-max names the pin and the rail", func(t *testing.T) {
		fs := runSupplyRule(t, supplyDesign("+5V", false, "ACME-33"), param.ParamSet{"ACME-33": ldoSpec("ACME-33", 4.6)})
		if len(fs) != 1 {
			t.Fatalf("want 1 finding, got %d", len(fs))
		}
		assertContextContract(t, fs)
		by := ctxOf(fs[0])
		// The PIN is the interesting one: the subject is the whole part, and a part can have several
		// supply pins, so the ref des alone cannot say which terminal is over its limit.
		if p := by["pin"]; p.Kind != check.KindPin || p.Subject != fs[0].Subject || p.Pin == "" {
			t.Errorf("pin context = %+v, want a pin of the subject part", p)
		}
		if r := by["rail"]; r.Kind != check.KindNet || r.Subject != "+5V" {
			t.Errorf("rail context = %+v, want the +5V net", r)
		}
	})

	t.Run("rail-nominal names the pin and the rail", func(t *testing.T) {
		fs := runRailNominal(t, supplyDesign("+5V", false, "ACME-33"), param.ParamSet{"ACME-33": ldoRecommendedSpec("ACME-33", 3.0, 3.6)})
		if len(fs) != 1 {
			t.Fatalf("want 1 finding, got %d", len(fs))
		}
		assertContextContract(t, fs)
		by := ctxOf(fs[0])
		if by["pin"].Kind != check.KindPin || by["rail"].Subject != "+5V" {
			t.Errorf("context = %+v, want a pin and the +5V rail", fs[0].Context)
		}
	})

	t.Run("resonator-redundant-load-caps names the terminal and the cap", func(t *testing.T) {
		fs := resonatorRedundantLoadCaps.Eval(check.NewModel(resonatorFixture()))
		if len(fs) == 0 {
			t.Fatal("want at least 1 finding")
		}
		assertContextContract(t, fs)
		by := ctxOf(fs[0])
		if by["terminal"].Kind != check.KindNet {
			t.Errorf("terminal context = %+v, want a net", by["terminal"])
		}
		// The cap is what a reader DELETES, so it is the actionable half and must be reachable.
		if by["capacitor"].Kind != check.KindComponent {
			t.Errorf("capacitor context = %+v, want a component", by["capacitor"])
		}
	})

	t.Run("rail-not-classified names the supply pin", func(t *testing.T) {
		fs := railNotClassified.Eval(check.NewModel(houseNamedDesign()))
		if len(fs) == 0 {
			t.Fatal("want at least 1 finding")
		}
		assertContextContract(t, fs)
		c := ctxOf(fs[0])["supply-pin"]
		if c.Kind != check.KindPin || c.Subject == "" || c.Pin == "" {
			t.Errorf("supply-pin context = %+v, want a ref/pin pair", c)
		}
	})

	t.Run("copper-clearance names the other net in the pair", func(t *testing.T) {
		// A clearance violation is SYMMETRIC and gets filed under one of the two nets, so before this
		// the other end had no way back into the drawing.
		m := check.NewModelWithBoard(&ir.Design{}, drcBoard())
		fs := copperClearance.Eval(m)
		if len(fs) != 1 {
			t.Fatalf("want 1 finding, got %d", len(fs))
		}
		assertContextContract(t, fs)
		c := ctxOf(fs[0])["neighbour"]
		if c.Kind != check.KindNet || c.Subject == "" || c.Subject == fs[0].Subject {
			t.Errorf("neighbour context = %+v, want the OTHER net of the pair", c)
		}
	})

	t.Run("reverse-blocking names the transistor it cannot classify", func(t *testing.T) {
		// The inconclusive branch: the whole remedy is "seed THAT part's datasheet", so naming it only
		// in prose left the next step a manual search.
		fs := revFindings(revDesign("transistor", false))
		if len(fs) != 1 || !fs[0].Inconclusive {
			t.Fatalf("want the one inconclusive finding of the unclassifiable path, got %+v", fs)
		}
		assertContextContract(t, fs)
		if c := ctxOf(fs[0])["transistor"]; c.Kind != check.KindComponent || c.Subject == "" {
			t.Errorf("transistor context = %+v, want the part to seed", c)
		}
	})

	t.Run("load-switch-trip names the controller and the sense resistor", func(t *testing.T) {
		fs := runLoadSwitchRule(loadSwitchBoard(0.01), param.ParamSet{
			"DEMO-HSS-CTRL": hssCtrlSpec(0.05),
			"DEMO-NFET-40":  passFetSpec(0.02, 3),
		})
		if len(fs) != 1 {
			t.Fatalf("want 1 finding, got %d", len(fs))
		}
		assertContextContract(t, fs)
		by := ctxOf(fs[0])
		if by["controller"].Kind != check.KindComponent || by["sense"].Kind != check.KindComponent {
			t.Errorf("context = %+v, want a controller and a sense resistor", fs[0].Context)
		}
		// The subject is the FET because it is what overheats, but the fix is usually one of these.
		for _, c := range fs[0].Context {
			if c.Subject == fs[0].Subject {
				t.Errorf("context repeats the subject %q", fs[0].Subject)
			}
		}
	})
}
