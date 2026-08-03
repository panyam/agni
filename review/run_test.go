package review

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/panyam/agni/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	_ "github.com/panyam/agni/stdlib/profiles" // registers the built-in profile rules into DefaultCatalog
)

// A profile item whose interface is absent reads not-applicable, not a silent pass (WS3-051). The
// binding resolves to the real CAN rules; a stub PresenceFunc supplies the absent/present verdict the
// CLI derives from profiles.Present.
func TestProfileAbsentNotApplicable(t *testing.T) {
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "x", Title: "CAN bus", Binding: Binding{Profile: "CAN"}},
	}}}}
	out := func(present PresenceFunc) Outcome {
		return Run(RunParams{Model: check.NewModel(oneDesign()), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d", Present: present}).Areas[0].Items[0].Outcome
	}
	if got := out(func(string) (Presence, bool) { return IfaceAbsent, true }); got != NotApplicable {
		t.Errorf("absent interface: want not-applicable, got %s", got)
	}
	if got := out(func(string) (Presence, bool) { return IfacePresent, true }); got != Pass {
		t.Errorf("present interface, no issue: want pass, got %s", got)
	}
	if got := out(nil); got != Pass {
		t.Errorf("nil presence: want pass (unchanged behavior), got %s", got)
	}
}

// A profile item whose interface is host-bound but declared on no component (and whose convention is
// not in use) reads not-automated, NOT a hollow pass — the intended host check could not evaluate
// (WS3-090). The gate returns before running the rules, so a rule-bearing profile is unaffected.
func TestHostUnsatisfiedNotAutomated(t *testing.T) {
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "x", Title: "LIN bus", Binding: Binding{Profile: "CAN"}},
	}}}}
	got := Run(RunParams{Model: check.NewModel(oneDesign()), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d",
		Present: func(string) (Presence, bool) { return IfaceHostUnsatisfied, true }}).Areas[0].Items[0]
	if got.Outcome != NotAutomated {
		t.Errorf("host-unsatisfied interface: want not-automated, got %s", got.Outcome)
	}
	if !strings.Contains(got.Note, "host-bound") {
		t.Errorf("want a host-bound reason note, got %q", got.Note)
	}
}

// An item bound to an interface with NO shipped rule (a presence-only declaration: a profile with
// signals but no requirements, so it compiles to zero rules) reads not-applicable when the interface
// is KNOWN and absent, not not-automated (WS3-068). "WiFiBT" has no built-in profile, so the binding
// resolves to zero catalog rules; the stub PresenceFunc supplies the known/absent verdict the CLI
// derives from the loaded presence-only profile.
func TestAbsentInterfaceWithoutRulesNotApplicable(t *testing.T) {
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "x", Title: "Wi-Fi present?", Binding: Binding{Profile: "WiFiBT"}},
	}}}}
	out := func(present PresenceFunc) Outcome {
		return Run(RunParams{Model: check.NewModel(oneDesign()), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d", Present: present}).Areas[0].Items[0].Outcome
	}
	// known + absent: the module is not on this board -> not-applicable (the WS3-068 fix).
	if got := out(func(string) (Presence, bool) { return IfaceAbsent, true }); got != NotApplicable {
		t.Errorf("known-absent no-rule interface: want not-applicable, got %s", got)
	}
	// known + present but nothing checks it -> not-automated (honest; presence does not manufacture a pass).
	if got := out(func(string) (Presence, bool) { return IfacePresent, true }); got != NotAutomated {
		t.Errorf("present no-rule interface: want not-automated, got %s", got)
	}
	// unknown interface (no profile/declaration) -> not-automated, unchanged.
	if got := out(func(string) (Presence, bool) { return IfaceAbsent, false }); got != NotAutomated {
		t.Errorf("unknown interface: want not-automated, got %s", got)
	}
	// nil presence: unchanged, resolves to zero rules -> not-automated.
	if got := out(nil); got != NotAutomated {
		t.Errorf("nil presence no-rule interface: want not-automated, got %s", got)
	}
}

// oneDesign has a single-pin net (fires the built-in single-pin-net rule) and no board geometry.
func oneDesign() *ir.Design {
	return &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}}},
		Nets: []*ir.Net{{
			Name:        "SIG",
			Prov:        &ir.Provenance{SourceFile: "t"},
			Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}},
		}},
	}
}

