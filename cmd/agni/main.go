// Command agni is the CLI umbrella for the EDA tooling engine: read a design into the
// neutral IR and run analyses over it (stats, checks, diff; rendering and other surfaces
// will join here). It is a runtime-agnostic-core driver (CONSTRAINTS C1): file I/O lives
// here at the edge, and all logic is in the edif/diff/check packages.
//
// Diff and checks operate on the IR, not on source files, so they are format-neutral;
// agni selects a reader by file type (readDesign) and everything downstream is the same
// regardless of source format.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/diff"
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/datasheet/param"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/internal/version"
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/readers/ipc2581"
	"github.com/panyam/agni/stdlib/profiles"        // registers built-in "profile" rules; LoadDir adds overlay profiles
	_ "github.com/panyam/agni/stdlib/relations"     // registers the built-in EDB query relations (netlist/board/datasheet)
	_ "github.com/panyam/agni/stdlib/rules/builtin" // registers the built-in EE rule catalog (anonymous source)
	_ "github.com/panyam/agni/stdlib/rules/datalog" // registers the "dl" datalog-authored rule source
	"github.com/panyam/agni/stdlib/rules/intent"
)

// diffListLimit caps how many items each diff section prints. It lives at the CLI edge,
// not in the diff package, and could become a --limit flag.
const diffListLimit = 40

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// rootCmd assembles the agni command tree.
func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "agni",
		Short: "EDA tooling engine: ingest designs into a neutral IR and analyze them",
		Long: "agni reads hardware designs into a neutral IR and runs analyses over it.\n" +
			"Diff and checks operate on the IR, not on source files, so they are format-neutral.",
		SilenceUsage:  true,
		SilenceErrors: true,
		// Gives `agni --version`. The same string internal/version stamps into a results
		// document's provenance, so the build a user reports and the build a report claims
		// cannot disagree. `agni version` adds the toolchain and platform detail.
		Version: version.Version(),
	}
	root.PersistentFlags().StringArrayVar(&cliMountSpecs, "mount", nil,
		"expose a folder as name=path (repeatable). Every command takes it, not just serve: with it, a "+
			"path inside that folder is addressed through the mount, so the CLI and a server naming the "+
			"same --mount produce identical artifact URIs for the same design and their stored reviews "+
			"are directly comparable. Without it the CLI mints a mount per argument, rooted at the "+
			"enclosing project when there is one.")
	root.PersistentFlags().BoolVar(&readAsNamed, "as-named", false,
		"read exactly the file named, even when its design.yaml declares it a companion view of a "+
			"different entry. Without it, analysis of a declared companion (a schematic export, a board) "+
			"reads the design's entry instead, because a companion is a view of the design rather than a "+
			"second source of it. Use this to read a companion as a netlist on purpose, e.g. to check that "+
			"two views of one design still agree.")
	root.PersistentFlags().StringArrayVar(&symbolPaths, "symbol-path", nil,
		"directory to search for .sym symbol files, needed to netlist xschem/gEDA schematics "+
			"(repeatable; the schematic's own directory is always searched). Defaults to "+
			envSymbolPath+" when unset.")
	root.AddCommand(statsCmd(), checkCmd(), diffCmd(), renderCmd(), emitCmd(), validateCmd(), censusCmd(), serveCmd(), deriveCmd(), nativeCmd(), queryCmd(), reviewCmd(), intakeCmd(), resultsCmd(), importResultsCmd(), healthcheckCmd(), versionCmd())
	return root
}

// symbolPaths holds the --symbol-path search directories for resolving xschem/gEDA symbols.
var symbolPaths []string

// envSymbolPath names the environment variable that supplies --symbol-path when the flag is
// absent. It exists for the container image, where the symbol libraries ship at a fixed location
// and EVERY subcommand needs them, not just the one the image's default CMD happens to run.
//
// Without it, `docker run <image> check board.kicad_sch` would silently lose the symbol paths,
// because overriding CMD replaces the whole argument list. That failure is the quiet kind: a
// schematic naming external symbols reads short, the rules evaluate cleanly over the short read,
// and the run reports fewer findings with no error to explain them.
//
// Colon-separated, like PATH, because that is what an operator expects of a path list in the
// environment. The flag wins outright when present rather than appending, so an explicit
// --symbol-path is never silently widened by ambient configuration.
const envSymbolPath = "AGNI_SYMBOL_PATH"

