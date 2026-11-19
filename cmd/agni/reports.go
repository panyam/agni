package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
)

func reportsCmd() *cobra.Command {
	var remoteURL string
	cmd := &cobra.Command{
		Use:   "reports",
		Short: "Check a design an agni server keeps against packs, and keep the run under the design",
		Long: `reports manages the reports an agni serve started with --store keeps (agni issue 1704). A report
is a check of a stored design against packs, named designs/<id>/reports/<report>. It records the
design's content hash and each pack's hash, so a later list says when the design's files or a pack
changed since it ran.

  agni reports create designs/bb-1.1.3 --pack jetson --remote http://localhost:8080
  agni reports list designs/bb-1.1.3 --remote http://localhost:8080`,
	}
	cmd.PersistentFlags().StringVar(&remoteURL, "remote", "", "the agni serve that keeps the reports, such as http://localhost:8080 (required)")
	client := func(cmd *cobra.Command) (webapiconnect.DesignServiceClient, error) {
		if remoteURL == "" {
			return nil, fmt.Errorf("agni reports needs --remote, the URL of an agni serve started with --store")
		}
		r, err := dialRemote(cmd.Context(), remoteURL)
		if err != nil {
			return nil, err
		}
		return webapiconnect.NewDesignServiceClient(r.http, r.url), nil
	}

	var packs []string
	var project, id string
	create := &cobra.Command{
		Use:   "create <design>",
		Short: "Check a stored design against packs, and keep the run",
		Long: `create checks the design, as <id> or designs/<id>, against the packs named, and keeps the run.
With --project, the project's default packs apply first, then the --pack ones. The run checks the
whole catalog the packs compose, as agni check with no --rule does.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			req := &webapi.CreateReportRequest{Parent: designName(args[0]), ReportId: id, Report: &webapi.Report{Packs: packNames(packs)}}
			if project != "" {
				req.Project = projectName(project)
			}
			r, err := unary(cmd.Context(), c.CreateReport, req)
			if err != nil {
				return err
			}
			printReport(cmd.OutOrStdout(), r)
			return nil
		},
	}
	create.Flags().StringArrayVar(&packs, "pack", nil, "a pack to apply, as <k> or packs/<k>; repeatable, each layering over the one before")
	create.Flags().StringVar(&project, "project", "", "a project, as <id> or projects/<id>, whose default packs apply first")
	create.Flags().StringVar(&id, "id", "", "the report's id; the server assigns one that sorts by time when empty")

	list := &cobra.Command{
		Use:   "list <design>",
		Short: "List a design's reports, newest first, each saying what changed since it ran",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			for token, pages := "", 0; ; pages++ {
				resp, err := unary(cmd.Context(), c.ListReports, &webapi.ListReportsRequest{Parent: designName(args[0]), PageToken: token})
				if err != nil {
					return err
				}
				for _, r := range resp.GetReports() {
					printReport(cmd.OutOrStdout(), r)
				}
				if token = resp.GetNextPageToken(); token == "" {
					return nil
				}
				if pages > 10000 {
					return fmt.Errorf("the server's listing has not ended after %d pages", pages)
				}
			}
		},
	}

	get := &cobra.Command{
		Use:   "get <report>",
		Short: "Print a report and its findings",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			r, err := unary(cmd.Context(), c.GetReport, &webapi.GetReportRequest{Name: args[0]})
			if err != nil {
				return err
			}
			printReport(cmd.OutOrStdout(), r)
			for _, f := range r.GetResults().GetFindings() {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s: %s:%s (%s)\n", f.GetSeverity(), f.GetRule(), f.GetSubject().GetKind(), f.GetSubject().GetRef(), f.GetMessage())
			}
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm <report>",
		Short: "Delete a report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			_, err = unary(cmd.Context(), c.DeleteReport, &webapi.DeleteReportRequest{Name: args[0]})
			return err
		},
	}

	cmd.AddCommand(create, list, get, rm)
	return cmd
}

// printReport writes one line per report: its name, packs, finding count, and what changed since.
func printReport(w io.Writer, r *webapi.Report) {
	var changed []string
	if r.GetDesignChanged() {
		changed = append(changed, "run on earlier design files")
	}
	for _, p := range r.GetPacksChanged() {
		changed = append(changed, p+" changed since")
	}
	note := "current"
	if len(changed) > 0 {
		note = strings.Join(changed, "; ")
	}
	fmt.Fprintf(w, "%s\t%s\t%d findings\t%s\n", r.GetName(), strings.Join(r.GetPacks(), ","), r.GetFindingCount(), note)
}