// debugDesign is oneDesign plus a J-prefixed connector whose description marks it a JTAG/debug
// connector, so ComponentClass classifies it test_connector (WS3-066).
func debugDesign() *ir.Design {
	d := oneDesign()
	d.Components = append(d.Components, &ir.Component{
		RefDes:     "J1",
		Attributes: map[string]string{"Description": "JTAG debug connector"},
		Prov:       &ir.Provenance{SourceFile: "t"},
	})
	return d
}

// TestPresentBinding (WS3-075): a present: binding reads pass when a component of the class exists and
// fail (one design-level finding) when none does — the "a debug connector must be on the board"
// primitive. It is never not-applicable: the component-class tier is always available.
func TestPresentBinding(t *testing.T) {
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "debug", Title: "debug interface available", Binding: Binding{Present: &PresentBinding{Class: "test_connector"}}},
	}}}}

	// A design carrying a debug connector -> pass, no findings.
	rep := Run(RunParams{Model: check.NewModel(debugDesign()), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d"})
	if got := rep.Areas[0].Items[0]; got.Outcome != Pass || len(got.Findings) != 0 {
		t.Fatalf("with a test_connector: want pass/no-findings, got %s / %+v", got.Outcome, got.Findings)
	}

	// A design with no debug connector -> fail with exactly one design-level finding.
	rep = Run(RunParams{Model: check.NewModel(oneDesign()), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d"})
	got := rep.Areas[0].Items[0]
	if got.Outcome != Fail || len(got.Findings) != 1 {
		t.Fatalf("without a test_connector: want fail/one-finding, got %s / %+v", got.Outcome, got.Findings)
	}
	if f := got.Findings[0]; f.Rule != "present/test_connector" || f.Subject != "test_connector" {
		t.Errorf("finding = %+v, want rule=present/test_connector subject=test_connector", f)
	}
}

// TestPresentBindingHasClass (WS10-016) pins that present: matches via HasClass, not the most-specific
// keyword class: a family tag (an LED is a diode) and a datasheet-enriched class (a smart high-side
// switch keyword-classes `ic` but its datasheet declares `efuse`, WS10-013) both satisfy the item. The
// `ic`+`efuse` set is exactly what enrichClassesFromParams produces — ComponentClass stays `ic` (never
// promoted), so a ComponentClass== test would miss it and the datasheet-seeded efuse would be invisible.
func TestPresentBindingHasClass(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{
			// keyword class ic, datasheet-enriched with efuse (efuse is not a most-specific keyword class).
			{RefDes: "U1", DeviceClasses: []string{"ic", "efuse"}, Prov: &ir.Provenance{SourceFile: "t"}},
			// most-specific led, family tag diode.
			{RefDes: "D1", DeviceClasses: []string{"led", "diode"}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{{Name: "SIG", Prov: &ir.Provenance{SourceFile: "t"},
			Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}}},
	}
	for _, cl := range []string{"efuse", "diode"} {
		man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
			{ID: cl, Title: cl + " present", Binding: Binding{Present: &PresentBinding{Class: cl}}},
		}}}}
		got := Run(RunParams{Model: check.NewModel(d), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d"}).Areas[0].Items[0]
		if got.Outcome != Pass {
			t.Errorf("present:{class:%s}: want pass (HasClass matches a non-most-specific tag), got %s / %+v", cl, got.Outcome, got.Findings)
		}
	}
	// A class on no component still fails, so present: is not vacuously passing.
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "cap", Title: "cap present", Binding: Binding{Present: &PresentBinding{Class: "capacitor"}}},
	}}}}
	if got := Run(RunParams{Model: check.NewModel(d), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d"}).Areas[0].Items[0]; got.Outcome != Fail {
		t.Errorf("present:{class:capacitor} on a design with none: want fail, got %s", got.Outcome)
	}
}