// resolveSymbolPaths applies the envSymbolPath fallback. Called once before any command runs.
func resolveSymbolPaths(getenv func(string) string) []string {
	if len(symbolPaths) > 0 {
		return symbolPaths
	}
	var out []string
	for p := range strings.SplitSeq(getenv(envSymbolPath), ":") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// newLoader builds the formats.Loader every command reads designs through, carrying the
// --symbol-path flag values (or the envSymbolPath fallback). Format dispatch itself lives in
// formats, so a second entrypoint (WASM, cloud function) constructs the same loader without
// importing cobra.
func newLoader() *formats.Loader {
	return &formats.Loader{SymbolPaths: resolveSymbolPaths(os.Getenv)}
}

// readDesign reads a design file into the IR through the formats registry, after the enclosing
// design's descriptor has had its say about which file that should be (resolveSource).
func readDesign(path string) (*ir.Design, error) {
	ws, err := workspace()
	if err != nil {
		return nil, err
	}
	src, err := newDesignResolver(ws).Resolve(context.Background(), path)
	if err != nil {
		return nil, err
	}
	noteSource(os.Stderr, src)
	return newLoader().ReadDesign(localOf(src.NetlistURI))
}

// noteSource writes a resolution note to w, if there is one. Notes go to stderr so a redirect never
// contaminates a `--format json` document on stdout.
func noteSource(w io.Writer, src designSource) {
	if src.Note != "" {
		fmt.Fprint(w, src.Note)
	}
}

// readModel builds the check Model for a path: the netlist IR plus, when the format
// carries one, the board-geometry sidecar — so geometric rules (WS3-008) see the board
// while netlist-only formats build the plain model. A sidecar that exists but fails to
// parse fails the run: silently checking without the board would report a clean board
// as passing rules it never ran.
func readModel(path string) (check.Model, error) {
	m, err := readModelWithParams(path, "")
	return m, err
}

// readModelWithParams additionally attaches the params tier (WS10-003) from a seeded
// directory of PartSpec textprotos when paramsDir is non-empty. A bad seed corpus
// fails the run (param.LoadSet is all-or-nothing) for the same reason a bad board
// sidecar does: checking against a silently smaller corpus would report false passes.
// The os.DirFS conversion happens here at the CLI edge; everything below takes fs.FS
// (CONSTRAINTS C1/C13). Net-subject sheet annotation reads the netlist through the returned
// Model (its Nets()), so this returns only the Model, never the raw design.
// The board tier reads the design's OWN sidecar here (a `.kicad_pcb`'s board, nil for a netlist);
// the separate-board-export override (WS3-089's --board-path) now lives in the review path, which
// goes through the ReviewService's BuildModel. check/query keep this local builder until they too
// become thin clients (WS9-048).
//
// The two tiers are resolved through the design descriptor (resolveSource), which is what lets them
// come from DIFFERENT files: a design that declares a netlist entry and a board companion checks its
// connectivity against the netlist and its copper against the board, which is C21's split expressed
// as behaviour instead of as a warning.
func readModelWithParams(path, paramsDir string) (check.Model, error) {
	ws, err := workspace()
	if err != nil {
		return nil, err
	}
	src, err := newDesignResolver(ws).Resolve(context.Background(), path)
	if err != nil {
		return nil, err
	}
	noteSource(os.Stderr, src)
	l := newLoader()
	d, err := l.ReadDesign(localOf(src.NetlistURI))
	if err != nil {
		return nil, err
	}
	bg, err := l.BoardGeometry(localOf(src.BoardURI))
	if err != nil {
		return nil, err
	}
	if paramsDir == "" {
		return check.NewModelWithBoard(d, bg), nil
	}
	specs, err := param.LoadSet(os.DirFS(paramsDir))
	if err != nil {
		return nil, fmt.Errorf("--params %s: %w", paramsDir, err)
	}
	return check.NewModelWithParams(d, bg, specs), nil
}

func statsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats <file>",
		Short: "Print component/section/net counts for one design",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			d, err := readDesign(args[0])
			if err != nil {
				return err
			}
			// Components are grouped by ref_des in the IR; each may hold several sections
			// (one per source instance). Report components, total sections, and multi-section count.
			sectionsTotal, multi := 0, 0
			for _, c := range d.Components {
				sectionsTotal += len(c.Sections)
				if len(c.Sections) > 1 {
					multi++
				}
			}
			fmt.Printf("design:              %s\n", d.Name)
			if d.SourceFormat != "" {
				fmt.Printf("source format:       %s\n", d.SourceFormat)
			}
			fmt.Printf("libraries:           %d\n", len(d.Libraries))
			fmt.Printf("components:          %d (unique ref_des)\n", len(d.Components))
			fmt.Printf("sections:            %d (source instances)\n", sectionsTotal)
			fmt.Printf("multi-section:       %d (one ref_des, several sections)\n", multi)
			fmt.Printf("nets:                %d\n", len(d.Nets))
			// Physical tier — shown only when a reader populated it (e.g. IPC-2581, KiCad PCB).
			if len(d.Footprints) > 0 {
				fmt.Printf("footprints:          %d\n", len(d.Footprints))
			}
			if len(d.Layers) > 0 {
				fmt.Printf("layers:              %d\n", len(d.Layers))
			}
			if d.Stackup != nil {
				fmt.Printf("stackup layers:      %d\n", len(d.Stackup.Layers))
			}
			if len(d.Bom) > 0 {
				fmt.Printf("bom lines:           %d\n", len(d.Bom))
			}
			return nil
		},
	}
}

