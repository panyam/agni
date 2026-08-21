package service

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestFindingProtoBusSubject checks that a bus finding carries its NAME to the client on
// Subject.bus_id (the bus's range-label identity, = the subject), so a bus-not-modeled finding
// highlights its own drawn bus by name (WS7-042b) — and that a non-bus finding never gets a bus id,
// keeping the two disjoint.
func TestFindingProtoBusSubject(t *testing.T) {
	bus := FindingProto(check.Finding{Subject: check.Entity{Kind: check.KindBus, Ref: "DATA[7:0]"}, Rule: "bus-not-modeled", Prov: &ir.Provenance{SourceFile: "x.kicad_sch"}})
	if bus.GetSubject().GetKind() != "bus" {
		t.Errorf("bus subject kind = %q, want %q", bus.GetSubject().GetKind(), "bus")
	}
	if bus.GetSubject().GetBusId() != "DATA[7:0]" {
		t.Errorf("bus subject bus_id = %q, want %q (the bus name join key)", bus.GetSubject().GetBusId(), "DATA[7:0]")
	}
	if bus.GetSubject().GetRef() != "DATA[7:0]" {
		t.Errorf("bus subject ref = %q, want the bus name %q", bus.GetSubject().GetRef(), "DATA[7:0]")
	}

	// A net finding must NOT get a bus id.
	net := FindingProto(check.Finding{Subject: check.Entity{Kind: check.KindNet, Ref: "SIG"}, Rule: "single-pin-net"})
	if net.GetSubject().GetBusId() != "" {
		t.Errorf("net subject bus_id = %q, want empty", net.GetSubject().GetBusId())
	}
}

// TestFindingProtoCarriesDatasheet: a datasheet-backed finding carries its citation on the wire
// (WS9-048), so the CLI's review/check --format json and the web check panel both show the source
// without parsing the message; a finding with no datasheet provenance leaves the field nil.
func TestFindingProtoCarriesDatasheet(t *testing.T) {
	backed := FindingProto(check.Finding{Subject: check.Entity{Kind: check.KindComponent, Ref: "U1"}, Rule: "supply-exceeds-abs-max", DatasheetProv: []*check.DatasheetCitation{{Doc: "SNOS412Q", DocRef: "snos412q", Page: 4, Section: "7.1 Absolute Maximum Ratings", Method: "hand", Confidence: 1.0}}})
	dss := backed.GetDatasheets()
	if len(dss) != 1 {
		t.Fatalf("datasheet-backed finding: want 1 citation on the wire, got %d", len(dss))
	}
	ds := dss[0]
	if ds == nil {
		t.Fatal("datasheet-backed finding has no datasheet citation on the wire")
	}
	if ds.GetDoc() != "SNOS412Q" || ds.GetPage() != 4 || ds.GetSection() != "7.1 Absolute Maximum Ratings" || ds.GetMethod() != "hand" {
		t.Errorf("citation = %+v", ds)
	}
	if plain := FindingProto(check.Finding{Subject: check.Entity{Kind: check.KindNet, Ref: "SIG"}, Rule: "single-pin-net"}); plain.GetDatasheets() != nil {
		t.Errorf("non-datasheet finding got a citation = %+v", plain.GetDatasheets())
	}
}

// TestPartitionAvailableReportsWhatCouldNotRun is the silence-reads-as-coverage failure at the
// finding tier.
//
// A board rule on a NETLIST cannot evaluate: check.Available gates it, it contributes no findings, and
// a findings list has no way to distinguish "checked and clean" from "never ran". On the viewer's
// default-open panel that reads as a healthy board, which is the bug this reports its way out of.
func TestPartitionAvailableReportsWhatCouldNotRun(t *testing.T) {
	// A netlist-only model: no board tier, no datasheet corpus.
	m := check.NewModel(&ir.Design{})
	rules := check.DefaultCatalog().Rules()
	runnable, skipped := partitionAvailable(rules, m)

	if len(skipped) == 0 {
		t.Fatal("a netlist supports only some of the shipped catalog; the rest must be reported, not omitted")
	}
	if len(runnable)+len(skipped) != len(rules) {
		t.Errorf("every selected rule belongs to exactly one half, got %d + %d of %d",
			len(runnable), len(skipped), len(rules))
	}
	for _, sk := range skipped {
		if sk.GetName() == "" {
			t.Error("a skipped rule must be named, or the panel cannot say which question went unanswered")
		}
		// The reason is check.Available's own words. A rule decides why it cannot run; a sentence
		// reconstructed here would be a second opinion that drifts from the gate itself.
		if sk.GetReason() == "" {
			t.Errorf("%s was skipped with no reason", sk.GetName())
		}
	}
	// The halves are disjoint by name, so nothing can be reported as both gated and run.
	byName := map[string]bool{}
	for _, r := range runnable {
		byName[r.Name] = true
	}
	for _, sk := range skipped {
		if byName[sk.GetName()] {
			t.Errorf("%s is in both halves", sk.GetName())
		}
	}
}

