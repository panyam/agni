package graph

import (
	"sort"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestStrategiesHasGridAndIsSorted(t *testing.T) {
	all := Strategies()
	if len(all) == 0 {
		t.Fatal("no strategies registered")
	}
	names := make([]string, len(all))
	for i, s := range all {
		names[i] = s.Name
		if s.Place == nil {
			t.Errorf("strategy %q has nil Place", s.Name)
		}
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("Strategies() not name-sorted: %v", names)
	}
	if _, err := ByName(DefaultStrategy); err != nil {
		t.Errorf("default strategy %q not registered: %v", DefaultStrategy, err)
	}
}

func TestByNameUnknownListsChoices(t *testing.T) {
	_, err := ByName("no-such-layout")
	if err == nil {
		t.Fatal("want error for unknown layout, got nil")
	}
	// The error must name the real choices so a CLI typo is self-correcting.
	if msg := err.Error(); !strings.Contains(msg, "no-such-layout") || !strings.Contains(msg, DefaultStrategy) {
		t.Errorf("error should name the bad input and the choices, got: %v", err)
	}
}

// layout is the tests' grid-layout convenience (the former exported Layout wrapper, which
// had no callers outside this package's tests).
func layout(d *ir.Design, opts ...Option) *geom.SchematicGeometry {
	g, err := LayoutWith(d, "grid", opts...)
	if err != nil {
		panic(err)
	}
	return g
}
