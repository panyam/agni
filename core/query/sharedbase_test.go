package query

import (
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Every query-backed rule over one model reads the same fact base, built once (agni issue 810), and
// a different model gets its own.
func TestSharedBaseIsOnePerModel(t *testing.T) {
	reg := facts.DefaultRegistry()
	m := check.NewModel(&ir.Design{})
	if sharedBase(reg, m) != sharedBase(reg, m) {
		t.Error("two rules over one model built two fact bases")
	}
	if sharedBase(reg, check.NewModel(&ir.Design{})) == sharedBase(reg, m) {
		t.Error("two models shared one fact base")
	}
}