// TestPartitionAvailableChangesNoFindings: running only the runnable half is not an optimisation.
// check.Run already skips a gated rule, so the findings are identical either way — the split exists
// so the response can REPORT the other half instead of leaving a caller to infer it from an absence.
func TestPartitionAvailableChangesNoFindings(t *testing.T) {
	m := check.NewModel(&ir.Design{})
	rules := check.DefaultCatalog().Rules()
	runnable, _ := partitionAvailable(rules, m)
	if a, b := len(check.Run(m, rules)), len(check.Run(m, runnable)); a != b {
		t.Errorf("partitioning must not change what fires: %d findings over all rules, %d over the runnable half", a, b)
	}
}

// TestFindingProtoCarriesContext: a finding whose message names an entity other than its subject
// carries that entity to the client as structured data (agni issue 349), so the panel can render it
// as its own clickable chip instead of the reader parsing it out of the sentence.
//
// The order assertion is the load-bearing one. Order is the rule author's and matches the order the
// message names them, so a panel rendering chips left to right reads in the same order as the
// sentence above it. A conversion that sorted or bucketed would silently break that, and nothing
// else in the pipeline would notice.
func TestFindingProtoCarriesContext(t *testing.T) {
	f := FindingProto(check.Finding{Subject: check.Entity{Kind: check.KindComponent, Ref: "Q1"}, Rule: "load-switch-trip-current", Context: []check.ContextSubject{
		{Entity: check.Entity{Kind: check.KindComponent, Ref: "U3"}, Role: "controller"},
		{Entity: check.Entity{Kind: check.KindComponent, Ref: "R7"}, Role: "sense"},
	}})
	got := f.GetContext()
	if len(got) != 2 {
		t.Fatalf("context entries = %d, want 2", len(got))
	}
	if got[0].GetSubject().GetRef() != "U3" || got[0].GetRole() != "controller" {
		t.Errorf("first context = (%q, %q), want (U3, controller)", got[0].GetSubject().GetRef(), got[0].GetRole())
	}
	if got[1].GetSubject().GetRef() != "R7" || got[1].GetRole() != "sense" {
		t.Errorf("second context = (%q, %q), want (R7, sense) in the author's order", got[1].GetSubject().GetRef(), got[1].GetRole())
	}
	if got[0].GetSubject().GetKind() != check.KindComponent {
		t.Errorf("context kind = %q, want %q", got[0].GetSubject().GetKind(), check.KindComponent)
	}

	// The common case, and always will be: a message naming only its subject carries no context.
	plain := FindingProto(check.Finding{Subject: check.Entity{Kind: check.KindNet, Ref: "SIG"}, Rule: "single-pin-net"})
	if len(plain.GetContext()) != 0 {
		t.Errorf("a finding naming only its subject must carry no context, got %d", len(plain.GetContext()))
	}
}

// TestFindingProtoContextRolesNeedNotBeUnique: two entities can play the SAME part, which is exactly
// the i2c-address-collision shape ("A and B both strap to address N"). A consumer must therefore
// treat context as a list rather than a map, and this pins that so a later "tidy" keying by role
// cannot silently drop one.
func TestFindingProtoContextRolesNeedNotBeUnique(t *testing.T) {
	f := FindingProto(check.Finding{Subject: check.Entity{Kind: check.KindNet, Ref: "SDA"}, Rule: "i2c-address-collision", Context: []check.ContextSubject{
		{Entity: check.Entity{Kind: check.KindComponent, Ref: "U1"}, Role: "device"},
		{Entity: check.Entity{Kind: check.KindComponent, Ref: "U2"}, Role: "device"},
	}})
	if n := len(f.GetContext()); n != 2 {
		t.Fatalf("two entities in the same role must both survive, got %d", n)
	}
}

// TestFindingProtoContextBusJoinKey: a bus context entity needs its name on bus_id for the same
// reason a bus SUBJECT does — a bus carries no net, so its name is the only geometry join key it has.
// Without this a bus named as context renders as a chip that highlights nothing.
func TestFindingProtoContextBusJoinKey(t *testing.T) {
	f := FindingProto(check.Finding{Subject: check.Entity{Kind: check.KindNet, Ref: "STRAP0"}, Rule: "strap-group-collision", Context: []check.ContextSubject{{Entity: check.Entity{Kind: check.KindBus, Ref: "ADDR[3:0]"}, Role: "bus"}}})
	if id := f.GetContext()[0].GetSubject().GetBusId(); id != "ADDR[3:0]" {
		t.Errorf("bus context bus_id = %q, want the bus name", id)
	}
}
