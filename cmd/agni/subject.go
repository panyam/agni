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
// (agni issue 897). Each is kind:ref, the spelling a verdict id gives its elements, with a pin's ref
// and designator joined by its last dot, so `pin:U7.3` is pin 3 of U7.
func parseSubjects(args []string) ([]*checkspb.Subject, error) {
	var out []*checkspb.Subject
	for _, a := range args {
		kind, ref, ok := strings.Cut(a, ":")
		if !ok || ref == "" {
			return nil, fmt.Errorf("--subject %q: want net:NAME, component:REF or pin:REF.PIN", a)
		}
		switch kind {
		case check.KindNet, check.KindComponent:
			out = append(out, &checkspb.Subject{Kind: kind, Ref: ref})
		case check.KindPin:
			i := strings.LastIndex(ref, ".")
			if i <= 0 || i == len(ref)-1 {
				return nil, fmt.Errorf("--subject %q: a pin is pin:REF.PIN, such as pin:U7.3", a)
			}
			out = append(out, &checkspb.Subject{Kind: kind, Ref: ref[:i], Pin: ref[i+1:]})
		default:
			return nil, fmt.Errorf("--subject %q: kind %q is not one a rule answers for (want net, component or pin)", a, kind)
		}
	}
	return out, nil
}

// noteDesignWide names the rules a scoped check left out because they conclude about the design as a
// whole, so a rule missing from the answer reads as not asked rather than as clean.
func noteDesignWide(w io.Writer, skipped []*webapi.SkippedRule) {
	var rules []string
	for _, s := range skipped {
		if s.GetReason() == check.DesignWideReason {
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
