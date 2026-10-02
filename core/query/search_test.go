package query

import (
	"regexp"
	"strings"
	"testing"
)

// The search template is runnable UI reached by typing, so a malformed one is a shipped bug no
// client test can catch, because the browser substitutes the reader's term and runs whatever it was
// handed.
func TestSearchQueryParses(t *testing.T) {
	s := Search()
	if _, err := Parse(s.Query); err != nil {
		t.Fatalf("search template does not parse: %v\n  query: %s", err, s.Query)
	}
	if s.Teaches == "" {
		t.Error("search template has no teaches copy, so a search leaves nothing behind")
	}
}

// The query binds the variable Bind names, and the value carries the reader's term in place of
// {term}, so a search builds no query text (agni issue 793).
func TestSearchQueryBindsItsPattern(t *testing.T) {
	s := Search()
	if strings.Contains(s.Query, "{") {
		t.Errorf("search query still carries a text placeholder: %s", s.Query)
	}
	if s.Bind == "" || !regexp.MustCompile(`\?`+regexp.QuoteMeta(s.Bind)+`\b`).MatchString(s.Query) {
		t.Errorf("search binds %q, which its query %s does not use", s.Bind, s.Query)
	}
	if n := strings.Count(s.Pattern, "{term}"); n != 1 {
		t.Errorf("want exactly one {term} in the pattern, got %d in %q", n, s.Pattern)
	}
	q, err := Parse(s.Query)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, c := range q.Columns() {
		if string(c) == s.Bind {
			t.Errorf("search projects ?%s, so every row would answer the pattern it was given", s.Bind)
		}
	}
}

// A search that ranged over an association would silently miss a part with no connections, which is
// the blind spot entity() was added to close. Pin the relation so a well-meaning rewrite to
// component.net goes red here rather than in a review nobody runs.
func TestSearchQueryRangesOverEntity(t *testing.T) {
	q, err := Parse(Search().Query)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	found := false
	for _, lit := range q.Goal.Literals {
		if lit.Pos != nil && lit.Pos.Relation == "entity" {
			found = true
		}
	}
	if !found {
		t.Errorf("search does not range over entity(): %s", Search().Query)
	}
}

// The term is matched case-insensitively. A search box that only matches case is one a newcomer
// tries twice and abandons, and (?i) is the only thing in the served pattern that says otherwise.
func TestSearchQueryIsCaseInsensitive(t *testing.T) {
	if !strings.HasPrefix(Search().Pattern, "(?i)") {
		t.Errorf("search pattern is case-sensitive: %s", Search().Pattern)
	}
}
