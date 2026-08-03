package profiles

import (
	"strings"
	"testing"

	"github.com/panyam/agni/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func net(name string, conns ...string) *ir.Net {
	n := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}}
	for _, c := range conns {
		p := strings.SplitN(c, ".", 2)
		n.Connections = append(n.Connections, &ir.Connection{ComponentRef: p[0], PinRef: p[1]})
	}
	return n
}

func comps(refs ...string) []*ir.Component {
	var out []*ir.Component
	for _, r := range refs {
		out = append(out, &ir.Component{RefDes: r, Prov: &ir.Provenance{SourceFile: "t"}})
	}
	return out
}

// spinorGood: all six signals wired end-to-end; CS pulled up to +3V3 through R1 (a resistor the
// reach walk crosses). No profile finding.
func spinorGood() *ir.Design {
	return &ir.Design{
		Components: comps("U1", "U2", "R1"),
		Nets: []*ir.Net{
			net("SPI_CS", "U1.1", "U2.1", "R1.1"),
			net("SPI_SCLK", "U1.2", "U2.2"),
			net("SPI_IO0", "U1.3", "U2.3"),
			net("SPI_IO1", "U1.4", "U2.4"),
			net("SPI_IO2", "U1.5", "U2.5"),
			net("SPI_IO3", "U1.6", "U2.6"),
			net("+3V3", "R1.2", "U2.7"),
		},
	}
}

// spinorBroken: IO2 net absent (missing), no pull-up resistor on CS (missing-pullup), and SCLK on a
// single-pin net (dangling).
func spinorBroken() *ir.Design {
	return &ir.Design{
		Components: comps("U1", "U2"),
		Nets: []*ir.Net{
			net("SPI_CS", "U1.1", "U2.1"),
			net("SPI_SCLK", "U1.2"),
			net("SPI_IO0", "U1.3", "U2.3"),
			net("SPI_IO1", "U1.4", "U2.4"),
			net("SPI_IO3", "U1.6", "U2.6"),
			net("+3V3", "U2.7"),
		},
	}
}

func TestSPINORSilent(t *testing.T) {
	if fs := check.Run(check.NewModel(spinorGood()), Compile(SPINOR)); len(fs) != 0 {
		t.Fatalf("good SPI-NOR bus: want 0 findings, got %d: %+v", len(fs), fs)
	}
}

func TestSPINORFires(t *testing.T) {
	got := map[string]check.Finding{}
	for _, f := range check.Run(check.NewModel(spinorBroken()), Compile(SPINOR)) {
		got[f.Rule] = f
	}
	if len(got) != 3 {
		t.Fatalf("want 3 distinct rules firing, got %d: %+v", len(got), got)
	}
	if f := got["spi_nor-signal-missing"]; f.Subject != "SPI_CS" || !strings.Contains(f.Message, "IO2") {
		t.Errorf("signal-missing: want anchor SPI_CS + IO2, got %+v", f)
	}
	if f := got["spi_nor-missing-pullup"]; f.Subject != "SPI_CS" {
		t.Errorf("missing-pullup: want SPI_CS, got %+v", f)
	}
	if f := got["spi_nor-signal-dangling"]; f.Subject != "SPI_SCLK" {
		t.Errorf("signal-dangling: want SPI_SCLK, got %+v", f)
	}
}

// Compile produces the four SPI-NOR rules (signal-missing convention, host-incomplete, missing-pullup,
// signal-dangling), registered under "profile".
func TestCompileAndRegistered(t *testing.T) {
	if got := len(Compile(SPINOR)); got != 4 {
		t.Fatalf("Compile(SPINOR): want 4 rules, got %d", got)
	}
	found := 0
	for _, r := range check.DefaultCatalog().Rules() {
		if strings.HasPrefix(r.Name, "profile/spi_nor-") {
			found++
		}
	}
	if found != 4 {
		t.Fatalf(`want 4 "profile/spi_nor-*" rules in DefaultCatalog, got %d`, found)
	}
}

func flashHost(ref string) *ir.Component {
	return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
		Attributes: map[string]string{"interface": "SPI_NOR"}}
}

// A component declaring interface=SPI_NOR with IO2 absent: the host-anchored path fires ONCE on the
// host (IO2), and the convention path is suppressed (no double-report); CS is pulled up and all nets
// are 2-pin, so no other rule fires.
func TestHostIncomplete(t *testing.T) {
	d := &ir.Design{
		Components: append(comps("U1", "R1"), flashHost("U2")),
		Nets: []*ir.Net{
			net("SPI_CS", "U1.1", "U2.1", "R1.1"),
			net("SPI_SCLK", "U1.2", "U2.2"),
			net("SPI_IO0", "U1.3", "U2.3"),
			net("SPI_IO1", "U1.4", "U2.4"),
			net("SPI_IO3", "U1.6", "U2.6"),
			net("+3V3", "R1.2", "U2.7"),
		},
	}
	fs := check.Run(check.NewModel(d), Compile(SPINOR))
	if len(fs) != 1 {
		t.Fatalf("want exactly 1 finding (host-incomplete IO2, convention suppressed), got %d: %+v", len(fs), fs)
	}
	if f := fs[0]; f.Rule != "spi_nor-host-incomplete" || f.Subject != "U2" || !strings.Contains(f.Message, "IO2") {
		t.Fatalf("want host-incomplete on U2/IO2, got %+v", f)
	}
}

// A host that declares the interface but is wired to none of its signals: host-anchored completeness
// flags every required signal (wholly-absent detection the convention path cannot do).
func TestHostWhollyAbsent(t *testing.T) {
	d := &ir.Design{
		Components: append(comps("U1"), flashHost("U2")),
		Nets:       []*ir.Net{net("GND", "U2.1", "U1.1")},
	}
	got := 0
	for _, f := range check.Run(check.NewModel(d), Compile(SPINOR)) {
		if f.Rule == "spi_nor-host-incomplete" && f.Subject == "U2" {
			got++
		}
	}
	if got != 6 {
		t.Fatalf("wholly-absent bus: want 6 host-incomplete findings, got %d", got)
	}
}
