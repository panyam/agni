package query

import (
	"fmt"
	"sort"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// numDesign carries the same logical number written three ways on three nets, so a query over it
// exercises every branch of valueEq: two numerics, two strings, and the mixed pair.
func numDesign() *ir.Design {
	d := &ir.Design{}
	for _, n := range []string{"10", "10.0", "N10"} {
		d.Nets = append(d.Nets, &ir.Net{Name: n, Prov: &ir.Provenance{SourceFile: "n"}})
	}
	for i := 0; i < 40; i++ { // over indexMinFacts, so the indexed path is the one under test
		ref := fmt.Sprintf("R%d", i)
		d.Components = append(d.Components, &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "n"}})
		d.Nets[i%len(d.Nets)].Connections = append(d.Nets[i%len(d.Nets)].Connections,
			&ir.Connection{ComponentRef: ref, PinRef: "1"})
	}
	return d
}

// The index buckets by value, and valueEq is NOT transitive: {S:"10.0",Num:10} equals
// {S:"10",Num:10} numerically, which equals {S:"10",Num:nil} by string, while the first and last are
// unequal. A keying scheme that canonicalised numbers would merge all three and change what the
// query means; one that keyed on the string alone would lose the numeric match.
//
// The assertion is equivalence with the unindexed evaluator rather than a hand-written expectation,
// because the property that matters is "indexing changed nothing", not any particular row set.
func TestIndexedResultsMatchUnindexed(t *testing.T) {
	m := check.NewModel(numDesign())
	for _, text := range []string{
		`component-on-net(?r, "10") => ?r`,
		`component-on-net(?r, "10.0") => ?r`,
		`component.class(?r,?c) => ?r`,
		`component-on-net(?a,?n), component-on-net(?c,?n), ?a != ?c => ?a, ?c`,
		`component-on-net(?r,?n) => ?n, count(?r)`,
	} {
		q := mustParse(t, text)
		indexed, err := (Naive{}).Eval(q, NewBase(m))
		if err != nil {
			t.Fatalf("%s: indexed: %v", text, err)
		}
		plain, err := (Naive{}).Eval(q, newUnindexedBase(m))
		if err != nil {
			t.Fatalf("%s: unindexed: %v", text, err)
		}
		if len(indexed) != len(plain) {
			t.Errorf("%s: indexed returned %d rows, unindexed %d", text, len(indexed), len(plain))
			continue
		}
		for i := range indexed {
			if a, b := rowValueKey(indexed[i]), rowValueKey(plain[i]); a != b {
				t.Errorf("%s: row %d: indexed %s != unindexed %s", text, i, a, b)
			}
		}
	}
}

// newUnindexedBase is the equivalence oracle: the same fact base with no index cache, which sends
// every probe down the original full-scan path. Comparing against it is what makes "indexing changed
// nothing" an assertion rather than a hope.
func newUnindexedBase(m check.Model) *Base {
	b := NewBase(m)
	b.edbIdx = nil // extendEDB falls back to the full scan when there is no cache to consult
	return b
}

