package intent

import (
	"slices"
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

// collisionVerdicts runs the cross-group rule and returns its verdicts.
func collisionVerdicts(d *ir.Design, groups ...StrapGroup) []check.Verdict {
	return strapCollisionRule(groups).Eval(check.NewModel(d))
}

// threeOnOneBus is a bus carrying three declared groups, two of which share an address. It is the
// shape the old failures-only body collapsed into a single finding and the shape that decides whether
// a pair subject was the right call.
func threeOnOneBus() (*ir.Design, []StrapGroup) {
	d := strapBitsDesign(map[string]string{
		"A2": "GND", "A1": "GND", "A0": "+3V3", // U12 -> 1
		"B2": "GND", "B1": "GND", "B0": "+3V3", // U13 -> 1, the clash
		"C2": "GND", "C1": "+3V3", "C0": "GND", // U14 -> 2, fine
	})
	return d, []StrapGroup{
		{Name: "g12", Device: "U12", Nets: []string{"A2", "A1", "A0"}, Value: 1, Bus: "MDIO"},
		{Name: "g13", Device: "U13", Nets: []string{"B2", "B1", "B0"}, Value: 1, Bus: "MDIO"},
		{Name: "g14", Device: "U14", Nets: []string{"C2", "C1", "C0"}, Value: 2, Bus: "MDIO"},
	}
}

// TestStrapCollisionStatesConsideredSet (agni issue 391): every pair of groups sharing a bus gets a
// verdict, so the two correctly-addressed pairs are now countable. They used to report what a bus
// nobody declared reported, which is nothing.
func TestStrapCollisionStatesConsideredSet(t *testing.T) {
	d, groups := threeOnOneBus()
	r := strapCollisionRule(groups)
	if !r.StatesConsideredSet {
		t.Fatal("the rule must declare a considered set, or a bus with no clash on it means nothing")
	}
	vs := r.Eval(check.NewModel(d))
	if len(vs) != 3 {
		t.Fatalf("verdicts = %d, want 3 (every pair on the bus): %+v", len(vs), vs)
	}
	got := map[string]check.Outcome{}
	for _, v := range vs {
		if len(v.Subjects) != 2 {
			t.Errorf("subjects = %+v, want a pair", v.Subjects)
			continue
		}
		got[check.EntityRef(v.Subjects[0])+"+"+check.EntityRef(v.Subjects[1])] = v.Outcome
	}
	for pair, want := range map[string]check.Outcome{
		"U12+U13": check.Fail, // the same address
		"U12+U14": check.Pass,
		"U13+U14": check.Pass,
	} {
		if got[pair] != want {
			t.Errorf("%s = %q, want %q", pair, got[pair], want)
		}
	}
}

// TestStrapCollisionSubjectShapeIsThePair: the shape is DECLARED, and it is what lets a consumer index
// this rule's verdicts and a person build an id without running the check. A rule emitting a tuple it
// did not declare is the failure this pins.
func TestStrapCollisionSubjectShapeIsThePair(t *testing.T) {
	d, groups := threeOnOneBus()
	r := strapCollisionRule(groups)
	want := []string{check.KindComponent, check.KindComponent}
	if len(r.SubjectShape) != 2 || r.SubjectShape[0] != want[0] || r.SubjectShape[1] != want[1] {
		t.Fatalf("SubjectShape = %v, want %v", r.SubjectShape, want)
	}
	for _, v := range r.Eval(check.NewModel(d)) {
		if len(v.Subjects) != len(want) {
			t.Errorf("emitted a %d-tuple against a declared 2-tuple: %s", len(v.Subjects), check.SubjectRefs(v))
			continue
		}
		for i, e := range v.Subjects {
			if e.Kind != want[i] {
				t.Errorf("element %d is %q, declared %q", i, e.Kind, want[i])
			}
		}
	}
}

// TestStrapCollisionFindingsUnchangedForATwoWayClash: one clash is still one finding, with the same
// subject, message and both devices as context. The pair subject changes what the rule can SAY about
// the bus, not what it reports as a defect.
func TestStrapCollisionFindingsUnchangedForATwoWayClash(t *testing.T) {
	d, groups := threeOnOneBus()
	fs := strapCollisionRule(groups).Findings(check.NewModel(d))
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1 for one clash: %+v", len(fs), fs)
	}
	if check.EntityRef(fs[0].Subject) != "A2" {
		t.Errorf("subject = %q, want the first colliding group's first net", check.EntityRef(fs[0].Subject))
	}
	for _, want := range []string{"U12", "U13", "address 1", "MDIO"} {
		if !strings.Contains(fs[0].Message, want) {
			t.Errorf("message missing %q: %s", want, fs[0].Message)
		}
	}
	if strings.Contains(fs[0].Message, "U14") {
		t.Errorf("the innocent third device must not appear in the clash message: %s", fs[0].Message)
	}
	var devices []string
	for _, c := range fs[0].Context {
		devices = append(devices, c.Entity.Ref)
	}
	if !slices.Equal(devices, []string{"U12", "U13"}) {
		t.Errorf("context = %v, want both colliding devices in message order", devices)
	}
}

