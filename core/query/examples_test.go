package query

import "testing"

// Every example is runnable UI, so a malformed one is a shipped bug: this parses each and requires
// its label/teaches copy. (Eval-on-a-real-design is the RPC-level test in internal/service, which
// also guards against a relation being renamed out from under an example.)
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