func checkCmd() *cobra.Command {
	var ruleNames, tagPairs []string
	var format, failOn, paramsDir, conventions, profilePath, intentPath, resultsOut string
	cmd := &cobra.Command{
		Use:   "check <file>",
		Short: "Run structural rule checks over one design",
		Long: "Run structural rule checks over one design. With no flags, every rule runs. --rule " +
			"narrows by rule name and --tag key=value narrows by any catalog tag (category, tier, " +
			"distribution, or a provider's own), so e.g. --tag category=connectivity runs one group. " +
			"--format json emits the full findings array (one object per finding, subjects and all) " +
			"for tooling; markdown renders the severity-organized report (worst first, grouped by " +
			"rule); report emits that report as JSON (the GetCheckReport wire shape); the default " +
			"text form is a per-rule summary. --fail-on error|warning|info exits non-zero when any " +
			"finding sits at or above the threshold, so check gates CI.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch format {
			case "text", "json", "markdown", "report":
			default:
				return fmt.Errorf("unknown --format %q (want: text, json, markdown, report)", format)
			}
			switch failOn {
			case "", "error", "warning", "info":
			default:
				return fmt.Errorf("unknown --fail-on %q (want: error, warning, info)", failOn)
			}
			facets := check.Facets{Names: ruleNames, Tags: map[string][]string{}}
			for _, p := range tagPairs {
				k, v, ok := strings.Cut(p, "=")
				if !ok {
					return fmt.Errorf("--tag must be key=value, got %q", p)
				}
				facets.Tags[k] = append(facets.Tags[k], v)
			}
			catalog := check.DefaultCatalog()
			// Compose any ad-hoc sources (overlay profiles, design intent) onto the catalog.
			// CatalogWith keeps the built-ins AND any RegisterSource'd suites; NewCatalog here would
			// silently drop registered sources. The flags accumulate so they may be combined.
			overlay := &webapi.OverlayConfig{}
			var extra []check.RuleSource
			// convSource is the convention's rules, kept OUT of the catalog handed to the service and
			// composed only into the local catalog facets resolve against (WS3-107). The service composes
			// the convention itself from the request, so putting it in both would ask it to add a source
			// the base already carries — a duplicate-source error now that composing EXTENDS the base
			// rather than rebuilding it. The two jobs are genuinely different: the service needs the
			// authoritative catalog, and the CLI needs a name space to resolve `--rule <config>/<rule>`
			// against before it calls.
			var convSource check.RuleSource
			// --conventions: the CLI reads the file (files are the CLI's world, not the service's) and
			// sends the convention as a VALUE on the request, which the service composes (WS3-102).
			if conventions != "" {
				cfg, err := naming.Load(conventions)
				if err != nil {
					return err
				}
				overlay.Conventions = service.ConventionProto(cfg)
				if len(cfg.Rules) > 0 {
					src, err := naming.Source(cfg)
					if err != nil {
						return err
					}
					convSource = src
				}
			}
			if profilePath != "" {
				ps, err := profiles.LoadDir(profilePath)
				if err != nil {
					return err
				}
				warnOverBroadProfiles(cmd.ErrOrStderr(), args[0], ps)
				extra = append(extra, profiles.Source("profile-overlay", ps))
			}
			if intentPath != "" {
				decl, err := intent.LoadFile(intentPath)
				if err != nil {
					return err
				}
				extra = append(extra, intent.Source("intent", decl))
			}
			if len(extra) > 0 {
				catalog = check.CatalogWith(extra...)
				noteSupersededRules(cmd.ErrOrStderr(), catalog)
			}
			// Resolve the --rule/--tag facets to rule NAMES against the catalog the RUN will use, which is
			// the service's catalog plus the convention the request carries. CheckDesign / GetCheckReport
			// select by name and the CLI owns facet resolution, so the two must see the same name space —
			// but only this local copy carries the convention, or the service would compose it twice.
			resolveAgainst := catalog
			if convSource != nil {
				var err error
				if resolveAgainst, err = catalog.With(convSource); err != nil {
					return err
				}
			}
			selected := resolveAgainst.Filter(facets)
			if len(selected) == 0 && format == "text" {
				fmt.Fprintln(cmd.OutOrStdout(), "no rules selected")
				return nil
			}
			names := make([]string, len(selected))
			for i, r := range selected {
				names[i] = r.Name
			}
			// Thin client of the in-process CheckService (WS9-048): the same service + BuildModel fact
			// base the web check panel runs, so CLI and web render one shape. The composed catalog and the
			// datasheet corpus are injected into the service; --conventions rides the request instead, so
			// its lexicon reaches the design read without touching process state.
			var specs param.ParamProvider
			if paramsDir != "" {
				set, err := param.LoadSet(os.DirFS(paramsDir))
				if err != nil {
					return fmt.Errorf("--params %s: %w", paramsDir, err)
				}
				specs = set
			}
			svc := service.NewCheckService(&localLoader{loader: newLoader()}, catalog, specs, "", nil)
			ctx := cmd.Context()
			var failFindings []*checkspb.Finding
			// --results-out takes one path through the service for every format (WS3-103): the document
			// holds findings in run order, and the severity pivot is rebuilt from it. Rendering from the
			// document rather than beside it is what makes the written artifact the SAME artifact the
			// terminal showed, instead of a second one that happens to agree today.
			if resultsOut != "" {
				resp, err := svc.CheckDesign(ctx, &webapi.CheckDesignRequest{Uri: cliArgURI(args[0]), Rules: names, Overlay: overlay})
				if err != nil {
					return err
				}
				doc := resultsDoc(args[0], selected, resp.GetFindings(), &checkspb.RunConfig{
					Params:      paramsDir != "",
					Profiles:    profilePath != "",
					Intent:      intentPath != "",
					Conventions: overlay.GetConventions().GetName(),
				})
				if err := writeResults(resultsOut, doc); err != nil {
					return err
				}
				if err := renderCheckResults(cmd.OutOrStdout(), doc, format); err != nil {
					return err
				}
				if failOn != "" && failsAtProto(resp.GetFindings(), failOn) {
					cmd.SilenceUsage = true
					return fmt.Errorf("findings at or above --fail-on %s", failOn)
				}
				return nil
			}
			switch format {
			case "markdown", "report":
				rresp, err := svc.GetCheckReport(ctx, &webapi.GetCheckReportRequest{Uri: cliArgURI(args[0]), Rules: names, Overlay: overlay})
				if err != nil {
					return err
				}
				if format == "markdown" {
					err = writeCheckMarkdown(cmd.OutOrStdout(), rresp.GetReport())
				} else {
					err = writeCheckReportJSON(cmd.OutOrStdout(), rresp.GetReport())
				}
				if err != nil {
					return err
				}
				failFindings = reportFindings(rresp.GetReport())
			default: // text, json — both need the raw findings
				resp, err := svc.CheckDesign(ctx, &webapi.CheckDesignRequest{Uri: cliArgURI(args[0]), Rules: names, Overlay: overlay})
				if err != nil {
					return err
				}
				if format == "json" {
					if err := writeCheckDesignJSON(cmd.OutOrStdout(), resp); err != nil {
						return err
					}
				} else {
					writeCheckText(cmd.OutOrStdout(), findingsFromProto(resp.GetFindings()), len(selected))
				}
				failFindings = resp.GetFindings()
			}
			if failOn != "" && failsAtProto(failFindings, failOn) {
				cmd.SilenceUsage = true
				return fmt.Errorf("findings at or above --fail-on %s", failOn)
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&ruleNames, "rule", nil, "run only these rules by name (repeatable)")
	cmd.Flags().StringArrayVar(&tagPairs, "tag", nil, "run only rules matching key=value tags (repeatable; e.g. --tag category=connectivity)")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text | json | markdown | report")
	cmd.Flags().StringVar(&failOn, "fail-on", "", "exit non-zero when findings at or above this severity exist: error | warning | info")
	cmd.Flags().StringVar(&paramsDir, "params", "", "directory of seeded PartSpec textprotos (the datasheet parameter corpus, WS10); enables datasheet-backed rules")
	cmd.Flags().StringVar(&profilePath, "profile-path", "", "directory of YAML interface-profile declarations; their rules join the catalog alongside the built-in profiles")
	cmd.Flags().StringVar(&conventions, "conventions", "", "compose an operator naming-convention config (YAML) into the catalog; its rules appear namespaced as <config name>/<rule name>")
	cmd.Flags().StringVar(&intentPath, "intent-path", "", "a YAML design-intent declaration (expected modules, voltage domains); its rules join the catalog")
	cmd.Flags().StringVar(&resultsOut, "results-out", "", "also write the run as a self-contained check-result document (JSON) at this path; render it later with `agni results`")
	return cmd
}

