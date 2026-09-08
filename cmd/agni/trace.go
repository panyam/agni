package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/service"
)

// traceCmd walks from one pin to another through series pass elements and prints what it crossed.
//
// The engine has been able to answer whether two points are connected for a long time; what it could
// not do is show the route, so a reviewer had no way to check the answer (agni issue 518). Every
// walk held the path and discarded it on the way out. This is the smallest surface over the walk
// that now returns it.
func traceCmd() *cobra.Command {
	var from, to, format, renderOut string
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
			// --render draws the answer, and it draws a no-route and an unresolved endpoint too,
			// which is the point: a picture of the two nets that do NOT join is the thing a reader
			// was going to go looking for anyway. traceSpecs decides what counts as a subject; the
			// drawing itself is the shared path every command uses.
			if renderOut != "" {
				if err := renderSubjects(cmd.ErrOrStderr(), args[0], renderOut, traceSpecs(t)); err != nil {
					return err
				}
			}
			if format == "json" {
				// protojson of the WIRE message, matching how check, diff, validate and params emit
				// theirs, so a script reading this CLI and a client reading TraceDesign parse one
				// shape. EmitUnpopulated keeps empty lists and zero fields present, so a no-route is
				// still a well-formed object rather than fields that appear and vanish per run.
				b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.
					Marshal(service.TraceProto(t))
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
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
	cmd.Flags().StringVar(&renderOut, "render", "",
		"also draw the answer to this .svg file: the route's nets and the parts crossed, on the "+
			"design's own schematic where it has one and on an auto-layout of its netlist where it "+
			"does not. Works for a no-route too, marking the two nets that fail to join.")
	cmd.MarkFlagRequired("from")
	cmd.MarkFlagRequired("to")
	return cmd
}

// The route reads in one colour and the parts crossed in another, so a reader can tell the wire the
// signal travels on from the part it travels through. The endpoint colour is deliberately the odd
// one out: it marks what was ASKED rather than what was found, which is what makes a no-route
// drawing legible.
const (
	traceRouteColor    = "#2563eb"
	traceCrossColor    = "#e11d48"
	traceEndpointColor = "#0f766e"
)

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

// traceSpecs turns a trace into the entities a drawing should point at: the nets it passed through,
// the parts it crossed, and the two endpoint pins.
//
// It draws the answer whatever the answer was. A route gets its nets and crossings; a no-route gets
// the two nets that fail to join, which is the picture a reader goes looking for the moment they
// read the words. An unresolved endpoint gets whichever end DID resolve, because half an answer
// located is more use than none, and the text beside it already says the other end named nothing.
//
// This is the typed half of the render path, and it lives here rather than in subjectrender.go for
// the reason findingSpecs lives beside the review renderer: deciding what counts as a subject of an
// answer is a claim about that answer, and it belongs where someone reviewing the answer will read it.
func traceSpecs(t check.Trace) []*geom.HighlightSpec {
	var specs []*geom.HighlightSpec
	netSpec := func(name, color string) *geom.HighlightSpec {
		return &geom.HighlightSpec{
			Nets: []string{name}, Color: color, Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_PATH,
		}
	}
	switch t.Outcome {
	case check.TraceRouted:
		for _, n := range t.Nets {
			specs = append(specs, netSpec(n.Name, traceRouteColor))
		}
		for _, c := range t.Crossings {
			specs = append(specs, &geom.HighlightSpec{
				Components: []string{c.RefDes}, Color: traceCrossColor,
				Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT,
			})
		}
	default:
		// Both ends, in the colour that says they are the question rather than the answer. An
		// endpoint that did not resolve carries no net, so it contributes nothing.
		for _, e := range []check.TraceEnd{t.From, t.To} {
			if e.Net != "" {
				specs = append(specs, netSpec(e.Net, traceEndpointColor))
			}
		}
	}
	for _, e := range []check.TraceEnd{t.From, t.To} {
		if e.RefDes != "" && e.Pin != "" {
			specs = append(specs, &geom.HighlightSpec{
				Pins: []*geom.PinRef{{RefDes: e.RefDes, Pin: e.Pin}}, Color: traceEndpointColor,
			})
		}
	}
	return specs
}
