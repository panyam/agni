package query

import (
	"regexp"
	"strings"
	"testing"
)

// A preset is runnable UI reached by a click, so a malformed one is a shipped bug that no client
// test can catch, because the browser fills placeholders and runs whatever it was handed.
func TestEntityQueriesParse(t *testing.T) {
	for _, e := range EntityQueries() {
		if _, err := Parse(e.Query); err != nil {
			t.Errorf("preset for %q does not parse: %v\n  query: %s", e.Kind, err, e.Query)
		}
		if e.Kind == "" || e.Teaches == "" {
			t.Errorf("preset %q missing kind/teaches copy", e.Query)
		}
	}
}

// The client binds each variable a preset names from the field of the same name on the selection,
// so a preset binding one the selection does not carry would ask with that variable free, and one
// its goal does not use would be refused by the engine (agni issue 793).
func TestEntityQueriesBindWhatTheSelectionCarries(t *testing.T) {
	// Which variables each kind may bind, given what a selection of that kind carries.
	allowed := map[string]map[string]bool{
		"pin":       {"ref": true, "pin": true},
		"component": {"ref": true},
		"net":       {"net": true},
		"bus":       {"bus": true},
	}
	for _, e := range EntityQueries() {
		if len(e.Binds) == 0 {
			t.Errorf("preset for %q binds nothing, so every click asks the same question", e.Kind)
		}
		if strings.Contains(e.Query, "{") {
			t.Errorf("preset for %q still carries a text placeholder: %s", e.Kind, e.Query)
		}
		q, err := Parse(e.Query)
		if err != nil {
			continue // TestEntityQueriesParse reports it
		}
		vals := map[string]Value{}
		for _, v := range e.Binds {
			if !allowed[e.Kind][v] {
				t.Errorf("preset for %q binds ?%s, which a %s selection does not carry", e.Kind, v, e.Kind)
			}
			if !regexp.MustCompile(`\?` + v + `\b`).MatchString(e.Query) {
				t.Errorf("preset for %q binds ?%s, which its query does not use", e.Kind, v)
			}
			vals[v] = Text("x")
		}
		// Every bound variable must be an input: a projected one would answer the value it was given.
		for _, c := range q.Columns() {
			if _, ok := vals[string(c)]; ok {
				t.Errorf("preset for %q projects ?%s, which it binds", e.Kind, c)
			}
		}
	}
}

// Every kind a viewer can pick needs a preset, or clicking it silently does nothing.
func TestEntityQueriesCoverEveryPickableKind(t *testing.T) {
	have := map[string]bool{}
	for _, e := range EntityQueries() {
		if have[e.Kind] {
			t.Errorf("two presets for kind %q", e.Kind)
		}
		have[e.Kind] = true
	}
	for _, kind := range []string{"pin", "component", "net", "bus"} {
		if !have[kind] {
			t.Errorf("no preset for %q, so clicking one does nothing", kind)
		}
	}
}
