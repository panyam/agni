package service

import (
	"reflect"
	"testing"

	"github.com/panyam/agni/core/review"
)

// fullManifest exercises EVERY field of review.Manifest and its Binding, including the ones no single
// realistic checklist sets together (an item carries at most one selector at a time, so the selectors
// are spread across items). It is the fixture the round-trip guards, so a field added to the Go struct
// without a matching proto field fails here rather than silently dropping off the wire.
func fullManifest() review.Manifest {
	return review.Manifest{
		Name: "gateway review",
		Areas: []review.Area{
			{Name: "Power", Items: []review.Item{
				{
					ID: "P1", Title: "bulk capacitance", Description: "a rail with no bulk cap browns out",
					Note:    "presence-checked only",
					Binding: review.Binding{Rule: "bulk-cap", Scope: review.ScopeBinding{Profiles: []string{"CAN", "LIN"}}},
				},
				{
					ID: "P2", Title: "crystal load caps",
					Binding: review.Binding{Tag: "category=timing", AppliesToClass: []string{"crystal", "ceramic_resonator"}},
				},
			}},
			{Name: "Interfaces", Items: []review.Item{
				{
					ID: "I1", Title: "CAN ESD",
					Binding: review.Binding{Profile: "CAN", Requirement: "esd"},
				},
				{
					ID: "I2", Title: "output current within the datasheet limit",
					Binding: review.Binding{Query: &review.QueryBinding{
						Match:       "component.mpn(?r,\"X\") => ?r",
						Subject:     "r",
						Kind:        "component",
						Message:     "{r} exceeds its rated output",
						Severity:    "error",
						ParamSymbol: "IOUT",
					}},
				},
				{
					ID: "I3", Title: "a debug connector is fitted",
					Binding: review.Binding{Present: &review.PresentBinding{Class: "test_connector"}},
				},
				{ID: "I4", Title: "reviewed by hand", Note: "the EE signs this off"},
			}},
		},
	}
}

// TestManifestProtoRoundTrip is the field-drift guard. Going Go -> proto -> Go and requiring deep
// equality means a conversion that forgets a field cannot pass: the returned struct would differ from
// the one that went in. Asserting on the proto instead would not catch it, since a field the
// conversion never writes is simply absent from both sides of that comparison.
func TestManifestProtoRoundTrip(t *testing.T) {
	want := fullManifest()
	got := ManifestFromProto(ManifestProto(want))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip lost or changed a field:\n got %+v\nwant %+v", got, want)
	}
}

// TestManifestFromProtoNil: a nil manifest converts to the zero value rather than panicking, so an
// absent field is rejected by validation (with a message naming the manifest) instead of crashing the
// service on a request that simply omitted it.
func TestManifestFromProtoNil(t *testing.T) {
	if got := ManifestFromProto(nil); !reflect.DeepEqual(got, review.Manifest{}) {
		t.Errorf("ManifestFromProto(nil) = %+v, want zero Manifest", got)
	}
	if err := review.Validate(ManifestFromProto(nil)); err == nil {
		t.Error("Validate(zero manifest): want error, got nil")
	}
}

// TestManifestProtoOmitsAbsentSubMessages: an item with no query/present/scope converts to a binding
// carrying nil for each, not an empty message. A zero-valued PresentBinding on the way back would be a
// present binding with an empty class, which validation rejects, so a manifest with plain rule items
// would fail to round trip.
func TestManifestProtoOmitsAbsentSubMessages(t *testing.T) {
	m := review.Manifest{Name: "t", Areas: []review.Area{{Name: "A", Items: []review.Item{
		{ID: "1", Binding: review.Binding{Rule: "bulk-cap"}},
	}}}}
	b := ManifestProto(m).GetAreas()[0].GetItems()[0].GetBinding()
	if b.GetQuery() != nil || b.GetPresent() != nil || b.GetScope() != nil {
		t.Errorf("absent sub-messages materialized: query=%v present=%v scope=%v", b.GetQuery(), b.GetPresent(), b.GetScope())
	}
	if err := review.Validate(ManifestFromProto(ManifestProto(m))); err != nil {
		t.Errorf("round-tripped plain rule item fails validation: %v", err)
	}
}
