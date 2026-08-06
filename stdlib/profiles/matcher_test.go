package profiles

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// pulledProfile is a prefix-discriminated bus whose CS line needs a pull-up. Its suffix (_CS) is one
// a real corpus carries many of, so the prefix is the only thing telling its net from a foreign one.
func pulledProfile() Profile {
	return Profile{Name: "PBus", Signals: []Signal{
		{Name: "CS", Prefix: "PBUS_", Suffix: "_CS", PullUp: true, Anchor: true},
		{Name: "CLK", Prefix: "PBUS_", Suffix: "_CLK"},
	}, Requirements: []Requirement{{Type: "missing-pullup"}}}
}

// prefixedBusWithForeignNet: the profile's own bus is complete and pulled up through R1, and a
// FOREIGN net shares the bare _CS suffix with no pull-up. Only the foreign net can produce a finding,
// so any finding here is an over-match.
func prefixedBusWithForeignNet() *ir.Design {
	return &ir.Design{
		Components: comps("U1", "U2", "R1", "U7"),
		Nets: []*ir.Net{
			net("PBUS_A_CS", "U1.1", "U2.1", "R1.1"),
			net("PBUS_A_CLK", "U1.2", "U2.2"),
			net("+3V3", "R1.2", "U2.7"),
			net("FLASH_B_CS", "U7.1", "U7.2"), // foreign bus, same suffix, no pull-up
		},
	}
}

// TestPrefixedProfilePullupIgnoresForeignNet: missing-pullup compiled its needs_pullup relation from
// the bare suffix, ignoring the signal's prefix, so a prefix-discriminated profile fired on any
// same-suffix net of a foreign bus once its own bus put it in use.
func TestPrefixedProfilePullupIgnoresForeignNet(t *testing.T) {
	fs := check.Run(check.NewModel(prefixedBusWithForeignNet()), Compile(pulledProfile()))
	for _, f := range fs {
		t.Errorf("missing-pullup fired on a net outside the prefixed bus: %s (%s)", f.Subject, f.Message)
	}
}

// TestPrefixedProfileDanglingIgnoresForeignNet: same defect in signal-dangling's sig_net relation.
// FLASH_B_CS has a single connection, so a prefix-blind sig_net reports it as this bus's dangling net.
func TestPrefixedProfileDanglingIgnoresForeignNet(t *testing.T) {
	p := pulledProfile()
	p.Requirements = []Requirement{{Type: "signal-dangling"}}
	d := prefixedBusWithForeignNet()
	d.Nets[3] = net("FLASH_B_CS", "U7.1") // one connection: dangling by the rule's definition

	for _, f := range check.Run(check.NewModel(d), Compile(p)) {
		t.Errorf("signal-dangling fired on a net outside the prefixed bus: %s (%s)", f.Subject, f.Message)
	}
}

// TestNetsScopeRespectsMatcher: the review scope (profiles.Nets/Components, the seam a per-interface
// item filters its findings through) matched by suffix alone, so a prefixed profile's scope pulled in
// foreign nets — and the components on them — that no rule of that profile can ever name.
func TestNetsScopeRespectsMatcher(t *testing.T) {
	m := check.NewModel(prefixedBusWithForeignNet())
	got := Nets(m, pulledProfile())
	if !got["PBUS_A_CS"] || !got["PBUS_A_CLK"] {
		t.Errorf("scope should hold the prefixed bus's own nets, got %v", got)
	}
	if got["FLASH_B_CS"] {
		t.Errorf("scope must not hold a foreign same-suffix net, got %v", got)
	}
	if Components(m, pulledProfile())["U7"] {
		t.Error("component scope must not reach a component that is only on a foreign net")
	}
}

// TestCoverageRespectsMatcher: the WS9-041 coverage panel bound each signal by suffix alone, so a
// prefixed profile could display a foreign net as the signal's net — a cell disagreeing with every
// finding, which the panel's contract says cannot happen.
func TestCoverageRespectsMatcher(t *testing.T) {
	d := prefixedBusWithForeignNet()
	// Put the foreign net FIRST so a suffix-only scan reaches it before the profile's own CS net.
	d.Nets = []*ir.Net{d.Nets[3], d.Nets[0], d.Nets[1], d.Nets[2]}

	cov := Coverage(pulledProfile(), check.NewModel(d))
	if cov == nil {
		t.Fatal("the prefixed bus is in use and should be detected")
	}
	if cov.Anchor != "PBUS_A_CS" {
		t.Errorf("anchor = %q, want PBUS_A_CS (a foreign same-suffix net must not bind the signal)", cov.Anchor)
	}
	for _, s := range cov.Signals {
		if s.Net == "FLASH_B_CS" {
			t.Errorf("signal %s bound the foreign net %q", s.Name, s.Net)
		}
	}
}

