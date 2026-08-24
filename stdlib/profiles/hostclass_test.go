package profiles

import (
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// flashProfile binds its host ONLY by datasheet device class, so nothing in the design declares the
// interface. Signals use the same suffix convention the built-ins do.
var flashProfile = Profile{
	Name:      "TESTFLASH",
	HostClass: "crystal",
	Signals: []Signal{
		{Name: "CS", Suffix: "_CS", Anchor: true},
		{Name: "SCLK", Suffix: "_SCLK"},
		{Name: "IO0", Suffix: "_IO0"},
	},
	Requirements: []Requirement{{Type: "host-incomplete"}},
}

// flashDesign places U1 (the flash, by MPN) wired to only two of the three signals, so a host-anchored
// completeness check has something to report. Nothing carries an interface attribute.
func flashDesign() *ir.Design {
	return &ir.Design{
		Components: []*ir.Component{
			{RefDes: "U1", Attributes: map[string]string{"MPN": "ACME-FLASH"}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U2", Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			net("BUS_CS", "U1.1", "U2.1"),
			net("BUS_SCLK", "U1.2", "U2.2"),
		},
	}
}

// flashSpec seeds a datasheet whose device_class is written the way a VENDOR would write it ("XTAL"),
// against a profile declaring the canonical key ("crystal"), so the folding is actually exercised
// rather than trivially matching.
func flashSpec() param.ParamSet {
	return param.ParamSet{"ACME-FLASH": &parampb.PartSpec{
		Mpn:         "ACME-FLASH",
		DeviceClass: "XTAL",
		Docs:        []*parampb.SourceDoc{{Title: "ACME-FLASH datasheet"}},
	}}
}

// TestHostClassIdentifiesHostInAllReaders is the WS3-044 acceptance, and the reason the host test moved
// to one shared predicate. "Is this component the host" has four consumers, and before this change each
// tested the declared attribute on its own. Extending only the datalog rule would have produced an
// interface whose host path emits findings while the review gate reports that same path as unable to
// evaluate — failing and not-automated at once. So the assertion is that they AGREE, not merely that
// the rule fires.
func TestHostClassIdentifiesHostInAllReaders(t *testing.T) {
	m := check.NewModelWithParams(flashDesign(), nil, flashSpec())

	if !flashProfile.HasHost() {
		t.Fatal("a class-bound profile must report HasHost")
	}
	if !HostDeclared(m, flashProfile) {
		t.Error("HostDeclared: the datasheet identifies U1, so the host path can evaluate")
	}
	if nets := Nets(m, flashProfile); len(nets) == 0 {
		t.Error("Nets: the review scope must be host-anchored, not fall back to convention")
	}
	r := flashProfile.hostIncompleteRule()
	if r == nil {
		t.Fatal("hostIncompleteRule compiled to nothing for a class-bound profile")
	}
	var got []string
	for _, f := range r.Findings(m) {
		got = append(got, check.EntityRef(f.Subject))
	}
	if len(got) != 1 || got[0] != "U1" {
		t.Errorf("host-incomplete findings = %v, want one on U1 (IO0 is absent)", got)
	}
}

// TestHostClassSilentWithoutParams: the class path is datasheet evidence, so with no seeded set there
// is no evidence and the profile finds no host. It must stay quiet rather than fall through to a
// guess, and HostDeclared must say the path could not evaluate so the review reads not-automated
// instead of a hollow pass (WS3-090).
func TestHostClassSilentWithoutParams(t *testing.T) {
	m := check.NewModel(flashDesign())

	if HostDeclared(m, flashProfile) {
		t.Error("HostDeclared must be false with no params: the datasheet evidence is absent")
	}
	if r := flashProfile.hostIncompleteRule(); r != nil {
		if f := r.Findings(m); len(f) != 0 {
			t.Errorf("want no findings without a seeded param set, got %+v", f)
		}
	}
}

// TestHostClassNormalizesBothSides pins the reason the relation stopped projecting device_class
// verbatim. The seeded spec says "XTAL" and the profile declares "crystal"; an exact string match would
// find no host and the whole check would go quiet, which is the failure shape that looks like a clean
// board.
//
// The folding covers what the WS10-015 vocabulary KNOWS: for those, case and vendor aliases both fold
// ("XTAL" and "Crystal" each reach crystal, "TCXO" reaches oscillator). A class the vocabulary does not
// recognize passes through UNCHANGED, including its case — so "LDO" and "ldo" are different strings and
// would not match. Two classes in the seeded corpus ("ldo", "mcu") are in exactly that position, which
// is the sharp edge here and part of why no built-in declares a HostClass yet.
func TestHostClassNormalizesBothSides(t *testing.T) {
	m := check.NewModelWithParams(flashDesign(), nil, flashSpec())
	u1 := m.Components()[0]
	if !flashProfile.IsHost(m, u1) {
		t.Errorf("IsHost: %q must match a profile declaring %q", "XTAL", flashProfile.HostClass)
	}
	other := Profile{Name: "X", HostClass: "capacitor"}
	if other.IsHost(m, u1) {
		t.Error("IsHost matched an unrelated device class")
	}
}

// TestAttributeHostUnaffected: the declared-attribute path is unchanged by the union, including for a
// profile that binds only by attribute and runs on a model with no params at all.
func TestAttributeHostUnaffected(t *testing.T) {
	p := Profile{Name: "ATTR", HostAttrKey: "interface", HostAttrVal: "TESTFLASH",
		Signals: flashProfile.Signals, Requirements: flashProfile.Requirements}
	d := flashDesign()
	d.Components[0].Attributes["interface"] = "TESTFLASH"
	m := check.NewModel(d)

	if !HostDeclared(m, p) {
		t.Error("an attribute-declared host must still be found with no params")
	}
	if !p.IsHost(m, m.Components()[0]) {
		t.Error("IsHost must still honour the attribute form")
	}
}
