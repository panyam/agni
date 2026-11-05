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
	var from, kind, format, table, outPath, remoteURL string
	var through, orderBy []string
	var hops int
	cmd := &cobra.Command{
		Use:   "walk <file>",
		Short: "Show the nets and parts around a part, hop by hop",
		Long: "walk starts at a part (--from R1) or one pin (--from U7.3) and walks outwards. Hop 1 is the\n" +
			"start's own nets, and each later hop crosses the parts on them into their other nets. A\n" +
			"ground, a rail or a global net is reached and never walked through.\n\n" +
			"--kind picks which parts a hop crosses:\n" + walkKindsHelp() + "\n\n" +
			"A part the walk does not cross is listed with the reason. --through crosses it anyway.",
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
			// A start that names nothing is a failed question, as an unresolved trace endpoint is.
			if w.GetReason() != "" {
				return fmt.Errorf("cannot walk: %s", w.GetReason())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "the part (R1) or pin (U7.3) to start from")
	cmd.Flags().StringVar(&kind, "kind", check.WalkKinds()[0].Name, "which parts a hop crosses (see above)")
	cmd.Flags().IntVar(&hops, "hops", check.DefaultWalkHops, "how many hops to walk, hop 1 being the start's own nets")
	cmd.Flags().StringSliceVar(&through, "through", nil, "parts to cross whatever --kind says, e.g. --through U7")
	cmd.Flags().StringVar(&format, "format", "text", "text|json|csv")
	cmd.Flags().StringVar(&table, "table", "walk", "with --format csv, which table: walk or walk_boundary")
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