// writeCheckText prints the default per-rule summary plus the first findings, the original
// human terminal form (reports and tooling use --format markdown/report/json).
func writeCheckText(w io.Writer, fs []check.Finding, rulesRun int) {
	if len(fs) == 0 {
		fmt.Fprintf(w, "no findings (%d rule(s) run)\n", rulesRun)
		return
	}
	byRule := map[string]int{}
	for _, f := range fs {
		byRule[f.Rule]++
	}
	fmt.Fprintln(w, "findings by rule:")
	for _, rule := range sortedKeys(byRule) {
		fmt.Fprintf(w, "  %-22s %d\n", rule, byRule[rule])
	}
	limit := min(len(fs), 50)
	fmt.Fprintf(w, "\nfirst %d:\n", limit)
	for _, f := range fs[:limit] {
		fmt.Fprintf(w, "  [%s] %s: %s (%s)\n", f.Severity, f.Rule, f.Subject, f.Message)
	}
	fmt.Fprintf(w, "\n%d finding(s) total\n", len(fs))
}

// writeCheckDesignJSON emits the CheckDesign response in protojson form, the same wire shape the RPC
// returns (findings sorted by rule then subject, each carrying its subject, provenance, sheet badges,
// and datasheet citation). The response is already sheet-annotated server-side (WS9-048), so the CLI
// marshals it verbatim — the conformance runner parses the same `.findings[]` whether it shells out or
// calls the API.
func writeCheckDesignJSON(w io.Writer, resp *webapi.CheckDesignResponse) error {
	b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

func reviewCmd() *cobra.Command {
	var checklist, paramsDir, profilePath, intentPath, boardPath, format, renderDir, companion, conventions, resultsOut string
	var coverage bool
	var ratifiedFloor float64
	cmd := &cobra.Command{
		Use:   "review <file>...",
		Short: "Run a review checklist (manifest) over one or more designs and report per-item outcomes",
		Long: "Run a project's review checklist against one design, or a project rollup across several. " +
			"--checklist points at a review manifest (YAML: review areas, each with items bound to a rule, " +
			"tag, profile, or inline datalog query). Each item resolves to pass, fail, not-applicable (the " +
			"rule's fact tier is absent), or not-automated (no shipped rule covers it). --profile-path adds " +
			"overlay interface profiles; --params enables datasheet-backed rules; --board-path attaches a " +
			"board-geometry export so board-tier DRC items resolve.\n\n" +
			"With ONE design, the report is per-item, organized by the manifest's review areas. With SEVERAL " +
			"(shell-globbed), it is a project rollup: a per-design outcome summary plus a per-item x design " +
			"traceability matrix. Automation is manifest-level (stated once); pass/fail/n-a is per design.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if checklist == "" {
				return fmt.Errorf("review needs --checklist <manifest.yaml>")
			}
			// The CLI is a thin client of the in-process ReviewService (WS9-048): it composes the
			// design-independent overlay config (catalog + profile index from --profile-path/--intent-path,
			// specs from --params) and constructs the service over a local no-containment loader, then
			// renders the proto response. Same service the web calls, so the two surfaces cannot drift.
			catalog, byName, err := composeReviewInputs(profilePath, intentPath)
			if err != nil {
				return err
			}
			noteSupersededRules(cmd.ErrOrStderr(), catalog)
			// Reading the convention file is the CLI's job; the service takes the value.
			overlay := &webapi.OverlayConfig{}
			if conventions != "" {
				cfg, err := naming.Load(conventions)
				if err != nil {
					return err
				}
				overlay.Conventions = service.ConventionProto(cfg)
			}
			// The checklist is read here for the same reason (WS9-050): it is configuration, so it
			// travels as a value and where it came from stays the caller's business. The CLI's answer
			// is "a YAML file the user named"; a browser resolves one through GetReviewManifest
			// instead, and neither answer is in the service's contract.
			man, err := loadManifest(checklist)
			if err != nil {
				return err
			}
			var specs param.ParamProvider
			if paramsDir != "" {
				set, err := param.LoadSet(os.DirFS(paramsDir))
				if err != nil {
					return fmt.Errorf("--params %s: %w", paramsDir, err)
				}
				specs = set
			}
			// The CLI stays a thin client of the service (WS9-048), which now creates review RESOURCES
			// (WS9-053). It runs over an IN-MEMORY store on purpose: `agni review` prints a report and
			// must not leave files behind as a side effect of being run, and --results-out is the
			// explicit, user-asked-for way to write one. Same engine path as a served create, so the two
			// surfaces still cannot disagree about an outcome.
			env := service.ReviewEnv{ProducerVersion: version.Version(), Profiles: profilePath != "", Intent: intentPath != ""}
			svc := service.NewReviewService(&localLoader{loader: newLoader()}, service.NewMemReviewStore(), catalog, byName, specs, env, "")
			// One create per design: a stored run is about ONE design, so the CLI's multi-design rollup is
			// several runs rather than one call. The loop is the rollup.
			var docs []*checkspb.CheckResults
			for _, design := range args {
				rv, err := svc.CreateReview(cmd.Context(), &webapi.CreateReviewRequest{
					Manifest: service.ManifestProto(man), DesignUri: cliArgURI(design), BoardUri: cliArgURI(boardPath), RatifiedFloor: ratifiedFloor,
					// --conventions rides the REQUEST as a value (WS3-102): the service composes it, so the CLI
					// and the web reach one composition path, and its lexicon half travels with the design
					// read instead of being installed in a process global.
					Overlay: overlay,
				})
				if err != nil {
					return err
				}
				docs = append(docs, rv.GetResults())
			}
			// Map the stored documents back to the Go view-model and render with the existing renderers —
			// the CLI analogue of the web tier's reportFromWire, so both surfaces start from one shape.
			reports := reportsFromDocs(docs)
			// --render bakes each design's findings into annotated schematic SVGs (WS7-043): the report's
			// finding->picture side. The summary goes to stderr so stdout stays a clean report to redirect.
			if renderDir != "" {
				summary, err := renderReviewImages(reports, renderDir, companion)
				if err != nil {
					return err
				}
				fmt.Fprint(cmd.ErrOrStderr(), summary)
			}
			// --results-out writes the per-design review document (WS3-103) and renders FROM it, the same
			// write-then-render-the-document path `check --results-out` takes. A rollup has no single
			// design to be about, and a document that averaged several would misrepresent every one of
			// them, so this is a one-design surface and says so rather than silently picking the first.
			if resultsOut != "" {
				if len(reports) != 1 {
					return fmt.Errorf("--results-out writes one design's document; got %d designs (run it per design)", len(reports))
				}
				// The service already built the whole document, snapshot and catalog included, so this
				// writes what the run produced rather than reassembling a second one beside it. The
				// previous code built a parallel document here and copied two fields across, which is
				// exactly how a --results-out file drifts from what the run actually recorded.
				doc := docs[0]
				if err := writeResults(resultsOut, doc); err != nil {
					return err
				}
				return renderReviewResults(cmd.OutOrStdout(), doc, format, coverage)
			}
			// --coverage is the rollup shortcut; --format {markdown,json} picks the surface. JSON carries the
			// FULL finding list per item (markdown caps the Detail cell), so tooling keeps every finding.
			var out string
			if len(reports) == 1 {
				rep := reports[0]
				switch {
				case coverage:
					out = review.RenderCoverageMarkdown(rep)
				case format == "json":
					out, err = review.RenderJSON(rep)
				case format == "" || format == "markdown":
					out = review.RenderMarkdown(rep)
				default:
					return fmt.Errorf("review: unknown --format %q (want markdown or json)", format)
				}
			} else {
				agg := review.Aggregate{Manifest: man.Name, Reports: reports}
				switch {
				case coverage:
					out = review.RenderAggregateCoverageMarkdown(agg)
				case format == "json":
					out, err = review.RenderAggregateJSON(agg)
				case format == "" || format == "markdown":
					out = review.RenderAggregateMarkdown(agg)
				default:
					return fmt.Errorf("review: unknown --format %q (want markdown or json)", format)
				}
			}
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), out)
			return err
		},
	}
	cmd.Flags().StringVar(&checklist, "checklist", "", "review manifest (YAML) declaring review areas and their checklist items")
	cmd.Flags().StringVar(&paramsDir, "params", "", "directory of seeded PartSpec textprotos; enables datasheet-backed rules")
	cmd.Flags().StringVar(&profilePath, "profile-path", "", "directory of YAML interface-profile declarations added to the catalog")
	cmd.Flags().StringVar(&intentPath, "intent-path", "", "a YAML design-intent declaration (expected modules, voltage domains); its rules join the catalog so intent-bound items resolve")
	cmd.Flags().StringVar(&conventions, "conventions", "", "an operator naming-convention config (YAML); its rules join the catalog namespaced as <config name>/<rule name>, and its lexicon teaches the run which net names are this project's power rails, grounds, and feedback nodes")
	cmd.Flags().StringVar(&boardPath, "board-path", "", "a board-geometry file (.kicad_pcb / IPC-2581 .xml|.cvg) attached to the netlist design so board-tier DRC items resolve pass/fail instead of not-applicable")
	cmd.Flags().BoolVar(&coverage, "coverage", false, "emit a per-area coverage rollup (covered/pass/fail/provisional/needs-intent/needs-data/computed-n-a/n-a/not-automated) instead of the per-item report")
	cmd.Flags().Float64Var(&ratifiedFloor, "ratified-floor", 0, "datasheet-confidence floor for a trustworthy finding; a fail whose findings are all mock or below this is 'provisional'. 0 uses the default (0.9)")
	cmd.Flags().StringVar(&format, "format", "markdown", "per-item report format: markdown (Detail cell capped) or json (full findings, for tooling)")
	cmd.Flags().StringVar(&renderDir, "render", "", "also write an annotated schematic SVG per design (each finding highlighted in place) to <dir>/<design-stem>/<sheet>.svg")
	cmd.Flags().StringVar(&companion, "companion", "", "geometry file (.eds) to draw the --render images on, joined to the netlist findings by net name; with one design only (else a sibling <stem>.eds is auto-detected per design)")
	cmd.Flags().StringVar(&resultsOut, "results-out", "", "also write the run as a self-contained check-result document (JSON) at this path; one design only. Render it later with `agni results`")
	return cmd
}

