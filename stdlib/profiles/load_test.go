package profiles

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// A naming map re-binds a core profile's signals to a project's own suffixes: it inherits the core
// structure/requirements and fires on a design named that way, where the core default (looking for
// the standard suffixes) is silent. This is the structure-vs-naming split (WS3-054).
func TestNamingMapRemapsSuffixes(t *testing.T) {
	p, err := Load(strings.NewReader(
		"override: SPI_NOR\nsuffixes: {CS: _XCS, SCLK: _XSCLK, IO0: _XIO0, IO1: _XIO1, IO2: _XIO2, IO3: _XIO3}\n"))
	if err != nil {
		t.Fatalf("Load naming map: %v", err)
	}
	if p.Name != "SPI_NOR" || p.anchorSuffix() != "_XCS" || len(p.Requirements) != 4 {
		t.Fatalf("remap wrong: name=%q anchor=%q reqs=%d", p.Name, p.anchorSuffix(), len(p.Requirements))
	}
	// A design in the project's naming (IO2 absent). Core SPI_NOR (wants _CS/_IO2) is silent; the
	// remapped profile fires signal-missing on the anchor.
	d := &ir.Design{
		Components: comps("U1", "U2"),
		Nets: []*ir.Net{
			net("BUS_XCS", "U1.1", "U2.1"), net("BUS_XSCLK", "U1.2", "U2.2"),
			net("BUS_XIO0", "U1.3", "U2.3"), net("BUS_XIO1", "U1.4", "U2.4"),
			net("BUS_XIO3", "U1.6", "U2.6"),
		},
	}
	if fs := check.Run(check.NewModel(d), Compile(SPINOR)); len(fs) != 0 {
		t.Fatalf("core SPI_NOR should be silent on the project naming, got %d: %+v", len(fs), fs)
	}
	fired := false
	for _, f := range check.Run(check.NewModel(d), Compile(p)) {
		if f.Rule == "spi_nor-signal-missing" && strings.Contains(f.Message, "IO2") {
			fired = true
		}
	}
	if !fired {
		t.Error("remapped profile should fire signal-missing (IO2) on the project-named design")
	}
}

func TestNamingMapErrors(t *testing.T) {
	cases := map[string]string{
		"unknown profile": "override: NOPE\nsuffixes: {CS: _X}",
		"unknown role":    "override: SPI_NOR\nsuffixes: {NOTAROLE: _X}",
	}
	for name, y := range cases {
		if _, err := Load(strings.NewReader(y)); err == nil {
			t.Errorf("%s: want error, got nil", name)
		}
	}
}

// TestBuiltinsMatchYAML holds each built-in's Go literal identical to its embedded YAML declaration,
// so the two forms kept side by side (per the TODO in spinor.go/emmc.go/can.go, for comparison until
// the YAML form becomes authoritative) provably cannot drift. A mismatch here means the YAML and the
// Go value have diverged.
func TestBuiltinsMatchYAML(t *testing.T) {
	for _, c := range []struct {
		name string
		lit  Profile
		yaml []byte
	}{
		{"SPI_NOR", SPINOR, spinorYAML},
		{"eMMC", EMMC, emmcYAML},
		{"CAN", CAN, canYAML},
		{"LIN", LIN, linYAML},
		{"A2B", A2B, a2bYAML},
		{"PCIe", PCIE, pcieYAML},
		{"SGMII", SGMII, sgmiiYAML},
	} {
		if got := mustParse(c.yaml); !reflect.DeepEqual(got, c.lit) {
			t.Errorf("%s: YAML declaration != Go literal\n yaml: %+v\n  go:  %+v", c.name, got, c.lit)
		}
	}
}

// The embedded built-in YAML round-trips to the expected declaration: Parse(spinorYAML) yields the
// six SPI-NOR signals with CS as the pulled-up anchor and the four requirements — the same value the
// old Go literal held, which the behavioral suite (TestSPINORFires/Silent/...) confirms compiles
// identically.
func TestParseBuiltinShape(t *testing.T) {
	p, err := Parse(spinorYAML)
	if err != nil {
		t.Fatalf("Parse(spinorYAML): %v", err)
	}
	if p.Name != "SPI_NOR" || len(p.Signals) != 6 || len(p.Requirements) != 4 {
		t.Fatalf("got name=%q signals=%d reqs=%d", p.Name, len(p.Signals), len(p.Requirements))
	}
	if p.anchorSuffix() != "_CS" {
		t.Errorf("anchor suffix: want _CS, got %q", p.anchorSuffix())
	}
	if !p.Signals[0].PullUp || !p.Signals[0].Anchor {
		t.Errorf("CS should be pullup+anchor, got %+v", p.Signals[0])
	}
}

