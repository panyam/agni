// Command trace is the trace rung of the Agni examples ladder: follow one pin to another
// through the series parts between them and read the route. It is the walkthrough form of
// `agni trace`. The narration lives in the sidecar walkthrough.md (demokit FromMarkdown);
// this file only binds the steps that run engine code.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/panyam/demokit"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/examples/common"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// The bundled fixture and the pins the walkthrough follows across it. They are package-level
// because the prose in walkthrough.md states what they do, and trace_test.go holds those claims
// against the fixture. A default that lived only inside main() could move without the test noticing,
// which is how a narrated example starts teaching something the code no longer does.
const (
	defaultDesign = "../common/designs/i2c-sensor.edn"
	defaultFrom   = "U1.3"
	defaultTo     = "J1.1"
)

// unreachable is a pin pair the bundled fixture joins through nothing, for the no-route step.
// Named here rather than asked, because the point of that step is the shape of the answer and a
// user-supplied pair could accidentally be connected.
var unreachable = [2]check.Endpoint{{RefDes: "U1", Pin: "3"}, {RefDes: "U1", Pin: "2"}}

func main() {
	design := common.AskPath("design", defaultDesign)
	// from / to carry the picks to the later steps, defaulted to the same values the markdown
	// inputs default to, so a non-interactive run stays coherent.
	from, to := defaultFrom, defaultTo

	demo := demokit.New("trace").
		Dir("trace").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("design").Input(design.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		design.Capture(ctx)
		fmt.Printf("Design: %s\n", design.Path())
		return nil
	})

	demo.Bind("pins").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		if v, ok := ctx.Inputs["from"].(string); ok && v != "" {
			from = v
		}
		if v, ok := ctx.Inputs["to"].(string); ok && v != "" {
			to = v
		}
		fmt.Printf("Tracing %s to %s\n", from, to)
		return nil
	})

	demo.Bind("run").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		t, err := traceOn(design, from, to)
		if err != nil {
			return demokit.Errf("%v", err)
		}
		fmt.Print(traceLines(*t))
		return nil
	})

	demo.Bind("terminus").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		t, err := traceOn(design, from, to)
		if err != nil {
			return demokit.Errf("%v", err)
		}
		if t.Outcome != check.TraceRouted || len(t.Nets) == 0 {
			fmt.Println("No route, so there is no last net to read.")
			return nil
		}
		last := t.Nets[len(t.Nets)-1]
		fmt.Printf("Last net on the route: %s\n", last.Name)
		if last.BusLike {
			fmt.Println("Rail-scale, so the walk stopped there rather than continue through it.")
			return nil
		}
		fmt.Println("Not rail-scale on this design, so the walk stopped because it had arrived.")
		fmt.Println("A supply on a real board crosses the fan-out cutoff and takes the other branch.")
		return nil
	})

	demo.Bind("noroute").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := design.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		t := check.TracePins(check.NewModel(d), unreachable[0], unreachable[1], check.DefaultTraceHops)
		fmt.Printf("%s to %s\n", unreachable[0], unreachable[1])
		fmt.Print(traceLines(t))
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}

// traceOn loads the chosen design and traces between two pins written as "REF.PIN".
func traceOn(design *common.PathInput, from, to string) (*check.Trace, error) {
	a, err := parseEndpoint(from)
	if err != nil {
		return nil, err
	}
	b, err := parseEndpoint(to)
	if err != nil {
		return nil, err
	}
	d, err := design.Load()
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", design.Path(), err)
	}
	t := check.TracePins(check.NewModel(d), a, b, check.DefaultTraceHops)
	return &t, nil
}

// parseEndpoint splits "U1.3" at the FIRST dot, since a ref-des does not contain one and a pin
// designator occasionally does. Same reading the CLI takes.
func parseEndpoint(s string) (check.Endpoint, error) {
	ref, pin, ok := strings.Cut(strings.TrimSpace(s), ".")
	if !ok || ref == "" || pin == "" {
		return check.Endpoint{}, fmt.Errorf("%q is not a pin: write it as <ref-des>.<pin>, e.g. U1.3", s)
	}
	return check.Endpoint{RefDes: ref, Pin: pin}, nil
}

// traceLines narrates one trace: the route as a line to read, then each net with what else sits
// on it. The three outcomes are kept apart, which is the part of the API worth showing.
func traceLines(t check.Trace) string {
	var b strings.Builder
	switch t.Outcome {
	case check.TraceUnresolved:
		fmt.Fprintf(&b, "unresolved: %s\n", t.Reason)
		fmt.Fprint(&b, "Nothing was walked. This is a failed question, not a disconnection.\n")
		return b.String()
	case check.TraceNoRoute:
		fmt.Fprintf(&b, "no route: %s\n", t.Reason)
		fmt.Fprintf(&b, "  %s sits on net %s\n", t.From, t.From.Net)
		fmt.Fprintf(&b, "  %s sits on net %s\n", t.To, t.To.Net)
		return b.String()
	}
	parts := []string{t.From.String()}
	for _, c := range t.Crossings {
		parts = append(parts, c.RefDes)
	}
	parts = append(parts, t.To.String())
	fmt.Fprintf(&b, "%s\n\n", strings.Join(parts, " -> "))
	for i, n := range t.Nets {
		fmt.Fprintf(&b, "  net %s\n", n.Name)
		for _, s := range n.Stubs {
			fmt.Fprintf(&b, "      also %s.%s (%s)\n", s.RefDes, s.Pin, s.Class)
		}
		if n.StubsElided > 0 {
			fmt.Fprintf(&b, "      and %d more\n", n.StubsElided)
		}
		if i < len(t.Crossings) {
			c := t.Crossings[i]
			fmt.Fprintf(&b, "  cross %s pin %s -> pin %s\n", c.RefDes, c.EnterPin, c.ExitPin)
		}
	}
	fmt.Fprintf(&b, "\n%d crossing(s), %d net(s), radius %d\n", len(t.Crossings), len(t.Nets), t.Radius)
	return b.String()
}
