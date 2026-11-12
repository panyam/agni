package reviewquery_test

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/review"
	_ "github.com/panyam/agni/stdlib/relations"   // the relations an inline query reads
	_ "github.com/panyam/agni/stdlib/reviewquery" // registers the compiler under test
	"github.com/panyam/jaala/ns"
)

// These tests run core/review's Validate through this package's registered compiler, as a real run
// does. core/review's own tests register a stub compiler, so they cannot.

const badQuery = "name: t\nareas: [{name: A, items: [{id: i, query: {match: 'garbage(', subject: r, message: m}}]}]"

const houseQuery = "name: t\nareas: [{name: A, items: [{id: i, query: {match: 'house.pmic_rail(?n) => ?n', subject: n, kind: net, message: 'rail {n}'}}]}]"

// TestValidateRefusesABadInlineQuery checks that Validate refuses an unparseable inline query (agni
// issue 779). Load reads a manifest before any design or project is known. It cannot compile a
// query that may name the run's own library, so it checks structure only. Validate has the run's
// vocabulary, so it compiles the query. Every caller validates before it runs, so a bad query still
// fails before anything is checked, one step later than it used to.
func TestValidateRefusesABadInlineQuery(t *testing.T) {
	m, err := review.Load(strings.NewReader(badQuery))
	if err != nil {
		t.Fatalf("Load refused a structurally sound manifest: %v", err)
	}
	err = review.Validate(m)
	if err == nil || strings.Contains(err.Error(), "no query compiler") {
		t.Errorf("err = %v, want the query's own parse error", err)
	}
}

// TestValidateReadsTheRunsVocabulary checks that a query naming a library member validates against
// a vocabulary that has the member. It is refused against one that does not. The error a manifest
// author sees therefore depends on the library the review will run with.
func TestValidateReadsTheRunsVocabulary(t *testing.T) {
	m, err := review.Load(strings.NewReader(houseQuery))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := review.Validate(m); err == nil || !strings.Contains(err.Error(), "house") {
		t.Errorf("against the shipped vocabulary: err = %v, want house unknown", err)
	}
	reg, err := facts.NewRegistry(append(facts.Registered(), facts.WithModules(ns.Module{
		Path: "house", Language: "datalog", Text: `pmic_rail(?n: net) :- net.rail(?n), str.prefix(?n, "PMIC_");`,
	}))...)
	if err != nil {
		t.Fatal(err)
	}
	if err := review.Validate(m, review.WithVocabulary(reg)); err != nil {
		t.Errorf("against a vocabulary holding house.pmic_rail: %v", err)
	}
}
