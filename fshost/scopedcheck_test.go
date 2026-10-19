package fshost

import (
	"testing"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

func netSubject(name string) []*checkspb.Subject {
	return []*checkspb.Subject{{Kind: check.KindNet, Ref: name}}
}

// A check asked about one net after the whole design was checked answers from the kept run, narrowed
// to that net, without evaluating the rule again (agni issue 897).
func TestAScopedCheckAnswersFromAKeptRun(t *testing.T) {
	f := newCheckFixture(t, check.Fail, true, nil)
	f.counted(t, &webapi.CheckDesignRequest{})
	f.wantEvals(t, 1, "the whole check")

	resp := f.check(t, &webapi.CheckDesignRequest{Rules: []string{countedRule}, Subjects: netSubject("GND")})
	f.wantEvals(t, 1, "a check of one net ran the rule again though its whole run was kept")
	if len(resp.GetVerdicts()) != 1 || resp.GetVerdicts()[0].GetSubjects()[0].GetRef() != "GND" {
		t.Fatalf("asked about GND, got verdicts %v, want the one about GND", resp.GetVerdicts())
	}

	resp = f.check(t, &webapi.CheckDesignRequest{Rules: []string{countedRule}, Subjects: netSubject("VCC")})
	if len(resp.GetVerdicts()) != 0 || len(resp.GetFindings()) != 0 {
		t.Fatalf("asked about VCC, which the rule says nothing about, got %d verdicts and %d findings",
			len(resp.GetVerdicts()), len(resp.GetFindings()))
	}
}

// A scoped run answers for some nets, so keeping it as the rule's run would answer a later whole
// check with that subset.
func TestAScopedRunIsNotKept(t *testing.T) {
	f := newCheckFixture(t, check.Fail, true, nil)
	f.check(t, &webapi.CheckDesignRequest{Rules: []string{countedRule}, Subjects: netSubject("VCC")})
	f.wantEvals(t, 1, "the scoped check")
	resp := f.check(t, &webapi.CheckDesignRequest{Rules: []string{countedRule}})
	f.wantEvals(t, 2, "the whole check was answered from a scoped run")
	if len(resp.GetVerdicts()) != 1 {
		t.Fatalf("the whole check gave %d verdicts, want the rule's one", len(resp.GetVerdicts()))
	}
}

// A design-wide rule has no answer for one net, so a scoped check names it as skipped rather than
// reporting it clean, and a whole check runs it as before.
func TestADesignWideRuleIsSkippedUnderAScope(t *testing.T) {
	f := newCheckFixture(t, check.Fail, true, nil)
	const rule = "symbol-unresolved"
	resp := f.check(t, &webapi.CheckDesignRequest{Rules: []string{rule}, Subjects: netSubject("GND")})
	if len(resp.GetSkipped()) != 1 || resp.GetSkipped()[0].GetName() != rule || resp.GetSkipped()[0].GetReason() != check.DesignWideReason || !resp.GetSkipped()[0].GetDesignWide() {
		t.Fatalf("scoped check skipped %v, want %s as design-wide", resp.GetSkipped(), rule)
	}
	if len(resp.GetVerdicts()) != 0 || len(resp.GetFindings()) != 0 {
		t.Fatalf("a skipped rule answered: %d verdicts, %d findings", len(resp.GetVerdicts()), len(resp.GetFindings()))
	}
	resp = f.check(t, &webapi.CheckDesignRequest{Rules: []string{rule}})
	for _, s := range resp.GetSkipped() {
		if s.GetName() == rule {
			t.Fatalf("a whole check skipped %s: %s", rule, s.GetReason())
		}
	}
}
