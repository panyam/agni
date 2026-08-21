package review

import (
	"testing"

	"github.com/panyam/agni/core/check"
)

func projReport() Report {
	return Report{Areas: []AreaResult{{Area: Area{Name: "A"}, Items: []ItemResult{
		{Item: Item{ID: "1", Title: "esd"}, Outcome: Fail, Findings: []check.Finding{
			{Subject: check.Entity{Kind: check.KindNet, Ref: "VBUS"}, Rule: "esd", Message: "no tvs"},
			{Subject: check.Entity{Kind: check.KindNet, Ref: "CAN_H"}, Rule: "esd", Message: "no tvs"},
		}},
		{Item: Item{ID: "2", Title: "pin roles"}, Outcome: Fail, Findings: []check.Finding{
			{Subject: check.Entity{Kind: check.KindPin, Ref: "U1", Pin: "3"}, Rule: "role", Message: "mistyped"},
			{Subject: check.Entity{Kind: check.KindPin, Ref: "U1", Pin: "7"}, Rule: "role", Message: "mistyped"},
		}},
		{Item: Item{ID: "3", Title: "uvlo"}, Outcome: NeedsData,
			Note:  "no seeded datasheet value for IOUT on this design",
			Unmet: []check.UnmetDependency{{MPN: "ACME-1", Symbol: "IOUT"}}},
		{Item: Item{ID: "4", Title: "budget"}, Outcome: NeedsData,
			Unmet: []check.UnmetDependency{{MPN: "ACME-1", Symbol: "IOUT"}, {MPN: "ACME-2", Symbol: "VIN"}}},
		{Item: Item{ID: "5", Title: "clean"}, Outcome: Pass},
	}}}}
}

// The property an entity-first UI rests on: grouping by subject partitions the findings, so the
// union over every subject is the whole report with nothing duplicated and nothing lost.
func TestProjectionPartitionsFindings(t *testing.T) {
	r := projReport()
	total := 0
	for _, ar := range r.Areas {
		for _, it := range ar.Items {
			total += len(it.Findings)
		}
	}
	seen := 0
	for _, v := range ForSubjects(r, SubjectsOf(r)) {
		for _, it := range v.Items {
			seen += len(it.Findings)
		}
	}
	if seen != total {
		t.Errorf("union over subjects saw %d findings, report has %d", seen, total)
	}
}

func TestForSubjectScopesToThatEntity(t *testing.T) {
	v := ForSubject(projReport(), Subject{Kind: check.KindNet, Subject: "VBUS"})
	if len(v.Items) != 1 || len(v.Items[0].Findings) != 1 {
		t.Fatalf("want the one esd finding on VBUS, got %+v", v.Items)
	}
	if got := check.EntityRef(v.Items[0].Findings[0].Subject); got != "VBUS" {
		t.Errorf("leaked a sibling subject: %q", got)
	}
	if v.Items[0].Outcome != Fail {
		t.Error("the item's outcome is a property of the item and must not be rewritten by the view")
	}
}

// A subject with no pin asks about the whole component, which is what lets a caller inspect a part
// without first enumerating its terminals.
func TestPinlessSubjectMatchesEveryPin(t *testing.T) {
	all := ForSubject(projReport(), Subject{Kind: check.KindPin, Subject: "U1"})
	if len(all.Items) != 1 || len(all.Items[0].Findings) != 2 {
		t.Fatalf("want both pin findings on U1, got %+v", all.Items)
	}
	one := ForSubject(projReport(), Subject{Kind: check.KindPin, Subject: "U1", Pin: "3"})
	if len(one.Items) != 1 || len(one.Items[0].Findings) != 1 {
		t.Fatalf("want only pin 3, got %+v", one.Items)
	}
}

// An entity nothing examined is an answer, not an absence. Omitting it would let a caller read
// "no view" as "nothing wrong", which is the confusion the outcome vocabulary exists to prevent.
func TestUnexaminedEntityStillYieldsAView(t *testing.T) {
	v := ForSubject(projReport(), Subject{Kind: check.KindNet, Subject: "NOWHERE"})
	if len(v.Items) != 0 {
		t.Errorf("want no items, got %+v", v.Items)
	}
	if v.Subject.Subject != "NOWHERE" {
		t.Error("the view must still name the subject it was asked about")
	}
}

// A blocked item has no findings by construction, so it has no subject. It is reported against every
// entity because the honest reading is that it could not answer for anything, this entity included.
func TestBlockedItemsReachEveryView(t *testing.T) {
	v := ForSubject(projReport(), Subject{Kind: check.KindNet, Subject: "VBUS"})
	if len(v.Blocked) != 2 {
		t.Fatalf("want both needs-data items, got %d", len(v.Blocked))
	}
	if len(v.Unmet) != 2 {
		t.Fatalf("one fact blocking two items must appear once: %+v", v.Unmet)
	}
	if v.Unmet[0].MPN != "ACME-1" || v.Unmet[1].MPN != "ACME-2" {
		t.Errorf("want deterministic order, got %+v", v.Unmet)
	}
}

// SubjectsOf reports what the RUN examined, never what the design contains, and the gap between
// those is exactly why an entity panel cannot stand in for a review pass.
func TestSubjectsOfEnumeratesAttentionNotTheDesign(t *testing.T) {
	got := SubjectsOf(projReport())
	if len(got) != 4 {
		t.Fatalf("want the four subjects findings mention, got %d: %+v", len(got), got)
	}
	for _, s := range got {
		if s.Subject == "" {
			t.Error("a subject with no name cannot be located")
		}
	}
}
