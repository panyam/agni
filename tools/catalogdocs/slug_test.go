package main

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/stdlib/profiles"
	"github.com/panyam/agni/stdlib/rules/datalog"
	"github.com/panyam/agni/stdlib/rules/intent"
)

// TestPageSlugsUnique guards the reason non-built-in rules are namespaced: a rule name shared across
// sources (or a future collision, e.g. the datalog crystal-load-caps twin of the built-in) must not
// resolve to the same page slug, or one page would overwrite the other. It composes the same source
// list genRules uses and asserts every documented rule's slug is unique.
func TestPageSlugsUnique(t *testing.T) {
	sources := []struct {
		rules  []*check.Rule
		prefix string
	}{
		{check.BuiltinRules(), ""},
		{intent.DocRules(), "intent"},
		{datalog.DocRules(), "dl"},
		{profiles.DocRules(), "profile"},
	}
	seen := map[string]string{} // slug -> "prefix/name" that claimed it
	total := 0
	for _, s := range sources {
		for _, r := range s.rules {
			if strings.TrimSpace(r.Detail) == "" {
				continue
			}
			total++
			slug := pageSlug(s.prefix, r.Name)
			label := linkLabel(s.prefix, r.Name)
			if prev, dup := seen[slug]; dup {
				t.Errorf("slug %q claimed by both %q and %q", slug, prev, label)
			}
			seen[slug] = label
		}
	}
	if total == 0 {
		t.Fatal("no documented rules across any source")
	}
	// The non-built-in sources must actually contribute, or the catalog silently regressed to
	// built-ins only.
	for _, want := range []string{"intent-module-missing", "dl-power-pin-mistyped", "profile-signal-missing"} {
		if _, ok := seen[want]; !ok {
			t.Errorf("expected a page slug %q from a non-built-in source, none present", want)
		}
	}
}
