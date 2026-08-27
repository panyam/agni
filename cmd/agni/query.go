package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/query"
	rpt "github.com/panyam/agni/core/report"
	"github.com/panyam/agni/datasheet/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/service"
	"github.com/spf13/cobra"
)

// queryCmd runs an ad-hoc datalog query over the design's fact base (WS3-029). The fact relations
// are the same ones rules assert over (net.max_voltage, component.mpn, param, component-on-net,
// plus the built-in reaches), so search and rules share one vocabulary; every answer row prints the
// provenance of the facts that produced it.
func queryCmd() *cobra.Command {
	var paramsDir string
	var conventions string
	var boardPath string
	var showExamples bool
	var showRelations bool
	var verbose bool
	var specLib bool
	var format, title string
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
			// Validated here rather than at render time, so a misspelled format fails before a
			// nine-megabyte netlist is parsed. `diff` learned this the same way.
			switch format {
			case "text", "csv", "json", "markdown", "html":
			default:
				return fmt.Errorf("unknown --format %q: want text, csv, json, markdown, or html", format)
			}
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
				return renderTable(cmd.OutOrStdout(), format, tableFromRows(q, rows, title, args[0], filepath.Base(paramsDir)))
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
			// Reading the convention file is the CLI's job; the service takes the value (C22), the same
			// shape `check` and `review` already use.
			overlay := &webapi.OverlayConfig{}
			if conventions != "" {
				cfg, err := naming.Load(conventions)
				if err != nil {
					return err
				}
				overlay.Config = &webapi.AnalysisConfig{Conventions: cfg}
			}
			svc := service.NewQueryService(&localLoader{loader: newLoader()}, specs, cliProjects())
			designURI, err := cliArgURI(args[0])
			if err != nil {
				return err
			}
			boardURI, err := cliArgURI(boardPath)
			if err != nil {
				return err
			}
			resp, err := svc.RunQuery(cmd.Context(), &webapi.RunQueryRequest{
				Uri: designURI, Query: args[1], Overlay: overlay, BoardUri: boardURI,
			})
			if err != nil {
				return err
			}
			return renderTable(cmd.OutOrStdout(), format, tableFromProto(resp, title, args[1], designURI))
		},
	}
	c.Flags().StringVar(&paramsDir, "params", "", "directory of seeded PartSpec textprotos (datasheet corpus) — enables the param relation")
	c.Flags().StringVar(&conventions, "conventions", "", "a naming-convention config (YAML) whose LEXICON is applied to the design read, so rail/feedback/pin.type answer under the project's own vocabulary rather than the built-in one. The config's rules half is not used here (a query runs no rules)")
	c.Flags().StringVar(&boardPath, "board-path", "", "a separate board-geometry export (.kicad_pcb / IPC-2581) to attach, so the board.* relations have facts to range over; without it they are empty")
	c.Flags().BoolVar(&specLib, "speclib", false, "query the whole seeded datasheet corpus (--params) with no <file>: the param/part.audience relations range over the whole spec library, not one design's parts")
	c.Flags().BoolVar(&showExamples, "examples", false, "print starter queries (the concept ladder the web panel shows) and exit")
	c.Flags().BoolVar(&showRelations, "relations", false, "print the queryable relation catalog (grouped by kind) and exit")
	c.Flags().StringVar(&format, "format", "text", "output format: text (the aligned terminal table), csv (spreadsheet-safe, header row, table only), json (rows with their citations kept apart), markdown or html (a VIEW: the question above its answer, ready to hand to someone). markdown and html carry the query; csv deliberately does not, because its first row has to be the header")
	c.Flags().StringVar(&title, "title", "", "name this view, shown as the heading in --format markdown and html. A saved question is a view; without a title it renders under its own query")
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

// renderTable writes a query answer in the requested format. One dispatch for both evaluation paths
// (the service and the --speclib direct one), so a format can never work on one and not the other.
func renderTable(w io.Writer, format string, t rpt.Table) error {
	switch format {
	case "csv":
		return rpt.TableCSV(w, t)
	case "json":
		return rpt.TableJSON(w, t)
	case "markdown":
		return rpt.TableMarkdown(w, t)
	case "html":
		return rpt.TableHTML(w, t)
	default:
		return rpt.TableText(w, t)
	}
}

// SOURCE IS THE DESIGN'S URI, NEVER THE HOST PATH. A view is an artifact that gets committed,
// mailed and pasted into a ticket, so a heading reading "/Users/someone/work/..." publishes the
// machine that ran it. That is the leak agni issue 501 fixed in provenance.source_file, and a new
// output format inherits the RULE but not the fix, so it has to be made again here.
//
// tableFromProto builds the view from a RunQueryResponse, the shape the in-process QueryService
// returns and the same one the web panel renders. The proto's per-cell sheet badges and locate
// reasons are the panel's navigation channel and have no meaning in a file, so they stop here.
func tableFromProto(resp *webapi.RunQueryResponse, title, query, source string) rpt.Table {
	t := rpt.Table{Title: title, Query: query, Source: source, Columns: resp.GetColumns()}
	for _, r := range resp.GetRows() {
		t.Rows = append(t.Rows, rpt.TableRow{Cells: r.GetCells(), Cites: r.GetCites()})
	}
	return t
}

// tableFromRows builds the view from Go rows, the --speclib path, which evaluates against the spec
// library rather than a design and so never goes through the service.
func tableFromRows(q query.Query, rows []query.Row, title, queryText, corpus string) rpt.Table {
	cols := q.Columns()
	t := rpt.Table{Title: title, Query: queryText, Source: corpus, Columns: make([]string, 0, len(cols))}
	for _, c := range cols {
		t.Columns = append(t.Columns, string(c))
	}
	for _, r := range rows {
		cells := make([]string, 0, len(cols))
		for _, c := range cols {
			cells = append(cells, r.Bind[c].S)
		}
		t.Rows = append(t.Rows, rpt.TableRow{Cells: cells, Cites: r.Cites})
	}
	return t
}