func TestLoadValid(t *testing.T) {
	y := `
name: MYBUS
host: {attr: interface, value: MYBUS}
signals:
  - {name: CLK, suffix: _CLK, anchor: true}
  - {name: DAT, suffix: _DAT}
requirements:
  - {type: signal-missing}
  - {type: signal-dangling}
`
	p, err := Load(strings.NewReader(y))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := len(Compile(p)); got != 2 {
		t.Fatalf("Compile(loaded): want 2 rules, got %d", got)
	}
}

// An overlay can author a glob- or regex-matched signal in YAML (WS3-057), which is what lets a
// project whose bus identity is the PREFIX and whose suffix is shared with a foreign bus declare an
// interface at all. The compiled rules carry the pattern through, so the profile stays off CAN.
func TestLoadPatternSignals(t *testing.T) {
	y := `
name: ETHX
signals:
  - {name: H, glob: "ETH_SW*_H", anchor: true}
  - {name: L, regex: '^ETH_SW\d+_P\d+_._L$'}
requirements:
  - {type: signal-dangling}
`
	p, err := Load(strings.NewReader(y))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Signals[0].Glob != "ETH_SW*_H" || p.Signals[1].Regex == "" {
		t.Fatalf("matchers did not survive the load: %+v", p.Signals)
	}
	if got := len(Compile(p)); got != 1 {
		t.Fatalf("Compile(loaded): want 1 rule, got %d", got)
	}
	d := &ir.Design{Components: comps("U1", "U2"), Nets: []*ir.Net{
		net("ETH_SW1_P1_A_H", "U1.1", "U2.1"),
		net("ETH_SW1_P1_A_L", "U1.2"), // in use, and this one is dangling
		net("CAN_00_L", "U3.1"),       // foreign, also single-pin: must NOT be reported
	}}
	fs := check.Run(check.NewModel(d), Compile(p))
	if len(fs) != 1 || fs[0].Subject != "ETH_SW1_P1_A_L" {
		t.Fatalf("want only the ETH _L net reported dangling, got %+v", fs)
	}
}

// A naming map REPLACES a signal's matcher rather than adding a second form to it: remapping the
// suffix of a glob-matched signal must not leave both declared, which is not a matcher at all.
func TestNamingMapReplacesPatternMatcher(t *testing.T) {
	core := Profile{Name: "GlobBus", Signals: []Signal{{Name: "H", Glob: "ETH_SW*_H", Anchor: true}}}
	got := applyNamingMap(core, map[string]string{"H": "_HOUSE_H"})
	if err := validateSignalMatcher(got.Signals[0]); err != nil {
		t.Fatalf("remapped signal should be a sound matcher: %v", err)
	}
	if got.Signals[0].Glob != "" || got.Signals[0].Suffix != "_HOUSE_H" {
		t.Errorf("remap should clear the glob and set the suffix, got %+v", got.Signals[0])
	}
}

// Load reports an unknown requirement type with the list of known types (teaching error), the surface
// a customer authoring an overlay profile hits on a typo.
func TestLoadUnknownRequirementTeaches(t *testing.T) {
	y := `
name: MYBUS
signals: [{name: CLK, suffix: _CLK, anchor: true}]
requirements: [{type: signl-missing}]
`
	_, err := Load(strings.NewReader(y))
	if err == nil {
		t.Fatal("want error for unknown requirement type")
	}
	if !strings.Contains(err.Error(), "signl-missing") || !strings.Contains(err.Error(), "signal-missing") {
		t.Errorf("error should name the bad type and list known types, got: %v", err)
	}
}

