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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/check"
	"github.com/panyam/agni/check/naming"
	_ "github.com/panyam/agni/datalogrules" // registers the "dl" datalog-authored rule source
	"github.com/panyam/agni/diff"
	"github.com/panyam/agni/readers/formats"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/intent"
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/readers/ipc2581"
	"github.com/panyam/agni/datasheet/param"
	"github.com/panyam/agni/profiles" // registers built-in "profile" rules; LoadDir adds overlay profiles
	"github.com/panyam/agni/review"
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
	}
	root.PersistentFlags().StringArrayVar(&symbolPaths, "symbol-path", nil,
		"directory to search for .sym symbol files, needed to netlist xschem/gEDA schematics "+
			"(repeatable; the schematic's own directory is always searched)")
	root.AddCommand(statsCmd(), checkCmd(), diffCmd(), renderCmd(), emitCmd(), validateCmd(), censusCmd(), serveCmd(), deriveCmd(), nativeCmd(), queryCmd(), reviewCmd(), intakeCmd())
	return root
}

// symbolPaths holds the --symbol-path search directories for resolving xschem/gEDA symbols.
var symbolPaths []string

// newLoader builds the formats.Loader every command reads designs through, carrying the
// --symbol-path flag values. Format dispatch itself lives in formats, so a second
// entrypoint (WASM, cloud function) constructs the same loader without importing cobra.
func newLoader() *formats.Loader {
	return &formats.Loader{SymbolPaths: symbolPaths}
}

// readDesign reads a design file into the IR through the formats registry.
func readDesign(path string) (*ir.Design, error) {
	warnEdsSibling(path, os.Stderr)
	return newLoader().ReadDesign(path)
}

