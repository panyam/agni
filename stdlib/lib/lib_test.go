package lib_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/query"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/stdlib/lib"
	"github.com/panyam/jaala/datalog"
	"github.com/panyam/jaala/ns"
)

// probeDesign has one part of every shape the library separates. R1 sits between two probed nets,
// R2 between a probed net and an unprobed one, R3 between two unprobed nets, and U1 on three nets,
// so it is not two-terminal at all. TP1 and TP2 are the test points that make A and B probed.
func probeDesign() *ir.Design {
	p := &ir.Provenance{SourceFile: "probe"}
	conn := func(ref, pin string) *ir.Connection { return &ir.Connection{ComponentRef: ref, PinRef: pin} }
	return &ir.Design{
		Components: []*ir.Component{
			{RefDes: "R1", Prov: p}, {RefDes: "R2", Prov: p}, {RefDes: "R3", Prov: p},
			{RefDes: "U1", Prov: p}, {RefDes: "TP1", Prov: p}, {RefDes: "TP2", Prov: p},
		},
		Nets: []*ir.Net{
			{Name: "A", Connections: []*ir.Connection{conn("R1", "1"), conn("R2", "1"), conn("U1", "1"), conn("TP1", "1")}, Prov: p},
			{Name: "B", Connections: []*ir.Connection{conn("R1", "2"), conn("U1", "2"), conn("TP2", "1")}, Prov: p},
			{Name: "C", Connections: []*ir.Connection{conn("R2", "2"), conn("R3", "1"), conn("U1", "3")}, Prov: p},
			{Name: "D", Connections: []*ir.Connection{conn("R3", "2")}, Prov: p},
		},
	}
}

// answers runs q over the probe design and returns each row's projected values joined by "/",
// sorted, so a comparison reads the whole answer at once.
func answers(t *testing.T, q string) []string {
	t.Helper()
	parsed := query.MustParse(q)
	rows, err := (query.Naive{}).Eval(parsed, query.NewBase(check.NewModel(probeDesign())))
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	var out []string
	for _, r := range rows {
		vals := make([]string, len(parsed.Select))
		for i, s := range parsed.Select {
			vals[i] = r.Bind[s.Var].S
		}
		out = append(out, strings.Join(vals, "/"))
	}
	sort.Strings(out)
	return out
}

func TestLibraryMembersAnswerOnTheProbeDesign(t *testing.T) {
	for q, want := range map[string]string{
		`net.has_test_point(?n) => ?n`:                     "A,B",
		`component.two_terminal(?r, ?a, ?b) => ?r, ?a, ?b`: "R1/A/B,R2/A/C,R3/C/D",
		`component.probed_both(?r) => ?r`:                  "R1",
		`component.probed_one(?r, ?p, ?u) => ?r, ?p, ?u`:   "R2/A/C",
		`component.two_terminal(?r, ?a, ?b), not component.probed_both(?r), not component.probed_one(?r, ?x, ?y) => ?r`: "R3",
	} {
		if got := strings.Join(answers(t, q), ","); got != want {
			t.Errorf("%s = %q, want %q", q, got, want)
		}
	}
}

// TestEveryMemberIsDocumentedAndTyped holds the library to what drill-down shows a reader: a doc, and
// argument types the author declared rather than ones the engine had to infer.
func TestEveryMemberIsDocumentedAndTyped(t *testing.T) {
	derived := facts.DefaultRegistry().Derived()
	if len(derived) < 4 {
		t.Fatalf("%d derived members registered, want the library's four at least", len(derived))
	}
	for _, d := range derived {
		e, err := query.Describe(d.Name)
		if err != nil {
			t.Fatalf("describe %s: %v", d.Name, err)
		}
		if e.Doc == "" {
			t.Errorf("%s has no doc comment above its first rule", d.Name)
		}
		for _, a := range e.Args {
			if a.Inferred || a.ArgType.IsZero() {
				t.Errorf("%s argument %s carries no declared type", d.Name, a.Name)
			}
		}
	}
}

// TestModulesThatReadEachOtherRegisterTogether is why lib registers its modules as one batch.
// component.dl reads net.has_test_point, and it sorts first, so one module at a time it is refused
// before net.dl arrives.
func TestModulesThatReadEachOtherRegisterTogether(t *testing.T) {
	base := facts.Registered()
	var ms []ns.Module
	for _, m := range lib.Modules() {
		ms = append(ms, ns.Module{Path: m.Path, Language: datalog.LanguageName, Text: m.Text})
	}
	// The process default already holds the library, registered last because it imports everything
	// else this binary registers, so compose from everything before it.
	var without []facts.Option
	without = append(without, base[:len(base)-1]...)
	if r, err := facts.NewRegistry(without...); err != nil || len(r.Derived()) != 0 {
		t.Fatalf("dropping the last registration did not drop exactly the library (err %v)", err)
	}
	if _, err := facts.NewRegistry(append(without, facts.WithModules(ms...))...); err != nil {
		t.Fatalf("the library composed as one batch was refused: %v", err)
	}
	one := append(append([]facts.Option(nil), without...), facts.WithModule(ms[0].Path, ms[0].Language, ms[0].Text))
	if _, err := facts.NewRegistry(one...); err == nil {
		t.Errorf("module %q composed alone although it reads a member of a module not yet registered", ms[0].Path)
	}
}

func TestABrokenModuleFailsComposition(t *testing.T) {
	bad := ns.Module{Path: "net", Language: datalog.LanguageName, Text: `broken(?n: net) :- net.no_such_relation(?n);`}
	if _, err := facts.NewRegistry(append(facts.Registered(), facts.WithModules(bad))...); err == nil {
		t.Error("a module reading a relation nobody registered composed; it must fail at load")
	}
}
