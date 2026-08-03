package builtin

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// specParityFixture is a deliberately messy design touching every twin's machinery: typed
// pins in all directions, every guard attribute, ref-des classes, input diagnostics, diff
// pairs, and I2C nets. Parity does not care which rules fire on it, only that both
// evaluation paths agree, so breadth beats realism.
func specParityFixture() *ir.Design {
	attrs := func(kv ...string) map[string]string {
		m := map[string]string{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return m
	}
	withAttr := func(n *ir.Net, kv ...string) *ir.Net {
		n.Attributes = attrs(kv...)
		return n
	}
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			{Name: "MCU", Pins: []*ir.Pin{
				{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
				{Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},
				{Designator: "3", Direction: ir.PinDirection_PIN_DIRECTION_OUTPUT},
				{Designator: "4", Direction: ir.PinDirection_PIN_DIRECTION_NO_CONNECT},
				{Designator: "5", Direction: ir.PinDirection_PIN_DIRECTION_INOUT},
				// Bare on every placement: unconnected-pin's Go Eval fires on passive pins,
				// and before dirString mapped PASSIVE the twin read them as "unspecified"
				// and skipped — the divergence this pin holds the parity gate to.
				{Designator: "6", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE},
			}},
			{Name: "REG", Pins: []*ir.Pin{
				{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_OUT},
			}},
		}}},
		Components: []*ir.Component{
			{RefDes: "U1", Sections: []*ir.ComponentSection{{PartRef: "MCU", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "p"}},
			{RefDes: "U2", Sections: []*ir.ComponentSection{{PartRef: "MCU", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "p"}},
			{RefDes: "U3", Sections: []*ir.ComponentSection{{PartRef: "MCU", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "p"}}, // pins 2,3,5 bare; 4 is NC but wired
			{RefDes: "REG1", Sections: []*ir.ComponentSection{{PartRef: "REG", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "p"}},
			{RefDes: "C1", Prov: &ir.Provenance{SourceFile: "p"}},
			{RefDes: "R1", Prov: &ir.Provenance{SourceFile: "p"}},
			{RefDes: "F1", Prov: &ir.Provenance{SourceFile: "p"}},
			{RefDes: "TVS1", Prov: &ir.Provenance{SourceFile: "p"}},
			{RefDes: "J1", Prov: &ir.Provenance{SourceFile: "p"}},
			{RefDes: "R9", Prov: &ir.Provenance{SourceFile: "p"}}, // on no net
			{RefDes: "", Prov: &ir.Provenance{SourceFile: "p"}},   // anonymous: skipped by unconnected-component
		},
		Nets: []*ir.Net{
			// power shapes: undriven power-in, driven+decoupled, power-driven flag, bulk-less global rail
			tnet("VDD_BARE", "U1.1"),
			tnet("3V3", "U1.1", "REG1.1", "C1.1"),
			withAttr(tnet("5V", "U2.1"), "power_driven", "true"),
			withAttr(tnet("12V_RAIL", "R1.2"), "global", "true"),
			withAttr(tnet("EXT", "U2.1"), "external", "true"),
			tnet("GND", "U1.1", "U2.1", "C1.2"),
			// pin-direction shapes: all-input float, driver conflict, inout bus
			tnet("FLOAT", "U1.2", "U2.2"),
			tnet("CONFLICT", "U1.3", "U2.3"),
			tnet("BUS", "U1.5", "U2.5"),
			// protection shapes: connector+power_in bare, fused, clamped signal, unclamped signal
			tnet("VIN", "J1.1", "U1.1"),
			tnet("VIN_OK", "J1.2", "U2.1", "F1.1"),
			tnet("SIG_OUT", "J1.3", "U1.3", "TVS1.1"),
			tnet("SIG_BARE", "J1.4", "U1.2"),
			// naming shapes: I2C without and with pull-up, orphan and paired diff nets
			tnet("SDA", "U1.5"),
			tnet("SCL", "U1.5", "R1.1"),
			tnet("LVDS_P", "U1.3"),
			tnet("USB_D+", "U1.3"),
			tnet("USB_D-", "U1.2"),
			// stubs: real, tool-marked, NO_CONNECT-pinned
			tnet("STUB", "R1.1"),
			tnet("unconnected-(U1-Pad4)", "U1.4"),
			tnet("NCNET", "U2.4"),
			// pin-level shapes: U3 partially wired (bare pins 2,3,5), NC pin wired into a real net
			tnet("U3_PWR", "U3.1", "REG1.1"),
			tnet("BADNC", "U3.4", "R1.2"),
			// virtual power-symbol connections (WS1-014): direction rides the attribute
			vnet("VRAIL", vconn("#PWR07", "1", "power_in"), conn("U1.1")),
			vnet("VDRIVE", vconn("#FLG01", "1", "power_out"), vconn("#FLG02", "1", "power_out")),
		},
		InputDiagnostics: &ir.InputDiagnostics{
			DanglingEndpoints: []*ir.DanglingEndpoint{
				{X: 10, Y: -20, Prov: &ir.Provenance{SourceFile: "p"}},
				{X: 0, Y: 0},
			},
			RefDesCollisions: []*ir.RefDesCollision{
				{RefDes: "U7", Instances: []*ir.Provenance{{SourceFile: "a"}, {SourceFile: "b"}}},
				{RefDes: "U8"}, // no instances: prov must come out nil on both paths
			},
		},
	}
}

// rulesByName indexes the registered catalog for the twin tests.
func rulesByName() map[string]*check.Rule {
	out := make(map[string]*check.Rule, len(rules))
	for _, r := range rules {
		out[r.Name] = r
	}
	return out
}

// conn/vconn/vnet build connections for the virtual-power shapes: vconn carries the
// WS1-014 direction attribute a power symbol's pin travels on.
func conn(ref string) *ir.Connection {
	p := strings.SplitN(ref, ".", 2)
	return &ir.Connection{ComponentRef: p[0], PinRef: p[1]}
}

func vconn(ref, pin, dir string) *ir.Connection {
	return &ir.Connection{ComponentRef: ref, PinRef: pin, Attributes: map[string]string{"direction": dir}}
}

func vnet(name string, cs ...*ir.Connection) *ir.Net {
	return &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "p"}, Connections: cs}
}

