package profiles

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// pcieLike is a completeness profile whose signals share generic serdes suffixes (_TXP/_RXP/...); a
// prefix, when set, is what tells its nets apart from a foreign serdes of the same suffix.
func pcieLike(prefix string) Profile {
	return Profile{Name: "PCIeTest", Signals: []Signal{
		{Name: "PETP", Prefix: prefix, Suffix: "_TXP", Anchor: true},
		{Name: "PETN", Prefix: prefix, Suffix: "_TXN"},
		{Name: "PERP", Prefix: prefix, Suffix: "_RXP"},
		{Name: "PERN", Prefix: prefix, Suffix: "_RXN"},
		{Name: "PERST", Prefix: prefix, Suffix: "_RSTn"},
	}, Requirements: []Requirement{{Type: "signal-missing"}}}
}

// foreignSerdesOnly is a design with NO PCIe — only a UWB serdes that shares the _TXP/_TXN suffixes.
// A suffix-only profile over-matches it (anchors on FIRA_SERDES_TXP and reports the PCIe signals it
// lacks); a prefix keeps the profile off it entirely (WS3-057).
func foreignSerdesOnly() *ir.Design {
	return &ir.Design{Nets: []*ir.Net{
		net("FIRA_SERDES_TXP", "U9.1"),
		net("FIRA_SERDES_TXN", "U9.2"),
	}}
}

func TestSignalPrefixDiscriminatesForeignAnchor(t *testing.T) {
	d := check.NewModel(foreignSerdesOnly())
	// Suffix-only: the _TXP anchor latches onto the foreign serdes and reports PCIe's other signals
	// "missing" — the false positive. (Guards that the prefix is what fixes it, not the design.)
	if fs := check.Run(d, Compile(pcieLike(""))); len(fs) == 0 {
		t.Fatal("suffix-only profile: expected an over-match false positive on the foreign serdes, got none")
	}
	// Prefixed: the anchor requires the PCIE_ prefix too, so no PCIE_ nets means the profile is not
	// in use and reports nothing on the unrelated serdes.
	if fs := check.Run(d, Compile(pcieLike("PCIE_"))); len(fs) != 0 {
		t.Fatalf("prefixed profile: want 0 findings on a design with no PCIe, got %d: %+v", len(fs), fs)
	}
}
