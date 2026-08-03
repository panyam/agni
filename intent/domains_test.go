package intent

import (
	"testing"

	"github.com/panyam/agni/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestVoltageDomainPassesWhenRailsMatch(t *testing.T) {
	decl := declOf(t, "name: I\nvoltage_domains:\n  - {name: io_3v3, nominal: 3.3, rails: [3V3]}")
	d := &ir.Design{Nets: []*ir.Net{{Name: "3V3"}, {Name: "GND"}}}
	if fs := check.Run(check.NewModel(d), Compile(decl)); len(fs) != 0 {
		t.Errorf("a rail whose name matches its declared nominal must not fire, got %+v", fs)
	}
}

func TestVoltageDomainFiresOnAbsentRail(t *testing.T) {
	decl := declOf(t, "name: I\nvoltage_domains:\n  - {name: io_3v3, nominal: 3.3, rails: [3V3, VDD_IO]}")
	// Only 3V3 exists; the declared VDD_IO rail is absent -> one finding.
	d := &ir.Design{Nets: []*ir.Net{{Name: "3V3"}}}
	fs := check.Run(check.NewModel(d), Compile(decl))
	if len(fs) != 1 || fs[0].Rule != RuleVoltageDomain || fs[0].Subject != "VDD_IO" {
		t.Fatalf("want one absent-rail finding for VDD_IO, got %+v", fs)
	}
}

func TestVoltageDomainFiresOnWrongDomain(t *testing.T) {
	// A rail named 5V0 is declared to be in the 3.3V domain: its name declares a different voltage
	// than the domain, so it is on the wrong domain and must fire.
	decl := declOf(t, "name: I\nvoltage_domains:\n  - {name: io_3v3, nominal: 3.3, rails: [5V0]}")
	d := &ir.Design{Nets: []*ir.Net{{Name: "5V0"}}}
	fs := check.Run(check.NewModel(d), Compile(decl))
	if len(fs) != 1 || fs[0].Subject != "5V0" {
		t.Fatalf("want one wrong-domain finding for 5V0, got %+v", fs)
	}
}

func TestVoltageDomainSkipsUnparseableRailName(t *testing.T) {
	// VDD_CORE's name encodes no voltage token, so the nominal is unverifiable; presence is confirmed
	// and the rule refuses to guess (no finding).
	decl := declOf(t, "name: I\nvoltage_domains:\n  - {name: core, nominal: 0.8, rails: [VDD_CORE]}")
	d := &ir.Design{Nets: []*ir.Net{{Name: "VDD_CORE"}}}
	if fs := check.Run(check.NewModel(d), Compile(decl)); len(fs) != 0 {
		t.Errorf("an unparseable rail name should verify presence only, got %+v", fs)
	}
}
