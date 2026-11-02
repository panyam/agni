package profiles

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// sdSlot is a 4-bit SD card bus named the way the MNT Reform motherboard names it: SD2_CMD, SD2_CLK,
// SD2_DATA0 to SD2_DATA3 between the host U1 and the socket J1, and a card detect line. CMD has no
// pull-up on this side, as on the Reform, where it sits beyond a level shifter. withDetect leaves the
// card detect line out when false.
func sdSlot(withDetect bool) *ir.Design {
	d := &ir.Design{
		Components: comps("U1", "J1"),
		Nets: []*ir.Net{
			net("SD2_CMD", "U1.1", "J1.1"),
			net("SD2_CLK", "U1.2", "J1.2"),
			net("SD2_DATA0", "U1.3", "J1.3"),
			net("SD2_DATA1", "U1.4", "J1.4"),
			net("SD2_DATA2", "U1.5", "J1.5"),
			net("SD2_DATA3", "U1.6", "J1.6"),
		},
	}
	if withDetect {
		d.Nets = append(d.Nets, net("SD2_CD", "U1.7", "J1.7"))
	}
	return d
}

// An SD card slot shares eMMC's CMD, CLK and DAT names, and its card detect line says it is a
// removable card, which has four data lines and no reset. The eMMC profile reported nine missing
// signals, a missing pull-up and a dangling clock on such a bus (agni issue 1246).
func TestAnSDCardSlotIsNotAnEMMC(t *testing.T) {
	m := check.NewModel(sdSlot(true))
	if fs := check.RunBackground(m, Compile(EMMC)); len(fs) != 0 {
		t.Errorf("an SD slot with card detect: want no eMMC finding, got %d: %+v", len(fs), fs)
	}
	if InUse(m, EMMC) || Anchored(m, EMMC) || Named(m, EMMC) {
		t.Errorf("an SD slot reads as an eMMC: InUse %v, Anchored %v, Named %v", InUse(m, EMMC), Anchored(m, EMMC), Named(m, EMMC))
	}
	if n := Nets(m, EMMC); len(n) != 0 {
		t.Errorf("an SD slot's nets scope an eMMC review item: %v", n)
	}
	if c := Coverages(context.Background(), EMMC, m, nil); len(c) != 0 {
		t.Errorf("an SD slot shows eMMC coverage: %+v", c)
	}

	// Without the card detect line the same bus does read as an eMMC, which is what makes the line,
	// rather than anything else in the fixture, the reason the bus above was left alone.
	m = check.NewModel(sdSlot(false))
	rules := map[string]int{}
	for _, f := range check.RunBackground(m, Compile(EMMC)) {
		rules[f.Rule]++
	}
	if rules["emmc-signal-missing"] != 5 || rules["emmc-missing-pullup"] != 1 {
		t.Errorf("the same bus with no card detect: want DAT4-7 and RST missing and the pull-up, got %v", rules)
	}
}

// emmcSpelled is emmcGood with the data lines named DATAn, as some designs and vendor references spell
// them.
func emmcSpelled() *ir.Design {
	d := emmcGood()
	for _, n := range d.Nets {
		if strings.HasPrefix(n.Name, "MMC_DAT") {
			n.Name = strings.Replace(n.Name, "_DAT", "_DATA", 1)
		}
	}
	return d
}

func TestAnEMMCWithDataLinesSpelledDATAIsComplete(t *testing.T) {
	if fs := check.RunBackground(check.NewModel(emmcSpelled()), Compile(EMMC)); len(fs) != 0 {
		t.Fatalf("an eMMC whose data lines are MMC_DATA0-7: want no finding, got %d: %+v", len(fs), fs)
	}
}

