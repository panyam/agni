package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/datasheet/param"
	"github.com/panyam/agni/query"
	"github.com/spf13/cobra"
)

// queryCmd runs an ad-hoc datalog query over the design's fact base (WS3-029). The fact relations
// are the same ones rules assert over (net.max_voltage, component.mpn, param, component-on-net,
// plus the built-in reaches), so search and rules share one vocabulary; every answer row prints the
// provenance of the facts that produced it.
func queryCmd() *cobra.Command {
	var paramsDir string
	var showExamples bool
	var showRelations bool
	var verbose bool
	var specLib bool
	c := &cobra.Command{
		Use:   "query <file> <query>",
		Short: "Search the design fact base with a datalog query",
		Long: `Run an ad-hoc datalog query over the design's fact relations and print each answer with
its provenance. Relations:

  net.max_voltage(net, volts)        component.mpn(ref_des, mpn)
  net.nominal_voltage(net, volts)    component-on-net(ref_des, net)
  param(mpn, symbol, max)            param.range(mpn, symbol, kind, min, max)  [--params]
  reaches(from, net)                 (transitive: through series pass elements)

A term is a ?variable, a "string", or a number; relations join on shared variables; => projects.
--examples prints a set of starter queries (the same set the web panel shows).

  agni query board.kicad_sch --params seed/ \
    'component.mpn(?r,?m), param(?m,"VIN",?vmax), component-on-net(?r,?n), net.max_voltage(?n,?rail), ?vmax < ?rail => ?r, ?vmax, ?n, ?rail'`,
		// --examples / --relations need no file/query; --speclib queries the corpus (--params) with no
		// <file>, so just the <query>; otherwise both <file> and <query> are required.
		Args: func(cmd *cobra.Command, args []string) error {
			if showExamples || showRelations {
				return nil
			}
			if specLib {
				if paramsDir == "" {
					return fmt.Errorf("--speclib needs --params <dir> (the datasheet corpus to query)")
				}
				return cobra.ExactArgs(1)(cmd, args)
			}
			return cobra.ExactArgs(2)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if showRelations {
				printRelations(cmd.OutOrStdout(), verbose)
				return nil
			}
			if showExamples {
				printExamples(cmd.OutOrStdout())
				return nil
			}
			if specLib {
				specs, err := param.LoadSet(os.DirFS(paramsDir))
				if err != nil {
					return err
				}
				q, err := query.Parse(args[0])
				if err != nil {
					return err
				}
				rows, err := (query.Naive{}).Eval(q, query.NewSpecLibBase(specs))
				if err != nil {
					return err
				}
				printQueryRows(q, rows)
				return nil
			}
			// Thin client of the in-process QueryService (WS9-048): the CLI provides an os-backed
			// no-containment loader and the datasheet corpus, then renders the proto rows — the same
			// service (and BuildModel fact base) the web query panel evaluates over, so the two can't
			// drift. --examples/--relations (static) and --speclib (a corpus query, no design) stay
			// CLI-direct above; only the design query routes through the service.
			var specs param.ParamProvider
			if paramsDir != "" {
				set, err := param.LoadSet(os.DirFS(paramsDir))
				if err != nil {
					return err
				}
				specs = set
			}
			svc := service.NewQueryService(&localLoader{loader: newLoader()}, specs)
			resp, err := svc.RunQuery(cmd.Context(), &webapi.RunQueryRequest{Path: args[0], Query: args[1]})
			if err != nil {
				return err
			}
			printQueryRowsProto(cmd.OutOrStdout(), resp)
			return nil
		},
	}
	c.Flags().StringVar(&paramsDir, "params", "", "directory of seeded PartSpec textprotos (datasheet corpus) — enables the param relation")
	c.Flags().BoolVar(&specLib, "speclib", false, "query the whole seeded datasheet corpus (--params) with no <file>: the param/part.audience relations range over the whole spec library, not one design's parts")
	c.Flags().BoolVar(&showExamples, "examples", false, "print starter queries (the concept ladder the web panel shows) and exit")
	c.Flags().BoolVar(&showRelations, "relations", false, "print the queryable relation catalog (grouped by kind) and exit")
	c.Flags().BoolVar(&verbose, "verbose", false, "with --relations, also print each relation's full reference doc")
	return c
}

// printRelations writes the queryable relation catalog (WS14-005), the same set the web panel's
// picker shows, grouped by kind in the catalog's stable order. Each line is the relation template
// plus its one-line summary; with verbose, the relation's full reference doc (its Detail markdown)
// follows, so `--relations --verbose` is the CLI counterpart of the panel's click-to-inspect.
func printRelations(w io.Writer, verbose bool) {
	var kind string
	for _, r := range query.Catalog() {
		if r.Kind != kind {
			kind = r.Kind
			fmt.Fprintf(w, "\n[%s]\n", kind)
		}
		args := ""
		if len(r.Args) > 0 {
			args = "(?" + strings.Join(r.Args, ", ?") + ")"
		}
		fmt.Fprintf(w, "  %s%s\n      %s\n", r.Name, args, r.Summary)
		if verbose && r.Detail != "" {
			for _, line := range strings.Split(strings.TrimRight(r.Detail, "\n"), "\n") {
				fmt.Fprintf(w, "      %s\n", line)
			}
			fmt.Fprintln(w)
		}
	}
}

// printExamples writes the shared teaching-query catalog (WS14-002) — the same set the web panel
// renders — in concept-ladder order, each as a runnable line the user can copy.
func printExamples(w io.Writer) {
	for _, e := range query.Examples() {
		fmt.Fprintf(w, "%s  (%s)\n  %s\n\n", e.Label, e.Teaches, e.Query)
	}
}

// printQueryRowsProto renders a RunQueryResponse as the same table printQueryRows prints from Go rows:
// the projected columns plus a provenance column, then the result count. The proto's per-cell sheet
// badges and locate reasons (the web panel's navigation channel) are not shown — the CLI table is the
// same text as before the thin-client conversion.
func printQueryRowsProto(w io.Writer, resp *webapi.RunQueryResponse) {
	if len(resp.GetRows()) == 0 {
		fmt.Fprintln(w, "no results")
		return
	}
	cols := resp.GetColumns()
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(append(append([]string{}, cols...), "provenance"), "\t"))
	for _, r := range resp.GetRows() {
		cells := append(append([]string{}, r.GetCells()...), strings.Join(r.GetCites(), " ; "))
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	tw.Flush()
	fmt.Fprintf(w, "\n%d result(s)\n", len(resp.GetRows()))
}

func printQueryRows(q query.Query, rows []query.Row) {
	if len(rows) == 0 {
		fmt.Println("no results")
		return
	}
	cols := q.Columns()
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	hdr := make([]string, 0, len(cols)+1)
	for _, c := range cols {
		hdr = append(hdr, string(c))
	}
	fmt.Fprintln(w, strings.Join(append(hdr, "provenance"), "\t"))
	for _, r := range rows {
		cells := make([]string, 0, len(cols)+1)
		for _, c := range cols {
			cells = append(cells, r.Bind[c].S)
		}
		fmt.Fprintln(w, strings.Join(append(cells, strings.Join(r.Cites, " ; ")), "\t"))
	}
	w.Flush()
	fmt.Printf("\n%d result(s)\n", len(rows))
}