// TestRunOutcomes exercises all four outcomes with core rules (no profile registration needed): a
// firing rule -> fail; an inline query with no match -> pass; a board rule on a netlist -> not
// applicable; an item with no binding and one naming a rule that has not shipped -> not automated.
func TestRunOutcomes(t *testing.T) {
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "fail", Title: "single pin", Binding: Binding{Rule: "single-pin-net"}},
		{ID: "pass", Title: "no forbidden part", Binding: Binding{Query: &QueryBinding{
			Match: `component.mpn(?r, "NOPE") => ?r`, Subject: "r", Message: "{r} forbidden"}}},
		{ID: "na", Title: "track width", Binding: Binding{Rule: "track-width"}},
		{ID: "auto", Title: "manual thing"},
		{ID: "ghost", Title: "future rule", Binding: Binding{Rule: "does-not-exist-yet"}},
	}}}}

	rep := Run(RunParams{Model: check.NewModel(oneDesign()), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d"})
	got := map[string]Outcome{}
	for _, a := range rep.Areas {
		for _, it := range a.Items {
			got[it.Item.ID] = it.Outcome
		}
	}
	want := map[string]Outcome{
		"fail": Fail, "pass": Pass, "na": NotApplicable, "auto": NotAutomated, "ghost": NotAutomated,
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("item %q: want %s, got %s", id, w, got[id])
		}
	}
	if tl := rep.Tally(); tl.Fail != 1 || tl.Pass != 1 || tl.NotApplicable != 1 || tl.NotAutomated != 2 {
		t.Errorf("tally: %s", tl)
	}
}

