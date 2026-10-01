// Command dft-coverage is the design-for-test rung of the Agni examples ladder. It asks which nets a
// probe can reach and which two-terminal passives can be measured on an assembled board, using the
// fact relations rather than the rule catalog.
//
// The narration lives in the sidecar walkthrough.md (demokit FromMarkdown); this file only binds the
// steps that run engine code.
//
// The grouped counts come from the QUERY's aggregates, not from a fold in this file, so "test points
// per net" is one query (docsite/content/guide/querying.md#count-parts-per-net-aggregation). Every
// step prints the CLI line that reproduces it.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/examples/common"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	_ "github.com/panyam/agni/stdlib/lib"           // register the shipped derived relations the coverage buckets call
	_ "github.com/panyam/agni/stdlib/relations"     // register the fact relations the queries below bind
	_ "github.com/panyam/agni/stdlib/rules/builtin" // register the rule catalog; check.BuiltinRules is EMPTY without it, silently
	"github.com/panyam/demokit"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// passiveDef is the one idea this walk defines for itself: a `passive` is a resistor or a capacitor,
// the parts this coverage report counts. Datalog has no disjunction, so "resistor or capacitor" is two
// rules for one relation, and adding a rule extends every question below.
//
// Everything else comes from the shipped library (stdlib/lib): net.has_test_point,
// component.two_terminal, component.probed_both and component.probed_one. `agni query --relations
// component.probed_one` prints any of them with its definition.
const passiveDef = `passive(?p) :- component.class(?p, "resistor"); ` +
	`passive(?p) :- component.class(?p, "capacitor"); `

// The coverage buckets, each one query. The same string is printed and executed, so the CLI line
// beside each step is the query that ran.
const (
	bothQuery    = passiveDef + `passive(?p), component.probed_both(?p) => ?p`
	oneQuery     = passiveDef + `passive(?p), component.probed_one(?p, ?probed, ?unprobed) => ?p, ?unprobed`
	neitherQuery = passiveDef + `passive(?p), component.two_terminal(?p, ?a, ?b), not net.has_test_point(?a), not net.has_test_point(?b) => ?p`
	byMPNQuery   = passiveDef + `passive(?p), component.two_terminal(?p, ?a, ?b), not net.has_test_point(?a), not net.has_test_point(?b), ` +
		`component.mpn(?p, ?m) => ?m, count(distinct ?p), list(distinct ?p)`
	unprobedQuery = `entity(?net, "net"), not net.has_test_point(?net) => ?net`
)

