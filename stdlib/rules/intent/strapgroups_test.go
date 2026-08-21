package intent

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// strapBitsDesign wires each named strap net to a rail (bias high), to ground (bias low), or to
// nothing (unbiased: the pin sitting at the part's internal default, which the netlist cannot see).
func strapBitsDesign(bits map[string]string) *ir.Design {
	d := &ir.Design{Components: []*ir.Component{{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}}}}
	rails := map[string]*ir.Net{}
	i := 0
	for _, net := range sortedKeys(bits) {
		to := bits[net]
		n := &ir.Net{
			Name: net, Prov: &ir.Provenance{SourceFile: "t"},
			Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}},
		}
		d.Nets = append(d.Nets, n)
		if to == "" {
			continue
		}
		i++
		ref := "R" + string(rune('0'+i))
		d.Components = append(d.Components, &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}})
		n.Connections = append(n.Connections, &ir.Connection{ComponentRef: ref, PinRef: "1"})
		if rails[to] == nil {
			rails[to] = &ir.Net{Name: to, Prov: &ir.Provenance{SourceFile: "t"}}
			d.Nets = append(d.Nets, rails[to])
		}
		rails[to].Connections = append(rails[to].Connections, &ir.Connection{ComponentRef: ref, PinRef: "2"})
	}
	return d
}

func groupFindings(t *testing.T, d *ir.Design, g StrapGroup) []check.Finding {
	t.Helper()
	return strapGroupRule(g).Findings(check.NewModel(d))
}

// TestStrapGroupDecodesMSBFirst (WS3-120): the group's value is read MSB-first from the declared net
// order, which is the declaration's job because nothing in a netlist says which pin is the high bit.
func TestStrapGroupDecodesMSBFirst(t *testing.T) {
	// PHYAD2=1, PHYAD1=0, PHYAD0=1 -> 0b101 = 5.
	d := strapBitsDesign(map[string]string{"PHYAD2": "+3V3", "PHYAD1": "GND", "PHYAD0": "+3V3"})
	g := StrapGroup{Name: "PHYAD", Device: "U12", Nets: []string{"PHYAD2", "PHYAD1", "PHYAD0"}, Value: 5}
	if fs := groupFindings(t, d, g); len(fs) != 0 {
		t.Errorf("a group encoding its declared value must be silent: %+v", fs)
	}
	// The same bits declared as a different number must fire, and name what it saw.
	g.Value = 2
	fs := groupFindings(t, d, g)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding (encodes 5, declared 2), got %+v", fs)
	}
	for _, want := range []string{"encodes 5", "declares 2", "PHYAD2=1", "PHYAD1=0"} {
		if !strings.Contains(fs[0].Message, want) {
			t.Errorf("message missing %q, a reviewer needs the observed bits: %s", want, fs[0].Message)
		}
	}
	// Reversing the declared order reads the same copper as a different number: bit order matters and
	// comes from the declaration alone.
	rev := StrapGroup{Name: "PHYAD", Device: "U12", Nets: []string{"PHYAD0", "PHYAD1", "PHYAD2"}, Value: 5}
	if fs := groupFindings(t, d, rev); len(fs) != 0 {
		t.Errorf("0b101 reversed is still 5, so this must be silent: %+v", fs)
	}
}

