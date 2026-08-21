package intent

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// twoStage is the declaration every test below varies against: VDD_CORE up first, signalled by
// CORE_PG, gating VDD_IO through IO_EN. The middle handles (IO's own power-good, CORE's enable) are
// declared where a test needs the reversed-chain diagnosis.
const twoStage = `
name: I
sequences:
  - name: SoC power tree
    relation: enable-gated
    order:
      - {rail: VDD_CORE, good: CORE_PG, enable: CORE_EN}
      - {rail: VDD_IO, good: IO_PG, enable: IO_EN}
`

// conn and net build a design's connectivity with the ref-des spelled by the caller; sequence tests
// care about which parts sit on which nets, not about pin numbering.
func conn(ref string) *ir.Connection { return &ir.Connection{ComponentRef: ref, PinRef: "1"} }

func net(name string, refs ...string) *ir.Net {
	n := &ir.Net{Name: name}
	for _, r := range refs {
		n.Connections = append(n.Connections, conn(r))
	}
	return n
}

func runSeq(t *testing.T, decl Declaration, d *ir.Design) []check.Finding {
	t.Helper()
	return check.Run(check.NewModel(d), Compile(decl))
}

// TestSequenceSameNetChainIsSilent: the declaration names ONE net as both the earlier stage's
// power-good and the later stage's enable, which is how an open-drain PG tied to an EN pin reads.
func TestSequenceSameNetChainIsSilent(t *testing.T) {
	decl := declOf(t, `
name: I
sequences:
  - name: SoC power tree
    relation: enable-gated
    order:
      - {rail: VDD_CORE, good: PG_EN}
      - {rail: VDD_IO, enable: PG_EN}
`)
	d := &ir.Design{Nets: []*ir.Net{net("VDD_CORE"), net("VDD_IO"), net("PG_EN", "U1", "U2")}}
	if fs := runSeq(t, decl, d); len(fs) != 0 {
		t.Errorf("one net serving as both handles is a chain, got %+v", fs)
	}
	// The same declaration where the shared net carries no parts the model can see (a source that
	// names the net but lists no connections). A net cannot fail to reach itself, so this must stay
	// silent rather than reporting that nothing connects PG_EN to PG_EN.
	bare := &ir.Design{Nets: []*ir.Net{net("VDD_CORE"), net("VDD_IO"), net("PG_EN")}}
	if fs := runSeq(t, decl, bare); len(fs) != 0 {
		t.Errorf("a net is linked to itself whatever sits on it, got %+v", fs)
	}
}

// TestSequenceChainThroughSeriesResistorIsSilent: the divider that drops an open-drain power-good to
// the enable pin's threshold sits between the two nets. A series element is a part on both nets, so
// the one-part test credits it with no separate series walk.
func TestSequenceChainThroughSeriesResistorIsSilent(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "R1", DeviceClasses: []string{"resistor"}}},
		Nets: []*ir.Net{
			net("VDD_CORE"), net("VDD_IO"),
			net("CORE_PG", "R1"), net("IO_EN", "R1"),
		},
	}
	if fs := runSeq(t, declOf(t, twoStage), d); len(fs) != 0 {
		t.Errorf("a chain through one series resistor must not fire, got %+v", fs)
	}
}

// TestSequenceChainAbsentFires is the honest-guard test. Both declared handle nets are on the design
// and nothing connects them, so the later rail is free to come up first. This must be a FINDING: a
// design with no gating chain must never read as sequencing correct.
func TestSequenceChainAbsentFires(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1"}, {RefDes: "U2"}},
		Nets: []*ir.Net{
			net("VDD_CORE", "U1"), net("VDD_IO", "U2"),
			net("CORE_PG", "U1"), net("IO_EN", "U2"),
		},
	}
	fs := runSeq(t, declOf(t, twoStage), d)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding for an unenforced order, got %d: %+v", len(fs), fs)
	}
	if fs[0].Rule != "sequence-soc-power-tree" {
		t.Errorf("finding should carry the per-sequence rule name, got %q", fs[0].Rule)
	}
	if check.EntityRef(fs[0].Subject) != "IO_EN" {
		t.Errorf("subject should be the enable net nothing drives, got %q", fs[0].Subject)
	}
	if !strings.Contains(fs[0].Message, "CORE_PG") || !strings.Contains(fs[0].Message, "IO_EN") {
		t.Errorf("message should name both ends of the missing link, got %q", fs[0].Message)
	}
}

