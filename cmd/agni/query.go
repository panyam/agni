package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"

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
	var orderBy []string
	var setPath string
	var relDesign string
	var libDirs []string
	var budget int64
	var bindArgs []string
	var remoteURL string
	var serverVal string
	var srvSpec serverSpec
	var witnesses bool
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
			// A csv's first row has to be the header and every row a row of the table, so a tree has
			// nowhere to go (agni issue 794).
			if witnesses && format == "csv" {
				return fmt.Errorf("--witnesses prints a tree under each row, and a csv holds the table alone. Use --format json, text, markdown or html")
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
			// Before redirectOut, so a --server this process cannot honour refuses the run before the
			// output file exists (agni issue 637).
			var err error
			if serverVal != "" && (specLib || showRelations || showExamples) {
				return fmt.Errorf("--server links a design's view, and this run reads no design")
			}
			if srvSpec, err = resolveServer(serverVal); err != nil {
				return err
			}
			closeOut, err := redirectOut(cmd, outPath)
			if err != nil {
				return err
			}
			defer closeOut()
			// A budget rides the command's context to every query it evaluates, as a server's does
			// (agni issue 792). None by default: a local run's caller is the person waiting.
			cmd.SetContext(query.WithBudget(cmd.Context(), budget))
			if remoteURL != "" && (showRelations || specLib) {
				return fmt.Errorf("--remote sends a design query to a server; --relations and --speclib read no design and run here")
			}
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
				return fmt.Errorf("--bind applies to one query; a set binds each query's variables under its own bind: key")
			}
			if specLib {
				specs, err := param.LoadSet(os.DirFS(paramsDir))
				if err != nil {
					return err
				}
				// The service answers the corpus as it answers a design, through one projection (agni
				// issue 1045), so the CLI builds no response of its own.
				resp, err := service.NewQueryService(nil, nil, nil).RunSpecLibQuery(cmd.Context(), specs, args[0], filepath.Base(paramsDir), bind, witnesses)
				if err != nil {
					return withCaret(rpcFailed(err))
				}
				return renderTable(cmd.OutOrStdout(), format, resp, tableFromProto(resp, title, args[0], filepath.Base(paramsDir)), orderBy)
			}
			// A design query goes through the in-process QueryService (WS9-048), the same service and
			// BuildModel fact base the web query panel uses, so the two cannot drift. The CLI supplies
			// an os-backed loader and the datasheet corpus.
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
			// --remote sends the same request to a running server (agni issue 998), so everything below
			// reads one answer whichever engine produced it.
			var svc queryAPI
			var snaps snapshotAPI
			addr := cliArgURI
			if remoteURL != "" {
				if err := refuseLocalOnly(cmd, "params"); err != nil {
					return err
				}
				r, err := dialRemote(cmd.Context(), remoteURL)
				if err != nil {
					return err
				}
				svc, snaps, addr = r.query(), r.snapshots(), r.uri
			} else {
				var specs param.ParamProvider
				if paramsDir != "" {
					set, err := param.LoadSet(os.DirFS(paramsDir))
					if err != nil {
						return err
					}
					specs = set
				}
				ll := &localLoader{loader: newLoader()}
				svc, snaps = service.NewQueryService(ll, specs, cliProjects()), service.NewSnapshotService(ll, cliProjects(), specs, "")
			}
			designURI, err := addr(args[0])
			if err != nil {
				return err
			}
			boardURI, err := addr(boardPath)
			if err != nil {
				return err
			}
			// --conventions and --lib ride the snapshot's definition, so the lexicon reaches the read.
			snap, drop, err := ingest(cmd.Context(), snaps, &webapi.Snapshot{Uri: designURI, Overlay: overlay, BoardUri: boardURI, AsNamed: readAsNamed})
			if err != nil {
				return rpcFailed(err)
			}
			defer drop()
			if setPath != "" {
				if queryLimit != 0 {
					return fmt.Errorf("--limit caps one query's answer; a set's queries write their own limit")
				}
				return runQuerySet(cmd, svc, setPath, snap, format, title, budget, witnesses, queryLinker(cmd, srvSpec, args[0]))
			}
			resp, err := svc.RunQuery(cmd.Context(), &webapi.RunQueryRequest{
				Snapshot: snap, Query: args[1],
				OmitLocations: queryOmitLocations,
				Bindings:      service.BindingsProto(bind),
				Limit:         queryLimit,
				Witnesses:     witnesses,
				// The context carries the budget in this process; a server reads it off the request.
				WorkBudget: budget,
			})
			if err != nil {
				return withCaret(rpcFailed(err))
			}
			// The link asks the same question again in the viewer (agni issue 634), with the values it
			// was bound to, so it re-asks this question rather than a wider one.
			if link := queryLinker(cmd, srvSpec, args[0]); link != nil {
				if u := link(func(meta rpt.Report) string { return rpt.QueryURL(meta, args[1], title, bindArgs) }); u != "" {
					defer fmt.Fprintf(cmd.ErrOrStderr(), "\nlook at it: %s\n", u)
				}
			}
			return renderTable(cmd.OutOrStdout(), format, resp, tableFromProto(resp, title, args[1], designURI), orderBy)
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
	c.Flags().Int32Var(&queryLimit, "limit", 0, "answer only the first N rows and say how many were left out (agni issue 896); where the query writes its own limit, the smaller applies. Not taken with --set")
	c.Flags().BoolVar(&witnesses, "witnesses", false, "say how each row was derived: under each row, the fact or rule behind every clause of the query, a library member's own rule with its body facts filled in, and a `not` that held as an absence (agni issue 794). text and html print the tree under its row, markdown after the table, json in each row's witnesses; csv refuses it. Off by default, since recording derivations costs the evaluation memory")
	c.Flags().BoolVar(&queryOmitLocations, "omit-locations", false, "leave out where each answer cell is drawn (its sheets and locate reason) and skip loading the drawing; for a script that draws nothing, this is most of a large json answer's size")
	c.Flags().StringVar(&setPath, "set", "", "a query set (a .yaml file, - for stdin, or the name of a set in the design's project queries/ directory): named queries sharing a preamble of rules, all answered over ONE read of the design. Takes the design alone, no query argument. Every query's answer is written, and the command exits non-zero if any could not be answered")
	c.Flags().StringSliceVar(&orderBy, "order-by", nil, "order the csv rows by these columns, comma-separated, a leading - for descending: --order-by=rule,-subject. A column sorts by its type, so a net or part column puts R2 before R10. csv only, since it orders the projected table the TableService serves (agni issue 862)")
	c.Flags().StringVar(&title, "title", "", "name this view, shown as the heading in --format markdown and html. A saved question is a view; without a title it renders under its own query")
	c.Flags().StringArrayVar(&bindArgs, "bind", nil, "give a query variable a value, as name=value (repeatable): `--bind n=GND` asks the query with ?n bound to GND, exactly as if \"GND\" were written in its place. A value that parses as a number binds a number; quote it (n=\"3\") to bind text. Naming a variable twice binds it to both values, and the query answers for each (`--bind n=I2C_SDA --bind n=I2C_SCL`). A name the query does not use is an error")
	c.Flags().Int64Var(&budget, "budget", 0, "stop a query past this much work, in the units a fact base counts (comparisons, generator rows and the connections a circuit walk examines), as `agni serve --query-budget` does. 0, the default, sets none")
	c.Flags().StringArrayVar(&libDirs, "lib", nil, "a directory of derived-relation modules (<module.path>.dl, optional docs/<member.path>.md) sent with the query, beside any the design's project carries. Repeatable")
	remoteFlag(c, &remoteURL)
	serverFlag(c, &serverVal)
	withSelfServer(c, &srvSpec)
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
	// A design's project library joins the tree through a snapshot of it, and with no design the
	// --lib directories ride the request alone.
	req := &webapi.ListRelationsRequest{Overlay: &webapi.OverlayConfig{Config: cfg}}
	if design != "" {
		uri, err := cliArgURI(design)
		if err != nil {
			return err
		}
		ll := &localLoader{loader: newLoader()}
		snap, drop, err := ingest(ctx, service.NewSnapshotService(ll, cliProjects(), nil, ""), &webapi.Snapshot{Uri: uri, Overlay: req.Overlay})
		if err != nil {
			return rpcFailed(err)
		}
		defer drop()
		req = &webapi.ListRelationsRequest{Snapshot: snap}
	}
	describe := func(p string) (*webapi.RelationEntry, error) {
		if p == "" {
			p = "."
		}
		resp, err := svc.ListRelations(ctx, &webapi.ListRelationsRequest{Path: p, Snapshot: req.Snapshot, Overlay: req.Overlay})
		if err != nil {
			return nil, rpcFailed(err)
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

// queryOmitLocations is `agni query --omit-locations`, carried on both the single query and the set.
var queryOmitLocations bool

// queryLimit is `agni query --limit`, RunQueryRequest.limit.
var queryLimit int32

// renderTable writes a query answer in the requested format. One dispatch for both evaluation paths
// (the service and the --speclib direct one), so a format can never work on one and not the other.
func renderTable(w io.Writer, format string, resp *webapi.RunQueryResponse, t rpt.Table, orderBy []string) error {
	if len(orderBy) > 0 && format != "csv" {
		return fmt.Errorf("--order-by orders a csv table; --format %s has none", format)
	}
	switch format {
	case "csv":
		// The TableService's projection, so this csv and a workbook of the same answer agree (agni 862).
		return writeTableCSV(w, service.QueryTable("query", resp), orderBy)
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
//
// path is a set file, - for stdin, or the name of a set the design's project keeps in its queries/
// directory (agni issue 745), which ListQuerySets answers as it does for the viewer. Only a name
// ending .yaml or .yml is read as a file, the rule --checklist follows.
func runQuerySet(cmd *cobra.Command, svc queryAPI, path, snap, format, title string, budget int64, witnesses bool, link linker) error {
	named := path
	set, path, err := readQuerySet(cmd, svc, path, snap)
	if err != nil {
		return err
	}
	if title != "" {
		set.Title = title
	}
	resp, err := svc.RunQueries(cmd.Context(), &webapi.RunQueriesRequest{
		Set: service.QuerySetProto(set), Snapshot: snap, WorkBudget: budget,
		OmitLocations: queryOmitLocations, Witnesses: witnesses,
	})
	if err != nil {
		return rpcFailed(err)
	}
	if err := renderQuerySet(cmd.OutOrStdout(), format, set, resp); err != nil {
		return err
	}
	// One link per query. A set the project keeps is linked by its name and the query's, so the link
	// stays short and follows the set; a set read from a file is not on the server, so its link
	// carries each query's text as the set would ask it (agni issue 634).
	if link != nil {
		fromProject := named != "-" && !strings.HasSuffix(named, ".yaml") && !strings.HasSuffix(named, ".yml")
		for i, q := range set.Queries {
			u := link(func(meta rpt.Report) string {
				if fromProject {
					return rpt.QuerySetURL(meta, named, q.Name)
				}
				return rpt.QueryURL(meta, set.Text(i), q.Name, query.BindArgs(q.Bind))
			})
			if u != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "look at %s: %s\n", q.Name, u)
			}
		}
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

// readQuerySet reads the set --set names, and the name to report it under.
func readQuerySet(cmd *cobra.Command, svc queryAPI, path, snap string) (query.QuerySet, string, error) {
	var b []byte
	var err error
	switch {
	case path == "-":
		// The Python client's CLI transport pipes a set in on stdin rather than writing a file.
		path = "stdin"
		b, err = io.ReadAll(cmd.InOrStdin())
	case strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml"):
		b, err = os.ReadFile(path) // #nosec G304 -- reads a file the user named
	default:
		return projectQuerySet(cmd, svc, path, snap)
	}
	if err != nil {
		return query.QuerySet{}, path, err
	}
	set, err := query.ParseQuerySet(b)
	if err != nil {
		return query.QuerySet{}, path, fmt.Errorf("%s: %w", path, err)
	}
	return set, path, nil
}

// projectQuerySet finds the set called name among those the design's project keeps, and names the
// ones it does keep when there is no such set.
func projectQuerySet(cmd *cobra.Command, svc queryAPI, name, snap string) (query.QuerySet, string, error) {
	resp, err := svc.ListQuerySets(cmd.Context(), &webapi.ListQuerySetsRequest{Snapshot: snap})
	if err != nil {
		return query.QuerySet{}, name, rpcFailed(err)
	}
	var have []string
	for _, s := range resp.GetQuerySets() {
		if s.GetName() == name {
			set, err := service.QuerySetFromProto(s.GetSet())
			return set, s.GetSource(), err
		}
		have = append(have, s.GetName())
	}
	switch {
	case resp.GetProject() == "":
		return query.QuerySet{}, name, fmt.Errorf("--set %s: the design belongs to no project, so only a set file (.yaml) can be named", name)
	case len(have) == 0:
		return query.QuerySet{}, name, fmt.Errorf("--set %s: %s keeps no query sets (a queries/ directory of .yaml files)", name, resp.GetProject())
	}
	return query.QuerySet{}, name, fmt.Errorf("--set %s: %s keeps no set by that name; it keeps %s", name, resp.GetProject(), strings.Join(have, ", "))
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
	ts := service.QuerySetTable(resp, func(i int) rpt.Table {
		return rpt.Table{Query: set.Queries[i].Query, Bindings: query.FormatBindings(set.Queries[i].Bind)}
	})
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
// the web panel renders, through service.QueryResultTable.
func tableFromProto(resp *webapi.RunQueryResponse, title, queryText, source string) rpt.Table {
	return service.QueryResultTable(resp, title, queryText, source)
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

// parseBindings reads each --bind name=value into the values the query is asked with; a name given
// more than once binds its variable to each value.
func parseBindings(args []string) (query.Bindings, error) {
	b, err := query.ParseBindings(args)
	if err != nil {
		return nil, fmt.Errorf("--bind: %w", err)
	}
	return b, nil
}

// withCaret adds the line a query that would not parse stopped on, with a caret under the place, as
// the engine drew it, read off the error in this process or off the detail a --remote server
// attached. Any other error is returned as it is.
func withCaret(err error) error {
	at, ok := query.ErrorPosition(err)
	caret := at.Caret
	if !ok {
		var ce *connect.Error
		if !errors.As(err, &ce) {
			return err
		}
		for _, d := range ce.Details() {
			if v, derr := d.Value(); derr == nil {
				if tp, isPos := v.(*webapi.TextPosition); isPos {
					caret = tp.GetCaret()
				}
			}
		}
	}
	if caret == "" {
		return err
	}
	return fmt.Errorf("%w\n  %s", err, strings.ReplaceAll(caret, "\n", "\n  "))
}

// linker mints one link from the run's link metadata, which it resolves once, or "" when no link is
// safe to make.
type linker func(build func(meta rpt.Report) string) string

// queryLinker returns the linker for a query run under --server, or nil when the run asked for no
// links. Whether a link is safe is decided by viewerLinkMeta, the helper check and trace use, from
// this machine's mount table whichever engine answered.
func queryLinker(cmd *cobra.Command, spec serverSpec, design string) linker {
	if !spec.active() {
		return nil
	}
	var meta *rpt.Report
	return func(build func(rpt.Report) string) string {
		if meta == nil {
			localURI, err := cliArgURI(design)
			if err != nil {
				return ""
			}
			m := viewerLinkMeta(cmd, cmd.Context(), &localLoader{loader: newLoader()}, localURI, spec)
			meta = &m
		}
		return build(*meta)
	}
}