// TestStrapGroupPartialIsInconclusive is the case the ticket said to decide before writing the rule.
// A bit with no resistor is NORMAL (fit one only for the non-default state), so its level is unknown
// to the engine. The group must report inconclusive, naming the unread pins, rather than decoding the
// missing bit as 0 and asserting an address.
func TestStrapGroupPartialIsInconclusive(t *testing.T) {
	d := strapBitsDesign(map[string]string{"PHYAD2": "", "PHYAD1": "GND", "PHYAD0": "+3V3"})
	g := StrapGroup{Name: "PHYAD", Device: "U12", Nets: []string{"PHYAD2", "PHYAD1", "PHYAD0"}, Value: 1}
	fs := groupFindings(t, d, g)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding, got %+v", fs)
	}
	if !fs[0].Inconclusive {
		t.Errorf("an unreadable group must be INCONCLUSIVE, not a defect: %+v", fs[0])
	}
	if !strings.Contains(fs[0].Message, "PHYAD2") {
		t.Errorf("message must name the pin it could not read: %s", fs[0].Message)
	}
	// Note the trap this guards: decoding PHYAD2 as 0 would give 0b001 = 1, which MATCHES the
	// declared value and would have reported a clean pass on evidence the engine does not have.
	if strings.Contains(fs[0].Message, "encodes") {
		t.Errorf("an unreadable group must not claim an encoded value: %s", fs[0].Message)
	}
}

// TestStrapGroupDefaultResolvesPartial: declaring the part's internal pull supplies the one fact the
// netlist lacks, so a normally-built board (resistors only on the non-default bits) decodes and is
// checked properly instead of reading inconclusive forever.
func TestStrapGroupDefaultResolvesPartial(t *testing.T) {
	d := strapBitsDesign(map[string]string{"PHYAD2": "", "PHYAD1": "", "PHYAD0": "+3V3"})
	g := StrapGroup{Name: "PHYAD", Device: "U12", Nets: []string{"PHYAD2", "PHYAD1", "PHYAD0"}, Value: 1, Default: "low"}
	if fs := groupFindings(t, d, g); len(fs) != 0 {
		t.Errorf("default low + only PHYAD0 pulled high is 0b001 = 1, the declared value: %+v", fs)
	}
	// The default participates in the number, so a wrong default reading is a real mismatch.
	g.Default = "high"
	fs := groupFindings(t, d, g)
	if len(fs) != 1 || fs[0].Inconclusive {
		t.Fatalf("default high gives 0b111 = 7, a mismatch against 1: %+v", fs)
	}
	if !strings.Contains(fs[0].Message, "declared default") {
		t.Errorf("message should say which bits came from the default rather than from copper: %s", fs[0].Message)
	}
}

// TestStrapGroupSilentWhenNetAbsent: a declared net the design does not have is the presence forms'
// business. Reporting it here would double-report and would guess at a group that is not there.
func TestStrapGroupSilentWhenNetAbsent(t *testing.T) {
	d := strapBitsDesign(map[string]string{"PHYAD1": "GND", "PHYAD0": "+3V3"})
	g := StrapGroup{Name: "PHYAD", Device: "U12", Nets: []string{"PHYAD2", "PHYAD1", "PHYAD0"}, Value: 1}
	if fs := groupFindings(t, d, g); len(fs) != 0 {
		t.Errorf("a group whose net is absent from the design must be silent here: %+v", fs)
	}
}

// TestStrapCollisionFires: the check with the real value. Both devices are individually correct and
// the clash is only visible across them.
func TestStrapCollisionFires(t *testing.T) {
	d := strapBitsDesign(map[string]string{
		"A2": "GND", "A1": "GND", "A0": "+3V3", // U12 -> 1
		"B2": "GND", "B1": "GND", "B0": "+3V3", // U13 -> 1, the same address
	})
	groups := []StrapGroup{
		{Name: "PHYAD U12", Device: "U12", Nets: []string{"A2", "A1", "A0"}, Value: 1, Bus: "MDIO"},
		{Name: "PHYAD U13", Device: "U13", Nets: []string{"B2", "B1", "B0"}, Value: 1, Bus: "MDIO"},
	}
	fs := strapCollisionRule(groups).Findings(check.NewModel(d))
	if len(fs) != 1 {
		t.Fatalf("want 1 collision finding, got %+v", fs)
	}
	for _, want := range []string{"U12", "U13", "address 1", "MDIO"} {
		if !strings.Contains(fs[0].Message, want) {
			t.Errorf("message missing %q: %s", want, fs[0].Message)
		}
	}
}

