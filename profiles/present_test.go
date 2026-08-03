package profiles

import (
	"testing"

	"github.com/panyam/agni/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func nets(names ...string) []*ir.Net {
	out := make([]*ir.Net, len(names))
	for i, n := range names {
		out[i] = &ir.Net{Name: n}
	}
	return out
}

// TestInUseHonoursPrefix is the WS3-090 core: InUse matches the rules' netMatch (suffix AND prefix),
// so a prefix-discriminated profile is NOT in use just because foreign nets share a bare suffix. The
// old suffix-only presence read these as "present", which false-passed an absent interface.
func TestInUseHonoursPrefix(t *testing.T) {
	pcie := Profile{Name: "PCIe", Signals: []Signal{
		{Name: "TX", Prefix: "PCIE_", Suffix: "_TX"},
		{Name: "RX", Prefix: "PCIE_", Suffix: "_RX"},
	}}
	// Foreign nets share the bare suffix but not the PCIE_ prefix -> not in use.
	if InUse(check.NewModel(&ir.Design{Nets: nets("LIN_TX", "CAN_RX")}), pcie) {
		t.Error("foreign same-suffix nets must not read as PCIe in use (the false-pass bug)")
	}
	// The interface's own prefixed nets -> in use.
	if !InUse(check.NewModel(&ir.Design{Nets: nets("PCIE_TX", "PCIE_RX")}), pcie) {
		t.Error("PCIE_-prefixed nets should read as in use")
	}
	// One matching signal is below the two-distinct floor.
	if InUse(check.NewModel(&ir.Design{Nets: nets("PCIE_TX")}), pcie) {
		t.Error("a lone signal must not read as in use")
	}
}

// TestInUseSuffixOnlyUnchanged: a profile with no prefix still matches by suffix, so the fix does not
// regress suffix-based profiles (the common case).
func TestInUseSuffixOnlyUnchanged(t *testing.T) {
	can := Profile{Name: "CAN", Signals: []Signal{{Name: "H", Suffix: "_CANH"}, {Name: "L", Suffix: "_CANL"}}}
	if !InUse(check.NewModel(&ir.Design{Nets: nets("CAN_CANH", "CAN_CANL")}), can) {
		t.Error("two matching suffix nets should read as in use")
	}
	if InUse(check.NewModel(&ir.Design{Nets: nets("FOO", "BAR")}), can) {
		t.Error("no matching nets must not read as in use")
	}
}

// TestHostDeclared: true only when a component carries the host attribute; false with no annotation and
// for a profile with no host binding.
func TestHostDeclared(t *testing.T) {
	lin := Profile{Name: "LIN", HostAttrKey: "interface", HostAttrVal: "LIN_HOUSE",
		Signals: []Signal{{Name: "TX", Suffix: "_TX"}}}
	withHost := &ir.Design{Components: []*ir.Component{{RefDes: "U1", Attributes: map[string]string{"interface": "LIN_HOUSE"}}}}
	if !HostDeclared(check.NewModel(withHost), lin) {
		t.Error("a component declaring the host attribute should read host-declared")
	}
	if HostDeclared(check.NewModel(&ir.Design{Components: []*ir.Component{{RefDes: "U1"}}}), lin) {
		t.Error("no component declaring the host must read host NOT declared")
	}
	noHost := Profile{Name: "CAN", Signals: []Signal{{Name: "H", Suffix: "_CANH"}}}
	if HostDeclared(check.NewModel(withHost), noHost) {
		t.Error("a profile with no host binding is never host-declared")
	}
}
