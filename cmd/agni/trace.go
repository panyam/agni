package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/render"
	rpt "github.com/panyam/agni/core/report"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// traceCmd walks from one pin to another through series pass elements and prints what it crossed,
// so a reviewer can check a connectivity answer against its route (agni issue 518).
func traceCmd() *cobra.Command {
	var from, to, format, renderOut, serverVal, traceOutPath string
	var srvSpec serverSpec
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
			// Before redirectOut, so a --server this process cannot honour refuses the run before the
			// output file exists (agni issue 637).
			var err error
			if srvSpec, err = resolveServer(serverVal); err != nil {
				return err
			}
			closeOut, err := redirectOut(cmd, traceOutPath)
			if err != nil {
				return err
			}
			defer closeOut()
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
			// Thin client of the in-process DesignService, like `check` and `query` (WS9-048), so the
			// terminal and the viewer get their answer from ONE implementation.
			//
			// The native renderer is nil because TraceDesign never reaches it.
			ll := &localLoader{loader: newLoader()}
			svc := service.NewDesignService(ll, nil, render.Style{}, cliProjects())
			uri, err := cliArgURI(args[0])
			if err != nil {
				return err
			}
			resp, err := svc.TraceDesign(cmd.Context(), &webapi.TraceDesignRequest{
				Uri:  string(uri),
				From: &webapi.TraceEndpoint{RefDes: a.RefDes, Pin: a.Pin},
				To:   &webapi.TraceEndpoint{RefDes: b.RefDes, Pin: b.Pin},
				Hops: int32(hops),
			})
			if err != nil {
				return err
			}
			t := service.TraceFromProto(resp.GetTrace())

			// Whether a link is safe to make is decided by viewerLinkMeta, the helper `check` also
			// uses. The link is printed for every outcome, since "these two pins do not join" is
			// worth sending someone too.
			//
			// The guard is only an EARLY-OUT that skips the design resolution and content hash for a
			// run that asked for no links. viewerLinkMeta and TraceURL already yield nothing without
			// a server, so the refusal lives there.
			//
			// IT GUARDS THE RESOLVED SPEC, not a flag variable. Reading a flag variable here meant
			// `trace --server <url>` printed no link and no reason once #633 replaced --url-base with
			// --server (agni issue 636).
			if srvSpec.url != "" {
				meta := viewerLinkMeta(cmd, cmd.Context(), ll, string(uri), srvSpec)
				if u := rpt.TraceURL(meta, a.String(), b.String(), hops); u != "" {
					defer fmt.Fprintf(cmd.ErrOrStderr(), "\nlook at it: %s\n", u)
				}
			}
			// An endpoint that names nothing is a failed QUESTION and exits non-zero. A no-route is
			// an answer and exits clean, so a script can tell "they are not joined" from "you named
			// a pin that does not exist" (issue 518).
			if t.Outcome == check.TraceUnresolved && format == "text" {
				return fmt.Errorf("cannot trace: %s", t.Reason)
			}
			// --render draws every outcome, a no-route and an unresolved endpoint included.
			// traceSpecs picks the subjects, and renderSubjects is the drawing path every command
			// shares.
			if renderOut != "" {
				if err := renderSubjects(cmd.ErrOrStderr(), args[0], renderOut, traceSheet(resp.GetTrace()), traceSpecs(t)); err != nil {
					return err
				}
			}
			if format == "json" {
				// protojson of the WIRE message (C31). EmitUnpopulated keeps empty lists and zero
				// fields present, so a no-route has the same fields as a route.
				b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.
					Marshal(resp.GetTrace())
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
	outFileFlag(cmd, &traceOutPath)
	serverFlag(cmd, &serverVal)
	withSelfServer(cmd, &srvSpec)
	cmd.Flags().StringVar(&renderOut, "render", "",
		"also draw the answer to this .svg file: the route's nets and the parts crossed, on the "+
			"design's own schematic where it has one and on an auto-layout of its netlist where it "+
			"does not. Works for a no-route too, marking the two nets that fail to join.")
	cmd.MarkFlagRequired("from")
	cmd.MarkFlagRequired("to")
	return cmd
}

// The route and the parts crossed get different colours, so a reader can tell the wire from the part
// the signal passes through. The endpoint colour marks what was ASKED rather than what was found,
// which keeps a no-route drawing legible.
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

// traceSpecs turns a trace into the entities a drawing should point at, which are the nets it passed
// through, the parts it crossed, and the two endpoint pins.
//
// A route gets its nets and crossings, a no-route gets the two nets that fail to join, and an
// unresolved endpoint gets whichever end DID resolve. It sits beside the command rather than in
// subjectrender.go for the reason findingSpecs sits beside the review renderer.
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
		// An endpoint that did not resolve carries no net, so it contributes nothing.
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

// traceSheet is the sheet a rendered trace should open on. That is the FROM endpoint's sheet, else
// the first route net drawn anywhere, else "" for the caller's default. From wins because the reader
// named that pin first.
//
// The server decides where each net lives and this only chooses among what it returned (C32, agni
// issue 657). One SVG shows one sheet, while the viewer offers every sheet of every net as a badge.
func traceSheet(p *webapi.Trace) string {
	if ids := p.GetFrom().GetSheetIds(); len(ids) > 0 {
		return ids[0]
	}
	for _, n := range p.GetNets() {
		if ids := n.GetSheetIds(); len(ids) > 0 {
			return ids[0]
		}
	}
	return ""
}
