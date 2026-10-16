package service

import (
	"reflect"
	"sort"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/ir"
	"google.golang.org/protobuf/proto"
)

// fullFinding is the C26 fixture for check.Finding, with EVERY wire-carried field set to a
// distinguishable non-zero value and every repeated field holding more than one element, since a
// converter that keeps only the first element round-trips a one-element list cleanly.
func fullFinding() check.Finding {
	return check.Finding{
		Severity:     "warning",
		Inconclusive: true,
		Rule:         "gateway-profiles/can-host-incomplete",
		Subject:      check.Entity{Kind: check.KindPin, Ref: "U4", Pin: "7", NetID: "82ddd812ce0e"},
		Message:      "CAN host U4 is missing CANH and CANL",
		Prov:         &ir.Provenance{SourceFile: "designs/gateway/gateway.edn", NativeId: "U4"},
		Context: []check.ContextSubject{
			{Entity: check.Entity{Kind: check.KindNet, Ref: "CAN1_CANH", NetID: "9de85e683cf7"}, Role: "signal"},
			{Entity: check.Entity{Kind: check.KindPin, Ref: "U9", Pin: "2"}, Role: "source"},
		},
		DatasheetProv: []*check.DatasheetCitation{
			{Doc: "ACME-XLAT Rev A", DocRef: "ds", Page: 4, Section: "Absolute Maximum Ratings", Method: "hand", Confidence: 0.9, Verification: "verified", VerifiedRevision: "Rev A", Corpus: "project"},
			{Doc: "ACME-CAN Rev C", DocRef: "can", Page: 11, Section: "Pin Functions", Method: "docling", Confidence: 0.7, Verification: "unverified", VerifiedRevision: "Rev B", Corpus: "shared"},
		},
		VerdictIDs: []string{
			"gateway-profiles/can-host-incomplete:(component:U4,signal:CANH)",
			"gateway-profiles/can-host-incomplete:(component:U4,signal:CANL)",
		},
	}
}

// C26: FindingProto and FindingFromProto carry a deep-equality round-trip test. Provenance is a proto
// message held by pointer, so it is compared with proto.Equal and the rest with reflect.DeepEqual.
func TestFindingProtoRoundTrip(t *testing.T) {
	want := fullFinding()
	got := FindingFromProto(FindingProto(want))
	if !proto.Equal(got.Prov, want.Prov) {
		t.Errorf("provenance changed: got %v, want %v", got.Prov, want.Prov)
	}
	got.Prov, want.Prov = nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip lost or changed a field\n got: %+v\nwant: %+v", got, want)
	}
}

// A review item's findings travel inside its document, so the slice converters have to keep every
// finding and every id, in order.
func TestFindingsSurviveAReviewDocument(t *testing.T) {
	second := fullFinding()
	second.Rule, second.VerdictIDs = "copper-clearance", []string{"copper-clearance:(net:A,net:B)", "copper-clearance:(net:A,net:C)"}
	want := []check.Finding{fullFinding(), second}
	got := FindingsFromProto(FindingProtos(want))
	if len(got) != 2 {
		t.Fatalf("findings = %d, want 2", len(got))
	}
	for i := range want {
		if !reflect.DeepEqual(got[i].VerdictIDs, want[i].VerdictIDs) {
			t.Errorf("finding %d verdict ids = %v, want %v", i, got[i].VerdictIDs, want[i].VerdictIDs)
		}
	}
}

// The census C26 asks for: a new field on check.Finding fails here until someone decides whether it
// travels on the wire and adds it to the converters and to fullFinding.
func TestFindingFieldCensus(t *testing.T) {
	wire := []string{"Context", "DatasheetProv", "Inconclusive", "Message", "Prov", "Rule", "Severity", "Subject", "VerdictIDs"}
	var got []string
	rt := reflect.TypeOf(check.Finding{})
	for i := 0; i < rt.NumField(); i++ {
		got = append(got, rt.Field(i).Name)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, wire) {
		t.Errorf("check.Finding's fields changed.\n got: %v\nwant: %v\n\n"+
			"A new field is not covered by TestFindingProtoRoundTrip until it is in fullFinding(). "+
			"Add it to the proto, FindingProto and FindingFromProto, then list it here.", got, wire)
	}
}
