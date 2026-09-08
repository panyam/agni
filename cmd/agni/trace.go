package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/core/check"
)

// traceCmd walks from one pin to another through series pass elements and prints what it crossed.
//
// The engine has been able to answer whether two points are connected for a long time; what it could
// not do is show the route, so a reviewer had no way to check the answer (agni issue 518). Every
// walk held the path and discarded it on the way out. This is the smallest surface over the walk
// that now returns it.
func traceCmd() *cobra.Command {
	var from, to, format string
	var hops int
	cmd := &cobra.Command{
		Use:   "trace <file>",
		Short: "Show the series path between two pins, and what sits on it",
		Long: "trace walks from one pin to another through series pass elements (resistors, inductors,\n" +
			"ferrites, fuses) and prints the route: the parts crossed, the nets passed through, and the\n" +
			"test points and other parts sitting on each one.\n\n" +
			"A capacitor is a DC block and is never crossed, and a rail or plane may be the END of a\n" +
			"route but is never passed through.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := parseEndpoint(from, "--from")
			if err != nil {
				return err
			}
			b, err := parseEndpoint(to, "--to")
			if err != nil {
				return err
			}
			if format != "text" && format != "json" {
				return fmt.Errorf("--format %s: want text or json", format)
			}
			d, _, err := readDesignWithConfig(args[0])
			if err != nil {
				return err
			}
			t := check.TracePins(check.NewModel(d), a, b, hops)
			// An endpoint that names nothing is a failed QUESTION, not an answer about the design,
			// so it exits non-zero. A no-route is an answer and exits clean: a script asking whether
			// two pins are joined must be able to tell "they are not" from "you named a pin that
			// does not exist", which is the confusion issue 518 says must not happen.
			if t.Outcome == check.TraceUnresolved && format == "text" {
				return fmt.Errorf("cannot trace: %s", t.Reason)
			}
			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(t); err != nil {
					return err
				}
				if t.Outcome == check.TraceUnresolved {
					return fmt.Errorf("cannot trace: %s", t.Reason)
				}
				return nil
			}
			writeTraceText(cmd.OutOrStdout(), t)
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start pin, as <ref-des>.<pin> (e.g. U7.3)")
	cmd.Flags().StringVar(&to, "to", "", "end pin, as <ref-des>.<pin> (e.g. U12.4)")
	cmd.Flags().IntVar(&hops, "hops", check.DefaultTraceHops,
		"how many series crossings to search through. Unlike the protection radii this is a search "+
			"budget rather than an electrical claim, and the answer states the value it rests on, so a "+
			"no-route can be re-asked wider.")
	cmd.Flags().StringVar(&format, "format", "text", "text|json")
	cmd.MarkFlagRequired("from")
	cmd.MarkFlagRequired("to")
	return cmd
}

// parseEndpoint reads "U7.3" into its two halves, splitting at the FIRST dot because a ref-des does
// not contain one and a pin designator occasionally does.
func parseEndpoint(s, flag string) (check.Endpoint, error) {
	ref, pin, ok := strings.Cut(strings.TrimSpace(s), ".")
	if !ok || ref == "" || pin == "" {
		return check.Endpoint{}, fmt.Errorf("%s %q: name a pin as <ref-des>.<pin>, e.g. U7.3", flag, s)
	}
	return check.Endpoint{RefDes: ref, Pin: pin}, nil
}

// writeTraceText renders a trace for a person: the route as one line to copy, then the same route
// expanded with what sits on each net, then what the answer rests on.
func writeTraceText(w io.Writer, t check.Trace) {
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
		len(t.Crossings), plural(len(t.Crossings), "crossing", "crossings"),
		len(t.Nets), plural(len(t.Nets), "net", "nets"), t.Radius)
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