// TestStrapCollisionThreeWayIsThreeFindings is the one deliberate behaviour change. Three parts on one
// address used to be a single finding whose message said "U12 and U13 and U14 BOTH strap to", which
// is wrong past two. Pairwise it is three findings and each sentence is true.
func TestStrapCollisionThreeWayIsThreeFindings(t *testing.T) {
	d := strapBitsDesign(map[string]string{
		"A2": "GND", "A1": "GND", "A0": "+3V3",
		"B2": "GND", "B1": "GND", "B0": "+3V3",
		"C2": "GND", "C1": "GND", "C0": "+3V3",
	})
	groups := []StrapGroup{
		{Name: "g12", Device: "U12", Nets: []string{"A2", "A1", "A0"}, Value: 1, Bus: "MDIO"},
		{Name: "g13", Device: "U13", Nets: []string{"B2", "B1", "B0"}, Value: 1, Bus: "MDIO"},
		{Name: "g14", Device: "U14", Nets: []string{"C2", "C1", "C0"}, Value: 1, Bus: "MDIO"},
	}
	fs := strapCollisionRule(groups).Findings(check.NewModel(d))
	if len(fs) != 3 {
		t.Fatalf("findings = %d, want 3 (one per colliding pair): %+v", len(fs), fs)
	}
	for _, f := range fs {
		if strings.Count(f.Message, "strap to address") != 1 || len(f.Context) != 2 {
			t.Errorf("each finding must name exactly two devices: %s / %+v", f.Message, f.Context)
		}
	}
}

// TestStrapCollisionUndecidableIsNotConsidered: an unreadable group must never be decoded, because a
// fabricated address can fabricate a collision. It equally must not produce a PASS, which would assert
// the two do not clash on evidence nobody has. It used to leave through the same silence a correctly
// addressed pair did.
func TestStrapCollisionUndecidableIsNotConsidered(t *testing.T) {
	// U13's high bit carries no resistor and its group declares no default, so its value is unknown.
	d := strapBitsDesign(map[string]string{
		"A2": "GND", "A1": "GND", "A0": "+3V3", // U12 -> 1
		"B2": "", "B1": "GND", "B0": "+3V3", // U13 -> unknown (would be 1 if B2 were assumed 0)
	})
	groups := []StrapGroup{
		{Name: "g12", Device: "U12", Nets: []string{"A2", "A1", "A0"}, Value: 1, Bus: "MDIO"},
		{Name: "g13", Device: "U13", Nets: []string{"B2", "B1", "B0"}, Value: 1, Bus: "MDIO"},
	}
	vs := collisionVerdicts(d, groups...)
	if len(vs) != 1 {
		t.Fatalf("verdicts = %+v, want one for the pair", vs)
	}
	if vs[0].Outcome != check.NotConsidered {
		t.Fatalf("outcome = %q, want not-considered: an unreadable address is neither a clash nor a clearance", vs[0].Outcome)
	}
	for _, want := range []string{"U13", "B2"} {
		if !strings.Contains(vs[0].Reason, want) {
			t.Errorf("reason missing %q, so it does not say which half could not be read or why: %s", want, vs[0].Reason)
		}
	}
	if fs := strapCollisionRule(groups).Findings(check.NewModel(d)); len(fs) != 0 {
		t.Errorf("an undecidable pair must not produce a finding: %+v", fs)
	}
}