// An M.2 slot's unused SMBus clock ends _CLK, and the eMMC profile took it as the eMMC clock and
// reported it dangling. It is named from no eMMC bus's stem, so it is no eMMC signal.
func TestAnotherBussClockIsNotTheEMMCClock(t *testing.T) {
	d := emmcGood()
	d.Components = append(d.Components, comps("J9")...)
	d.Nets = append(d.Nets, net("M2_SMB_CLK", "J9.1"))
	m := check.NewModel(d)
	if fs := check.RunBackground(m, Compile(EMMC)); len(fs) != 0 {
		t.Errorf("a one-pin M2_SMB_CLK beside a complete eMMC: want no finding, got %d: %+v", len(fs), fs)
	}
	if Nets(m, EMMC)["M2_SMB_CLK"] {
		t.Error("M2_SMB_CLK is in the eMMC review scope")
	}
}

// An SD slot beside an eMMC is common on a SoC board. The SD bus's DAT0 must not stand in for the
// eMMC's, and the SD bus itself is still left alone.
func TestAnSDSlotBesideAnEMMCLendsItNothing(t *testing.T) {
	d := emmcGood()
	var nets []*ir.Net
	for _, n := range d.Nets {
		if n.Name != "MMC_DAT0" {
			nets = append(nets, n)
		}
	}
	sd := sdSlot(true)
	d.Nets = append(nets, sd.Nets...)
	d.Components = append(d.Components, comps("U9", "J1")...)
	// sdSlot's host is U1, which here is the eMMC host, so the SD slot gets a host of its own.
	for _, n := range sd.Nets {
		for _, c := range n.Connections {
			if c.ComponentRef == "U1" {
				c.ComponentRef = "U9"
			}
		}
	}
	var got []string
	for _, f := range check.RunBackground(check.NewModel(d), Compile(EMMC)) {
		got = append(got, f.Rule+" "+check.EntityRef(f.Subject)+" "+f.Message)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0], "emmc-signal-missing MMC_CMD ") || !strings.Contains(got[0], "DAT0") {
		t.Fatalf("want the eMMC's own DAT0 reported missing at MMC_CMD and nothing on the SD slot, got %q", got)
	}
}

// Two eMMC buses on one board are judged apart, so the second one's missing DAT4 is reported even
// though the first carries a DAT4 (agni issue 1210's shape).
func TestTwoEMMCBusesAreJudgedApart(t *testing.T) {
	d := &ir.Design{Components: comps("U1", "U2", "U3", "R1", "R2")}
	for _, b := range []struct {
		stem, dev string
		pin       int
		skip      string
	}{{"MMC0", "U2", 0, ""}, {"MMC1", "U3", 20, "DAT4"}} {
		sigs := []string{"CMD", "CLK", "DAT0", "DAT1", "DAT2", "DAT3", "DAT4", "DAT5", "DAT6", "DAT7", "RST"}
		for i, s := range sigs {
			if s == b.skip {
				continue
			}
			conns := []string{"U1." + itoa(b.pin+i+1), b.dev + "." + itoa(i+1)}
			if s == "CMD" {
				conns = append(conns, map[string]string{"MMC0": "R1.1", "MMC1": "R2.1"}[b.stem])
			}
			d.Nets = append(d.Nets, net(b.stem+"_"+s, conns...))
		}
	}
	d.Nets = append(d.Nets, net("+3V3", "R1.2", "R2.2", "U2.12", "U3.12"))
	m := check.NewModel(d)
	var got []string
	for _, f := range check.RunBackground(m, Compile(EMMC)) {
		got = append(got, f.Rule+" "+check.EntityRef(f.Subject)+" "+f.Message)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0], "emmc-signal-missing MMC1_CMD ") || !strings.Contains(got[0], "DAT4") {
		t.Fatalf("want DAT4 missing on MMC1 alone, got %q", got)
	}
	covs := Coverages(context.Background(), EMMC, m, nil)
	if len(covs) != 2 || covs[0].Anchor != "MMC0_CMD" || covs[1].Anchor != "MMC1_CMD" {
		t.Fatalf("want one coverage matrix per bus, anchored at MMC0_CMD and MMC1_CMD, got %+v", covs)
	}
	for _, s := range covs[1].Signals {
		if s.Name == "DAT4" && s.State != StateMissing {
			t.Errorf("MMC1's DAT4 coverage = %q (net %q), want missing", s.State, s.Net)
		}
	}
}