// ethLike is the WS3-057 forcing case: the bus identity is the PREFIX (ETH_SW1_P1) and the suffix
// (_H/_L) is shared with CAN, so affix matching cannot express it. A glob can.
func ethLike() Profile {
	return Profile{Name: "EthTest", Signals: []Signal{
		{Name: "H", Glob: "ETH_SW*_H", Anchor: true},
		{Name: "L", Glob: "ETH_SW*_L"},
		{Name: "RST", Glob: "ETH_SW*_RST"},
	}, Requirements: []Requirement{{Type: "signal-missing"}}}
}

func canLike() Profile {
	return Profile{Name: "CanTest", Signals: []Signal{
		{Name: "H", Glob: "CAN_*_H", Anchor: true},
		{Name: "L", Glob: "CAN_*_L"},
		{Name: "RST", Glob: "CAN_*_RST"},
	}, Requirements: []Requirement{{Type: "signal-missing"}}}
}

// TestGlobDiscriminatesSharedSuffix is the ticket's acceptance case: an Ethernet profile matching
// ETH_SW*_H and a CAN profile matching CAN_*_H do not cross-match on a design carrying both. Each
// fires only for ITS OWN incomplete bus, and neither anchors on the other's nets.
func TestGlobDiscriminatesSharedSuffix(t *testing.T) {
	// Ethernet is complete; CAN is in use (H + RST) but missing its _L line.
	d := &ir.Design{
		Components: comps("U1", "U2", "U3", "U4"),
		Nets: []*ir.Net{
			net("ETH_SW1_P1_1000M_A_H", "U1.1", "U2.1"),
			net("ETH_SW1_P1_1000M_A_L", "U1.2", "U2.2"),
			net("ETH_SW1_P1_RST", "U1.3", "U2.3"),
			net("CAN_00_H", "U3.1", "U4.1"),
			net("CAN_00_RST", "U3.3", "U4.3"),
		},
	}
	m := check.NewModel(d)

	if fs := check.Run(m, Compile(ethLike())); len(fs) != 0 {
		t.Errorf("Ethernet is complete; want 0 findings, got %d: %+v", len(fs), fs)
	}
	fs := check.Run(m, Compile(canLike()))
	if len(fs) != 1 {
		t.Fatalf("CAN is missing _L; want 1 finding, got %d: %+v", len(fs), fs)
	}
	if !strings.HasPrefix(fs[0].Subject, "CAN_") {
		t.Errorf("CAN's finding anchored on %q, want a CAN_ net (no cross-match onto Ethernet)", fs[0].Subject)
	}
	if !strings.Contains(fs[0].Message, "L") {
		t.Errorf("the missing CAN signal should be L, got %q", fs[0].Message)
	}
}

// TestGlobProfileSilentOnForeignBusOnly: with no Ethernet at all, the glob profile is not in use and
// says nothing about the CAN nets that share its _H/_L suffixes.
func TestGlobProfileSilentOnForeignBusOnly(t *testing.T) {
	d := &ir.Design{Components: comps("U3", "U4"), Nets: []*ir.Net{
		net("CAN_00_H", "U3.1", "U4.1"),
		net("CAN_00_L", "U3.2", "U4.2"),
	}}
	if fs := check.Run(check.NewModel(d), Compile(ethLike())); len(fs) != 0 {
		t.Errorf("no Ethernet on this design; want 0 findings, got %d: %+v", len(fs), fs)
	}
}

// TestRegexSignalMatching: the escape hatch expresses multi-instance naming a glob cannot — here
// "an ETH port's H line, but only the 1000M ones" — and stays off the 10M port and off CAN.
func TestRegexSignalMatching(t *testing.T) {
	p := Profile{Name: "EthGig", Signals: []Signal{
		{Name: "H", Regex: `^ETH_SW\d+_P\d+_1000M_._H$`, Anchor: true},
		{Name: "L", Regex: `^ETH_SW\d+_P\d+_1000M_._L$`},
		{Name: "RST", Regex: `^ETH_SW\d+_P\d+_1000M_RST$`},
	}, Requirements: []Requirement{{Type: "signal-missing"}}}
	d := &ir.Design{
		Components: comps("U1", "U2"),
		Nets: []*ir.Net{
			net("ETH_SW1_P1_1000M_A_H", "U1.1", "U2.1"),
			net("ETH_SW1_P1_1000M_A_L", "U1.2", "U2.2"),
			net("ETH_SW1_P3_10M_A_H", "U1.3", "U2.3"), // different speed: outside the regex
			net("CAN_00_L", "U1.4", "U2.4"),           // foreign bus sharing the bare _L suffix
		},
	}
	m := check.NewModel(d)
	if !InUse(m, p) {
		t.Fatal("the 1000M H and L lines are present; a regex signal should read as matched")
	}
	fs := check.Run(m, Compile(p))
	if len(fs) != 1 || fs[0].Subject != "ETH_SW1_P1_1000M_A_H" {
		t.Fatalf("want one finding anchored at the 1000M H net, got %+v", fs)
	}
	if !strings.Contains(fs[0].Message, "RST") {
		t.Errorf("the missing signal should be RST, got %q", fs[0].Message)
	}
}

