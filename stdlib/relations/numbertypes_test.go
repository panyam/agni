package relations

import (
	"testing"

	"github.com/panyam/agni/core/facts"
	"github.com/panyam/jaala/ns"
)

// TestEveryNumericSlotIsTypedANumber: a relation position filled from a numeric slot declares itself a
// number, so the engine reads a constant or bound value there as one (panyam/jaala#65). Undeclared, a
// text "3" compared with a count answers nothing, and "abc" is not refused.
func TestEveryNumericSlotIsTypedANumber(t *testing.T) {
	reg := facts.DefaultRegistry()
	vocab := reg.Vocabulary()
	typed := 0
	for _, name := range vocab.BaseRelations() {
		fields, ok := reg.SchemaOf(name)
		if !ok {
			continue
		}
		s, _ := vocab.Schema(name)
		for i, f := range fields {
			if f != facts.FieldNum && f != facts.FieldMin {
				continue
			}
			if i >= len(s.Types) || s.Types[i].Type != ns.TypeNumber {
				t.Errorf("%s position %d holds a number and is not declared one", name, i)
				continue
			}
			typed++
		}
	}
	// The catalog has twenty numeric positions across eighteen relations. Fewer means the loop above
	// saw relations it could not read, and passed over them.
	if typed < 20 {
		t.Errorf("checked %d numeric positions, want at least 20", typed)
	}
}
