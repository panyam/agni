package query

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
)

const auditYAML = `
title: Audit
preamble: |
  has_tp(?n) :- component.net(?tp,?n), component.class(?tp,"test_point");
queries:
  - name: Probed nets
    query: entity(?n,"net"), has_tp(?n) => ?n
    description: nets carrying a test point
  - name: Parts
    query: component.net(?r,?n) => count(distinct ?r)
`

func TestParseQuerySetReadsEveryField(t *testing.T) {
	s, err := ParseQuerySet([]byte(auditYAML))
	if err != nil {
		t.Fatalf("ParseQuerySet: %v", err)
	}
	if s.Title != "Audit" || len(s.Queries) != 2 || s.Queries[0].Name != "Probed nets" || s.Queries[0].Description != "nets carrying a test point" {
		t.Errorf("set = %+v", s)
	}
	if !strings.Contains(s.Preamble, "has_tp") {
		t.Errorf("preamble = %q, want the has_tp rule", s.Preamble)
	}
}

// The preamble's rules reach every query, so a query naming a relation only the preamble defines
// answers over the design rather than failing as unknown.
func TestCompiledQueryCarriesThePreamble(t *testing.T) {
	s, err := ParseQuerySet([]byte(auditYAML))
	if err != nil {
		t.Fatal(err)
	}
	q, err := s.Compile(0)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	rows, err := Naive{}.Eval(context.Background(), q, NewBase(check.NewModel(aggFixture())))
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	var nets []string
	for _, r := range rows {
		nets = append(nets, r.Bind["n"].S)
	}
	if strings.Join(nets, ",") != "BOTH,DOUBLE,TPONLY" {
		t.Errorf("probed nets = %v, want BOTH,DOUBLE,TPONLY", nets)
	}
}

func TestQuerySetRejects(t *testing.T) {
	cases := map[string]struct{ yaml, want string }{
		"empty file":       {``, "empty file"},
		"no queries":       {`title: x`, "no queries"},
		"unknown key":      {"preambel: x\nqueries: [{name: a, query: 'entity(?n,?k)'}]", "preambel"},
		"unnamed query":    {`queries: [{query: 'entity(?n,?k)'}]`, "query 1 has no name"},
		"empty query":      {`queries: [{name: a}]`, `"a" has no query`},
		"repeated name":    {"queries: [{name: a, query: 'entity(?n,?k)'}, {name: a, query: 'entity(?n,?k)'}]", "named twice"},
		"goal in preamble": {"preamble: 'entity(?n,?k)'\nqueries: [{name: a, query: 'entity(?n,?k)'}]", "rules only"},
	}
	for name, tc := range cases {
		_, err := ParseQuerySet([]byte(tc.yaml))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want one mentioning %q", name, err, tc.want)
		}
	}
}

// A malformed query is its own problem. The set still validates, and Compile reports the one query.
func TestABadQueryDoesNotInvalidateTheSet(t *testing.T) {
	s, err := ParseQuerySet([]byte("queries: [{name: good, query: 'entity(?n,?k)'}, {name: bad, query: 'entity(?n'}]"))
	if err != nil {
		t.Fatalf("ParseQuerySet: %v", err)
	}
	if _, err := s.Compile(0); err != nil {
		t.Errorf("good query: %v", err)
	}
	if _, err := s.Compile(1); err == nil {
		t.Error("bad query compiled, want a parse error")
	}
}
