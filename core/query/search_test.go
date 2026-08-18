package query

import (
	"strings"
	"testing"
)

// The search template is runnable UI reached by typing, so a malformed one is a shipped bug no
// client test can catch: the browser substitutes the reader's term and runs whatever it was handed.
func TestSearchQueryParses(t *testing.T) {
	s := Search()
	if _, err := Parse(s.Query); err != nil {
		t.Fatalf("search template does not parse: %v\n  query: %s", err, s.Query)
	}
	if s.Teaches == "" {
		t.Error("search template has no teaches copy, so a search leaves nothing behind")
	}
}

// {term} must sit inside a string literal. Outside one it splices a bare token into the query,
// which parses as a variable or fails outright.
func TestSearchQueryPlaceholderIsQuoted(t *testing.T) {
	s := Search()
	if n := strings.Count(s.Query, "{term}"); n != 1 {
		t.Fatalf("want exactly one {term} placeholder, got %d in %s", n, s.Query)
	}
	seg := strings.SplitN(s.Query, "{term}", 2)
	if !strings.HasSuffix(seg[0], `"`) && !strings.Contains(seg[0], `"(?i)`) {
		t.Errorf("{term} does not open inside a string literal: %s", s.Query)
	}
	if !strings.HasPrefix(seg[1], `"`) {
		t.Errorf("{term} does not close inside a string literal: %s", s.Query)
	}
}

// A search that ranged over an association would silently miss a part with no connections, which is
// the blind spot entity() was added to close. Pin the relation so a well-meaning rewrite to
// component-on-net goes red here rather than in a review nobody runs.
func TestSearchQueryRangesOverEntity(t *testing.T) {
	q, err := Parse(strings.ReplaceAll(Search().Query, "{term}", "CAN"))
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
// tries twice and abandons, and (?i) is the whole of how this template says otherwise.
func TestSearchQueryIsCaseInsensitive(t *testing.T) {
	if !strings.Contains(Search().Query, "(?i)") {
		t.Errorf("search template is case-sensitive: %s", Search().Query)
	}
}