// rowValueKey renders a row by VALUE. Value carries *float64, so printing the struct compares pointer
// addresses, which differ between two evaluations of the same query and say nothing.
func rowValueKey(r Row) string {
	keys := make([]string, 0, len(r.Bind))
	for k := range r.Bind {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	var out string
	for _, k := range keys {
		v := r.Bind[Var(k)]
		out += k + "=" + v.S
		if v.Num != nil {
			out += fmt.Sprintf("#%g", *v.Num)
		}
		out += ";"
	}
	return out
}

// The case the keying design exists for, and the one the string-only fixture above cannot reach.
//
// A fact's numeric field canonicalises its string (fieldValue sets S: ftoa(*f.Num)), so a param
// value of 20 is filed under "20". The parser keeps a constant's LITERAL text, so writing 20.0 in a
// query produces {S:"20.0", Num:20}. valueEq calls those equal (both numeric); a bucket keyed on the
// string alone would not, and the row would vanish with no error anywhere.
//
// Asserting both spellings return the same row is what makes the second key in valueKeys load-bearing
// rather than defensive decoration.
func TestNumericConstantMatchesCanonicalFact(t *testing.T) {
	// param facts project per PLACED part, so the design needs enough seeded components to push the
	// relation past indexMinFacts. Below it the probe takes the full-scan path and the test proves
	// nothing about the index. Only REG-24 carries VIN 20.
	d := &ir.Design{}
	specs := param.ParamSet{"REG-24": regSpec("REG-24", 20)}
	d.Components = append(d.Components, &ir.Component{
		RefDes: "U1", Attributes: map[string]string{"MPN": "REG-24"}, Prov: &ir.Provenance{SourceFile: "reg"}})
	for i := 0; i < indexMinFacts+4; i++ {
		mpn := fmt.Sprintf("OTHER-%d", i)
		specs[mpn] = regSpec(mpn, float64(100+i))
		d.Components = append(d.Components, &ir.Component{
			RefDes: fmt.Sprintf("U%d", i+2), Attributes: map[string]string{"MPN": mpn}, Prov: &ir.Provenance{SourceFile: "reg"}})
	}
	m := check.NewModelWithParams(d, nil, specs)
	canonical := runQuery(t, m, `param(?mpn,"VIN",20) => ?mpn`)
	spelled := runQuery(t, m, `param(?mpn,"VIN",20.0) => ?mpn`)
	if len(canonical) == 0 {
		t.Fatal("setup: the canonical spelling matched nothing, so the comparison proves nothing")
	}
	if len(spelled) != len(canonical) {
		t.Errorf("20.0 matched %d rows, 20 matched %d: a numerically equal constant fell in a bucket the probe never looked in",
			len(spelled), len(canonical))
	}
}

// A derived tuple that is valsEqual to one already stored must still deduplicate once the dedup set
// is a hash bucket. The three spellings of ten are the case that would slip through a keying scheme
// where insert and probe disagree, and a duplicate here is not cosmetic: the fixpoint's "did
// anything change" flag drives termination.
func TestDerivedDedupUnaffectedByNumericSpelling(t *testing.T) {
	m := check.NewModel(numDesign())
	rows := runQuery(t, m, `onten(?r) :- component-on-net(?r,"10"); onten(?r) :- component-on-net(?r,"10"); onten(?x) => ?x`)
	seen := map[string]bool{}
	for _, r := range rows {
		k := fmt.Sprint(r.Bind)
		if seen[k] {
			t.Errorf("duplicate derived row %v: the dedup set let an equal tuple through", r.Bind)
		}
		seen[k] = true
	}
}

// One Base serving several rule-bearing queries in turn is a real pattern (the profile coverage pass
// does it), and both index caches have to behave under it: the EDB index is shared on purpose
// because facts are immutable, while a derived relation belongs to one query and its index must not
// outlive it. A stale IDB index would hold positions into a previous query's tuple slice.
func TestBaseReuseAcrossRuleBearingQueries(t *testing.T) {
	b := NewBase(check.NewModel(benchDesign(50)))
	first := evalOn(t, b, `d(?a,?n) :- component-on-net(?a,?n); d(?x,?y) => ?x`)
	second := evalOn(t, b, `d(?a,?n) :- component-on-net(?a,?n), prefix(?a,"R1"); d(?x,?y) => ?x`)
	again := evalOn(t, b, `d(?a,?n) :- component-on-net(?a,?n); d(?x,?y) => ?x`)
	if len(second) >= len(first) {
		t.Fatalf("setup: the narrowed query returned %d rows, not fewer than %d", len(second), len(first))
	}
	if len(again) != len(first) {
		t.Errorf("re-running the first query on the reused Base gave %d rows, first time %d: a derived index outlived its query",
			len(again), len(first))
	}
}

func evalOn(t *testing.T, b *Base, text string) []Row {
	t.Helper()
	rows, err := (Naive{}).Eval(mustParse(t, text), b)
	if err != nil {
		t.Fatalf("%s: %v", text, err)
	}
	return rows
}

// Work counts candidate comparisons, so a shape that is linear in the fact base must not grow
// quadratically in it. Asserting the RATIO rather than a duration is what makes this a complexity
// test: it is deterministic, identical on every machine, and it fails the moment a scan returns.
//
// The evaluator's own doc comment used to assert this property in prose ("naïve join is sufficient
// because one design's fact base is small") with nothing enforcing it, which is exactly how a
// two-atom join came to take 15.7 seconds on a real board.
func TestWorkScalesSubQuadratically(t *testing.T) {
	shapes := []struct {
		name string
		text string
	}{
		{"one-atom", `component-on-net(?a,?n) => ?a`},
		{"two-atom-shared", `component-on-net(?a,?n), component-on-net(?c,?n), ?a != ?c => ?a`},
		{"three-atom-chain", `component-on-net(?a,?n), component-on-net(?c,?n), component-on-net(?e,?n) => ?a`},
		// A cycle in the join graph: a-n, c-n, c-m, a-m closes back on itself. Binary-join plans are
		// provably suboptimal on cyclic conjunctive queries, which is the case worst-case-optimal
		// joins exist for. Indexing does not make that go away, so this shape is here to SHOW where
		// the ceiling is rather than to claim it is gone.
		{"triangle-cyclic", `component-on-net(?a,?n), component-on-net(?c,?n), component-on-net(?c,?m), component-on-net(?a,?m), ?n != ?m => ?a`},
		{"negation", `component-on-net(?a,?n), not component.class(?a,"resistor") => ?a`},
		{"aggregation", `component-on-net(?a,?n) => ?n, count(?a)`},
		{"recursion", `conn(?a,?b) :- component-on-net(?a,?b); linked(?a,?c) :- conn(?a,?n), conn(?c,?n), ?a != ?c; linked("R1",?x) => ?x`},
	}
	for _, s := range shapes {
		t.Run(s.name, func(t *testing.T) {
			w1 := workFor(t, s.text, 200)
			w2 := workFor(t, s.text, 400)
			if w1 == 0 {
				t.Fatalf("no work recorded at n=200; the counter is not wired to this shape")
			}
			if ratio := float64(w2) / float64(w1); ratio > 3 {
				t.Errorf("work grew %.1fx for 2x the design (%d -> %d): quadratic, not linear", ratio, w1, w2)
			}
		})
	}
}

func workFor(t *testing.T, text string, n int) int64 {
	t.Helper()
	b := NewBase(check.NewModel(benchDesign(n)))
	if _, err := (Naive{}).Eval(mustParse(t, text), b); err != nil {
		t.Fatalf("eval at n=%d: %v", n, err)
	}
	return b.Work()
}