func TestParseValidationErrors(t *testing.T) {
	cases := map[string]string{
		"missing name":  "signals: [{name: CLK, suffix: _CLK}]",
		"no signals":    "name: X",
		"signal fields": "name: X\nsignals: [{name: CLK}]",
		"two anchors":   "name: X\nsignals: [{name: A, suffix: _A, anchor: true}, {name: B, suffix: _B, anchor: true}]",
		// WS3-057 matcher validation: a signal declares exactly one sound form.
		"two matcher forms": "name: X\nsignals: [{name: A, suffix: _A, glob: 'A*'}]",
		"bad regex":         "name: X\nsignals: [{name: A, regex: '^ETH_(SW'}]",
		"universal glob":    "name: X\nsignals: [{name: A, glob: '*'}]",
		"universal regex":   "name: X\nsignals: [{name: A, regex: '.*'}]",
	}
	for name, y := range cases {
		if _, err := Parse([]byte(y)); err == nil {
			t.Errorf("%s: want error, got nil", name)
		}
	}
}

// LoadDir loads every *.yaml in a directory (the --profile-path path); Source compiles them into a
// catalog-splicable RuleSource.
func TestLoadDirAndSource(t *testing.T) {
	dir := t.TempDir()
	write := func(fn, body string) {
		if err := os.WriteFile(filepath.Join(dir, fn), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.yaml", "name: BUSA\nsignals: [{name: CLK, suffix: _ACLK, anchor: true}, {name: D, suffix: _AD}]\nrequirements: [{type: signal-dangling}]\n")
	write("b.yml", "name: BUSB\nsignals: [{name: CLK, suffix: _BCLK, anchor: true}, {name: D, suffix: _BD}]\nrequirements: [{type: signal-dangling}]\n")
	write("notes.txt", "ignored")
	ps, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(ps) != 2 {
		t.Fatalf("want 2 profiles, got %d", len(ps))
	}
	if src := Source("test", ps); len(src.Rules()) != 2 {
		t.Fatalf("Source: want 2 rules (one dangling per profile), got %d", len(src.Rules()))
	}
	// sanity: the compiled source is usable by the checker.
	_ = check.NewSource
}

// TestSignalMissingRequiresAnchor (WS3-099): a profile declaring the convention completeness
// requirement without a usable anchor compiles it to NOTHING, silently. Paired with any requirement
// that does compile (signal-dangling), the item then runs clean and scores a pass while the check the
// author asked for never existed — the same false-pass shape, arriving through author error rather
// than design state. Both entry points reject it: Parse with a teaching error, Compile with a panic.
func TestSignalMissingRequiresAnchor(t *testing.T) {
	const noAnchor = `
name: NOANCHOR
signals:
  - {name: A, suffix: _A}
  - {name: B, suffix: _B}
requirements:
  - {type: signal-missing}
`
	if _, err := Parse([]byte(noAnchor)); err == nil {
		t.Error("Parse must reject signal-missing with no anchor signal")
	} else if !strings.Contains(err.Error(), "anchor") {
		t.Errorf("want an error naming the anchor, got %v", err)
	}
	// An anchor with no OTHER signal is the same silent drop: there is nothing left to report missing.
	const anchorOnly = `
name: ANCHORONLY
signals:
  - {name: A, suffix: _A, anchor: true}
requirements:
  - {type: signal-missing}
`
	if _, err := Parse([]byte(anchorOnly)); err == nil {
		t.Error("Parse must reject signal-missing with no non-anchor signal to check")
	}
	// A well-formed profile still parses, so the guard does not reject the normal shape.
	const ok = `
name: FINE
signals:
  - {name: A, suffix: _A, anchor: true}
  - {name: B, suffix: _B}
requirements:
  - {type: signal-missing}
`
	if _, err := Parse([]byte(ok)); err != nil {
		t.Errorf("a profile with an anchor and a second signal must parse: %v", err)
	}
	// A profile declaring NO completeness requirement needs no anchor.
	const danglingOnly = `
name: DANGONLY
signals:
  - {name: A, suffix: _A}
requirements:
  - {type: signal-dangling}
`
	if _, err := Parse([]byte(danglingOnly)); err != nil {
		t.Errorf("a profile without signal-missing needs no anchor: %v", err)
	}
	// The Go-literal path panics, the same posture Compile takes for an unsound matcher.
	defer func() {
		if recover() == nil {
			t.Error("Compile must panic on signal-missing with no anchor")
		}
	}()
	Compile(Profile{Name: "GOLIT", Signals: []Signal{{Name: "A", Suffix: "_A"}, {Name: "B", Suffix: "_B"}},
		Requirements: []Requirement{{Type: "signal-missing"}}})
}

// A requirement whose params are missing is a customer-authoring error, so it must fail at LOAD with
// a teaching error rather than panicking deep inside Compile (WS3-047). termination is the built-in
// with params: it bridges two named net suffixes, and without them its generated datalog would match
// on the empty suffix, i.e. every net.
func TestLoadTerminationMissingParamTeaches(t *testing.T) {
	base := "name: MYBUS\nsignals: [{name: H, suffix: _H, anchor: true}, {name: L, suffix: _L}]\n"
	for name, req := range map[string]string{
		"no params":  "requirements: [{type: termination}]\n",
		"no low":     "requirements: [{type: termination, params: {high: _H}}]\n",
		"no high":    "requirements: [{type: termination, params: {low: _L}}]\n",
		"empty low":  "requirements: [{type: termination, params: {high: _H, low: ''}}]\n",
		"typo'd key": "requirements: [{type: termination, params: {high: _H, lo: _L}}]\n",
	} {
		_, err := Load(strings.NewReader(base + req))
		if err == nil {
			t.Errorf("%s: want a load error, got nil", name)
			continue
		}
		// The error has to name the requirement type and the params it wanted, or it teaches nothing.
		for _, want := range []string{"termination", "high", "low"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error should mention %q, got: %v", name, want, err)
			}
		}
	}
}

// The same rejection reaches the actual --profile-path surface, not just the io.Reader one.
func TestLoadDirRejectsBadRequirementParams(t *testing.T) {
	dir := t.TempDir()
	body := "name: MYBUS\nsignals: [{name: H, suffix: _H, anchor: true}, {name: L, suffix: _L}]\nrequirements: [{type: termination, params: {high: _H}}]\n"
	if err := os.WriteFile(filepath.Join(dir, "mybus.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir should reject a profile whose requirement params are incomplete")
	}
	if !strings.Contains(err.Error(), "mybus.yaml") || !strings.Contains(err.Error(), "termination") {
		t.Errorf("error should name the file and the requirement, got: %v", err)
	}
}

// A Go-literal profile bypasses Load, so Compile keeps its own gate — the same twin posture the file
// already takes for an unsound matcher and a missing anchor. Removing it in favour of Load alone
// would leave every built-in unguarded.
func TestCompilePanicsOnIncompleteRequirementParams(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Compile must panic on a termination requirement with no params")
		}
		if !strings.Contains(fmt.Sprint(r), "termination") {
			t.Errorf("panic should teach what is wrong, got %v", r)
		}
	}()
	Compile(Profile{
		Name:         "MYBUS",
		Signals:      []Signal{{Name: "H", Suffix: "_H", Anchor: true}, {Name: "L", Suffix: "_L"}},
		Requirements: []Requirement{{Type: "termination"}},
	})
}

