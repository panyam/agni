package profiles

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// emmcGood: all eleven signals wired end-to-end; CMD pulled up to +3V3 through R1 (a resistor the
// reach walk crosses). No profile finding.
func emmcGood() *ir.Design {
	return &ir.Design{
		Components: comps("U1", "U2", "R1"),
		Nets: []*ir.Net{
			net("MMC_CMD", "U1.1", "U2.1", "R1.1"),
			net("MMC_CLK", "U1.2", "U2.2"),
			net("MMC_DAT0", "U1.3", "U2.3"),
			net("MMC_DAT1", "U1.4", "U2.4"),
			net("MMC_DAT2", "U1.5", "U2.5"),
			net("MMC_DAT3", "U1.6", "U2.6"),
			net("MMC_DAT4", "U1.7", "U2.7"),
			net("MMC_DAT5", "U1.8", "U2.8"),
			net("MMC_DAT6", "U1.9", "U2.9"),
			net("MMC_DAT7", "U1.10", "U2.10"),
			net("MMC_RST", "U1.11", "U2.11"),
			net("+3V3", "R1.2", "U2.12"),
		},
	}
}

// emmcBroken: DAT4 net absent (missing), no pull-up resistor on CMD (missing-pullup), and CLK on a
// single-pin net (dangling).
func emmcBroken() *ir.Design {
	return &ir.Design{
		Components: comps("U1", "U2"),
		Nets: []*ir.Net{
			net("MMC_CMD", "U1.1", "U2.1"),
			net("MMC_CLK", "U1.2"),
			net("MMC_DAT0", "U1.3", "U2.3"),
			net("MMC_DAT1", "U1.4", "U2.4"),
			net("MMC_DAT2", "U1.5", "U2.5"),
			net("MMC_DAT3", "U1.6", "U2.6"),
			net("MMC_DAT5", "U1.8", "U2.8"),
			net("MMC_DAT6", "U1.9", "U2.9"),
			net("MMC_DAT7", "U1.10", "U2.10"),
			net("MMC_RST", "U1.11", "U2.11"),
		},
	}
}

func TestEMMCSilent(t *testing.T) {
	if fs := check.Run(check.NewModel(emmcGood()), Compile(EMMC)); len(fs) != 0 {
		t.Fatalf("good eMMC bus: want 0 findings, got %d: %+v", len(fs), fs)
	}
}

func TestEMMCFires(t *testing.T) {
	got := map[string]check.Finding{}
	for _, f := range check.Run(check.NewModel(emmcBroken()), Compile(EMMC)) {
		got[f.Rule] = f
	}
	if len(got) != 3 {
		t.Fatalf("want 3 distinct rules firing, got %d: %+v", len(got), got)
	}
	if f := got["emmc-signal-missing"]; f.Subject != "MMC_CMD" || !strings.Contains(f.Message, "DAT4") {
		t.Errorf("signal-missing: want anchor MMC_CMD + DAT4, got %+v", f)
	}
	if f := got["emmc-missing-pullup"]; f.Subject != "MMC_CMD" {
		t.Errorf("missing-pullup: want MMC_CMD, got %+v", f)
	}
	if f := got["emmc-signal-dangling"]; f.Subject != "MMC_CLK" {
		t.Errorf("signal-dangling: want MMC_CLK, got %+v", f)
	}
}

// Compile produces the four eMMC rules (signal-missing convention, host-incomplete, missing-pullup,
// signal-dangling), registered under "profile" — the same four SPI-NOR yields, with no compiler
// change: a new interface is a data value (WS3-046).
func TestEMMCCompileAndRegistered(t *testing.T) {
	if got := len(Compile(EMMC)); got != 4 {
		t.Fatalf("Compile(EMMC): want 4 rules, got %d", got)
	}
	found := 0
	for _, r := range check.DefaultCatalog().Rules() {
		if strings.HasPrefix(r.Name, "profile/emmc-") {
			found++
		}
	}
	if found != 4 {
		t.Fatalf(`want 4 "profile/emmc-*" rules in DefaultCatalog, got %d`, found)
	}
}

func emmcHost(ref string) *ir.Component {
	return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
		Attributes: map[string]string{"interface": "eMMC"}}
}

