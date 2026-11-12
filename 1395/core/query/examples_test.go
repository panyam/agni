package query

import "testing"

// Every example is runnable UI, so a malformed one is a shipped bug. This test parses each one and
// requires its label/teaches copy. TestExamplesEvaluate in service/ evaluates each example on a
// real design. It also guards against a relation being renamed out from under an example.
func TestExamplesParseAndDescribe(t *testing.T) {
	ex := Examples()
	if len(ex) == 0 {
		t.Fatal("no examples")
	}
	for _, e := range ex {
		if _, err := Parse(e.Query); err != nil {
			t.Errorf("example %q does not parse: %v\n  query: %s", e.Label, err, e.Query)
		}
		if e.Label == "" || e.Teaches == "" {
			t.Errorf("example %q missing label/teaches copy", e.Query)
		}
	}
}