// composeReviewInputs builds the design-INDEPENDENT review inputs shared by the CLI (reviewCmd) and
// the served ReviewService: the rule catalog with any overlay sources spliced in (interface profiles
// from profilePath, a design-intent declaration from intentPath), and the by-Name profile index the
// presence check reads. Both overlay sources are overlay-only, so with neither path an intent/profile
// item reads not-automated rather than silently passing. Empty paths yield the built-in catalog and
// the built-in profile index alone.
func composeReviewInputs(profilePath, intentPath string) (*check.Catalog, map[string][]profiles.Profile, error) {
	overlay, err := loadOverlayProfiles(profilePath)
	if err != nil {
		return nil, nil, err
	}
	return composeReviewInputsFrom(overlay, intentPath)
}

// loadOverlayProfiles reads the overlay interface profiles at path, or nil for an empty path. Split
// out so a surface composing SEVERAL catalogs from one --profile-path (serve, which feeds both the
// CheckService and the ReviewService) reads the directory once and fails startup once, rather than
// loading it per service and reporting the same bad profile twice.
func loadOverlayProfiles(profilePath string) ([]profiles.Profile, error) {
	if profilePath == "" {
		return nil, nil
	}
	return profiles.LoadDir(profilePath)
}

// composeReviewInputsFrom is composeReviewInputs over profiles that are already loaded, plus any
// extra sources the caller composed itself.
//
// extra exists for serve, whose --conventions is a startup DEPLOYMENT default rather than a
// per-request value, so its rules have to join this composition instead of riding a request (WS3-109).
// It is a source rather than a path because reading the config is the caller's business (C22), and it
// goes through this one composer rather than a second CatalogWith so a caller cannot compose a catalog
// that silently omits the profile and intent sources. That is not hypothetical: serve used to REBUILD
// its review catalog for the conventions case, which dropped both tiers whenever an operator passed
// --conventions together with --profile-path or --intent-path.
func composeReviewInputsFrom(overlay []profiles.Profile, intentPath string, extra ...check.RuleSource) (*check.Catalog, map[string][]profiles.Profile, error) {
	var sources []check.RuleSource
	byName := map[string][]profiles.Profile{}
	for _, p := range profiles.Profiles {
		byName[p.Name] = append(byName[p.Name], p)
	}
	if len(overlay) > 0 {
		sources = append(sources, profiles.Source("profile-overlay", overlay))
		// An overlay profile REPLACES the same-named built-in here, tracking the catalog, whose overlay
		// source supersedes that built-in's rules (WS3-056). This map is the review's absence gate:
		// reviewClosures reports an interface as evaluating if ANY profile under its name is in use, and
		// unions every one of their nets for scoping. Keeping the built-in here while the catalog drops it
		// would let the gate clear on a profile whose rules are no longer in the run, and an item scoped by
		// it would score a clean pass on an interface nothing checked. That is the WS3-090 twin
		// disagreement, which is silent by construction.
		//
		// Cleared in a separate pass before any overlay profile is added. Clearing and appending in one
		// pass would make a later profile wipe an earlier one of the same name. That specific input is
		// rejected upstream (identical rule names fail catalog composition), so the two-pass form is not
		// load-bearing today, but it costs nothing and the one-pass form is wrong for a reason unrelated
		// to why it currently cannot happen.
		for _, p := range overlay {
			delete(byName, p.Name)
		}
		for _, p := range overlay {
			byName[p.Name] = append(byName[p.Name], p)
		}
	}
	if intentPath != "" {
		decl, err := intent.LoadFile(intentPath)
		if err != nil {
			return nil, nil, err
		}
		sources = append(sources, intent.Source("intent", decl))
	}
	sources = append(sources, extra...)
	catalog := check.DefaultCatalog()
	if len(sources) > 0 {
		catalog = check.CatalogWith(sources...)
	}
	return catalog, byName, nil
}