// TestStrapCollisionMissingNetIsNotConsidered: a declared net the board does not carry is the presence
// forms' defect to report, not this rule's, but this rule did LOOK and could not judge. It used to
// vanish through the same silence as a clean pair.
func TestStrapCollisionMissingNetIsNotConsidered(t *testing.T) {
	d := strapBitsDesign(map[string]string{
		"A2": "GND", "A1": "GND", "A0": "+3V3",
		"B1": "GND", "B0": "+3V3", // no B2 net at all
	})
	groups := []StrapGroup{
		{Name: "g12", Device: "U12", Nets: []string{"A2", "A1", "A0"}, Value: 1, Bus: "MDIO"},
		{Name: "g13", Device: "U13", Nets: []string{"B2", "B1", "B0"}, Value: 1, Bus: "MDIO"},
	}
	vs := collisionVerdicts(d, groups...)
	if len(vs) != 1 || vs[0].Outcome != check.NotConsidered {
		t.Fatalf("verdicts = %+v, want one not-considered", vs)
	}
	if !strings.Contains(vs[0].Reason, "B2") {
		t.Errorf("reason does not name the missing net: %s", vs[0].Reason)
	}
}

// TestStrapCollisionSameDeviceIsNotASubject: one part declaring two groups on a bus is a declaration
// to read, not two parts answering one address. A verdict there would name the same device twice and
// key on itself.
func TestStrapCollisionSameDeviceIsNotASubject(t *testing.T) {
	d := strapBitsDesign(map[string]string{
		"A2": "GND", "A1": "GND", "A0": "+3V3",
		"B2": "GND", "B1": "GND", "B0": "+3V3",
	})
	groups := []StrapGroup{
		{Name: "addr", Device: "U12", Nets: []string{"A2", "A1", "A0"}, Value: 1, Bus: "MDIO"},
		{Name: "alt", Device: "U12", Nets: []string{"B2", "B1", "B0"}, Value: 1, Bus: "MDIO"},
	}
	if vs := collisionVerdicts(d, groups...); len(vs) != 0 {
		t.Errorf("verdicts = %+v, want none: both groups are on one part", vs)
	}
}

// TestStrapCollisionPassNamesBothAddresses is the witness check (build/evidence.md). "These two do not
// clash" restates the outcome and reads the same on every clean pair on the board. Naming BOTH decoded
// addresses is what a reviewer checks against the datasheet, and asserting two different pairs read
// differently is what catches a witness carrying a constant.
func TestStrapCollisionPassNamesBothAddresses(t *testing.T) {
	d, groups := threeOnOneBus()
	var passes []string
	for _, v := range collisionVerdicts(d, groups...) {
		if v.Outcome == check.Pass {
			if v.Witness == nil {
				t.Fatalf("a pass with no witness: %+v", v)
			}
			passes = append(passes, v.Witness.Statement)
		}
	}
	if len(passes) != 2 {
		t.Fatalf("passes = %d, want 2", len(passes))
	}
	if passes[0] == passes[1] {
		t.Errorf("both clean pairs read identically (%q), so the pass is decoration", passes[0])
	}
	for _, p := range passes {
		if !strings.Contains(p, "MDIO") {
			t.Errorf("statement %q does not name the bus the addresses share", p)
		}
	}
	// U12 straps to 1 and U14 to 2, so the pair naming them must carry both numbers.
	var u12u14 string
	for _, p := range passes {
		if strings.Contains(p, "U12") && strings.Contains(p, "U14") {
			u12u14 = p
		}
	}
	if !strings.Contains(u12u14, "1") || !strings.Contains(u12u14, "2") {
		t.Errorf("statement %q does not carry both decoded addresses", u12u14)
	}
}