// TestSequenceAbsentHandleNetFires: a declared gating net the design does not carry at all. The chain
// is not there to enforce anything, so this fires rather than being skipped. That is the opposite of
// how an absent RAIL is treated (see the test below), because the handles ARE the assertion.
func TestSequenceAbsentHandleNetFires(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{net("VDD_CORE"), net("VDD_IO")}}
	fs := runSeq(t, declOf(t, twoStage), d)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding when both handles are missing, got %d: %+v", len(fs), fs)
	}
	for _, want := range []string{"CORE_PG", "IO_EN", "not on the design"} {
		if !strings.Contains(fs[0].Message, want) {
			t.Errorf("message should contain %q, got %q", want, fs[0].Message)
		}
	}
}

// TestSequenceAbsentRailIsSilent: a declared rail the design does not carry is NOT this rule's
// business. Missing nets are what the voltage-domain and subsystem forms report, and firing here as
// well would put one defect under two review items.
func TestSequenceAbsentRailIsSilent(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{net("CORE_PG", "U1", "U2"), net("IO_EN", "U1", "U2")}}
	if fs := runSeq(t, declOf(t, twoStage), d); len(fs) != 0 {
		t.Errorf("an absent rail is a presence question, not a sequencing finding, got %+v", fs)
	}
}

// TestSequenceReversedChainFires: the design gates the EARLIER rail on the LATER one's power-good.
// It is the defect this rule is most worth having for, because it looks correct on the schematic, so
// it gets its own message rather than reading as a missing link.
func TestSequenceReversedChainFires(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1"}, {RefDes: "U2"}},
		Nets: []*ir.Net{
			net("VDD_CORE", "U1"), net("VDD_IO", "U2"),
			net("CORE_PG", "U1"), net("IO_EN", "U2"),
			// The wrong way round: VDD_IO's power-good drives VDD_CORE's enable.
			net("IO_PG", "U2", "U1"), net("CORE_EN", "U2", "U1"),
		},
	}
	fs := runSeq(t, declOf(t, twoStage), d)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding for a reversed chain, got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "the other way round") {
		t.Errorf("a reversed chain needs its own diagnosis, got %q", fs[0].Message)
	}
	if check.EntityRef(fs[0].Subject) != "CORE_EN" {
		t.Errorf("subject should be the wrongly-gated enable, got %q", fs[0].Subject)
	}
}

// TestSequenceReversedChainBeatsAbsentHandles: an order declared the wrong way round usually names
// the wrong handles too, so the declared pair does not exist on the design at all while the mirror
// pair is wired. The reversed diagnosis has to win, or the author is told their nets are missing and
// goes looking for the wrong thing.
func TestSequenceReversedChainBeatsAbsentHandles(t *testing.T) {
	decl := declOf(t, `
name: I
sequences:
  - name: SoC power tree
    relation: enable-gated
    order:
      - {rail: VDD_IO, good: IO_PG, enable: IO_EN}
      - {rail: VDD_CORE, good: CORE_PG, enable: CORE_EN}
`)
	// IO_PG and CORE_EN are not on the design. CORE_PG and IO_EN are, wired through R1.
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "R1", DeviceClasses: []string{"resistor"}}},
		Nets: []*ir.Net{
			net("VDD_CORE"), net("VDD_IO"),
			net("CORE_PG", "R1"), net("IO_EN", "R1"),
		},
	}
	fs := runSeq(t, decl, d)
	if len(fs) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "the other way round") {
		t.Errorf("the reversed chain is the diagnosis to report, got %q", fs[0].Message)
	}
}

// TestSequenceLinkThroughSmallPartIsCredited: a single-gate buffer between the power-good and the
// enable is a real chain, and the series walk cannot cross an active part. Crediting a small part is
// what keeps a genuinely sequenced board off a false fail.
func TestSequenceLinkThroughSmallPartIsCredited(t *testing.T) {
	// U3 sits on four nets, the shape of a single-gate buffer in a 5-pin package. The count is a
	// literal, not gatingFanLimit-derived, so raising the limit cannot quietly make this test agree
	// with itself.
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U3", DeviceClasses: []string{"ic"}}},
		Nets: []*ir.Net{
			net("VDD_CORE"), net("VDD_IO"),
			net("CORE_PG", "U3"), net("IO_EN", "U3"),
			net("VCC_U3", "U3"), net("GND_U3", "U3"),
		},
	}
	if fs := runSeq(t, declOf(t, twoStage), d); len(fs) != 0 {
		t.Errorf("a buffered chain must not fire, got %+v", fs)
	}
}