func diffCmd() *cobra.Command {
	var format string
	c := &cobra.Command{
		Use:   "diff <old> <new>",
		Short: "Structural diff between two revisions of a design (over the IR)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := readDesign(args[0])
			if err != nil {
				return err
			}
			b, err := readDesign(args[1])
			if err != nil {
				return err
			}
			rep := diff.Designs(a, b)
			w := cmd.OutOrStdout()
			if format == "json" {
				return writeDiffJSON(w, rep)
			}
			fmt.Fprintf(w, "diff %s -> %s\n\n", args[0], args[1])
			fmt.Fprint(w, rep.Render(diffListLimit))
			return nil
		},
	}
	c.Flags().StringVar(&format, "format", "text",
		"output format: text (human summary) or json (the DiffDesignsResponse wire shape the web API serves)")
	return c
}

func emitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "emit <in> [out]",
		Short: "Emit an IPC-2581 file from a design (any input format; stdout if out omitted)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			d, err := readDesign(args[0])
			if err != nil {
				return err
			}
			w := io.Writer(os.Stdout)
			if len(args) == 2 {
				out, err := os.Create(args[1])
				if err != nil {
					return err
				}
				defer out.Close()
				w = out
			}
			return ipc2581.Write(w, d)
		},
	}
}

// sortedKeys returns the map keys in sorted order for stable output.
func sortedKeys(m map[string]int) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
