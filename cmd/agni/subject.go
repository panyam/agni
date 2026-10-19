package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// parseSubjects reads `check --subject` values into the wire subjects a scoped CheckDesign takes
// (agni issue 897), in check.SubjectParam's spelling, which a verdict link's subject= carries too.
func parseSubjects(args []string) ([]*checkspb.Subject, error) {
	var out []*checkspb.Subject
	for _, a := range args {
		e, err := check.ParseSubjectParam(a)
		if err != nil {
			return nil, fmt.Errorf("--subject %w", err)
		}
		out = append(out, &checkspb.Subject{Kind: e.Kind, Ref: e.Ref, Pin: e.Pin})
	}
	return out, nil
}

// subjectEntities reads a verdict's wire subjects back into the entities a link names.
func subjectEntities(subs []*checkspb.Subject) []check.Entity {
	out := make([]check.Entity, 0, len(subs))
	for _, s := range subs {
		out = append(out, check.Entity{Kind: s.GetKind(), Ref: s.GetRef(), Pin: s.GetPin()})
	}
	return out
}

// noteDesignWide names the rules a scoped check left out because they conclude about the design as a
// whole, so a rule missing from the answer reads as not asked rather than as clean.
func noteDesignWide(w io.Writer, skipped []*webapi.SkippedRule) {
	var rules []string
	for _, s := range skipped {
		if s.GetDesignWide() {
			rules = append(rules, s.GetName())
		}
	}
	if len(rules) == 0 {
		return
	}
	sort.Strings(rules)
	fmt.Fprintf(w, "note: %d rule(s) conclude about the design as a whole and were not asked about --subject: %s. Run without --subject for their answer.\n",
		len(rules), strings.Join(rules, ", "))
}