// A not-automated item's Note (why it is not automated) renders in the report; a failing item still
// shows its findings, not its note.
func TestReportRendersNote(t *testing.T) {
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "planned", Title: "voltage compat", Note: "needs a datasheet param rule (WS3-036)",
			Binding: Binding{Rule: "rail-nominal-out-of-recommended"}},
		{ID: "fail", Title: "single pin", Note: "should not show for a fail",
			Binding: Binding{Rule: "single-pin-net"}},
	}}}}
	md := RenderMarkdown(Run(RunParams{Model: check.NewModel(oneDesign()), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d"}))
	if !strings.Contains(md, "needs a datasheet param rule (WS3-036)") {
		t.Errorf("not-automated note should render:\n%s", md)
	}
	if strings.Contains(md, "should not show for a fail") {
		t.Errorf("a fail row should show findings, not its note:\n%s", md)
	}
}

// The coverage rollup summarizes a run per area: automated = the items whose binding resolved (pass +
// fail + n/a), not-automated the rest.
func TestCoverageRollup(t *testing.T) {
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "fail", Title: "single pin", Binding: Binding{Rule: "single-pin-net"}},
		{ID: "na", Title: "track width", Binding: Binding{Rule: "track-width"}},
		{ID: "auto", Title: "manual"},
		{ID: "ghost", Title: "future", Binding: Binding{Rule: "nope"}},
	}}}}
	md := RenderCoverageMarkdown(Run(RunParams{Model: check.NewModel(oneDesign()), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d"}))
	for _, want := range []string{
		"**2 of 4 covered** — 0 pass, 1 fail, 1 n/a; 2 not-automated",
		"| A | 2/4 | 0 | 1 | 0 | 0 | 0 | 1 | 2 |",
		"| **Total** | 2/4 | 0 | 1 | 0 | 0 | 0 | 1 | 2 |",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("coverage missing %q\n%s", want, md)
		}
	}
}

// A failing item with more than maxDetailFindings findings renders a capped summary: the first few
// findings plus "(+N more)", not the full dump. On the real automotive EVT the esd item fails on 250+ nets,
// which made one unreadable 100KB table cell.
func TestReportCapsFindings(t *testing.T) {
	var fs []check.Finding
	for i := 0; i < 10; i++ {
		fs = append(fs, check.Finding{Rule: "esd-protection", Kind: check.KindNet,
			Subject: "NET_" + string(rune('A'+i)), Message: "no ESD device"})
	}
	rep := Report{Manifest: "t", Areas: []AreaResult{{Area: Area{Name: "A"}, Items: []ItemResult{
		{Item: Item{ID: "esd", Title: "ESD"}, Outcome: Fail, Findings: fs},
	}}}}
	md := RenderMarkdown(rep)
	if !strings.Contains(md, "(+7 more)") {
		t.Errorf("capped summary should note the remaining findings:\n%s", md)
	}
	// The first maxDetailFindings show; the rest do not.
	for _, want := range []string{"NET_A", "NET_B", "NET_C"} {
		if !strings.Contains(md, want) {
			t.Errorf("first %d findings should render, missing %q:\n%s", maxDetailFindings, want, md)
		}
	}
	if strings.Contains(md, "NET_D") {
		t.Errorf("findings past the cap should not render:\n%s", md)
	}
}

// A scoped binding (WS3-058) keeps only the bound rule's findings on the named interface's nets, and
// reads not-applicable when EVERY named interface is absent. single-pin-net fires on SIG_A and SIG_B;
// IFACE owns SIG_A, IFACE2 owns SIG_B. Scoping to one interface leaves its net; scoping to a span of
// interfaces unions their nets; scoping to a span where any interface is present still runs; scoping to
// a span where all are absent reads not-applicable rather than a whole-design fail.
func TestScopedBindingFiltersAndPresence(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}}},
		Nets: []*ir.Net{
			{Name: "SIG_A", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}},
			{Name: "SIG_B", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "2"}}},
		},
	}
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "scoped", Title: "iface", Binding: Binding{Rule: "single-pin-net", Scope: ScopeBinding{Profiles: []string{"IFACE"}}}},
		{ID: "absent", Title: "gone", Binding: Binding{Rule: "single-pin-net", Scope: ScopeBinding{Profiles: []string{"GONE"}}}},
		{ID: "span", Title: "both buses", Binding: Binding{Rule: "single-pin-net", Scope: ScopeBinding{Profiles: []string{"IFACE", "IFACE2"}}}},
		{ID: "bothgone", Title: "all absent", Binding: Binding{Rule: "single-pin-net", Scope: ScopeBinding{Profiles: []string{"GONE", "GONE2"}}}},
		{ID: "onepresent", Title: "one present", Binding: Binding{Rule: "single-pin-net", Scope: ScopeBinding{Profiles: []string{"GONE", "IFACE"}}}},
	}}}}
	present := func(name string) (Presence, bool) {
		switch name {
		case "IFACE", "IFACE2":
			return IfacePresent, true
		case "GONE", "GONE2":
			return IfaceAbsent, true
		}
		return IfaceAbsent, false
	}
	scope := func(name string) map[string]bool {
		switch name {
		case "IFACE":
			return map[string]bool{"SIG_A": true}
		case "IFACE2":
			return map[string]bool{"SIG_B": true}
		}
		return map[string]bool{}
	}
	rep := Run(RunParams{Model: check.NewModel(d), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d", Present: present, Scope: scope})
	items := map[string]ItemResult{}
	for _, it := range rep.Areas[0].Items {
		items[it.Item.ID] = it
	}
	if got := items["scoped"]; got.Outcome != Fail || len(got.Findings) != 1 || got.Findings[0].Subject != "SIG_A" {
		t.Errorf("scoped item: want Fail on SIG_A only, got %s findings=%+v", got.Outcome, got.Findings)
	}
	if got := items["absent"].Outcome; got != NotApplicable {
		t.Errorf("scoped-to-absent item: want not-applicable, got %s", got)
	}
	if got := items["span"]; got.Outcome != Fail || len(got.Findings) != 2 {
		t.Errorf("span item: want Fail on both nets (union), got %s findings=%+v", got.Outcome, got.Findings)
	}
	if got := items["bothgone"].Outcome; got != NotApplicable {
		t.Errorf("span all-absent: want not-applicable, got %s", got)
	}
	if got := items["onepresent"]; got.Outcome != Fail || len(got.Findings) != 1 || got.Findings[0].Subject != "SIG_A" {
		t.Errorf("span one-present: want Fail on the present bus's net only, got %s findings=%+v", got.Outcome, got.Findings)
	}
}

// A COMPONENT-subject rule can be scoped per interface (WS3-083): duplicate-ref-des fires on U1 and U2;
// the interface owns U1 only, so a component-scoped binding keeps U1's finding and drops U2's — where a
// net-only scope (nil CompScope) would drop BOTH and hollow-pass. An absent interface is still
// not-applicable via presence, before any filtering.
func TestComponentScopedBinding(t *testing.T) {
	collision := func(ref string) *ir.RefDesCollision {
		return &ir.RefDesCollision{RefDes: ref, Instances: []*ir.Provenance{{SourceFile: "t"}, {SourceFile: "t"}}}
	}
	d := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U2", Prov: &ir.Provenance{SourceFile: "t"}},
		},
		InputDiagnostics: &ir.InputDiagnostics{RefDesCollisions: []*ir.RefDesCollision{collision("U1"), collision("U2")}},
	}
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{
		{ID: "scoped", Title: "iface parts", Binding: Binding{Rule: "duplicate-ref-des", Scope: ScopeBinding{Profiles: []string{"IFACE"}}}},
		{ID: "absent", Title: "gone", Binding: Binding{Rule: "duplicate-ref-des", Scope: ScopeBinding{Profiles: []string{"GONE"}}}},
	}}}}
	present := func(name string) (Presence, bool) {
		switch name {
		case "IFACE":
			return IfacePresent, true
		case "GONE":
			return IfaceAbsent, true
		}
		return IfaceAbsent, false
	}
	compScope := func(name string) map[string]bool {
		if name == "IFACE" {
			return map[string]bool{"U1": true}
		}
		return map[string]bool{}
	}
	rep := Run(RunParams{Model: check.NewModel(d), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d", Present: present, CompScope: compScope})
	items := map[string]ItemResult{}
	for _, it := range rep.Areas[0].Items {
		items[it.Item.ID] = it
	}
	if got := items["scoped"]; got.Outcome != Fail || len(got.Findings) != 1 || got.Findings[0].Subject != "U1" {
		t.Errorf("component-scoped item: want Fail on U1 only, got %s findings=%+v", got.Outcome, got.Findings)
	}
	if got := items["absent"].Outcome; got != NotApplicable {
		t.Errorf("component-scoped-to-absent item: want not-applicable, got %s", got)
	}
}

// filterToScope keeps a net finding whose net is in scope and a component finding whose subject is in
// scope, and drops out-of-scope subjects of either kind plus every pin finding (no pin→component map).
func TestFilterToScope(t *testing.T) {
	fs := []check.Finding{
		{Kind: check.KindNet, Subject: "SIG_A"},
		{Kind: check.KindNet, Subject: "SIG_X"},
		{Kind: check.KindComponent, Subject: "U1"},
		{Kind: check.KindComponent, Subject: "U9"},
		{Kind: check.KindPin, Subject: "U1.1"},
	}
	got := filterToScope(fs, map[string]bool{"SIG_A": true}, map[string]bool{"U1": true})
	if len(got) != 2 {
		t.Fatalf("want 2 kept (SIG_A net + U1 component), got %d: %+v", len(got), got)
	}
	kept := map[string]bool{}
	for _, f := range got {
		kept[string(f.Kind)+":"+f.Subject] = true
	}
	if !kept["net:SIG_A"] || !kept["component:U1"] {
		t.Errorf("want net:SIG_A and component:U1 kept, got %v", kept)
	}
	for _, drop := range []string{"net:SIG_X", "component:U9", "pin:U1.1"} {
		if kept[drop] {
			t.Errorf("%s should be dropped by the scope", drop)
		}
	}
}

// RenderJSON emits the FULL finding list for a failing item — including findings past maxDetailFindings
// that the markdown Detail cell caps — so tooling and the future web report lose nothing.
func TestRenderJSONFullFindings(t *testing.T) {
	var fs []check.Finding
	for i := 0; i < 10; i++ {
		fs = append(fs, check.Finding{Rule: "esd-protection", Kind: check.KindNet,
			Subject: "NET_" + string(rune('A'+i)), Message: "no ESD device",
			Prov: &ir.Provenance{SourceFile: "evt.edn"}})
	}
	rep := Report{Manifest: "t", Design: "evt", Areas: []AreaResult{{Area: Area{Name: "A"}, Items: []ItemResult{
		{Item: Item{ID: "esd", Title: "ESD"}, Outcome: Fail, Findings: fs},
	}}}}
	js, err := RenderJSON(rep)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var got jsonReport
	if err := json.Unmarshal([]byte(js), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, js)
	}
	if len(got.Areas) != 1 || len(got.Areas[0].Items) != 1 {
		t.Fatalf("shape: %+v", got)
	}
	item := got.Areas[0].Items[0]
	if len(item.Findings) != 10 {
		t.Errorf("want all 10 findings uncapped, got %d", len(item.Findings))
	}
	// The last finding (past the markdown cap) is present, with subject + source for the eventual link.
	last := item.Findings[9]
	if last.Subject != "NET_J" || last.SourceFile != "evt.edn" {
		t.Errorf("last finding = %+v, want subject NET_J from evt.edn", last)
	}
}

