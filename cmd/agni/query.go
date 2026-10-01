package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/core/query"
	rpt "github.com/panyam/agni/core/report"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"github.com/panyam/agni/stdlib/lib"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
)

// queryCmd runs an ad-hoc datalog query over the design's fact base (WS3-029). The relations are
// the ones rules assert over, so search and rules share one vocabulary. Every answer row prints the
// provenance of the facts that produced it.
func queryCmd() *cobra.Command {
	var paramsDir string
	var conventions string
	var boardPath string
	var outPath string
	var showExamples bool
	var showRelations bool
	var verbose bool
	var specLib bool
	var format, title string
	var setPath string
	var relDesign string
	var libDirs []string
	var budget int64
	var bindArgs []string
	c := &cobra.Command{
		Use:   "query <file> <query> | query <file> --set <queries.yaml> | query --relations [path]",
		Short: "Search the design fact base with a datalog query",
		Long: `Run an ad-hoc datalog query over the design's fact relations and print each answer with
its provenance. Relations:

  net.max_voltage(net, volts)        component.mpn(ref_des, mpn)
  net.nominal_voltage(net, volts)    component.net(ref_des, net)
  param.max(mpn, symbol, max)            param.range(mpn, symbol, kind, min, max)  [--params]
  net.reaches(from, net)                 (transitive: through series pass elements)

A term is a ?variable, a "string", or a number; relations join on shared variables; => projects.
--examples prints a set of starter queries (the same set the web panel shows).

  agni query board.kicad_sch --params seed/ \
    'component.mpn(?r,?m), param.max(?m,"VIN",?vmax), component.net(?r,?n), net.max_voltage(?n,?rail), ?vmax < ?rail => ?r, ?vmax, ?n, ?rail'`,
		// --examples takes no arguments, --relations an optional path, --speclib and --set take one,
		// and a design query takes <file> and <query>.
		Args: func(cmd *cobra.Command, args []string) error {
			// Validated here rather than at render time, so a misspelled format fails before a
			// nine-megabyte netlist is parsed.
			switch format {
			case "text", "csv", "json", "markdown", "html":
			default:
				return fmt.Errorf("unknown --format %q: want text, csv, json, markdown, or html", format)
			}
			if showExamples {
				return nil
			}
			if showRelations {
				if format != "text" && format != "json" {
					return fmt.Errorf("--relations writes text or json, not %s", format)
				}
				return cobra.MaximumNArgs(1)(cmd, args)
			}
			if setPath != "" {
				if specLib {
					return fmt.Errorf("--set cannot be combined with --speclib: a query set is asked of a design")
				}
				if format == "csv" {
					return fmt.Errorf("--set cannot write csv: a csv holds one table, and a set is several. Use --format json, or ask one query at a time")
				}
				return cobra.ExactArgs(1)(cmd, args)
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
			closeOut, err := redirectOut(cmd, outPath)
			if err != nil {
				return err
			}
			defer closeOut()
			// A budget rides the command's context to every query it evaluates, as a server's does
			// (agni issue 792). None by default: a local run's caller is the person waiting.
			cmd.SetContext(query.WithBudget(cmd.Context(), budget))
			if showRelations {
				path := ""
				if len(args) == 1 {
					path = args[0]
				}
				cfg := &webapi.AnalysisConfig{}
				if err := addLibraries(cfg, libDirs); err != nil {
					return err
				}
				return printRelations(cmd.Context(), cmd.OutOrStdout(), path, relDesign, cfg, format, verbose)
			}
			if showExamples {
				printExamples(cmd.OutOrStdout())
				return nil
			}
			bind, err := parseBindings(bindArgs)
			if err != nil {
				return err
			}
			if len(bind) > 0 && setPath != "" {
				return fmt.Errorf("--bind applies to one query; a set binds each query's variables under its own bind:")
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
				opts := query.EvalOptions(cmd.Context())
				if len(bind) > 0 {
					opts = append(opts, query.Bind(bind))
				}
				rows, err := query.Default.Eval(cmd.Context(), q, query.NewSpecLibBase(specs), opts...)
				if err != nil {
					return err
				}
				resp := respFromRows(q, rows, args[0], filepath.Base(paramsDir))
				resp.Bindings = service.BindingsProto(bind)
				return renderTable(cmd.OutOrStdout(), format, resp, tableFromProto(resp, title, args[0], filepath.Base(paramsDir)))
			}
			// A design query goes through the in-process QueryService (WS9-048), the same service and
			// BuildModel fact base the web query panel uses, so the two cannot drift. The CLI supplies
			// an os-backed loader and the datasheet corpus.
			var specs param.ParamProvider
			if paramsDir != "" {
				set, err := param.LoadSet(os.DirFS(paramsDir))
				if err != nil {
					return err
				}
				specs = set
			}
			// The CLI reads the convention file and the service takes the value (C22), as in `check`
			// and `review`.
			overlay := &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{}}
			if conventions != "" {
				cfg, err := naming.Load(conventions)
				if err != nil {
					return err
				}
				overlay.Config.Conventions = cfg
			}
			if err := addLibraries(overlay.Config, libDirs); err != nil {
				return err
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
			if setPath != "" {
				return runQuerySet(cmd, svc, setPath, designURI, boardURI, overlay, format, title)
			}
			resp, err := svc.RunQuery(cmd.Context(), &webapi.RunQueryRequest{
				Uri: designURI, Query: args[1], Overlay: overlay, BoardUri: boardURI, AsNamed: readAsNamed,
				Bindings: service.BindingsProto(bind),
			})
			if err != nil {
				return err
			}
			return renderTable(cmd.OutOrStdout(), format, resp, tableFromProto(resp, title, args[1], designURI))
		},
	}
	c.Flags().StringVar(&paramsDir, "params", "", "directory of seeded PartSpec textprotos (datasheet corpus) — enables the param relation")
	c.Flags().StringVar(&conventions, "conventions", "", "a naming-convention config (YAML) whose LEXICON is applied to the design read, so rail/feedback/pin.type answer under the project's own vocabulary rather than the built-in one. The config's rules half is not used here (a query runs no rules)")
	c.Flags().StringVar(&boardPath, "board-path", "", "a separate board-geometry export (.kicad_pcb / IPC-2581) to attach, so the board.* relations have facts to range over; without it they are empty")
	c.Flags().BoolVar(&specLib, "speclib", false, "query the whole seeded datasheet corpus (--params) with no <file>: the param/part.audience relations range over the whole spec library, not one design's parts")
	c.Flags().BoolVar(&showExamples, "examples", false, "print starter queries (the concept ladder the web panel shows) and exit")
	c.Flags().BoolVar(&showRelations, "relations", false, "print the namespace tree of everything a query can call and exit. A path argument narrows it: a module (`net`) lists its members, a member (`net.has_test_point`) prints its signature, kind, doc and, for a derived relation, its definition")
	c.Flags().StringVar(&format, "format", "text", "output format: text (the aligned terminal table), csv (spreadsheet-safe, header row, table only), json (rows with their citations kept apart), markdown or html (a VIEW: the question above its answer, ready to hand to someone). markdown and html carry the query; csv deliberately does not, because its first row has to be the header")
	outFileFlag(c, &outPath)
	c.Flags().StringVar(&setPath, "set", "", "a query set (YAML, or - for stdin): named queries sharing a preamble of rules, all answered over ONE read of the design. Takes the design alone, no query argument. Every query's answer is written, and the command exits non-zero if any could not be answered")
	c.Flags().StringVar(&title, "title", "", "name this view, shown as the heading in --format markdown and html. A saved question is a view; without a title it renders under its own query")
	c.Flags().StringArrayVar(&bindArgs, "bind", nil, "give a query variable a value, as name=value (repeatable): `--bind n=GND` asks the query with ?n bound to GND, exactly as if \"GND\" were written in its place. A value that parses as a number binds a number; quote it (n=\"3\") to bind text. A name the query does not use is an error")
	c.Flags().Int64Var(&budget, "budget", 0, "stop a query past this much work, in the units a fact base counts (comparisons plus generator rows), as `agni serve --query-budget` does. 0, the default, sets none")
	c.Flags().StringArrayVar(&libDirs, "lib", nil, "a directory of derived-relation modules (<module.path>.dl, optional docs/<member.path>.md) sent with the query, beside any the design's project carries. Repeatable")
	c.Flags().StringVar(&relDesign, "design", "", "with --relations, a design whose project's own library (lib/) joins the catalog, so its members list beside the shipped ones")
	c.Flags().BoolVar(&verbose, "verbose", false, "with --relations and no member path, also print each member's full reference doc")
	return c
}

// printRelations writes what sits at path in the namespace tree, through the same ListRelations rpc
// the viewer's picker asks (agni issue 751), so the two cannot describe a member differently. A
// design, when named, joins its project's own library to the tree (agni issue 773). The
// root prints every module with its members' signatures and one-line docs, a module prints its own
// members, and a member prints everything the rpc carries. --format json writes the rpc's
// ListRelationsResponse for the path (C31); for the root that is the entry, not the flat catalog.
func printRelations(ctx context.Context, w io.Writer, path, design string, cfg *webapi.AnalysisConfig, format string, verbose bool) error {
	svc := service.NewQueryService(nil, nil, cliProjects())
	uri, err := cliArgURI(design)
	if err != nil {
		return err
	}
	describe := func(p string) (*webapi.RelationEntry, error) {
		if p == "" {
			p = "."
		}
		resp, err := svc.ListRelations(ctx, &webapi.ListRelationsRequest{Path: p, Uri: uri, Overlay: &webapi.OverlayConfig{Config: cfg}})
		if err != nil {
			return nil, err
		}
		return resp.GetEntry(), nil
	}
	e, err := describe(path)
	if err != nil {
		return err
	}
	if format == "json" {
		b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(&webapi.ListRelationsResponse{Entry: e})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(b))
		return err
	}
	if e.GetEntryKind() != "module" {
		printMember(w, e)
		return nil
	}
	var walk func(mod *webapi.RelationEntry, recurse bool) error
	walk = func(mod *webapi.RelationEntry, recurse bool) error {
		var subs []*webapi.RelationEntry
		header := false
		for _, m := range mod.GetMembers() {
			if m.GetEntryKind() == "module" {
				subs = append(subs, m)
				continue
			}
			if !header {
				name := mod.GetPath()
				if name == "" {
					name = "(root)"
				}
				fmt.Fprintf(w, "\n[%s]\n", name)
				header = true
			}
			fmt.Fprintf(w, "  %s  %s\n      %s\n", m.GetSignature(), m.GetEntryKind(), m.GetDoc())
			if verbose && m.GetDetail() != "" {
				for _, line := range strings.Split(strings.TrimRight(m.GetDetail(), "\n"), "\n") {
					fmt.Fprintf(w, "      %s\n", line)
				}
				fmt.Fprintln(w)
			}
		}
		for _, sub := range subs {
			if !recurse {
				fmt.Fprintf(w, "  %s  module (agni query --relations %s)\n", sub.GetPath(), sub.GetPath())
				continue
			}
			full, err := describe(sub.GetPath())
			if err != nil {
				return err
			}
			if err := walk(full, true); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(e, path == "")
}

// printMember writes one member: its signature and kind, which argument types were inferred rather
// than declared, its doc, a derived relation's definition, and its reference page when it has one.
func printMember(w io.Writer, e *webapi.RelationEntry) {
	fmt.Fprintln(w, e.GetSignature())
	kind := e.GetEntryKind()
	if e.GetModule() != "" {
		kind += ", defined in module " + e.GetModule()
	}
	fmt.Fprintf(w, "  %s\n", kind)
	if inf := e.GetInferred(); len(inf) > 0 {
		fmt.Fprintf(w, "  inferred from the rules: %s\n", strings.Join(inf, ", "))
	}
	if d := e.GetDoc(); d != "" {
		fmt.Fprintf(w, "\n  %s\n", d)
	}
	if def := e.GetDefinition(); len(def) > 0 {
		fmt.Fprintln(w, "\n  defined as")
		for _, clause := range def {
			fmt.Fprintf(w, "    %s\n", clause)
		}
	}
	if d := e.GetDetail(); d != "" {
		fmt.Fprintln(w)
		for _, line := range strings.Split(strings.TrimRight(d, "\n"), "\n") {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}
}

// printExamples writes the shared teaching-query catalog (WS14-002), the set the web panel renders,
// in concept-ladder order, each as a runnable line the user can copy.
func printExamples(w io.Writer) {
	for _, e := range query.Examples() {
		fmt.Fprintf(w, "%s  (%s)\n  %s\n\n", e.Label, e.Teaches, e.Query)
	}
}

// renderTable writes a query answer in the requested format. One dispatch for both evaluation paths
// (the service and the --speclib direct one), so a format can never work on one and not the other.
func renderTable(w io.Writer, format string, resp *webapi.RunQueryResponse, t rpt.Table) error {
	switch format {
	case "csv":
		return rpt.TableCSV(w, t)
	case "json":
		// protojson of the WIRE message RunQuery returns (C31, agni issue 603), including
		// column_kinds and the echoed query and design.
		b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(resp)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(b))
		return err
	case "markdown":
		return rpt.TableMarkdown(w, t)
	case "html":
		return rpt.TableHTML(w, t)
	default:
		return rpt.TableText(w, t)
	}
}

// runQuerySet answers a query set over one read of the design (agni issue 729) and renders every
// answer. Reading the file is the CLI's job; the service takes the set as a value (C22). The whole
// document is written before any failure is reported, so a script gets the answers that exist and a
// non-zero exit that says some are missing, rather than either alone.
func runQuerySet(cmd *cobra.Command, svc *service.QueryService, path, designURI, boardURI string, overlay *webapi.OverlayConfig, format, title string) error {
	var b []byte
	var err error
	if path == "-" {
		// The Python client's CLI transport pipes a set in on stdin rather than writing a file.
		path = "stdin"
		b, err = io.ReadAll(cmd.InOrStdin())
	} else {
		b, err = os.ReadFile(path)
	}
	if err != nil {
		return err
	}
	set, err := query.ParseQuerySet(b)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if title != "" {
		set.Title = title
	}
	resp, err := svc.RunQueries(cmd.Context(), &webapi.RunQueriesRequest{
		Set: service.QuerySetProto(set), Uri: designURI, BoardUri: boardURI, Overlay: overlay, AsNamed: readAsNamed,
	})
	if err != nil {
		return err
	}
	if err := renderQuerySet(cmd.OutOrStdout(), format, set, resp); err != nil {
		return err
	}
	failed := 0
	for _, r := range resp.GetResults() {
		if r.GetError() != "" {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d queries in %s could not be answered", failed, len(resp.GetResults()), path)
	}
	return nil
}

// renderQuerySet writes a set's answers in the requested format. json is the wire message, as for a
// single query (C31); the others are one document with a section per query.
func renderQuerySet(w io.Writer, format string, set query.QuerySet, resp *webapi.RunQueriesResponse) error {
	if format == "json" {
		b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(resp)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(b))
		return err
	}
	ts := rpt.TableSet{Title: resp.GetTitle(), Source: resp.GetSource(), Preamble: resp.GetPreamble()}
	for i, r := range resp.GetResults() {
		sec := rpt.TableSection{Name: r.GetName(), Description: r.GetDescription(), Error: r.GetError()}
		if r.GetResult() != nil {
			sec.Table = tableFromProto(r.GetResult(), "", r.GetResult().GetQuery(), "")
		} else {
			sec.Table = rpt.Table{Query: set.Queries[i].Query, Bindings: query.FormatBindings(set.Queries[i].Bind)}
		}
		ts.Sections = append(ts.Sections, sec)
	}
	switch format {
	case "markdown":
		return rpt.TableSetMarkdown(w, ts)
	case "html":
		return rpt.TableSetHTML(w, ts)
	default:
		return rpt.TableSetText(w, ts)
	}
}

// tableFromProto builds the view from a RunQueryResponse, the shape the QueryService returns and
// the web panel renders. Per-cell sheet badges and locate reasons are panel navigation and mean
// nothing in a file, so they stop here.
//
// SOURCE IS THE DESIGN'S URI, NEVER THE HOST PATH. A view gets committed and pasted into tickets,
// so a host path publishes the machine that ran it. agni issue 501 fixed that leak in
// provenance.source_file, and a new output format has to keep the rule itself.
func tableFromProto(resp *webapi.RunQueryResponse, title, queryText, source string) rpt.Table {
	t := rpt.Table{Title: title, Query: queryText, Source: source, Columns: resp.GetColumns(),
		Bindings: query.FormatBindings(service.BindingsFromProto(resp.GetBindings()))}
	for _, r := range resp.GetRows() {
		t.Rows = append(t.Rows, rpt.TableRow{Cells: r.GetCells(), Cites: r.GetCites()})
	}
	return t
}

// respFromRows builds the wire message from the --speclib path's Go rows, which evaluate against the
// spec library and never go through the service. It exists so BOTH paths render from one shape,
// and json works on a corpus as well as a design (agni issue 603).
//
// It carries no column_kinds. A spec-library answer ranges over a corpus, so no cell names an
// entity anything could locate.
func respFromRows(q query.Query, rows []query.Row, queryText, corpus string) *webapi.RunQueryResponse {
	cols := q.Columns()
	resp := &webapi.RunQueryResponse{Query: queryText, Source: corpus, Columns: make([]string, 0, len(cols))}
	for _, c := range cols {
		resp.Columns = append(resp.Columns, string(c))
	}
	for _, r := range rows {
		cells := make([]string, 0, len(cols))
		for _, c := range cols {
			cells = append(cells, r.Bind[c].S)
		}
		resp.Rows = append(resp.Rows, &webapi.QueryRow{Cells: cells, Cites: r.Cites})
	}
	return resp
}

// addLibraries reads each --lib directory and adds its modules and pages to cfg as values, so the CLI
// sends a library exactly as any other client does (agni issue 788) rather than handing the service a
// path. A module's source names the directory and file it came from, for errors.
func addLibraries(cfg *webapi.AnalysisConfig, dirs []string) error {
	for _, dir := range dirs {
		mods, docs, err := lib.Read(os.DirFS(dir))
		if err != nil {
			return fmt.Errorf("--lib %s: %w", dir, err)
		}
		if len(mods) == 0 {
			return fmt.Errorf("--lib %s holds no .dl module", dir)
		}
		for _, m := range mods {
			cfg.LibraryModules = append(cfg.LibraryModules, &webapi.LibraryModule{
				Path: m.Path, Text: m.Text, Source: filepath.Join(dir, m.File),
			})
		}
		for p, d := range docs {
			if cfg.LibraryDocs == nil {
				cfg.LibraryDocs = map[string]string{}
			}
			cfg.LibraryDocs[p] = d
		}
	}
	return nil
}

// parseBindings reads each --bind name=value into the values the query is asked with.
func parseBindings(args []string) (map[string]query.Value, error) {
	if len(args) == 0 {
		return nil, nil
	}
	out := map[string]query.Value{}
	for _, a := range args {
		name, v, err := query.ParseBinding(a)
		if err != nil {
			return nil, fmt.Errorf("--bind: %w", err)
		}
		out[name] = v
	}
	return out, nil
}