// TestSequenceLinkThroughControllerIsNotCredited is the other half of that judgement, and the one the
// motivating design turns on: a power-good landing on an MCU that also drives the enable means the
// order lives in FIRMWARE, which is not in the netlist. Crediting it would let any board whose
// supervisory signals converge on one processor read as correctly sequenced.
func TestSequenceLinkThroughControllerIsNotCredited(t *testing.T) {
	nets := []*ir.Net{net("VDD_CORE"), net("VDD_IO"), net("CORE_PG", "U9"), net("IO_EN", "U9")}
	// U9 touches 24 nets, the shape of a small MCU. The count is a LITERAL rather than
	// gatingFanLimit+1: derived from the constant, this test would move with the limit and could never
	// detect it being widened, which is a test that agrees with whatever the code says.
	for i := 0; i < 20; i++ {
		nets = append(nets, net("GPIO"+string(rune('A'+i)), "U9"))
	}
	d := &ir.Design{Components: []*ir.Component{{RefDes: "U9", DeviceClasses: []string{"soc"}}}, Nets: nets}
	fs := runSeq(t, declOf(t, twoStage), d)
	if len(fs) != 1 {
		t.Fatalf("a firmware path is not netlist evidence of an order, want 1 finding, got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "free to come up first") {
		t.Errorf("want the unenforced-order finding, got %q", fs[0].Message)
	}
}

// TestSequenceVirtualSymbolIsNotAGatingPart: a KiCad-style virtual connectivity symbol (#PWR/#FLG) is
// a marker, not a part, so two nets it appears on are not gated by it. It is small by every measure
// the fan test uses, which is exactly why it needs excluding by name.
func TestSequenceVirtualSymbolIsNotAGatingPart(t *testing.T) {
	d := &ir.Design{
		Nets: []*ir.Net{
			net("VDD_CORE"), net("VDD_IO"),
			net("CORE_PG", "#PWR01"), net("IO_EN", "#PWR01"),
		},
	}
	if fs := runSeq(t, declOf(t, twoStage), d); len(fs) != 1 {
		t.Fatalf("a virtual symbol must not credit a gating link, want 1 finding, got %d: %+v", len(fs), fs)
	}
}

// TestSequencesCompileToDistinctRules: several sequences bind and report independently, which is the
// whole reason this is one rule per declaration rather than one shared intent/power-sequence
// (WS3-058: a shared name gives six review items one verdict).
func TestSequencesCompileToDistinctRules(t *testing.T) {
	decl := declOf(t, `
name: I
sequences:
  - name: SoC power tree
    relation: enable-gated
    order: [{rail: VDD_CORE, good: CORE_PG}, {rail: VDD_IO, enable: IO_EN}]
  - name: modem rails
    relation: enable-gated
    order: [{rail: VBAT_MODEM, good: MODEM_PG}, {rail: MODEM_IO, enable: MODEM_EN}]
`)
	names := map[string]bool{}
	for _, r := range Compile(decl) {
		names[r.Name] = true
	}
	if !names["sequence-soc-power-tree"] || !names["sequence-modem-rails"] {
		t.Errorf("each sequence needs its own rule name, got %v", names)
	}
}

// TestSequenceWithoutGatingPairCompilesToNothing: a Declaration built in Go can carry a sequence Parse
// would reject. It must compile to NO rule, because a rule with no link to judge can only ever pass,
// which is the hollow verdict this family exists to avoid. Parse is where an author meets this
// (TestParseRejectsUncheckableSequence); this guard covers the programmatic path.
func TestSequenceWithoutGatingPairCompilesToNothing(t *testing.T) {
	decl := Declaration{
		Name: "I",
		Sequences: []Sequence{{
			Name:     "PMIC internal",
			Relation: SequenceEnableGated,
			Order:    []SequenceStage{{Rail: "VDD_CORE"}, {Rail: "VDD_IO"}},
		}},
	}
	if rules := Compile(decl); len(rules) != 0 {
		t.Errorf("a sequence with no gating handles must compile to no rule, got %+v", rules)
	}
}

// TestParseSequenceValid: the YAML form round-trips into the domain type.
func TestParseSequenceValid(t *testing.T) {
	d := declOf(t, twoStage)
	if len(d.Sequences) != 1 {
		t.Fatalf("want 1 sequence, got %+v", d.Sequences)
	}
	s := d.Sequences[0]
	if s.Name != "SoC power tree" || s.Relation != SequenceEnableGated || len(s.Order) != 2 {
		t.Fatalf("sequence parsed wrong: %+v", s)
	}
	if s.Order[0].Rail != "VDD_CORE" || s.Order[0].Good != "CORE_PG" || s.Order[1].Enable != "IO_EN" {
		t.Errorf("stage field mapping wrong: %+v", s.Order)
	}
}

// TestParseRejectsUncheckableSequence is the load-time half of the honest guard. A board sequenced
// inside a PMIC or by firmware has no gating nets to name, and the right answer is that it cannot
// declare a sequence at all, not a rule that passes on evidence nobody has. The error has to say so,
// or an author will invent net names to satisfy the schema.
func TestParseRejectsUncheckableSequence(t *testing.T) {
	_, err := Parse([]byte(`
name: I
sequences:
  - name: PMIC internal
    relation: enable-gated
    order: [{rail: VDD_CORE}, {rail: VDD_IO}]
`))
	if err == nil {
		t.Fatal("a sequence with no good -> enable pair must be rejected at load")
	}
	for _, want := range []string{"PMIC internal", "good", "enable"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should teach; want %q in %q", want, err.Error())
		}
	}
}