// TestSpecParity holds every declarative twin to its Go Eval: identical findings, in order,
// over both fixture designs. This is the gate that makes flipping a rule to spec-canonical a
// safe one-line change. It iterates Specs, not Rules: spec-only rules (the matrix rows) have
// no Go side to compare — their Eval IS the interpreter — and every Specs key must name a
// registered rule (no orphan twins).
func TestSpecParity(t *testing.T) {
	byName := rulesByName()
	for _, tc := range []struct {
		name string
		d    *ir.Design
	}{
		{"ruleFixture", ruleFixture()},
		{"parityFixture", specParityFixture()},
	} {
		m := check.NewModel(tc.d)
		for name, spec := range specs {
			r := byName[name]
			if r == nil {
				t.Errorf("Specs[%q] names no registered rule (orphan twin)", name)
				continue
			}
			got, want := spec.Eval(m), r.Eval(m)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s/%s: spec findings diverge\n spec: %+v\n   go: %+v", tc.name, name, got, want)
			}
		}
	}
}

// TestSpecMetadata asserts each rule's hand-written Reads and Primitives equal its twin's
// derived ones (as sets): the C14 metadata is now checkable against the body instead of
// trusted prose. A failure means either the rule's tags drifted or the twin diverged.
func TestSpecMetadata(t *testing.T) {
	byName := rulesByName()
	for name, spec := range specs {
		r := byName[name]
		if r == nil {
			continue // parity test reports the orphan
		}
		if err := spec.Validate(); err != nil {
			t.Errorf("%s: invalid spec: %v", r.Name, err)
			continue
		}
		if got, want := spec.DerivedReads(), sortedCopySet(r.Reads); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: derived reads %v != declared %v", r.Name, got, want)
		}
		if got, want := spec.DerivedPrimitives(), sortedCopySet(r.Primitives); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: derived primitives %v != declared %v", r.Name, got, want)
		}
	}
}

func sortedCopySet(xs []string) []string {
	out := append([]string(nil), xs...)
	sort.Strings(out)
	return out
}

