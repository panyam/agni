package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/service"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

// checklistsCmd is the CLI face of ReviewService.ListChecklists (agni issue 859). It lists the
// checklists a design's project declares, inherited ones included. `agni review` runs the first one
// when no --checklist is named. With --remote it lists a pack's checklists instead, the ones
// `agni review designs/<d> --pack <id>` runs.
func checklistsCmd() *cobra.Command {
	var format, remote string
	c := &cobra.Command{
		Use:   "checklists <design folder> | checklists packs/<id> --remote <url>",
		Short: "List the review checklists a design's project or a server's pack declares",
		Long: "List the review checklists a design's project declares, in the order the project writes " +
			"them, inherited ones first, so the first is the one `agni review` runs by default. " +
			"With --remote, name a pack, packs/<id>, and it lists the pack's checklists, the ones " +
			"`agni review designs/<d> --pack <id>` runs. A design on a server holds no checklists of its " +
			"own, since they belong to the pack it is reviewed under. --format json is the " +
			"ListChecklistsResponse the ListChecklists rpc returns, each checklist carrying its manifest, " +
			"which a client sends to CreateReview to run it.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				return fmt.Errorf("unknown --format %q (want: text, json)", format)
			}
			isPack := strings.HasPrefix(args[0], "packs/")
			switch {
			case remote != "" && !isPack:
				return fmt.Errorf("agni checklists --remote lists a pack's checklists: name it as packs/<id>. A design on a server takes its checklists from the pack it is reviewed under")
			case remote == "" && isPack:
				return fmt.Errorf("%s is a pack on a server: add --remote <url>", args[0])
			case isPack:
				return packChecklists(cmd, remote, args[0], format)
			}
			u, err := cliArgURI(args[0])
			if err != nil {
				return err
			}
			ll := &localLoader{loader: newLoader()}
			snap, drop, err := ingest(cmd.Context(), service.NewSnapshotService(ll, cliProjects(), nil, ""), &webapi.Snapshot{Uri: u})
			if err != nil {
				return rpcFailed(err)
			}
			defer drop()
			svc := service.NewReviewService(nil, nil, nil, nil, nil, service.ReviewEnv{}, "", cliProjects())
			resp, err := svc.ListChecklists(cmd.Context(), &webapi.ListChecklistsRequest{Snapshot: snap})
			if err != nil {
				return rpcFailed(err)
			}
			w := cmd.OutOrStdout()
			if format == "json" {
				return printJSON(w, resp)
			}
			switch {
			case resp.GetProject() == "":
				fmt.Fprintf(w, "%s belongs to no project, so it has no declared checklists; run `agni review --checklist <manifest.yaml>`\n", args[0])
			case len(resp.GetChecklists()) == 0:
				fmt.Fprintf(w, "%s declares no checklists; add one under checklists: in its project.yaml\n", resp.GetProject())
			default:
				printChecklists(w, resp.GetChecklists())
			}
			return nil
		},
	}
	c.Flags().StringVar(&format, "format", "text", "output format: text (one checklist per line, the default first) or json (the ListChecklistsResponse the ListChecklists rpc returns)")
	c.Flags().StringVar(&remote, "remote", "", "the agni serve that keeps the pack named as packs/<id>")
	return c
}

// packChecklists lists the checklists of the pack name on the server at remote, the first being the
// one `agni review --pack` runs when no --checklist is named. Its JSON is a ListChecklistsResponse
// with no project, so a client reads both forms of the command alike.
func packChecklists(cmd *cobra.Command, remote, name, format string) error {
	r, err := dialRemote(cmd.Context(), remote)
	if err != nil {
		return err
	}
	p, err := unary(cmd.Context(), webapiconnect.NewPackServiceClient(r.http, r.url).GetPack, &webapi.GetPackRequest{Name: packName(name)})
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	list := p.GetConfig().GetChecklists()
	if format == "json" {
		return printJSON(w, &webapi.ListChecklistsResponse{Checklists: list})
	}
	if len(list) == 0 {
		fmt.Fprintf(w, "%s holds no checklists; add one under checklists: in its project.yaml and push it again\n", p.GetName())
		return nil
	}
	printChecklists(w, list)
	return nil
}

func printChecklists(w io.Writer, list []*webapi.NamedChecklist) {
	for i, c := range list {
		mark := ""
		if i == 0 {
			mark = "  (default)"
		}
		fmt.Fprintf(w, "%s  %s%s\n", c.GetName(), c.GetManifest().GetName(), mark)
	}
}

func printJSON(w io.Writer, m proto.Message) error {
	out, err := protoJSON(m)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(w, out)
	return err
}
