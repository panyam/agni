package main

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/internal/expect"
)

// TestConformance is the reader -> IR -> check regression gate (WS6-004), driven by expectation
// sidecars (WS6-006). Each fixture in testdata/conformance has a <fixture>.expect.yaml. The sidecar
// names the rules that must fire and their exact subjects. Adding a case means adding a fixture and
// a sidecar, with no edit here. The test runs at the CLI edge so the reader is in the loop. A fixture
// that should fire but does not points to a rule bug or a reader gap, and that loop surfaced WS1-010
// and WS1-006. Fixtures are our own synthetic files (C10), never fetched corpus material.
//
// The assertion has three parts. Every `fires` rule must produce exactly its listed subjects. A rule
// that fires but is listed in neither `fires` nor `pending` is an unexpected finding and fails, and
// this guards against over-firing. `pending` rules are staged before they exist, and they are
// neither required nor forbidden.
func TestConformance(t *testing.T) {
	sidecars, err := filepath.Glob(filepath.Join("testdata", "conformance", "*.expect.yaml"))
	if err != nil {
		t.Fatalf("glob sidecars: %v", err)
	}
	if len(sidecars) == 0 {
		t.Fatal("no expectation sidecars found under testdata/conformance")
	}
	for _, sc := range sidecars {
		fixture := strings.TrimSuffix(sc, ".expect.yaml")
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			exp, err := expect.Load(sc)
			if err != nil {
				t.Fatalf("load expectations: %v", err)
			}
			// Every fixture runs with the shared seeded params dir (WS10-003), so
			// datasheet-backed rules are in the exhaustive-firing contract like any
			// other rule; fixtures whose parts carry no seeded MPN are unaffected.
			m, err := readModelWithParams(fixture, filepath.Join("testdata", "conformance", "params"))
			if err != nil {
				t.Fatalf("readModel: %v", err)
			}
			got := map[string][]string{}
			for _, f := range check.RunBackground(m, check.BuiltinRules()) {
				got[f.Rule] = append(got[f.Rule], f.Subject.Ref)
			}
			for rule, e := range exp.Fires {
				if !equalSubjects(got[rule], e.Subjects) {
					msg := ""
					if e.Why != "" {
						msg = " (" + e.Why + ")"
					}
					t.Errorf("%s: fired %v, expected %v%s", rule, sortedCopy(got[rule]), sortedCopy(e.Subjects), msg)
				}
			}
			for rule := range got {
				if _, ok := exp.Fires[rule]; ok {
					continue
				}
				if _, ok := exp.Pending[rule]; ok {
					continue // staged, not yet asserted
				}
				t.Errorf("unexpected finding from %q on %v (not in fires or pending)", rule, sortedCopy(got[rule]))
			}
		})
	}
}

// TestConformanceSpecParity holds each rule's declarative twin (check.Specs, WS3-003) to its
// Go Eval over every conformance fixture. These are the same designs the harness gates, but through
// real reader output rather than hand-built IR, so reader-shaped data (attributes, section
// pin maps, diagnostics) exercises the interpreter too.
func TestConformanceSpecParity(t *testing.T) {
	sidecars, err := filepath.Glob(filepath.Join("testdata", "conformance", "*.expect.yaml"))
	if err != nil || len(sidecars) == 0 {
		t.Fatalf("glob sidecars: %v (%d found)", err, len(sidecars))
	}
	for _, sc := range sidecars {
		fixture := strings.TrimSuffix(sc, ".expect.yaml")
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			m, err := readModel(fixture)
			if err != nil {
				t.Fatalf("readModel: %v", err)
			}
			for _, r := range check.BuiltinRules() {
				spec, ok := check.BuiltinSpecs()[r.Name]
				if !ok {
					continue // spec-only rule, whose Eval is the interpreter, so nothing to compare
				}
				if got, want := spec.Eval(context.Background(), m), r.Findings(context.Background(), m); !reflect.DeepEqual(got, want) {
					t.Errorf("%s: spec findings diverge\n spec: %+v\n   go: %+v", r.Name, got, want)
				}
			}
		})
	}
}

// equalSubjects compares two subject lists order-independently, treating nil and empty as equal.
func equalSubjects(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := sortedCopy(a), sortedCopy(b)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func sortedCopy(s []string) []string {
	c := append([]string(nil), s...)
	sort.Strings(c)
	return c
}