func main() {
	design := common.AskPath("design", "../common/designs/probe-coverage.edn")

	demo := demokit.New("dft-coverage").
		Dir("dft-coverage").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("pick").Input(design.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		design.Capture(ctx)
		fmt.Printf("Selected %s.\n", design.Path())
		return nil
	})

	demo.Bind("inventory").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := design.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		cli("agni query <design> 'component.class(?c, ?k) => ?k, count(?c)'")
		byClass := map[string]int{}
		for _, r := range must(rows(d, `component.class(?c, ?k) => ?k, count(?c)`)) {
			byClass[r[0]] = atoi(r[1])
		}
		fmt.Println(tally("class", byClass))
		return nil
	})

	demo.Bind("probed").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := design.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		cli("agni query <design> '" + unprobedQuery + "'")
		var unprobed []string
		for _, r := range must(rows(d, unprobedQuery)) {
			unprobed = append(unprobed, r[0])
		}
		sort.Strings(unprobed)
		fmt.Printf("%d of %d nets carry a test point.\n", len(d.GetNets())-len(unprobed), len(d.GetNets()))
		if len(unprobed) > 0 {
			fmt.Printf("Unprobed: %s\n", strings.Join(unprobed, ", "))
		}
		return nil
	})

	demo.Bind("passives").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := design.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		// Both ends, one end, neither. The first two are library members; the third negates
		// net.has_test_point on both of the nets component.two_terminal binds, so neither negated atom
		// has an unbound variable (an unbound one is unsafe and returns nothing rather than an error,
		// agni issue 522).
		cli("agni query <design> '" + oneQuery + "'")
		fmt.Printf("both ends probed (measurable in circuit): %d\n", len(must(rows(d, bothQuery))))
		fmt.Printf("one end probed  (present, not measurable): %d  %s\n", len(must(rows(d, oneQuery))), refs(must(rows(d, oneQuery))))
		fmt.Printf("neither end     (invisible to test)      : %d  %s\n", len(must(rows(d, neitherQuery))), refs(must(rows(d, neitherQuery))))
		return nil
	})

	demo.Bind("by-mpn").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := design.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		cli("agni query <design> '" + byMPNQuery + "'")
		got := must(rows(d, byMPNQuery))
		if len(got) == 0 {
			fmt.Println("Every passive has at least one end probed.")
			return nil
		}
		for _, r := range got {
			fmt.Printf("  %4s  %s   %s\n", r[1], r[0], r[2])
		}
		fmt.Println("\nOne uncovered part is an oversight. Several of one MPN is a placement habit.")
		return nil
	})

	demo.Bind("beyond").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := design.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		cli("agni check <design> --verdicts")
		m := check.NewModel(d)
		byOutcome := map[check.Outcome]int{}
		var undecided []string
		for _, v := range check.RunVerdicts(m, check.BuiltinRules()) {
			byOutcome[v.Outcome]++
			// Reason is populated for NotConsidered alone, in the rule author's words. A findings list
			// has no way to express it.
			if v.Outcome == check.NotConsidered && v.Reason != "" {
				undecided = append(undecided, fmt.Sprintf("  %s on %s\n      %s", v.Rule, subjectRefs(v.Subjects), v.Reason))
			}
		}
		fmt.Printf("%d pass, %d fail, %d not-considered, across %d rules.\n\n",
			byOutcome[check.Pass], byOutcome[check.Fail], byOutcome[check.NotConsidered],
			len(check.BuiltinRules()))
		sort.Strings(undecided)
		if len(undecided) > 2 {
			undecided = undecided[:2]
		}
		fmt.Println("Questions the catalog could not decide here, in the rule author's words:")
		fmt.Println(strings.Join(undecided, "\n"))
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}

// subjectRefs names a verdict's subject tuple. A verdict is about a TUPLE because some rules ask
// about a relation between entities.
func subjectRefs(es []check.Entity) string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Ref)
	}
	return strings.Join(out, ",")
}

// refsCap bounds the ref-des list a bucket prints. On a real board these buckets run to several
// hundred and bury the lines around them (agni issue 644). The count is never truncated, only the
// naming.
const refsCap = 12

// refs joins the first column of each row, which is the ref-des on every query here.
func refs(rs [][]string) string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r[0])
	}
	sort.Strings(out)
	if len(out) > refsCap {
		return strings.Join(out[:refsCap], ", ") + fmt.Sprintf(", ... (%d more)", len(out)-refsCap)
	}
	return strings.Join(out, ", ")
}

// cli prints the command a reader can paste into a second terminal to reproduce the step. Every step
// that runs engine code prints one.
func cli(line string) { fmt.Printf("$ %s\n\n", line) }

// rows evaluates one datalog query against the design and returns each answer's bindings, in the
// order the query's head names them. A parse or evaluation failure returns an error.
func rows(d *ir.Design, q string) ([][]string, error) {
	parsed, err := query.Parse(q)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	// No provider: component.mpn reads the MPN every model joins from the design (agni issue 748), and
	// only the datasheet relations need specs.
	got, err := query.Default.Eval(parsed, query.NewBase(check.NewModel(d)))
	if err != nil {
		return nil, fmt.Errorf("eval: %w", err)
	}
	cols := parsed.Columns()
	out := make([][]string, 0, len(got))
	for _, r := range got {
		vals := make([]string, 0, len(cols))
		for _, c := range cols {
			vals = append(vals, r.Bind[c].S)
		}
		out = append(out, vals)
	}
	return out, nil
}

// must panics on a query failure rather than returning an empty answer, because an empty answer
// from a broken query looks the same as a clean design.
func must(rs [][]string, err error) [][]string {
	if err != nil {
		panic(err)
	}
	return rs
}

// atoi reads an aggregate column, which arrives as text like every other binding.
func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// tally renders a count map in descending order, ties broken by name so a run is reproducible.
func tally(label string, m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", label)
	for _, k := range keys {
		fmt.Fprintf(&b, "  %4d  %s\n", m[k], k)
	}
	return strings.TrimRight(b.String(), "\n")
}
