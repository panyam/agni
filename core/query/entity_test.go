package query

import (
	"regexp"
	"strings"
	"testing"
)

// A preset is runnable UI reached by a click, so a malformed one is a shipped bug that no client
// test can catch: the browser fills placeholders and runs whatever it was handed.
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

// The client substitutes by placeholder name, so a preset naming one the client cannot fill would
// reach the reader with a literal "{whatever}" in the query.
func TestEntityQueriesUseKnownPlaceholders(t *testing.T) {
	known := map[string]bool{"ref": true, "pin": true, "net": true, "bus": true}
	// Which placeholders each kind may use, given what a selection of that kind carries.
	allowed := map[string]map[string]bool{
		"pin":       {"ref": true, "pin": true},
		"component": {"ref": true},
		"net":       {"net": true},
		"bus":       {"bus": true},
	}
	re := regexp.MustCompile(`\{(\w+)\}`)

	for _, e := range EntityQueries() {
		for _, m := range re.FindAllStringSubmatch(e.Query, -1) {
			name := m[1]
			if !known[name] {
				t.Errorf("preset for %q uses unknown placeholder {%s}", e.Kind, name)
			}
			if !allowed[e.Kind][name] {
				t.Errorf("preset for %q uses {%s}, which a %s selection does not carry", e.Kind, name, e.Kind)
			}
		}
		// A placeholder outside quotes would splice a bare token into the query, which parses as a
		// variable or fails; every one must sit inside a string literal.
		for _, seg := range strings.Split(e.Query, "{")[1:] {
			if !strings.HasPrefix(seg, "ref}\"") && !strings.HasPrefix(seg, "pin}\"") && !strings.HasPrefix(seg, "net}\"") && !strings.HasPrefix(seg, "bus}\"") {
				t.Errorf("preset for %q has a placeholder outside a string literal: {%s", e.Kind, seg)
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