// TestStrapCollisionScopedByBus: two devices at the same address on DIFFERENT buses do not clash, and
// a group with no declared bus opts out entirely.
func TestStrapCollisionScopedByBus(t *testing.T) {
	d := strapBitsDesign(map[string]string{
		"A2": "GND", "A1": "GND", "A0": "+3V3",
		"B2": "GND", "B1": "GND", "B0": "+3V3",
	})
	diffBus := []StrapGroup{
		{Name: "g1", Device: "U12", Nets: []string{"A2", "A1", "A0"}, Value: 1, Bus: "MDIO"},
		{Name: "g2", Device: "U13", Nets: []string{"B2", "B1", "B0"}, Value: 1, Bus: "I2C"},
	}
	if fs := strapCollisionRule(diffBus).Findings(check.NewModel(d)); len(fs) != 0 {
		t.Errorf("same address on different buses is not a collision: %+v", fs)
	}
	noBus := []StrapGroup{
		{Name: "g1", Device: "U12", Nets: []string{"A2", "A1", "A0"}, Value: 1},
		{Name: "g2", Device: "U13", Nets: []string{"B2", "B1", "B0"}, Value: 1},
	}
	if fs := strapCollisionRule(noBus).Findings(check.NewModel(d)); len(fs) != 0 {
		t.Errorf("a group with no declared bus opts out of collision checking: %+v", fs)
	}
}

// TestStrapCollisionExcludesUndecidable is the guard the ticket flagged: an undecidable group must
// never be decoded with assumed bits, because a fabricated address can fabricate a COLLISION — a
// confident accusation that two innocent parts clash.
func TestStrapCollisionExcludesUndecidable(t *testing.T) {
	// U13's high bit carries no resistor and its group declares no default, so its value is unknown.
	// Assuming that bit is 0 would decode it to 1 and collide with U12.
	d := strapBitsDesign(map[string]string{
		"A2": "GND", "A1": "GND", "A0": "+3V3", // U12 -> 1
		"B2": "", "B1": "GND", "B0": "+3V3", // U13 -> unknown (would be 1 if B2 were assumed 0)
	})
	groups := []StrapGroup{
		{Name: "g1", Device: "U12", Nets: []string{"A2", "A1", "A0"}, Value: 1, Bus: "MDIO"},
		{Name: "g2", Device: "U13", Nets: []string{"B2", "B1", "B0"}, Value: 1, Bus: "MDIO"},
	}
	if fs := strapCollisionRule(groups).Findings(check.NewModel(d)); len(fs) != 0 {
		t.Errorf("an undecidable group must not produce a collision; assuming its bits would accuse two innocent parts: %+v", fs)
	}
	// The gap is still visible: the group's own rule reports it inconclusive.
	own := groupFindings(t, d, groups[1])
	if len(own) != 1 || !own[0].Inconclusive {
		t.Errorf("the undecidable group must be reported inconclusive by its own rule, not silently dropped: %+v", own)
	}
}