// TestInUseAgreesWithRulesForGlobAndRegex holds the WS3-090 twin discipline across the new forms: the
// Go presence gate and the compiled in_use gate must reach the same verdict, or an interface the rules
// cannot fire on is scored as a clean pass by a review.
func TestInUseAgreesWithRulesForGlobAndRegex(t *testing.T) {
	designs := map[string]*ir.Design{
		"foreign only": {Components: comps("U3"), Nets: []*ir.Net{net("CAN_00_H", "U3.1"), net("CAN_00_L", "U3.2")}},
		"one signal":   {Components: comps("U1"), Nets: []*ir.Net{net("ETH_SW1_P1_A_H", "U1.1")}},
		"in use":       {Components: comps("U1"), Nets: []*ir.Net{net("ETH_SW1_P1_A_H", "U1.1"), net("ETH_SW1_P1_A_L", "U1.2")}},
	}
	globbed := ethLike()
	regexed := Profile{Name: "EthRe", Signals: []Signal{
		{Name: "H", Regex: `^ETH_SW.*_H$`, Anchor: true},
		{Name: "L", Regex: `^ETH_SW.*_L$`},
	}, Requirements: []Requirement{{Type: "signal-dangling"}}}

	for _, p := range []Profile{globbed, regexed} {
		for name, d := range designs {
			m := check.NewModel(d)
			// signal-dangling fires only under in_use, so a finding is proof the compiled gate held.
			probe := p
			probe.Requirements = []Requirement{{Type: "signal-dangling"}}
			rulesSawInUse := len(check.Run(m, Compile(probe))) > 0
			if got := InUse(m, p); got != rulesSawInUse {
				t.Errorf("%s/%s: InUse = %v but the compiled in_use gate = %v", p.Name, name, got, rulesSawInUse)
			}
		}
	}
}

// TestSuffixOnlyProfilesUnchanged: every built-in is suffix-only, so routing the previously
// suffix-only rules through the matcher must leave them byte-identical. spinorBroken's three findings
// are the ones TestCoverageBroken pins.
func TestSuffixOnlyProfilesUnchanged(t *testing.T) {
	m := check.NewModel(spinorBroken())
	names := map[string]int{}
	for _, f := range check.Run(m, Compile(SPINOR)) {
		names[f.Rule]++
	}
	for _, want := range []string{"spi_nor-signal-missing", "spi_nor-missing-pullup", "spi_nor-signal-dangling"} {
		if names[want] == 0 {
			t.Errorf("suffix-only SPI_NOR should still fire %s, got %v", want, names)
		}
	}
}

func TestValidateSignalMatcher(t *testing.T) {
	bad := map[string]Signal{
		"no matcher":      {Name: "A"},
		"two forms":       {Name: "A", Suffix: "_A", Glob: "X*"},
		"glob and regex":  {Name: "A", Glob: "X*", Regex: "^X"},
		"bad regex":       {Name: "A", Regex: "^ETH_(SW"},
		"universal glob":  {Name: "A", Glob: "*"},
		"universal regex": {Name: "A", Regex: ".*"},
		"empty branch":    {Name: "A", Regex: "^(ETH_H|)$"},
	}
	for name, s := range bad {
		if err := validateSignalMatcher(s); err == nil {
			t.Errorf("%s: want an error, got nil", name)
		}
	}
	good := map[string]Signal{
		"suffix":        {Name: "A", Suffix: "_A"},
		"prefix+suffix": {Name: "A", Prefix: "P_", Suffix: "_A"},
		"prefix only":   {Name: "A", Prefix: "P_"},
		"glob":          {Name: "A", Glob: "ETH_SW*_H"},
		"regex":         {Name: "A", Regex: `^ETH_SW\d+_H$`},
	}
	for name, s := range good {
		if err := validateSignalMatcher(s); err != nil {
			t.Errorf("%s: want no error, got %v", name, err)
		}
	}
}

// TestCompilePanicsOnUnsoundMatcher: a Go-literal profile never goes through Parse, so Compile is its
// only gate. Generating rules from a matcher-less signal would select every net on the design.
func TestCompilePanicsOnUnsoundMatcher(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Compile should panic on a signal with no matcher")
		}
		if !strings.Contains(r.(string), "declares no matcher") {
			t.Errorf("panic should teach what is wrong, got %v", r)
		}
	}()
	Compile(Profile{Name: "X", Signals: []Signal{{Name: "A"}}, Requirements: []Requirement{{Type: "signal-dangling"}}})
}