// A component declaring interface=eMMC with DAT4 absent: the host-anchored path fires ONCE on the
// host (DAT4), and the convention path is suppressed (no double-report); CMD is pulled up and every
// net is 2-pin, so no other rule fires.
func TestEMMCHostIncomplete(t *testing.T) {
	d := &ir.Design{
		Components: append(comps("U1", "R1"), emmcHost("U2")),
		Nets: []*ir.Net{
			net("MMC_CMD", "U1.1", "U2.1", "R1.1"),
			net("MMC_CLK", "U1.2", "U2.2"),
			net("MMC_DAT0", "U1.3", "U2.3"),
			net("MMC_DAT1", "U1.4", "U2.4"),
			net("MMC_DAT2", "U1.5", "U2.5"),
			net("MMC_DAT3", "U1.6", "U2.6"),
			net("MMC_DAT5", "U1.8", "U2.8"),
			net("MMC_DAT6", "U1.9", "U2.9"),
			net("MMC_DAT7", "U1.10", "U2.10"),
			net("MMC_RST", "U1.11", "U2.11"),
			net("+3V3", "R1.2", "U2.12"),
		},
	}
	fs := check.Run(check.NewModel(d), Compile(EMMC))
	if len(fs) != 1 {
		t.Fatalf("want exactly 1 finding (host-incomplete DAT4, convention suppressed), got %d: %+v", len(fs), fs)
	}
	if f := fs[0]; f.Rule != "emmc-host-incomplete" || f.Subject != "U2" || !strings.Contains(f.Message, "DAT4") {
		t.Fatalf("want host-incomplete on U2/DAT4, got %+v", f)
	}
}

// A host that declares the interface but is wired to none of its signals: host-anchored completeness
// flags every one of the eleven required signals (wholly-absent detection the convention path cannot
// do).
func TestEMMCHostWhollyAbsent(t *testing.T) {
	d := &ir.Design{
		Components: append(comps("U1"), emmcHost("U2")),
		Nets:       []*ir.Net{net("GND", "U2.1", "U1.1")},
	}
	got := 0
	for _, f := range check.Run(check.NewModel(d), Compile(EMMC)) {
		if f.Rule == "emmc-host-incomplete" && f.Subject == "U2" {
			got++
		}
	}
	if got != 11 {
		t.Fatalf("wholly-absent bus: want 11 host-incomplete findings, got %d", got)
	}
}

// emmcWideRail is emmcGood with a REALISTIC rail: +3V3 fans out to a couple of dozen loads instead of
// the two in emmcGood. Everything else is identical, and CMD is still pulled up to it through R1.
//
// That one difference is the whole bug (WS3-108). The generated pull-up check asked
// `reaches(n, rail)`, and the WS3-011 series walk deliberately refuses to cross INTO a net whose
// fan-out exceeds maxWalkFan (16) — a guard that is right for its own purpose, since it stops a walk
// wandering across a plane as though it were a series path. But a pull-up TERMINATES on a rail, and a
// rail is wide almost by definition, so the one destination the rule needs was the one kind of net the
// walk would not enter. Verified on a real board: EMMC_CMD pulled up by R209 to a genuine 1.8V rail
// with 51 pins on it, reported as reaching no rail.
//
// emmcGood keeps a two-pin rail, which is why the suite passed while the rule could not do its job on
// any real design.
func emmcWideRail() *ir.Design {
	d := emmcGood()
	loads := make([]string, 0, 24)
	loads = append(loads, "R1.2", "U2.12")
	for i := 0; i < 22; i++ {
		ref := "C" + itoa(i)
		d.Components = append(d.Components, comps(ref)...)
		loads = append(loads, ref+".1")
	}
	for i, n := range d.Nets {
		if n.Name == "+3V3" {
			d.Nets[i] = net("+3V3", loads...)
		}
	}
	return d
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

// TestEMMCPullupSeesAWideRail is the WS3-108 regression. A signal pulled to a rail through a single
// resistor must read as pulled whatever that rail's fan-out.
func TestEMMCPullupSeesAWideRail(t *testing.T) {
	d := emmcWideRail()
	for _, n := range d.Nets {
		if n.Name == "+3V3" && len(n.Connections) <= 16 {
			t.Fatalf("fixture is not exercising the guard: +3V3 has %d connections, need > 16", len(n.Connections))
		}
	}
	for _, f := range check.Run(check.NewModel(d), Compile(EMMC)) {
		if strings.Contains(f.Rule, "missing-pullup") {
			t.Errorf("CMD is pulled up to +3V3 through R1, but the rule reports %+v", f)
		}
	}
}