// TestCollidableGroups: the collision rule is compiled only where a collision is expressible. Below
// that it could never fail, and an item bound to it would read a pass it did not earn.
func TestCollidableGroups(t *testing.T) {
	for _, tc := range []struct {
		name string
		gs   []StrapGroup
		want bool
	}{
		{"two on one bus", []StrapGroup{{Bus: "MDIO"}, {Bus: "MDIO"}}, true},
		{"two on different buses", []StrapGroup{{Bus: "MDIO"}, {Bus: "I2C"}}, false},
		{"one group", []StrapGroup{{Bus: "MDIO"}}, false},
		{"no buses declared", []StrapGroup{{}, {}}, false},
	} {
		if got := collidableGroups(tc.gs); got != tc.want {
			t.Errorf("%s: collidableGroups = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The intent half of the agni issue 361 sweep: these rules are filed under a NET (the group's first
// strap) while their sentences are about a DEVICE, so before this the part the finding is about had
// no way back into the drawing.
func TestStrapGroupFindingsNameTheirDevice(t *testing.T) {
	t.Run("a mis-encoded group names the device it straps", func(t *testing.T) {
		d := strapBitsDesign(map[string]string{"PHYAD2": "+3V3", "PHYAD1": "GND", "PHYAD0": "+3V3"})
		g := StrapGroup{Name: "PHYAD", Device: "U12", Nets: []string{"PHYAD2", "PHYAD1", "PHYAD0"}, Value: 2}
		fs := groupFindings(t, d, g)
		if len(fs) != 1 {
			t.Fatalf("want 1 finding, got %+v", fs)
		}
		if len(fs[0].Context) != 1 {
			t.Fatalf("want 1 context entity (the device), got %+v", fs[0].Context)
		}
		c := fs[0].Context[0]
		if c.Kind != check.KindComponent || c.Ref != "U12" || c.Role != "device" {
			t.Errorf("context = %+v, want U12 as the component playing device", c)
		}
		// The group NAME is a declaration from the intent file, not something on the design, so it
		// must NOT become a chip: it would highlight nothing.
		for _, x := range fs[0].Context {
			if x.Ref == "PHYAD" {
				t.Error("the group name is a declaration, not a design entity; it must not be context")
			}
		}
	})

	t.Run("an unreadable group names the device and the nets it could not read", func(t *testing.T) {
		d := strapBitsDesign(map[string]string{"PHYAD2": "", "PHYAD1": "GND", "PHYAD0": "+3V3"})
		g := StrapGroup{Name: "PHYAD", Device: "U12", Nets: []string{"PHYAD2", "PHYAD1", "PHYAD0"}, Value: 1}
		fs := groupFindings(t, d, g)
		if len(fs) != 1 || !fs[0].Inconclusive {
			t.Fatalf("want 1 inconclusive finding, got %+v", fs)
		}
		roles := map[string]int{}
		for _, c := range fs[0].Context {
			roles[c.Role]++
		}
		if roles["device"] != 1 {
			t.Errorf("want exactly one device context, got %+v", fs[0].Context)
		}
		// The subject is the group's FIRST net, so the unreadable one is reachable only through this.
		if roles["undecided"] == 0 {
			t.Errorf("the nets the rule could not read must be reachable: %+v", fs[0].Context)
		}
	})

	t.Run("an address collision names both devices in the same role", func(t *testing.T) {
		// The case that made context a LIST with non-unique roles. Two entities play exactly the same
		// part here, so a map keyed by role would silently drop one of them.
		d := strapBitsDesign(map[string]string{
			"A2": "GND", "A1": "GND", "A0": "+3V3",
			"B2": "GND", "B1": "GND", "B0": "+3V3",
		})
		groups := []StrapGroup{
			{Name: "PHYAD U12", Device: "U12", Nets: []string{"A2", "A1", "A0"}, Value: 1, Bus: "MDIO"},
			{Name: "PHYAD U13", Device: "U13", Nets: []string{"B2", "B1", "B0"}, Value: 1, Bus: "MDIO"},
		}
		fs := strapCollisionRule(groups).Findings(check.NewModel(d))
		if len(fs) != 1 {
			t.Fatalf("want 1 collision finding, got %+v", fs)
		}
		var devices []string
		for _, c := range fs[0].Context {
			if c.Role != "device" {
				t.Errorf("unexpected role %q in a collision finding", c.Role)
			}
			devices = append(devices, c.Ref)
		}
		if len(devices) != 2 || devices[0] != "U12" || devices[1] != "U13" {
			t.Errorf("context devices = %v, want [U12 U13] in message order", devices)
		}
		// MDIO is a declared bus label from the intent file, not a design entity.
		for _, c := range fs[0].Context {
			if c.Ref == "MDIO" {
				t.Error("the bus is a declaration label, not a design entity; it must not be context")
			}
		}
	})
}
