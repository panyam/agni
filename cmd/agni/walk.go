package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/num"
	"github.com/panyam/agni/core/render"
	rpt "github.com/panyam/agni/core/report"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// walkCmd walks outwards from one part or pin, hop by hop, and prints what it reached (agni issue
// 1339). It asks WalkDesign, so the terminal and the viewer read one answer.
func walkCmd() *cobra.Command {
	var from, kind, format, table, outPath, remoteURL, goal, at, to string
	var through, orderBy []string
	var hops, goalHops int
	cmd := &cobra.Command{
		Use:   "walk <file>",
		Short: "Show the nets and parts around a part, hop by hop",
		Long: "walk starts at a part (--from R1) or one pin (--from U7.3) and walks outwards. Hop 1 is the\n" +
			"start's own nets, and each later hop crosses the parts on them into their other nets. A\n" +
			"ground, a rail or a global net is reached and never walked through.\n\n" +
			"--kind picks which parts a hop crosses:\n" + walkKindsHelp() + "\n\n" +
			"A part the walk does not cross is listed with the reason. --through crosses it anyway.\n\n" +
			"--goal asks a goal question from a net the walk reached (--at, default the start's own net):\n" +
			"  pin      the series route to the pin --to names, as agni trace walks it\n" +
			"  pull-up  every pull-up resistor that takes the net to a rail\n" +
			"  rail     the series route to the nearest rail\n" +
			"Each route runs from the start along the walk to --at, then on to the goal.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" && format != "csv" {
				return fmt.Errorf("--format %s: want text, json or csv", format)
			}
			closeOut, err := redirectOut(cmd, outPath)
			if err != nil {
				return err
			}
			defer closeOut()
			start := check.Endpoint{RefDes: strings.TrimSpace(from)}
			if ref, pin, ok := strings.Cut(start.RefDes, "."); ok {
				start = check.Endpoint{RefDes: ref, Pin: pin}
			}
			if start.RefDes == "" {
				return fmt.Errorf("--from %q: name a part (R1) or a pin (U7.3)", from)
			}
			var goalReq *webapi.WalkGoal
			switch {
			case goal != "":
				goalReq = &webapi.WalkGoal{Kind: goal, At: at, Hops: num.Int32(goalHops)}
				if goal == check.WalkGoalPin {
					ref, pin, ok := strings.Cut(strings.TrimSpace(to), ".")
					if !ok || ref == "" || pin == "" {
						return fmt.Errorf("--goal pin needs --to <ref-des>.<pin>, such as --to J4.2")
					}
					goalReq.To = &webapi.TraceEndpoint{RefDes: ref, Pin: pin}
				} else if to != "" {
					return fmt.Errorf("--to names the pin of --goal pin, not of --goal %s", goal)
				}
			case at != "" || to != "" || goalHops != 0:
				return fmt.Errorf("--at, --to and --goal-hops ask a goal question; name it with --goal pin|pull-up|rail")
			}
			ll := &localLoader{loader: newLoader()}
			var svc walkAPI = service.NewDesignService(ll, nil, render.Style{}, cliProjects())
			var snaps snapshotAPI = service.NewSnapshotService(ll, cliProjects(), nil, "")
			addr := cliArgURI
			if remoteURL != "" {
				if err := refuseLocalOnly(cmd); err != nil {
					return err
				}
				r, err := dialRemote(cmd.Context(), remoteURL)
				if err != nil {
					return err
				}
				svc, snaps, addr = r.walk(), r.snapshots(), r.uri
			}
			uri, err := addr(args[0])
			if err != nil {
				return err
			}
			snap, drop, err := ingest(cmd.Context(), snaps, &webapi.Snapshot{Uri: uri})
			if err != nil {
				return rpcFailed(err)
			}
			defer drop()
			resp, err := svc.WalkDesign(cmd.Context(), &webapi.WalkDesignRequest{
				Snapshot: snap,
				From:     &webapi.TraceEndpoint{RefDes: start.RefDes, Pin: start.Pin},
				Kind:     kind,
				Hops:     num.Int32(hops),
				Through:  through,
				Goal:     goalReq,
			})
			if err != nil {
				return rpcFailed(err)
			}
			w := resp.GetWalk()
			switch format {
			case "json":
				// protojson of the wire message (C31).
				b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(w)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
			case "csv":
				t, err := walkTable(resp, table)
				if err != nil {
					return err
				}
				if err := writeTableCSV(cmd.OutOrStdout(), t, orderBy); err != nil {
					return err
				}
			default:
				rpt.WriteWalkText(cmd.OutOrStdout(), service.WalkFromProto(w))
			}
			// A start that names nothing is a failed question, as an unresolved trace endpoint is, and
			// so is a goal that names nothing. A goal with no route is an answer and exits 0.
			if w.GetReason() != "" {
				return fmt.Errorf("cannot walk: %s", w.GetReason())
			}
			if g := w.GetGoal(); g.GetOutcome() == webapi.TraceOutcome_TRACE_OUTCOME_UNRESOLVED {
				return fmt.Errorf("cannot ask the goal: %s", g.GetReason())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "the part (R1) or pin (U7.3) to start from")
	cmd.Flags().StringVar(&kind, "kind", check.WalkKinds()[0].Name, "which parts a hop crosses (see above)")
	cmd.Flags().IntVar(&hops, "hops", check.DefaultWalkHops, "how many hops to walk, hop 1 being the start's own nets")
	cmd.Flags().StringSliceVar(&through, "through", nil, "parts to cross whatever --kind says, e.g. --through U7")
	cmd.Flags().StringVar(&goal, "goal", "", "ask a goal question from the walk: pin, pull-up or rail")
	cmd.Flags().StringVar(&at, "at", "", "with --goal, the reached net to ask from (default the start's own net)")
	cmd.Flags().StringVar(&to, "to", "", "with --goal pin, the pin to route to, e.g. --to J4.2")
	cmd.Flags().IntVar(&goalHops, "goal-hops", 0, fmt.Sprintf("with --goal pin or rail, the crossings to search from --at (default %d)", check.DefaultTraceHops))
	cmd.Flags().StringVar(&format, "format", "text", "text|json|csv")
	cmd.Flags().StringVar(&table, "table", "walk", "with --format csv, which table: walk, walk_boundary or walk_goal")
	cmd.Flags().StringSliceVar(&orderBy, "order-by", nil, "with --format csv, sort by these columns (prefix - to reverse)")
	outFileFlag(cmd, &outPath)
	remoteFlag(cmd, &remoteURL)
	cmd.MarkFlagRequired("from")
	return cmd
}

// walkKindsHelp lists the walk kinds for --help.
func walkKindsHelp() string {
	var b strings.Builder
	for _, k := range check.WalkKinds() {
		fmt.Fprintf(&b, "  %-12s %s\n", k.Name, k.Summary)
	}
	return strings.TrimRight(b.String(), "\n")
}

// walkTable picks one of the tables Tabulate projects from a walk (C35).
func walkTable(resp *webapi.WalkDesignResponse, name string) (*webapi.Table, error) {
	out, err := service.TableService{}.Tabulate(context.Background(), &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Walk{Walk: resp}})
	if err != nil {
		return nil, err
	}
	var names []string
	for _, t := range out.GetTables() {
		if t.GetName() == name {
			return t, nil
		}
		names = append(names, t.GetName())
	}
	return nil, fmt.Errorf("--table %s: a walk has %s", name, strings.Join(names, ", "))
}
