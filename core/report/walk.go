package report

import (
	"fmt"
	"io"

	"github.com/panyam/agni/core/check"
)

// pairNote marks a crossing that followed a through pair rather than every pin of the part.
func pairNote(c check.TraceCross) string {
	if c.Paired {
		return " (paired)"
	}
	return ""
}

// WriteWalkText renders a walk for a person, hop by hop. Each net says how the walk entered it and
// what else sits on it, a net the walk stopped at says why, and the parts it did not cross come last
// with their reasons.
func WriteWalkText(w io.Writer, wk check.Walk) {
	if wk.Reason != "" {
		fmt.Fprintf(w, "cannot walk: %s\n", wk.Reason)
		return
	}
	start := wk.From.RefDes
	if wk.From.Pin != "" {
		start = endpointLabel(wk.From)
	}
	fmt.Fprintf(w, "walk from %s, %s, %d %s\n", start, wk.Kind, wk.Radius, pluralWord(wk.Radius, "hop", "hops"))
	hop := 0
	for _, n := range wk.Nets {
		if n.Hop != hop {
			hop = n.Hop
			fmt.Fprintf(w, "\nhop %d\n", hop)
		}
		e := n.Entered
		how := fmt.Sprintf("from %s pin %s", e.RefDes, e.ExitPin)
		if e.FromNet != "" {
			how = fmt.Sprintf("through %s%s pin %s --> pin %s%s, from %s", e.RefDes, classNote(e.Class), e.EnterPin, e.ExitPin, pairNote(e), e.FromNet)
		}
		fmt.Fprintf(w, "  net %s   (%s)\n", n.Net.Name, how)
		for _, a := range n.AlsoEntered {
			fmt.Fprintf(w, "      also through %s%s pin %s --> pin %s%s, from %s\n", a.RefDes, classNote(a.Class), a.EnterPin, a.ExitPin, pairNote(a), a.FromNet)
		}
		if n.Ended != "" {
			fmt.Fprintf(w, "      ends here: %s\n", n.Ended)
		}
		writeNetParts(w, n.Net)
	}
	for _, e := range wk.Elided {
		fmt.Fprintf(w, "  and %d more %s at hop %d, past the cap\n", e.Nets, pluralWord(e.Nets, "net", "nets"), e.Hop)
	}
	if len(wk.Boundary) > 0 {
		fmt.Fprintf(w, "\nnot crossed\n")
		for _, b := range wk.Boundary {
			fmt.Fprintf(w, "  %s%s on %s, hop %d: %s\n", b.RefDes, classNote(b.Class), b.Net, b.Hop, b.Reason)
		}
	}
	fmt.Fprintf(w, "\nwalked: %d %s over %d %s, %d %s not crossed\n",
		len(wk.Nets), pluralWord(len(wk.Nets), "net", "nets"), hop, pluralWord(hop, "hop", "hops"),
		len(wk.Boundary), pluralWord(len(wk.Boundary), "part", "parts"))
	if wk.Goal != nil {
		writeWalkGoal(w, *wk.Goal)
	}
}

// writeWalkGoal renders a goal's answer after the walk. Each route opens with its one-line form
// (check.RenderRoute), then lists its nets and crossings as a trace does.
func writeWalkGoal(w io.Writer, g check.WalkGoalAnswer) {
	what := "to the " + g.Kind
	if g.Kind == check.WalkGoalPin {
		what = "to " + endpointLabel(g.To)
	}
	fmt.Fprintf(w, "\ngoal %s, from %s\n", what, g.At)
	switch g.Outcome {
	case check.TraceNoRoute:
		fmt.Fprintf(w, "  no route: %s\n", g.Reason)
		return
	case check.TraceUnresolved:
		fmt.Fprintf(w, "  cannot ask: %s\n", g.Reason)
		return
	}
	for i, r := range g.Routes {
		hops := make([]check.RouteHop, len(r.Crossings))
		for j, c := range r.Crossings {
			hops[j] = check.RouteHop{Through: c.RefDes, To: c.ToNet}
		}
		first := ""
		if len(r.Nets) > 0 {
			first = r.Nets[0].Name
		}
		fmt.Fprintf(w, "  route %d: %s\n", i+1, check.RenderRoute(first, hops))
		if r.Note != "" {
			fmt.Fprintf(w, "    %s\n", r.Note)
		}
		for j, n := range r.Nets {
			writeTraceNet(w, n)
			if j < len(r.Crossings) {
				c := r.Crossings[j]
				fmt.Fprintf(w, "  cross %s%s pin %s --> pin %s%s\n", c.RefDes, classNote(c.Class), c.EnterPin, c.ExitPin, pairNote(c))
			}
		}
		if g.Kind == check.WalkGoalPin {
			fmt.Fprintf(w, "  %s\n", endpointLabel(g.To))
		}
	}
	unit := "crossings"
	if g.Kind == check.WalkGoalPullUp {
		unit = "hops"
	}
	fmt.Fprintf(w, "\nrouted: %d %s, searched to a radius of %d %s from %s\n",
		len(g.Routes), pluralWord(len(g.Routes), "route", "routes"), g.Radius, unit, g.At)
}
