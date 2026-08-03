package datalog

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// plainComp is a component whose class comes from its ref-des prefix alone (no part data): Y -> crystal,
// C -> capacitor, U -> ic.
func plainComp(ref string) *ir.Component {
	return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
}

// tnet builds a net from "REF.PIN" connection tokens, the same shape the check-package fixtures use.
func tnet(name string, conns ...string) *ir.Net {
	n := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}}
	for _, c := range conns {
		parts := strings.SplitN(c, ".", 2)
		n.Connections = append(n.Connections, &ir.Connection{ComponentRef: parts[0], PinRef: parts[1]})
	}
	return n
}

// crystalParityDesign replicates the check-package crystal guard matrix so the datalog twin is
// exercised against every branch the Go rule is: Y1 fires (one cap-less terminal), Y3 silent (both
// terminals capped), Y2 skipped (supply rail = active oscillator), Y4 skipped (3 non-ground terminals,
// unrecognized Vcc), Y5 skipped (cap-less terminal is an external read-gap net), U9 non-crystal.
func crystalParityDesign() *ir.Design {
	oscVdd := tnet("OSC_VDD", "Y2.1")
	oscVdd.Attributes = map[string]string{netgraph.AttrPowerDriven: "true"}
	xext := tnet("XEXT5", "Y5.1")
	xext.Attributes = map[string]string{netgraph.AttrExternal: "true"}
	return &ir.Design{
		Components: []*ir.Component{
			plainComp("U1"), plainComp("U9"),
			plainComp("Y1"), plainComp("Y2"), plainComp("Y3"), plainComp("Y4"), plainComp("Y5"),
			plainComp("C1"), plainComp("C3"), plainComp("C4"), plainComp("C5"),
		},
		Nets: []*ir.Net{
			tnet("XIN1", "Y1.1", "U1.1", "C1.1"),
			tnet("XOUT1", "Y1.2", "U1.2"),
			tnet("XIN3", "Y3.1", "C3.1"),
			tnet("XOUT3", "Y3.2", "C4.1"),
			oscVdd,
			tnet("OSC_OUT", "Y2.2"),
			tnet("OSC_OUT4", "Y4.3"),
			tnet("$2N999", "Y4.4"),
			tnet("STBY4", "Y4.1"),
			tnet("SIG", "U9.1"),
			xext,
			tnet("XIN5", "Y5.2", "C5.1"),
		},
	}
}

// findingKeys renders findings as sorted "Subject|Message" keys for an order-insensitive compare.
func findingKeys(fs []check.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Subject+"|"+f.Message)
	}
	sort.Strings(out)
	return out
}

func goRuleByName(name string) *check.Rule {
	for _, r := range check.Rules {
		if r.Name == name {
			return r
		}
	}
	return nil
}

// TestCrystalDatalogParity (WS3-074): the unregistered datalog twin fires finding-for-finding
// identically to the built-in Go crystal-load-caps rule over the full guard matrix — the proof the
// datalog surface (component.class + net.ground + net.external) expresses the rule faithfully.
func TestCrystalDatalogParity(t *testing.T) {
	m := check.NewModel(crystalParityDesign())

	goRule := goRuleByName("crystal-load-caps")
	if goRule == nil {
		t.Fatal(`built-in rule "crystal-load-caps" not found in check.Rules`)
	}
	want := findingKeys(goRule.Eval(m))
	got := findingKeys(crystalLoadCapsDL.Eval(m))

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("datalog twin diverges from Go rule\n  go : %v\n  dl : %v", want, got)
	}
	// Pin the expected behavior too, so the test fails if BOTH rules regress the same way.
	if len(want) != 1 || want[0] != "Y1|crystal terminal net XOUT1 has no load capacitor" {
		t.Fatalf("findings = %v, want exactly one on Y1 (terminal XOUT1)", want)
	}
}

// TestTwinNotRegistered documents the Approach-A intent (WS3-074): the datalog crystal rule is a
// parity twin, not a shipped rule — the "dl" source carries no crystal rule, so DefaultCatalog holds
// only the built-in one and no duplicate findings can fire.
func TestTwinNotRegistered(t *testing.T) {
	for _, r := range check.DefaultCatalog().Rules() {
		if r.Name == "dl/crystal-load-caps" {
			t.Fatal(`"dl/crystal-load-caps" is registered; the WS3-074 twin must stay unregistered until the flip`)
		}
	}
}
