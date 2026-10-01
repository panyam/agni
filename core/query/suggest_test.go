package query

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/facts"
)

func TestSuggestRelation(t *testing.T) {
	cases := []struct{ in, want string }{
		{"compnent.net", "component.net"},     // one dropped letter
		{"net.max_volage", "net.max_voltage"}, // one dropped letter
		{"net.reches", "net.reaches"},         // transposition-ish
		{"contans", "contains"},
		{"xyzzy", ""},      // unrelated -> no misleading suggestion
		{"components", ""}, // close to nothing within threshold
	}
	for _, tc := range cases {
		if got := suggestRelation(facts.DefaultRegistry(), tc.in); got != tc.want {
			t.Errorf("suggestRelation(facts.DefaultRegistry(), %q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDidYouMeanFormatsOrEmpty(t *testing.T) {
	if h := didYouMean(facts.DefaultRegistry(), "compnent.net"); !strings.Contains(h, "did you mean") || !strings.Contains(h, "component.net") {
		t.Errorf("hint = %q, want a component.net suggestion", h)
	}
	if h := didYouMean(facts.DefaultRegistry(), "xyzzy"); h != "" {
		t.Errorf("hint for an unrelated token = %q, want empty", h)
	}
}

// suggestRelation is the name didYouMean suggests, or "" when it suggests none. The ranking moved into
// the engine with agni issue 731; this reads its answer back out of the hint it formats.
func suggestRelation(reg *facts.Registry, name string) string {
	const pre = `; did you mean "`
	h := didYouMean(reg, name)
	if !strings.HasPrefix(h, pre) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(h, pre), `"?`)
}
