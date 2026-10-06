package query

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
)

// aggFixture has no resistor at all, so every query below that asks about one matches nothing.
const noResistor = `component.class(?r,"resistor"), component.net(?r,?n)`

// TestAggregateOverNothingIsOneRow pins that with no group-by column the whole answer is one group,
// and it exists when nothing matched, so a count answers 0 rather than no rows (agni issue 726).
// COUNT and SUM are 0 and LIST is empty. MIN and MAX have no number to report, so they answer
// absent rather than an empty string that would read as a value (panyam/jaala#122).
func TestAggregateOverNothingIsOneRow(t *testing.T) {
	rows := runQuery(t, check.NewModel(aggFixture()),
		noResistor+` => count(?r), count(distinct ?r), list(?r), min(?r), max(?r), sum(?r)`)
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want exactly one row for an aggregate-only projection over nothing", rows)
	}
	b := rows[0].Bind
	for _, col := range []Var{"count(r)", "count(distinct r)", "sum(r)"} {
		if v := b[col]; v.S != "0" || v.Num == nil || *v.Num != 0 {
			t.Errorf("%s = %+v, want the number 0", col, v)
		}
	}
	for _, col := range []Var{"list(r)", "min(r)", "max(r)"} {
		v, ok := b[col]
		if !ok {
			t.Errorf("%s is missing from the row, want it present and empty", col)
			continue
		}
		if v.S != "" || v.Num != nil {
			t.Errorf("%s = %+v, want no value", col, v)
		}
		if wantAbsent := col != "list(r)"; v.Absent != wantAbsent {
			t.Errorf("%s absent = %v, want %v", col, v.Absent, wantAbsent)
		}
	}
	if len(rows[0].Cites) != 0 {
		t.Errorf("cites = %v, want none: nothing matched, so nothing is cited", rows[0].Cites)
	}
}

// TestAggregateOverSomethingStillCounts is the positive control for the test above. The same shape
// over a class the fixture does carry answers its real count, so the zero is not a constant.
func TestAggregateOverSomethingStillCounts(t *testing.T) {
	rows := runQuery(t, check.NewModel(aggFixture()),
		`component.class(?c,"capacitor"), component.net(?c,?n) => count(distinct ?c)`)
	if len(rows) != 1 || rows[0].Bind["count(distinct c)"].S != "3" {
		t.Errorf("rows = %+v, want one row counting C1, C2 and C3", rows)
	}
}

// TestGroupedAggregateOverNothingHasNoRows pins that a projection with a group-by column has no key
// to name a group by when nothing matched, so it stays empty, as SQL's GROUP BY does.
func TestGroupedAggregateOverNothingHasNoRows(t *testing.T) {
	rows := runQuery(t, check.NewModel(aggFixture()), noResistor+` => ?n, count(?r)`)
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want none", rows)
	}
}

// TestHavingStillFiltersTheEmptyGroup pins that having runs after the reduce, so it sees the zero
// and can reject it. Without that, "=> count(?r) having count(?r) > 0" would answer 0, which its
// own filter excludes.
func TestHavingStillFiltersTheEmptyGroup(t *testing.T) {
	m := check.NewModel(aggFixture())
	if rows := runQuery(t, m, noResistor+` => count(?r) having count(?r) > 0`); len(rows) != 0 {
		t.Errorf("having > 0: rows = %+v, want none", rows)
	}
	if rows := runQuery(t, m, noResistor+` => count(?r) having count(?r) = 0`); len(rows) != 1 {
		t.Errorf("having = 0: rows = %+v, want the one empty group", rows)
	}
}

// TestRuleFromQueryRejectsAnUnprojectedSubject exists because every answer row becomes a finding,
// and its subject is read off the row by name. An aggregate column is labelled count(r), not r, so
// a finding query projecting only aggregates has no subject to name. It used to compile and then
// report findings about the empty string, and since agni issue 726 it would do so over an empty
// design too.
func TestRuleFromQueryRejectsAnUnprojectedSubject(t *testing.T) {
	cases := map[string]FindingQuery{
		"subject behind an aggregate": {
			Query: MustParse(noResistor + ` => count(?r)`), Kind: check.KindComponent, SubjectVar: "r",
		},
		"pin not projected": {
			Query: MustParse(`pin.net(?ref, ?pin, ?net) => ?ref, ?net`), Kind: check.KindPin, SubjectVar: "ref", PinVar: "pin",
		},
		"tuple var not projected": {
			Query: MustParse(`pin.net(?ref, ?pin, ?net) => ?ref`), Kind: check.KindComponent, SubjectVar: "ref",
			TupleVars: []TupleVar{{Var: "net", Kind: check.KindNet}},
		},
		"domain does not project the subject": {
			Query: MustParse(`pin.net(?ref, ?pin, ?net) => ?ref`), Kind: check.KindComponent, SubjectVar: "ref",
			Domain: &Domain{Query: MustParse(`pin.net(?ref, ?pin, ?net) => ?net`)},
		},
	}
	for name, fq := range cases {
		fq.Rule = check.Rule{Name: "unprojected", Severity: "warning"}
		_, err := RuleFromQuery(fq)
		if err == nil || !strings.Contains(err.Error(), "does not project") {
			t.Errorf("%s: err = %v, want a complaint naming the unprojected variable", name, err)
		}
	}
}

// TestRuleFromQueryAcceptsAnImplicitProjection is the control for the rejection above. A query with
// no "=>" projects every goal variable, so its subject is a column and it must still compile.
func TestRuleFromQueryAcceptsAnImplicitProjection(t *testing.T) {
	_, err := RuleFromQuery(FindingQuery{
		Rule:  check.Rule{Name: "implicit", Severity: "warning"},
		Query: MustParse(`pin.net(?ref, ?pin, ?net)`), Kind: check.KindPin, SubjectVar: "ref", PinVar: "pin",
	})
	if err != nil {
		t.Errorf("err = %v, want an implicit projection to satisfy the subject", err)
	}
}