// TestSpecRule covers the constructor path a spec-authored rule (no Go twin) takes: metadata
// filled from derivation, Eval bound, and a panic on an invalid spec.
func TestSpecRule(t *testing.T) {
	s := &check.Spec{
		Over:    "nets",
		Where:   check.Cmp{L: check.Fact{Name: "net.pin_count"}, Op: "==", R: check.Lit{V: 0}},
		Message: "net {net.names:q} has no connections",
	}
	r := s.Rule(check.Rule{Name: "empty-net", Severity: "info"})
	if !reflect.DeepEqual(r.Reads, []string{"net.names", "net.pin_count"}) {
		t.Errorf("derived reads = %v", r.Reads)
	}
	if !reflect.DeepEqual(r.Primitives, []string{"count", "select"}) {
		t.Errorf("derived primitives = %v", r.Primitives)
	}
	fs := r.Eval(check.NewModel(&ir.Design{Nets: []*ir.Net{tnet("EMPTY"), tnet("OK", "R1.1")}}))
	if len(fs) != 1 || fs[0].Subject != "EMPTY" || fs[0].Message != `net "EMPTY" has no connections` {
		t.Errorf("eval = %+v", fs)
	}

	defer func() {
		if recover() == nil {
			t.Error("Rule() did not panic on an invalid spec")
		}
	}()
	(&check.Spec{Over: "nets", Where: check.IsTrue{T: check.Fact{Name: "no.such.fact"}}}).Rule(check.Rule{Name: "bad"})
}

// TestSpecValidate exercises each rejection the validator owes an author.
func TestSpecValidate(t *testing.T) {
	cases := []struct {
		name string
		s    *check.Spec
	}{
		{"unknown over", &check.Spec{Over: "sheets"}},
		{"unknown fact", &check.Spec{Over: "nets", Where: check.IsTrue{T: check.Fact{Name: "net.bogus"}}}},
		{"unbound var", &check.Spec{Over: "nets", Where: check.IsTrue{T: check.Var{Name: "x"}}}},
		{"unregistered func", &check.Spec{Over: "nets", Where: check.IsTrue{T: check.Call{Fn: "nope"}}}},
		{"unknown collection", &check.Spec{Over: "nets", Where: check.ExistsIn{Over: "net.pins"}}},
		{"unknown collection in count", &check.Spec{Over: "nets", Where: check.Cmp{L: check.CountOf{Over: "net.pins"}, Op: ">", R: check.Lit{V: 0}}}},
		{"bad operator", &check.Spec{Over: "nets", Where: check.Cmp{L: check.Lit{V: 1}, Op: "~=", R: check.Lit{V: 1}}}},
		{"bad pattern", &check.Spec{Over: "nets", Where: check.Match{T: check.Fact{Name: "net.names"}, Pattern: "("}}},
		{"bad placeholder", &check.Spec{Over: "nets", Message: "{nope}"}},
		{"bad node in let", &check.Spec{Over: "nets", Let: map[string]check.Term{"x": check.Fact{Name: "net.bogus"}}}},
	}
	for _, tc := range cases {
		if err := tc.s.Validate(); err == nil {
			t.Errorf("%s: Validate() accepted an invalid spec", tc.name)
		}
	}
}

// TestSpecScopeRestore guards eachMember's scope save/restore: a quantifier nested in
// another quantifier's Where must not leave its member bound when the outer one resumes.
// CONFLICT has two output pins; the outer count matches members whose direction equals the
// direction of at least one member (trivially true per member), which evaluates an inner
// quantifier per outer member and only totals correctly if the outer binding is restored.
func TestSpecScopeRestore(t *testing.T) {
	s := &check.Spec{
		Over: "nets",
		Where: check.Cmp{
			L: check.CountOf{Over: "net.connections", Where: check.And{Xs: []check.Expr{
				check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "output"}},
				check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "output"}}},
			}}},
			Op: "==", R: check.Lit{V: 2},
		},
		Message: "x",
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	m := check.NewModel(specParityFixture())
	var subjects []string
	for _, f := range s.Eval(m) {
		subjects = append(subjects, f.Subject)
	}
	if !reflect.DeepEqual(subjects, []string{"CONFLICT"}) {
		t.Errorf("subjects = %v, want [CONFLICT]", subjects)
	}
}