// TestParseRejectsBadSequences covers the rest of the load validation, each case being a declaration
// that would otherwise compile to a rule nobody can trust.
func TestParseRejectsBadSequences(t *testing.T) {
	cases := map[string]string{
		"no name": "name: N\nsequences:\n  - {relation: enable-gated, order: [{rail: A, good: A_PG}, {rail: B, enable: B_EN}]}",
		// An unknown relation would silently compile to the enable-gated reading, checking something
		// the author did not ask for.
		"unknown relation": "name: N\nsequences:\n  - {name: S, relation: before, order: [{rail: A, good: A_PG}, {rail: B, enable: B_EN}]}",
		"no relation":      "name: N\nsequences:\n  - {name: S, order: [{rail: A, good: A_PG}, {rail: B, enable: B_EN}]}",
		// A one-stage order has nothing to come before. It is also caught by the gating-pair check,
		// so the assertion below holds it to its OWN message: the two errors send an author to
		// different fixes.
		"one stage":     "name: N\nsequences:\n  - {name: S, relation: enable-gated, order: [{rail: A, good: A_PG}]}",
		"stage no rail": "name: N\nsequences:\n  - {name: S, relation: enable-gated, order: [{good: A_PG}, {rail: B, enable: B_EN}]}",
		// A rail twice in one order has no unambiguous position.
		"repeated rail": "name: N\nsequences:\n  - {name: S, relation: enable-gated, order: [{rail: A, good: A_PG}, {rail: A, enable: A_EN}]}",
		// Two sequences slugifying alike would collide on one rule name, so one would silently
		// shadow the other's review item.
		"slug collision": "name: N\nsequences:\n" +
			"  - {name: SoC power tree, relation: enable-gated, order: [{rail: A, good: A_PG}, {rail: B, enable: B_EN}]}\n" +
			"  - {name: 'soc-power-tree', relation: enable-gated, order: [{rail: C, good: C_PG}, {rail: D, enable: D_EN}]}",
		"name with no alphanumerics": "name: N\nsequences:\n  - {name: '---', relation: enable-gated, order: [{rail: A, good: A_PG}, {rail: B, enable: B_EN}]}",
	}
	for label, doc := range cases {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: expected a validation error, got nil", label)
		}
	}
	_, err := Parse([]byte(cases["one stage"]))
	if err == nil || !strings.Contains(err.Error(), "at least two stages") {
		t.Errorf("a one-stage order needs its own message, got %v", err)
	}
}

// TestNetFanCountsDistinctNets: the part-size measure counts NETS, not connections, so a part with
// several pins on one net (a load switch's paralleled outputs, an MCU's many grounds) is not inflated
// into a controller and stops crediting the gating links it really makes.
func TestNetFanCountsDistinctNets(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{net("VOUT", "U1", "U1", "U1"), net("EN", "U1")}}
	if got := netFan(check.NewModel(d))["U1"]; got != 2 {
		t.Errorf("netFan counted %d, want 2 (VOUT and EN)", got)
	}
}

// TestSequenceAloneIsANonEmptyDeclaration: sequences count toward the "declares nothing" check, so a
// declaration carrying only sequences loads.
func TestSequenceAloneIsANonEmptyDeclaration(t *testing.T) {
	if _, err := Parse([]byte(twoStage)); err != nil {
		t.Errorf("a sequences-only declaration must load, got %v", err)
	}
}
