// Command dft-coverage is the design-for-test rung of the Agni examples ladder: which nets can a
// probe reach, and which two-terminal passives can actually be measured on an assembled board. It is
// the walkthrough form of the coverage questions a DFT review asks, driven by the fact relations
// rather than by the rule catalog, so it doubles as the first example that runs a query.
//
// The narration lives in the sidecar walkthrough.md (demokit FromMarkdown); this file only binds the
// steps that run engine code.
//
// The grouped counts come from the QUERY, not from a fold in this file: `Select` takes count/min/max/sum
// over the group its variable columns form, so "test points per net" is one query rather than a join
// plus arithmetic here. Every step prints the CLI line that reproduces it, so nothing the walkthrough
// shows is reachable only from Go.
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
	_ "github.com/panyam/agni/stdlib/relations"     // register the fact relations the queries below bind
	_ "github.com/panyam/agni/stdlib/rules/builtin" // register the rule catalog; check.BuiltinRules is EMPTY without it, silently
	"github.com/panyam/demokit"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// passivePrelude names the two ideas every coverage question below is built on, as derived relations.
// A `passive` is a two-terminal part whose value an in-circuit tester wants to measure; `covered`
// pairs one with a net a probe can reach. Datalog has no disjunction, so "resistor or capacitor" is
// two rules for one relation, which is also what makes the vocabulary extensible: add a rule and
// every question below inherits it.
//
// The same string is printed and executed, so the CLI line beside each step is the query that ran.
const passivePrelude = `passive(?p) :- component.class(?p, "resistor"); ` +
	`passive(?p) :- component.class(?p, "capacitor"); ` +
	`covered(?p, ?n) :- passive(?p), component-on-net(?p, ?n), ` +
	`component.class(?tp, "test_point"), component-on-net(?tp, ?n); `

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
		cli(`agni query <design> 'component.class(?tp, "test_point"), component-on-net(?tp, ?net) => ?net, count(?tp)'`)
		probed := probedNets(d)
		var unprobed []string
		for _, n := range d.GetNets() {
			if !probed[n.GetName()] {
				unprobed = append(unprobed, n.GetName())
			}
		}
		sort.Strings(unprobed)
		fmt.Printf("%d of %d nets carry a test point.\n", len(probed), len(d.GetNets()))
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
		// Both ends, one end, neither. The first two differ only in the having clause; the third needs
		// negation, and is written through a unary `hascov` so the negated atom's variable is bound
		// (an unbound one is unsafe and returns nothing rather than an error, agni issue 522).
		both := passiveQuery(`covered(?p, ?n) => ?p, count(distinct ?n) having count(distinct ?n) > 1`)
		one := passiveQuery(`covered(?p, ?n) => ?p, count(distinct ?n) having count(distinct ?n) < 2`)
		neither := passiveQuery(`hascov(?p) :- covered(?p, ?n); nocov(?p) :- passive(?p), not hascov(?p); nocov(?p) => ?p`)
		cli("agni query <design> '" + one + "'")
		fmt.Printf("both ends probed (measurable in circuit): %d\n", len(must(rows(d, both))))
		fmt.Printf("one end probed  (present, not measurable): %d  %s\n", len(must(rows(d, one))), refs(must(rows(d, one))))
		fmt.Printf("neither end     (invisible to test)      : %d  %s\n", len(must(rows(d, neither))), refs(must(rows(d, neither))))
		return nil
	})

	demo.Bind("by-mpn").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := design.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		q := passiveQuery(`hascov(?p) :- covered(?p, ?n); nocov(?p) :- passive(?p), not hascov(?p); ` +
			`nocov(?p), component.mpn(?p, ?m) => ?m, count(distinct ?p), list(distinct ?p)`)
		cli("agni query <design> '" + q + "'")
		got := must(rows(d, q))
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
		m := check.NewModelWithParams(d, nil, nil)
		byOutcome := map[check.Outcome]int{}
		var undecided []string
		for _, v := range check.RunVerdicts(m, check.BuiltinRules()) {
			byOutcome[v.Outcome]++
			// Reason is populated for NotConsidered alone: it is the rule author saying why the
			// question could not be decided for this subject, which is the state a findings list has
			// no way to express.
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

// passiveQuery prefixes the shared derived relations onto one question.
func passiveQuery(q string) string { return passivePrelude + q }

// subjectRefs names a verdict's subject tuple. A verdict is about a TUPLE because some rules ask
// about a relation between entities, so this is the general shape even where it is one net.
func subjectRefs(es []check.Entity) string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Ref)
	}
	return strings.Join(out, ",")
}

// refs joins the first column of each row, which is the ref-des on every query here.
// refsCap bounds the ref-des list a bucket prints. The bundled fixture has three parts and the count
// alongside carries the answer anyway, so a full list is only ever useful at fixture scale: pointed at
// a real board these buckets run to several hundred, and the wall of text buries the three lines
// around it. The count is never truncated, only the naming.
const refsCap = 12

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
// that runs engine code prints one, so the walkthrough is never the only way to get the answer.
func cli(line string) { fmt.Printf("$ %s\n\n", line) }

// rows evaluates one datalog query against the design and returns each answer's bindings, in the
// order the query's head names them. An unparseable query or a failed evaluation yields no rows
// rather than a panic, which keeps a walkthrough step readable when a relation is not installed.
func rows(d *ir.Design, q string) ([][]string, error) {
	parsed, err := query.Parse(q)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	// NewModelWithParams, not NewModel: the model's MPN map is filled by the params constructor
	// alone, and component.mpn reads it rather than ir.Component.mpn. Built the other way the
	// relation is EMPTY on a design whose components all carry a part number, so a question about
	// part numbers answers "none" instead of failing. A nil spec provider is fine; only the datasheet
	// relations need one.
	got, err := (query.Naive{}).Eval(parsed, query.NewBase(check.NewModelWithParams(d, nil, nil)))
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

// must reports a query failure as a step error rather than an empty answer, because an empty answer
// from a broken query is indistinguishable from a clean design.
func must(rs [][]string, err error) [][]string {
	if err != nil {
		panic(err)
	}
	return rs
}

// probedNets names every net a test point sits on.
func probedNets(d *ir.Design) map[string]bool {
	out := map[string]bool{}
	for _, r := range must(rows(d, `component.class(?tp, "test_point"), component-on-net(?tp, ?net) => ?net`)) {
		out[r[0]] = true
	}
	return out
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
