package naming

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// A convention's value_unit reaches the lexicon a design is read with, and a bad one fails the load
// rather than reading as applied (agni issue 1285).
func TestAConventionsValueUnitsReachTheLexicon(t *testing.T) {
	cfg, err := Parse([]byte("name: house\nlexicon:\n  value_unit:\n    capacitor: uF_is_not_a_unit\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildLexicon(cfg); err == nil || !strings.Contains(err.Error(), "value_unit") {
		t.Errorf("an unreadable unit loaded: %v", err)
	}
	cfg, err = Parse([]byte("name: house\nlexicon:\n  value_unit:\n    capacitor: F\n"))
	if err != nil {
		t.Fatal(err)
	}
	lex, err := BuildLexicon(cfg)
	if err != nil {
		t.Fatal(err)
	}
	c := &ir.Component{RefDes: "C1", DeviceClasses: classify.Tags(string(classify.ClassCapacitor)), Attributes: map[string]string{"Value": "100"}}
	lex.StampValues(&ir.Design{Components: []*ir.Component{c}})
	if u := c.GetValue().GetUnit(); u != "F" {
		t.Errorf("a bare capacitor value read under the convention has unit %q, want F", u)
	}
}
