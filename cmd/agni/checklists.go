package main

import (
	"fmt"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"github.com/spf13/cobra"
)

// checklistsCmd is the CLI face of ReviewService.ListChecklists (agni issue 859): which checklists a
// design's project declares, inherited ones included, the first being the one `agni review` runs
// when no --checklist is named.
func checklistsCmd() *cobra.Command {
	var format string
	c := &cobra.Command{
		Use:   "checklists <design>",
		Short: "List the review checklists a design's project declares, inherited ones included",
		Long: "List the review checklists a design's project declares, in the order the project writes " +
			"them, inherited ones first, so the first is the one `agni review` runs by default. " +
			"--format json is the ListChecklistsResponse the ListChecklists rpc returns, each checklist " +
			"carrying its manifest, which a client sends to CreateReview to run it.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				return fmt.Errorf("unknown --format %q (want: text, json)", format)
			}
			u, err := cliArgURI(args[0])
			if err != nil {
				return err
			}
			svc := service.NewReviewService(nil, nil, nil, nil, nil, service.ReviewEnv{}, "", cliProjects())
			resp, err := svc.ListChecklists(cmd.Context(), &webapi.ListChecklistsRequest{DesignUri: u})
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if format == "json" {
				out, err := protoJSON(resp)
				if err != nil {
					return err
				}
				_, err = fmt.Fprint(w, out)
				return err
			}
			switch {
			case resp.GetProject() == "":
				fmt.Fprintf(w, "%s belongs to no project, so it has no declared checklists; run `agni review --checklist <manifest.yaml>`\n", args[0])
			case len(resp.GetChecklists()) == 0:
				fmt.Fprintf(w, "%s declares no checklists; add one under checklists: in its project.yaml\n", resp.GetProject())
			default:
				for i, c := range resp.GetChecklists() {
					mark := ""
					if i == 0 {
						mark = "  (default)"
					}
					fmt.Fprintf(w, "%s  %s%s\n", c.GetName(), c.GetManifest().GetName(), mark)
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&format, "format", "text", "output format: text (one checklist per line, the default first) or json (the ListChecklistsResponse the ListChecklists rpc returns)")
	return c
}