func TestLoadValidation(t *testing.T) {
	cases := map[string]string{
		"missing name":       "areas: [{name: A, items: []}]",
		"no areas":           "name: t",
		"area no name":       "name: t\nareas: [{items: []}]",
		"item no id":         "name: t\nareas: [{name: A, items: [{ask: x}]}]",
		"two bindings":       "name: t\nareas: [{name: A, items: [{id: i, rule: r, profile: P}]}]",
		"present + rule":     "name: t\nareas: [{name: A, items: [{id: i, rule: r, present: {class: test_connector}}]}]",
		"present no class":   "name: t\nareas: [{name: A, items: [{id: i, present: {class: ''}}]}]",
		"bad inline query":   "name: t\nareas: [{name: A, items: [{id: i, query: {match: 'garbage(', subject: r, message: m}}]}]",
		"query missing vars": "name: t\nareas: [{name: A, items: [{id: i, query: {match: 'component.mpn(?r,\"X\") => ?r'}}]}]",
	}
	for name, y := range cases {
		if _, err := Load(strings.NewReader(y)); err == nil {
			t.Errorf("%s: want error, got nil", name)
		}
	}
}

func TestLoadValid(t *testing.T) {
	y := `
name: mini
areas:
  - name: CAN
    items:
      - {id: "1", title: termination, description: "120 ohm across CANH/CANL at both bus ends", rule: profile/can-termination-missing}
      - {id: "2", title: transceiver, }
`
	m, err := Load(strings.NewReader(y))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Name != "mini" || len(m.Areas) != 1 || len(m.Areas[0].Items) != 2 {
		t.Fatalf("parsed wrong: %+v", m)
	}
	if got := m.Areas[0].Items[0]; got.Title != "termination" || got.Description != "120 ohm across CANH/CANL at both bus ends" {
		t.Errorf("Title/Description round-trip: got %+v", got)
	}
}