// A glob- or regex-declared signal belongs to a bus only past the stem's separator, so bus SD never
// takes SD2's lines.
func TestABusTakesNoLineOfALongerStem(t *testing.T) {
	dat0 := EMMC.Signals[2]
	if dat0.Regex == "" {
		t.Fatalf("fixture is wrong: want a regex-declared DAT0, got %+v", dat0)
	}
	if EMMC.onBus("SD2_DATA0", "SD", dat0) {
		t.Error("bus SD takes SD2_DATA0")
	}
	if !EMMC.onBus("SD2_DATA0", "SD2", dat0) {
		t.Error("bus SD2 does not take SD2_DATA0")
	}

	// The generated rules draw the same line: bus MMC is missing its DAT0 though MMCX_DAT0 is wired.
	d := emmcGood()
	for _, n := range d.Nets {
		if n.Name == "MMC_DAT0" {
			n.Name = "MMCX_DAT0"
		}
	}
	var got []string
	for _, f := range check.RunBackground(check.NewModel(d), Compile(EMMC)) {
		got = append(got, f.Rule+" "+f.Message)
	}
	if len(got) != 1 || !strings.Contains(got[0], "missing required signal DAT0") {
		t.Errorf("want DAT0 missing on bus MMC, got %q", got)
	}
}

func TestAnInstanceKeyIsValidated(t *testing.T) {
	anchor := Signal{Name: "CMD", Suffix: "_CMD", Anchor: true}
	clk := Signal{Name: "CLK", Suffix: "_CLK"}
	cd := Signal{Name: "CD", Suffix: "_CD"}
	for _, tc := range []struct {
		name string
		p    Profile
		want string
	}{
		{"excludes with no instance", Profile{Name: "x", Signals: []Signal{anchor, clk}, Excludes: []Signal{cd}}, "excludes needs an instance key"},
		{"an unknown instance", Profile{Name: "x", Instance: "per_port", Signals: []Signal{anchor, clk}}, "unknown instance"},
		{"a regex anchor", Profile{Name: "x", Instance: InstanceAnchorStem, Signals: []Signal{{Name: "CMD", Regex: "_CMD$", Anchor: true}, clk}}, "must declare a suffix"},
		{"an exclude needing a pull-up", Profile{Name: "x", Instance: InstanceAnchorStem, Signals: []Signal{anchor, clk}, Excludes: []Signal{{Name: "CD", Suffix: "_CD", PullUp: true}}}, "cannot be an anchor or need a pull-up"},
		{"an exclude with no matcher", Profile{Name: "x", Instance: InstanceAnchorStem, Signals: []Signal{anchor, clk}, Excludes: []Signal{{Name: "CD"}}}, "declares no matcher"},
	} {
		if err := Validate(tc.p); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want an error containing %q, got %v", tc.name, tc.want, err)
		}
	}
}

// The instance key and the exclude signals are profile YAML keys, so a profile file can declare them.
func TestAProfileFileDeclaresAnInstanceKey(t *testing.T) {
	p, err := Parse([]byte(`
name: SDish
instance: anchor_stem
signals:
  - {name: CMD, suffix: _CMD, anchor: true}
  - {name: CLK, suffix: _CLK}
excludes:
  - {name: CD, suffix: _CD}
requirements:
  - {type: signal-missing}
`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Instance != InstanceAnchorStem || len(p.Excludes) != 1 || p.Excludes[0].Suffix != "_CD" || len(p.Signals) != 2 {
		t.Errorf("parsed %+v", p)
	}
}
