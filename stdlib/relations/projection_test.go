package relations

import (
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/formats"
	_ "github.com/panyam/agni/readers/edif"
	_ "github.com/panyam/agni/readers/kicad"
)

// projectionModels are designs that between them project most relation families: the tutorial
// gateway read with its board and its project's datasheet corpus, and the fixtures that carry a rail
// voltage, a net class, ESD ratings and a typical value.
func projectionModels(t *testing.T) []check.Model {
	t.Helper()
	dir := "../../examples/tutorial-project/"
	l := &formats.Loader{}
	d, err := l.ReadDesign(dir + "designs/gateway/gateway.edn")
	if err != nil {
		t.Fatal(err)
	}
	b, err := l.BoardGeometry(dir + "designs/gateway/gateway.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	specs, err := param.LoadSet(os.DirFS(dir + "params"))
	if err != nil {
		t.Fatal(err)
	}
	esd := &ir.Design{Components: []*ir.Component{
		{RefDes: "U9", Mpn: "DEMO-XCVR", Prov: &ir.Provenance{SourceFile: "t"}},
	}}
	return []check.Model{
		check.NewModel(d, check.WithBoard(b), check.WithParamProvider(specs)),
		everyNumericFamily(),
		check.NewModel(esd, check.WithParamProvider(param.ParamSet{"DEMO-XCVR": esdSpec("DEMO-XCVR", 8000)})),
		check.NewModel(capDesign("+10V", "DEMO-CAP-6V3"), check.WithParamProvider(param.ParamSet{"DEMO-CAP-6V3": capSpec("DEMO-CAP-6V3", 6.3)})),
	}
}

// TestEveryRelationHasOneProjectorGroup holds modelGroups to the schema: each built-in relation is
// listed by exactly one group, so a fact base read a relation at a time finds every relation's rows.
func TestEveryRelationHasOneProjectorGroup(t *testing.T) {
	owners := map[string]int{}
	for _, g := range modelGroups {
		for _, r := range g.Relations {
			owners[r]++
		}
	}
	for rel := range builtinSchema {
		if owners[rel] != 1 {
			t.Errorf("relation %q is listed by %d projector groups, want 1", rel, owners[rel])
		}
	}
	for rel := range owners {
		if _, ok := builtinSchema[rel]; !ok {
			t.Errorf("a projector group lists %q, which the schema does not have", rel)
		}
	}
}

// TestEveryProjectorGroupEmitsOnlyWhatItLists runs each group over the projection models. A group
// emitting a relation it does not list would leave that relation empty when a query read it first.
func TestEveryProjectorGroupEmitsOnlyWhatItLists(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range projectionModels(t) {
		for _, g := range modelGroups {
			listed := map[string]bool{}
			for _, r := range g.Relations {
				listed[r] = true
			}
			for _, row := range g.Project(m) {
				seen[row.Relation] = true
				if !listed[row.Relation] {
					t.Errorf("the group listing %v emitted a %q row", g.Relations, row.Relation)
				}
			}
		}
	}
	// 41 of the 55 are exercised. The rest are each a group of one relation whose projector names
	// only that relation (the reader diagnostics, buses, the naming-lexicon roles, and the datasheet's
	// per-pin limits and audience), so none can emit under another group's name.
	if len(seen) < 40 {
		t.Fatalf("the models exercised %d relations of %d, too few for a clean sweep to mean much", len(seen), len(builtinSchema))
	}
}

// TestAProjectionListsWhatFactsDoes reads every relation through a Projection, one at a time, and
// compares it with that relation's rows in Facts, order included.
func TestAProjectionListsWhatFactsDoes(t *testing.T) {
	reg := facts.DefaultRegistry()
	rels := make([]string, 0, len(builtinSchema))
	for rel := range builtinSchema {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	compared := 0
	for _, m := range projectionModels(t) {
		want := factsByRelation(Facts(m))
		p := reg.Projection(m)
		for _, rel := range rels {
			got := p.Rows(rel)
			if !reflect.DeepEqual(got, want[rel]) && (len(got) > 0 || len(want[rel]) > 0) {
				t.Errorf("%s: the projection lists %d rows and Facts %d, or in another order", rel, len(got), len(want[rel]))
			}
			compared += len(got)
		}
	}
	if compared == 0 {
		t.Fatal("no rows compared")
	}
}
