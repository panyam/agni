package composetest

import (
	"context"
	"os"
	"testing"

	"github.com/panyam/agni"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/readers/formats"
	_ "github.com/panyam/agni/stdlib/lib" // registers the shipped derived relations, probed_* among them
)

// The probed_* library members ask net.has_test_point about one net at a time. When that member
// joined component.net and component.class itself, every net asked about re-read every test point,
// so the cost grew as parts times test points: on the 1123-component sample board probed_both cost
// 518,388 units of work and probed_one 826,335, and on a 3,973-component board 7 million and 6.5 s.
// Read off _test_points, which is worked out once for every net, they cost 7,230 and 14,287 here.
// The ceilings below sit well above that and far below the old cost.
func TestProbedMembersCostLinearlyOnTheSampleBoard(t *testing.T) {
	const path = "../../tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_sch"
	if _, err := os.Stat(path); err != nil {
		// Fatal rather than skipped: a missing corpus must not read as a pass.
		t.Fatalf("%s missing (run `make samples-oracle`): %v", path, err)
	}
	e, err := agni.New()
	if err != nil {
		t.Fatal(err)
	}
	d, err := (&formats.Loader{}).ReadDesign(path)
	if err != nil {
		t.Fatal(err)
	}
	m := check.NewModel(d)
	for _, c := range []struct {
		q       string
		count   string
		ceiling int64
	}{
		{`component.probed_both(?r) => count(?r)`, "215", 50_000},
		{`component.probed_one(?r, ?p, ?u) => count(?r)`, "393", 100_000},
	} {
		b := query.NewBaseFrom(e.Registry(), m)
		rows, err := query.Default.Eval(context.Background(), query.MustParse(c.q), b)
		if err != nil {
			t.Fatalf("%s: %v", c.q, err)
		}
		if len(rows) != 1 || rows[0].Bind["count(r)"].S != c.count {
			t.Fatalf("%s answered %v, want a count of %s", c.q, rows, c.count)
		}
		if w := b.Work(); w > c.ceiling {
			t.Errorf("%s cost %d units of work, over its ceiling of %d: a member is re-deriving per net again", c.q, w, c.ceiling)
		}
	}
}
