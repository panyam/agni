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
	bus := FindingProto(check.Finding{
		Rule: "bus-not-modeled", Kind: check.KindBus, Subject: "DATA[7:0]",
		Prov: &ir.Provenance{SourceFile: "x.kicad_sch"},
	})
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
	net := FindingProto(check.Finding{
		Rule: "single-pin-net", Kind: check.KindNet, Subject: "SIG",
	})
	if net.GetSubject().GetBusId() != "" {
		t.Errorf("net subject bus_id = %q, want empty", net.GetSubject().GetBusId())
	}
}

// TestFindingProtoCarriesDatasheet: a datasheet-backed finding carries its citation on the wire
// (WS9-048), so the CLI's review/check --format json and the web check panel both show the source
// without parsing the message; a finding with no datasheet provenance leaves the field nil.
func TestFindingProtoCarriesDatasheet(t *testing.T) {
	backed := FindingProto(check.Finding{
		Rule: "supply-exceeds-abs-max", Kind: check.KindComponent, Subject: "U1",
		DatasheetProv: []*check.DatasheetCitation{{Doc: "SNOS412Q", DocRef: "snos412q", Page: 4, Section: "7.1 Absolute Maximum Ratings", Method: "hand", Confidence: 1.0}},
	})
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
	if plain := FindingProto(check.Finding{Rule: "single-pin-net", Kind: check.KindNet, Subject: "SIG"}); plain.GetDatasheets() != nil {
		t.Errorf("non-datasheet finding got a citation = %+v", plain.GetDatasheets())
	}
}