// warnEdsSibling warns (to w) when reading an EDIF SCHEMATIC-geometry (.eds) export while the sibling
// NETLIST (.edn) exists. The two read different component counts — the .eds reflects what the schematic
// DRAWS, the .edn is authoritative for component identity — so a count taken off the .eds is a silent
// footgun (it read test_point 565 vs the .edn's 1385 on a real board). The design.yaml entry should be
// the .edn. Silent when there is no .eds, no sibling .edn, or the input already is the .edn.
func warnEdsSibling(path string, w io.Writer) {
	if !strings.EqualFold(filepath.Ext(path), ".eds") {
		return
	}
	stem := strings.TrimSuffix(path, filepath.Ext(path))
	for _, ext := range []string{".edn", ".EDN"} {
		if _, err := os.Stat(stem + ext); err == nil {
			fmt.Fprintf(w, "warning: %s is an EDIF schematic-geometry (.eds) export; component counts may be lower than the netlist. The sibling %s (.edn netlist) is authoritative for component identity — use it (your design.yaml entry).\n",
				filepath.Base(path), filepath.Base(stem+ext))
			return
		}
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
func readModelWithParams(path, paramsDir string) (check.Model, error) {
	l := newLoader()
	d, err := l.ReadDesign(path)
	if err != nil {
		return nil, err
	}
	bg, err := l.BoardGeometry(path)
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
	var format, failOn, paramsDir, conventions, profilePath, intentPath string
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
			// Compose any ad-hoc sources (naming conventions, overlay profiles) onto the catalog.
			// CatalogWith keeps the built-ins AND any RegisterSource'd suites; NewCatalog here would
			// silently drop registered sources. Both flags accumulate so they may be combined. This runs
			// BEFORE the design is read (WS3-071): ApplyLexicon installs the class lexicon the ingestion
			// classify pass reads, so a --conventions class override reaches the stamped device_classes
			// set instead of arriving after the model was already built.
			var extra []check.RuleSource
			if conventions != "" {
				cfg, err := naming.Load(conventions)
				if err != nil {
					return err
				}
				// The lexicon (rail/ground/feedback name overrides) applies process-wide before rules run.
				if err := naming.ApplyLexicon(cfg); err != nil {
					return err
				}
				// Convention RULES are optional: a config may carry only a lexicon.
				if len(cfg.Rules) > 0 {
					src, err := naming.Source(cfg)
					if err != nil {
						return err
					}
					extra = append(extra, src)
				}
			}
			if profilePath != "" {
				ps, err := profiles.LoadDir(profilePath)
				if err != nil {
					return err
				}
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
			}
			// Resolve the --rule/--tag facets to rule NAMES against the composed catalog: CheckDesign /
			// GetCheckReport select by name, and the CLI (not the service) owns facet resolution. The same
			// composed catalog is injected into the service, so the names resolve identically there.
			selected := catalog.Filter(facets)
			if len(selected) == 0 && format == "text" {
				fmt.Fprintln(cmd.OutOrStdout(), "no rules selected")
				return nil
			}
			names := make([]string, len(selected))
			for i, r := range selected {
				names[i] = r.Name
			}
			// Thin client of the in-process CheckService (WS9-048): the same service + BuildModel fact
			// base the web check panel runs, so CLI and web render one shape. --conventions ApplyLexicon
			// already ran above (a process-global the ingestion classify pass reads); the composed catalog
			// and the datasheet corpus are injected into the service.
			var specs param.ParamProvider
			if paramsDir != "" {
				set, err := param.LoadSet(os.DirFS(paramsDir))
				if err != nil {
					return fmt.Errorf("--params %s: %w", paramsDir, err)
				}
				specs = set
			}
			svc := service.NewCheckService(&localLoader{loader: newLoader()}, catalog, specs)
			ctx := cmd.Context()
			var failFindings []*webapi.Finding
			switch format {
			case "markdown", "report":
				rresp, err := svc.GetCheckReport(ctx, &webapi.GetCheckReportRequest{Path: args[0], Rules: names})
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
				resp, err := svc.CheckDesign(ctx, &webapi.CheckDesignRequest{Path: args[0], Rules: names})
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
	var checklist, paramsDir, profilePath, intentPath, boardPath, format string
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
			var specs param.ParamProvider
			if paramsDir != "" {
				set, err := param.LoadSet(os.DirFS(paramsDir))
				if err != nil {
					return fmt.Errorf("--params %s: %w", paramsDir, err)
				}
				specs = set
			}
			svc := service.NewReviewService(&localLoader{loader: newLoader()}, catalog, byName, specs)
			resp, err := svc.RunReview(cmd.Context(), &webapi.RunReviewRequest{
				ManifestPath: checklist, DesignPath: args, BoardPath: boardPath, RatifiedFloor: ratifiedFloor,
			})
			if err != nil {
				return err
			}
			// Map the proto response back to the Go view-model and render with the existing renderers — the
			// CLI analogue of the web tier's reportFromWire, so both surfaces start from one wire shape.
			reports := reportsFromProto(resp)
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
				agg := review.Aggregate{Manifest: resp.GetManifest(), Reports: reports}
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
	cmd.Flags().StringVar(&boardPath, "board-path", "", "a board-geometry file (.kicad_pcb / IPC-2581 .xml|.cvg) attached to the netlist design so board-tier DRC items resolve pass/fail instead of not-applicable")
	cmd.Flags().BoolVar(&coverage, "coverage", false, "emit a per-area coverage rollup (covered/pass/fail/provisional/needs-intent/computed-n-a/n-a/not-automated) instead of the per-item report")
	cmd.Flags().Float64Var(&ratifiedFloor, "ratified-floor", 0, "datasheet-confidence floor for a trustworthy finding; a fail whose findings are all mock or below this is 'provisional'. 0 uses the default (0.9)")
	cmd.Flags().StringVar(&format, "format", "markdown", "per-item report format: markdown (Detail cell capped) or json (full findings, for tooling)")
	return cmd
}

// composeReviewInputs builds the design-INDEPENDENT review inputs shared by the CLI (reviewCmd) and
// the served ReviewService: the rule catalog with any overlay sources spliced in (interface profiles
// from profilePath, a design-intent declaration from intentPath), and the by-Name profile index the
// presence check reads. Both overlay sources are overlay-only, so with neither path an intent/profile
// item reads not-automated rather than silently passing. Empty paths yield the built-in catalog and
// the built-in profile index alone.
func composeReviewInputs(profilePath, intentPath string) (*check.Catalog, map[string][]profiles.Profile, error) {
	var sources []check.RuleSource
	byName := map[string][]profiles.Profile{}
	for _, p := range profiles.Profiles {
		byName[p.Name] = append(byName[p.Name], p)
	}
	if profilePath != "" {
		ps, err := profiles.LoadDir(profilePath)
		if err != nil {
			return nil, nil, err
		}
		sources = append(sources, profiles.Source("profile-overlay", ps))
		for _, p := range ps {
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
