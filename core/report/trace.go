package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/panyam/agni/core/check"
)

// WriteTraceText renders a trace for a person: the route as one line to copy, then the same route
// expanded with what sits on each net, then what the answer rests on.
func WriteTraceText(w io.Writer, t check.Trace) {
	switch t.Outcome {
	case check.TraceNoRoute:
		fmt.Fprintf(w, "no route: %s --> %s\n\n", endpointLabel(t.From), endpointLabel(t.To))
		fmt.Fprintf(w, "  %s on net %s\n", endpointLabel(t.From), t.From.Net)
		fmt.Fprintf(w, "  %s on net %s\n\n", endpointLabel(t.To), t.To.Net)
		fmt.Fprintf(w, "%s.\n", t.Reason)
		fmt.Fprint(w, "The walk crosses resistors, inductors, ferrites and fuses.\n"+
			"A capacitor is a DC block and is never crossed.\n"+
			"A rail or plane may END a route and is never passed through.\n")
		return
	case check.TraceUnresolved:
		fmt.Fprintf(w, "cannot trace: %s\n", t.Reason)
		return
	}

	var refs []string
	for _, c := range t.Crossings {
		refs = append(refs, c.RefDes)
	}
	headline := append([]string{endpointLabel(t.From)}, refs...)
	headline = append(headline, endpointLabel(t.To))
	fmt.Fprintf(w, "%s\n\n", strings.Join(headline, " --> "))

	fmt.Fprintf(w, "route\n  %s\n", endpointLabel(t.From))
	for i, n := range t.Nets {
		writeTraceNet(w, n)
		if i < len(t.Crossings) {
			c := t.Crossings[i]
			fmt.Fprintf(w, "  cross %s%s pin %s --> pin %s\n",
				c.RefDes, classNote(c.Class), c.EnterPin, c.ExitPin)
		}
	}
	fmt.Fprintf(w, "  %s\n\n", endpointLabel(t.To))

	fmt.Fprintf(w, "routed: %d %s, %d %s, searched to a radius of %d\n",
		len(t.Crossings), pluralWord(len(t.Crossings), "crossing", "crossings"),
		len(t.Nets), pluralWord(len(t.Nets), "net", "nets"), t.Radius)
}

// writeTraceNet prints one net of the route and what else sits on it, probe points first, because a
// reviewer reading a trace is usually looking for where to put a probe.
func writeTraceNet(w io.Writer, n check.TraceNet) {
	rail := ""
	if n.BusLike {
		rail = "   (a rail or plane: a route may end here, never pass through)"
	}
	fmt.Fprintf(w, "  net %s%s\n", n.Name, rail)
	var probes, others []string
	for _, s := range n.Stubs {
		if s.Class == string(check.ClassTestPoint) {
			probes = append(probes, s.RefDes+"."+s.Pin)
			continue
		}
		others = append(others, s.RefDes+"."+s.Pin+classNote(s.Class))
	}
	if len(probes) > 0 {
		fmt.Fprintf(w, "      probe: %s\n", strings.Join(probes, ", "))
	}
	if len(others) > 0 {
		more := ""
		if n.StubsElided > 0 {
			more = fmt.Sprintf(", and %d more", n.StubsElided)
		}
		fmt.Fprintf(w, "      also: %s%s\n", strings.Join(others, ", "), more)
	}
}

// endpointLabel is "U7.3 (SDA)", falling back to the bare pin where the source carried no pin names.
func endpointLabel(e check.TraceEnd) string {
	if e.PinName == "" {
		return e.String()
	}
	return fmt.Sprintf("%s (%s)", e.String(), e.PinName)
}
func classNote(class string) string {
	if class == "" || class == "unknown" {
		return ""
	}
	return " (" + strings.ReplaceAll(class, "_", " ") + ")"
}

// pluralWord is one when n is 1 and many otherwise.
func pluralWord(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