// An overlay registering its own requirement type can ship a validator with it, so its params get the
// same load-time teaching error the built-in gets. This is the seam a customer extends through.
func TestRegisterRequirementWithValidatorRejectsAtLoad(t *testing.T) {
	RegisterRequirementWithValidator("house-rule",
		func(p Profile, _ Requirement) *check.Rule { return nil },
		func(params map[string]string) error {
			if params["mode"] == "" {
				return fmt.Errorf("needs a \"mode\" param (one of: strict, lax)")
			}
			return nil
		})
	defer delete(requirementRegistry, "house-rule")

	base := "name: MYBUS\nsignals: [{name: H, suffix: _H, anchor: true}]\n"
	_, err := Load(strings.NewReader(base + "requirements: [{type: house-rule}]\n"))
	if err == nil {
		t.Fatal("want a load error for the overlay requirement's missing param")
	}
	if !strings.Contains(err.Error(), "house-rule") || !strings.Contains(err.Error(), "mode") {
		t.Errorf("error should name the requirement and its param, got: %v", err)
	}
	if _, err := Load(strings.NewReader(base + "requirements: [{type: house-rule, params: {mode: strict}}]\n")); err != nil {
		t.Errorf("a satisfied validator should load cleanly, got: %v", err)
	}
}

// Every built-in still loads and validates: CAN is the one that declares termination, so this is the
// regression gate that the new check does not reject the profiles that ship.
func TestBuiltinsPassRequirementValidation(t *testing.T) {
	for _, p := range Profiles {
		if err := ValidateRequirements(p); err != nil {
			t.Errorf("built-in %q must satisfy its own requirement validators: %v", p.Name, err)
		}
	}
}
