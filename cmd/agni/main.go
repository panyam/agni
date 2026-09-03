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
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/diff"
	rpt "github.com/panyam/agni/core/report"
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/datasheet/param"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/internal/version"
	"github.com/panyam/agni/readers/edif"
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/readers/ipc2581"
	"github.com/panyam/agni/stdlib/profiles"        // registers built-in "profile" rules; LoadDir adds overlay profiles
	_ "github.com/panyam/agni/stdlib/relations"     // registers the built-in EDB query relations (netlist/board/datasheet)
	_ "github.com/panyam/agni/stdlib/reviewquery"   // compiles a review manifest's inline query bindings as datalog
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
		os.Exit(exitCode(err))
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
		// Tier-1 config is applied before ANY command runs, so a mount declared in a file is in the
		// table by the time the first argument is turned into a URI.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return applyEnvConfig(cmd.ErrOrStderr(), os.Getenv)
		},
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
	root.AddCommand(statsCmd(), checkCmd(), diffCmd(), renderCmd(), emitCmd(), validateCmd(), censusCmd(), serveCmd(), openCmd(), deriveCmd(), nativeCmd(), queryCmd(), reviewCmd(), startCmd(), intakeCmd(), resultsCmd(), importResultsCmd(), healthcheckCmd(), versionCmd())
	return root
}

// symbolPaths holds the --symbol-path search directories for resolving xschem/gEDA symbols.
var symbolPaths []string

// envConfigWebDir holds the web_dir an agni.yaml named, "" when no file named one. applyEnvConfig
// fills it; resolveWebDir consults it. It is a package var for the same reason cliMountSpecs is: the
// file is read once in PersistentPreRunE, before any command knows it needed the value.
var envConfigWebDir string

// envConfigNativeTools holds the native_tools an agni.yaml named. Same shape as envConfigWebDir and
// for the same reason: only serve consumes it, and by then the file has long been read.
var envConfigNativeTools []string

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

// envWebDir names the environment variable that supplies --web-dir when neither the flag nor an
// agni.yaml names one.
//
// It exists for the case the relative default cannot serve: an INSTALLED binary, run from wherever
// the user's design happens to live, whose assets sit at a fixed absolute path. From a checkout the
// default "web" already resolves per-directory, so two checkouts serve their own assets with no
// configuration; from /usr/local/bin there is no such relative answer.
//
// The flag wins outright, then agni.yaml, then this. Ambient configuration is last because it is the
// tier whose value nobody typed.
const envWebDir = "AGNI_WEB_DIR"

// defaultWebDir is where the viewer's assets live relative to a repo checkout. It is the value every
// in-tree caller used to pass positionally, so keeping it as the default is what makes dropping that
// argument a no-op for `make serve`, `make demo`, and the container's CMD.
const defaultWebDir = "web"

// applyEnvConfig fills the tier-1 flags from the nearest agni.yaml, for the ones the operator did not
// pass. It runs once, before any command.
//
// A passed flag WINS OUTRIGHT rather than merging, matching how --symbol-path already treats its
// environment variable: an operator who named a mount table is answering for the whole table, and a
// file quietly adding a mount they did not ask for is the ambient-config failure this tier is only
// allowed because it cannot change an answer.
//
// The file that was used is announced on stderr, for the same reason every other resolution here is:
// a mount table nobody typed is not recoverable from the output of a run that used it.
func applyEnvConfig(w io.Writer, getenv func(string) string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg, path, err := loadEnvConfig(cwd, getenv)
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	var used []string
	if len(cliMountSpecs) == 0 && len(cfg.Mounts) > 0 {
		cliMountSpecs = cfg.mountSpecs()
		used = append(used, fmt.Sprintf("%d mount(s)", len(cfg.Mounts)))
	}
	if len(symbolPaths) == 0 && len(cfg.SymbolPaths) > 0 {
		symbolPaths = cfg.SymbolPaths
		used = append(used, fmt.Sprintf("%d symbol path(s)", len(cfg.SymbolPaths)))
	}
	if cfg.WebDir != "" {
		envConfigWebDir = cfg.WebDir
		used = append(used, "a web dir")
	}
	if len(cfg.NativeTools) > 0 {
		envConfigNativeTools = cfg.NativeTools
		used = append(used, fmt.Sprintf("%d native tool(s)", len(cfg.NativeTools)))
	}
	if len(used) > 0 {
		fmt.Fprintf(w, "note: using %s from %s.\n", strings.Join(used, " and "), path)
	}
	return nil
}

// resolveWebDir answers where the viewer's assets are, and says where the answer came from.
//
// The source is returned rather than logged here so the caller can announce only the cases nobody
// typed. A flag is the operator's own words and needs no narration; a value from an agni.yaml or the
// environment is exactly the kind of resolution that, left silent, turns "the viewer is serving stale
// assets from another checkout" into an unfalsifiable afternoon.
func resolveWebDir(flag string, getenv func(string) string) (dir, source string) {
	switch {
	case flag != "":
		return flag, ""
	case envConfigWebDir != "":
		return envConfigWebDir, "agni.yaml"
	default:
		if v := strings.TrimSpace(getenv(envWebDir)); v != "" {
			return v, envWebDir
		}
		return defaultWebDir, ""
	}
}

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
	l := &formats.Loader{SymbolPaths: resolveSymbolPaths(os.Getenv)}
	// The CLI opens a design by absolute host path, so without this every locator in the read would
	// record this machine's directory layout, and `--format json` and `--results-out` would publish
	// it. The lookup is deferred into the closure because a command mints its argument's mount while
	// resolving the design, which happens after the loader is built.
	l.SourceName = func(path string) string {
		ws, err := workspace()
		if err != nil {
			return filepath.Base(path)
		}
		return ws.relName(path)
	}
	return l
}

// readDesign reads a design file into the IR through the formats registry, after the enclosing
// design's descriptor has had its say about which file that should be (resolveSource).
func readDesign(path string) (*ir.Design, error) {
	d, _, err := readDesignWithConfig(path)
	return d, err
}

// readDesignWithConfig is readDesign plus the resolved overlay, for a command that needs a config
// tier the READ itself does not consume.
//
// The overlay was always being composed here; readDesign kept only ReadOptions() from it and dropped
// the rest, including the datasheet corpus a project declares. `agni intake` then took its corpus
// from --params alone, so inside a project that declares params/ the datasheet-gap section was
// ABSENT rather than empty unless you named a directory the project already names (agni issue 474).
//
// Returning it rather than resolving a second time in the caller is the point. A second resolution
// can disagree with the first, and the review path carries a comment about exactly that: a run would
// then be filed against config other than the config it was read under.
func readDesignWithConfig(path string) (*ir.Design, service.Overlay, error) {
	ctx := context.Background()
	ws, err := workspace()
	if err != nil {
		return nil, service.Overlay{}, err
	}
	src, err := newDesignResolver(ws).Resolve(ctx, path)
	if err != nil {
		return nil, service.Overlay{}, err
	}
	noteSource(os.Stderr, src)
	// The design's PROJECT config reaches the read, the same way it reaches a service-backed one.
	//
	// This is the choke point for every command that does not go through a service — stats, diff,
	// emit, render, intake, profilediag — and it built its loader with no options at all, so all six
	// read under the BUILT-IN naming vocabulary however their project was configured. That is not a
	// cosmetic difference: net roles are resolved once at ingestion, so on the tutorial project the
	// built-ins see one rail where the project's own vocabulary sees four (agni issue 228).
	//
	// A descriptor that does not parse IS fatal, because it is this design's own configuration: the
	// read would otherwise silently use the built-in naming vocabulary and report a different answer
	// that looks like an answer. A design with NO descriptor still reads under the defaults, which is
	// the ordinary loose-file case (see designReadOptions).
	ov, err := designOverlay(ctx, path)
	if err != nil {
		return nil, service.Overlay{}, err
	}
	d, err := readerFor(newLoader(), ov.ReadOptions()...).ReadDesign(localOf(src.NetlistURI))
	return d, ov, err
}

// designReadOptions composes the per-read config a design's project supplies: its naming vocabulary,
// and the symbol libraries it declares.
//
// A design that belongs to NO project genuinely has no config, which is the ordinary case for a loose
// file, so that returns no options and no error. A descriptor that exists and does not PARSE is
// returned, because it is this design's own configuration and reading under the defaults instead
// would answer a different question without saying so.
//
// The two workspace lookups stay quiet on their own terms: failing to resolve the CLI's own workspace
// or to form a URI is not a statement about the design's project, and the caller is about to fail on
// its own path resolution anyway.
func designReadOptions(ctx context.Context, path string) ([]service.ReadOption, error) {
	ov, err := designOverlay(ctx, path)
	if err != nil {
		return nil, err
	}
	return ov.ReadOptions(), nil
}

// designOverlay resolves the whole overlay a design's project supplies, of which the read consumes
// only part. Same contract as designReadOptions above, which is now one line of it.
func designOverlay(ctx context.Context, path string) (service.Overlay, error) {
	ws, err := workspace()
	if err != nil {
		return service.Overlay{}, err
	}
	// A path that names nothing is left to the reader (cliWorkspace.URI), so what comes back here is
	// a failure to MINT: a governing project descriptor that exists and does not parse. Answering "no
	// read options" for that is how a run silently reads under the built-in vocabulary instead of the
	// project's, which is the same swallow one layer down (agni issue 312).
	u, err := ws.URI(path)
	if err != nil {
		return service.Overlay{}, err
	}
	return cliProjects().Overlay(ctx, u, &webapi.OverlayConfig{}, service.Overlay{}, "")
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
		RunE: func(cmd *cobra.Command, args []string) error {
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
			// Through the command's writer, like every neighbouring command. Printing to os.Stdout
			// directly makes the output unreachable from a test: a caller that sets cmd.SetOut gets an
			// empty buffer while the text still appears on the process stdout, so an assertion fails
			// with the expected string visibly printed a few lines above it.
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "design:              %s\n", d.Name)
			if d.SourceFormat != "" {
				fmt.Fprintf(w, "source format:       %s\n", d.SourceFormat)
			}
			fmt.Fprintf(w, "libraries:           %d\n", len(d.Libraries))
			fmt.Fprintf(w, "components:          %d (unique ref_des)\n", len(d.Components))
			fmt.Fprintf(w, "sections:            %d (source instances)\n", sectionsTotal)
			fmt.Fprintf(w, "multi-section:       %d (one ref_des, several sections)\n", multi)
			fmt.Fprintf(w, "nets:                %d\n", len(d.Nets))
			// Physical tier — shown only when a reader populated it (e.g. IPC-2581, KiCad PCB).
			if len(d.Footprints) > 0 {
				fmt.Fprintf(w, "footprints:          %d\n", len(d.Footprints))
			}
			if len(d.Layers) > 0 {
				fmt.Fprintf(w, "layers:              %d\n", len(d.Layers))
			}
			if d.Stackup != nil {
				fmt.Fprintf(w, "stackup layers:      %d\n", len(d.Stackup.Layers))
			}
			if len(d.Bom) > 0 {
				fmt.Fprintf(w, "bom lines:           %d\n", len(d.Bom))
			}
			return nil
		},
	}
}

func checkCmd() *cobra.Command {
	var ruleNames, tagPairs []string
	var format, failOn, paramsDir, conventions, profilePath, intentPath, resultsOut, boardPath string
	var verdicts bool
	var urlBase string
	cmd := &cobra.Command{
		Use:   "check <file>",
		Short: "Run structural rule checks over one design",
		Long: "Run structural rule checks over one design. With no flags, every rule runs. --rule " +
			"narrows by rule name and --tag key=value narrows by any catalog tag (category, tier, " +
			"distribution, or a provider's own), so e.g. --tag category=connectivity runs one group. " +
			"--format json emits the full findings array (one object per finding, subjects and all) " +
			"for tooling; markdown renders the severity-organized report (worst first, grouped by " +
			"rule); report emits that report as JSON (the GetCheckReport wire shape); html renders the " +
			"verdict report as a self-contained page and turns --verdicts on, since that is the only " +
			"table it has; the default text form is a per-rule summary, and it closes with what the " +
			"run considered. --fail-on error|warning|info exits non-zero when any finding sits at or " +
			"above the threshold, so check gates CI.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch format {
			case "text", "json", "csv", "markdown", "report":
			case "html":
				// html is the verdict REPORT, which has no findings-only form: it exists to show what
				// was checked, and the findings table already has three renderings. Asking for it IS
				// asking for the considered set, so it turns --verdicts on rather than refusing and
				// making the reader type a second flag that has no alternative.
				verdicts = true
			default:
				return fmt.Errorf("unknown --format %q (want: text, json, csv, markdown, report, html)", format)
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
			// --conventions: the CLI reads the file (files are the CLI's world, not the service's) and
			// sends the convention as a VALUE on the request, which the service composes (WS3-102).
			if conventions != "" {
				cfg, err := naming.Load(conventions)
				if err != nil {
					return err
				}
				overlay.Config = &webapi.AnalysisConfig{Conventions: cfg}
			}
			if profilePath != "" {
				// Naming the directory this design's project ALREADY composes is a mistake rather than a
				// request, and it used to be a silent one: the same profiles loaded twice under two source
				// names, every profile finding reported twice, and the coverage line counting each subject
				// again (agni issue 450). It is the same mistake --conventions treats as a duplicate-source
				// error one layer down, so it gets the same answer here.
				if err := refuseProfilePathTheProjectOwns(cmd.Context(), args[0], profilePath); err != nil {
					return err
				}
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
			}
			// Resolve the --rule/--tag facets to rule NAMES against the catalog the RUN will use, which is
			// the service's catalog plus the convention the request carries AND the resolved project's
			// own rules. CheckDesign / GetCheckReport select by name and the CLI owns facet resolution,
			// so the two must see the same name space — and a project's rules are part of it, or
			// `--rule gateway/signal-net-naming` would report "no rules selected" for a rule that runs.
			resolveAgainst, runOverlay, err := withProjectRules(cmd.Context(), catalog, args[0], overlay)
			// Noted HERE rather than above, because above is the catalog the FLAGS built and this is the
			// one the run uses. A project whose own profiles superseded a built-in got no note at all,
			// so the report described how the command line was spelled rather than how the run was
			// composed, which is C25's shape on a different message (agni issue 450).
			if err == nil {
				noteSupersededRules(cmd.ErrOrStderr(), resolveAgainst)
			}
			if err != nil {
				return err
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
			ll := &localLoader{loader: newLoader()}
			svc := service.NewCheckService(ll, catalog, specs, "", nil, cliProjects())
			ctx := cmd.Context()
			// Addressed once, ahead of the format branches: every request below names the same two
			// artifacts, and minting per call meant the same argument was turned into a URI up to four
			// times per run.
			designURI, err := cliArgURI(args[0])
			if err != nil {
				return err
			}
			boardURI, err := cliArgURI(boardPath)
			if err != nil {
				return err
			}
			var failFindings []*checkspb.Finding
			// --results-out takes one path through the service for every format (WS3-103): the document
			// holds findings in run order, and the severity pivot is rebuilt from it. Rendering from the
			// document rather than beside it is what makes the written artifact the SAME artifact the
			// terminal showed, instead of a second one that happens to agree today.
			if resultsOut != "" {
				resp, err := svc.CheckDesign(ctx, &webapi.CheckDesignRequest{Uri: designURI, Rules: names, Overlay: overlay, BoardUri: boardURI})
				if err != nil {
					return err
				}
				// Provenance comes off the composed overlay, not off the flags, for the same reason it
				// does on the review path: `agni check designs/gateway --results-out` inside a project
				// that declares conventions.yaml, profiles/ and params/ ran against all three and
				// recorded `run: {}`, because none of those three flags was passed. The flag values are
				// the DEPLOYMENT half of the union; the project's half comes from the overlay.
				doc := resultsDoc(designURI, selected, resp.GetFindings(), skippedProtos(resp.GetSkipped()), service.RunConfigProto(
					runOverlay.Provenance(service.RunProvenance{
						Params:      paramsDir != "",
						Profiles:    profilePath != "",
						Intent:      intentPath != "",
						Conventions: overlay.GetConfig().GetConventions().GetName(),
					}), 0))
				if err := writeResults(resultsOut, doc); err != nil {
					return err
				}
				if err := renderCheckResults(cmd.OutOrStdout(), doc, format); err != nil {
					return err
				}
				// The terminal shows THE RUN, so adding --results-out must not change what you see.
				// The rendering above comes from the document, which has no field for a considered set
				// (OUT_OF_SCOPE.md), so the coverage line is written here from the live response
				// instead. That is the one place the two legitimately differ: a replay of the document
				// later cannot reproduce this line, because the document never held it.
				if format == "text" {
					writeCoverage(cmd.OutOrStdout(), findingsFromProto(resp.GetFindings()), resp.GetVerdicts())
				}
				if failOn != "" && failsAtProto(resp.GetFindings(), failOn) {
					cmd.SilenceUsage = true
					return &gateError{msg: fmt.Sprintf("findings at or above --fail-on %s", failOn)}
				}
				return nil
			}
			switch format {
			case "markdown", "report":
				rresp, err := svc.GetCheckReport(ctx, &webapi.GetCheckReportRequest{Uri: designURI, Rules: names, Overlay: overlay, BoardUri: boardURI})
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
			default: // text, json, csv — all three need the raw findings
				resp, err := svc.CheckDesign(ctx, &webapi.CheckDesignRequest{Uri: designURI, Rules: names, Overlay: overlay, BoardUri: boardURI})
				if err != nil {
					return err
				}
				// --verdicts selects the CONSIDERED SET instead of the violations: what every
				// converted rule concluded about each subject it looked at, passes included. It is a
				// different table answering a different question, which is why it is a flag over the
				// same run rather than extra rows in the findings output. --fail-on still reads the
				// findings below, since a pass is not a gate condition.
				if verdicts {
					// A LINK IS A PROMISE, and this is the one place the promise is made. urlBase is
					// empty unless the operator says where the viewer is, and linkTarget is empty
					// for a design the CLI reached through a mount it minted rather than one the
					// operator named. Either one missing means every format emits no link rather than
					// one assembled from a guess, which would resolve on nobody's server (issue 392).
					//
					// A workspace that failed to build yields no link, which is the same fail-closed
					// answer: the error is reported by whichever call needed the workspace to do real
					// work, and a link is not worth inventing a second report for.
					//
					// REFUSING TO LINK IS SAID OUT LOUD. Fail-closed was already right and already
					// silent, so an operator who asked for links and got a page of rows with none had
					// nothing to read that named the missing half. The notes below are only printed
					// when --url-base was given, so a run that never asked for links stays quiet.
					ws, _ := workspace()
					mountPath, contentHash, why := verdictLinkTarget(ctx, ws, ll, designURI)
					if urlBase != "" && why != "" {
						fmt.Fprintf(cmd.ErrOrStderr(), "note: --url-base is set but no verdict links were emitted: %s\n", why)
					}
					if urlBase != "" && mountPath != "" {
						if m, ok := mounts.Find(ws.Mounts(), mountURIAuthority(designURI)); ok {
							keep, note := verifyServerMount(cmd.Context(), urlBase, m)
							if note != "" {
								fmt.Fprintf(cmd.ErrOrStderr(), "note: %s\n", note)
							}
							if !keep {
								mountPath = ""
							}
						}
					}
					meta := rpt.Report{
						Design:    designURI,
						Generated: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
						// BOTH HALVES NAME THE ENTRY, not the argument, and they come from one
						// resolution so they cannot drift apart (agni issue 489). The hash alone
						// resolved the entry before, which fixed the folder form's missing &hash=
						// (issue 479) and left the folder form's PATH pointing at a directory the
						// viewer cannot open, plus a companion form whose correct path carried the
						// entry's hash and read as a mismatch.
						ContentHash: contentHash,
						URLBase:     urlBase,
						MountPath:   mountPath,
					}
					switch format {
					case "csv":
						if err := writeVerdictCSV(cmd.OutOrStdout(), resp.GetVerdicts(), meta); err != nil {
							return err
						}
					case "json":
						if err := writeVerdictJSON(cmd.OutOrStdout(), resp.GetVerdicts()); err != nil {
							return err
						}
					case "html":
						// resolveAgainst, NOT catalog: the report has to read the catalog the RUN used, or
						// an operator's own rules arrive with no prose (agni issue 411). `catalog` is the
						// built-ins plus the ad-hoc flag sources; the project's rules and the
						// --conventions value are composed onto it by withProjectRules, and those are
						// exactly the rules a team wrote for its own boards. Looked up in the narrower
						// catalog they miss, and a rule with no summary, impact or remedy still renders,
						// under a bare name, which silently rewards using the built-ins over your own.
						if err := writeVerdictHTML(cmd.OutOrStdout(), resp, resolveAgainst.Rules(), meta); err != nil {
							return err
						}
					default:
						writeVerdictText(cmd.OutOrStdout(), buildVerdictReport(resp, resolveAgainst.Rules(), meta))
					}
					failFindings = resp.GetFindings()
					break
				}
				switch format {
				case "json":
					if err := writeCheckDesignJSON(cmd.OutOrStdout(), resp); err != nil {
						return err
					}
				case "csv":
					if err := writeCheckCSV(cmd.OutOrStdout(), resp.GetFindings()); err != nil {
						return err
					}
				default:
					writeCheckText(cmd.OutOrStdout(), findingsFromProto(resp.GetFindings()), len(selected), resp.GetVerdicts())
				}
				failFindings = resp.GetFindings()
			}
			if failOn != "" && failsAtProto(failFindings, failOn) {
				cmd.SilenceUsage = true
				return &gateError{msg: fmt.Sprintf("findings at or above --fail-on %s", failOn)}
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&ruleNames, "rule", nil, "run only these rules by name (repeatable)")
	cmd.Flags().StringArrayVar(&tagPairs, "tag", nil, "run only rules matching key=value tags (repeatable; e.g. --tag category=connectivity)")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text | json | csv | markdown | report | html (html is the verdict report and implies --verdicts)")
	cmd.Flags().StringVar(&urlBase, "url-base", "", "base address of a running viewer (e.g. http://localhost:8080) so every --verdicts format links each verdict to its proof: an anchor in the html report, a url column in the csv, a line under each row in the terminal. Omitted, or for a design reached by a path the server would not recognise, no link is emitted at all: a URL is a promise the reader can follow, and one assembled from a guessed address resolves on nobody's server")
	cmd.Flags().BoolVar(&verdicts, "verdicts", false, "report the CONSIDERED SET instead of the violations: what each rule concluded about every subject it looked at, with the evidence for a pass. Only rules that state one contribute; a rule absent from the output is declining to say, not reporting that it considered nothing. Honours --format text|csv|json|html, and --format html turns it on by itself. The default output states how much was considered without it")
	cmd.Flags().StringVar(&failOn, "fail-on", "", "exit non-zero when findings at or above this severity exist: error | warning | info")
	cmd.Flags().StringVar(&paramsDir, "params", "", "directory of seeded PartSpec textprotos (the datasheet parameter corpus, WS10); enables datasheet-backed rules")
	cmd.Flags().StringVar(&profilePath, "profile-path", "", "directory of YAML interface-profile declarations; their rules join the catalog alongside the built-in profiles")
	cmd.Flags().StringVar(&conventions, "conventions", "", "compose an operator naming-convention config (YAML) into the catalog; its rules appear namespaced as <config name>/<rule name>")
	cmd.Flags().StringVar(&intentPath, "intent-path", "", "a YAML design-intent declaration (expected modules, voltage domains); its rules join the catalog")
	cmd.Flags().StringVar(&boardPath, "board-path", "", "a board-geometry file (.kicad_pcb / IPC-2581 .xml|.cvg) attached to the netlist design so board-tier rules resolve instead of reading not-applicable. Only needed for a board that is NOT a declared companion of the design: a fab's returned file, or a layout not yet landed")
	cmd.Flags().StringVar(&resultsOut, "results-out", "", "also write the run as a self-contained check-result document (JSON) at this path; render it later with `agni results`")
	return cmd
}

// writeCheckText prints the default per-rule summary plus the first findings, the original
// human terminal form (reports and tooling use --format markdown/report/json).
//
// It closes with a COVERAGE line, because a findings list cannot say what was examined and a rule
// count is not an answer: twenty-nine rules finding nothing and twenty-nine rules that each looked at
// the wrong thing print the same line. The considered set rides on the same response, so this costs
// nothing to report, and it is stated by default because honesty about coverage should not be
// something a reader has to know to ask for. The per-subject rows stay behind --verdicts: they are
// six times the volume here and far more on a real board, so the DEFAULT states the claim and the
// flag shows the evidence.
func writeCheckText(w io.Writer, fs []check.Finding, rulesRun int, vs []*checkspb.Verdict) {
	if len(fs) == 0 {
		fmt.Fprintf(w, "no findings (%d rule(s) run)\n", rulesRun)
		writeCoverage(w, fs, vs)
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
		fmt.Fprintf(w, "  [%s] %s: %s (%s)\n", f.Severity, f.Rule, check.EntityRef(f.Subject), f.Message)
	}
	fmt.Fprintf(w, "\n%d finding(s) total\n", len(fs))
	writeCoverage(w, fs, vs)
}

// writeCoverage states what the run looked at, in the terms core/report already uses for the HTML
// report, so the two cannot describe the same run differently.
//
// The findings-only count is over rules that REPORTED SOMETHING without stating a considered set, not
// over every rule that ran. Those differ, and the difference matters: on the tutorial board 84 rules
// run and 22 state a considered set, but most of the other 62 simply had no subject in scope on this
// design. Calling them "violations only" would invent a coverage hole out of rules that correctly had
// nothing to say, which is the same false-confidence move in the opposite direction.
//
// NOT_CONSIDERED is counted apart from the judged outcomes because it is the one with no counterpart
// in a findings list: the rule was willing to judge and an input was missing.
func writeCoverage(w io.Writer, fs []check.Finding, vs []*checkspb.Verdict) {
	if len(vs) == 0 {
		return // nothing stated a considered set; claiming coverage would invent it
	}
	stating := map[string]bool{}
	judged, notConsidered := 0, 0
	for _, v := range vs {
		stating[v.GetRule()] = true
		if v.GetOutcome() == checkspb.Outcome_OUTCOME_NOT_CONSIDERED {
			notConsidered++
			continue
		}
		judged++
	}
	fmt.Fprintf(w, "%d subject(s) considered by %d rule(s)", judged, len(stating))
	if notConsidered > 0 {
		fmt.Fprintf(w, ", %d not considered", notConsidered)
	}
	fmt.Fprint(w, " (--verdicts for the detail)\n")
	silent := map[string]bool{}
	for _, f := range fs {
		if !stating[f.Rule] {
			silent[f.Rule] = true
		}
	}
	if len(silent) > 0 {
		fmt.Fprintf(w, "%d rule(s) reported violations without stating what they examined, so silence from those is not evidence of anything\n", len(silent))
	}
}

// writeCheckDesignJSON emits the CheckDesign response in protojson form, the same wire shape the RPC
// returns (findings sorted by rule then subject, each carrying its subject, provenance, sheet badges,
// and datasheet citation). The response is already sheet-annotated server-side (WS9-048), so the CLI
// marshals it verbatim — the conformance runner parses the same `.findings[]` whether it shells out or
// calls the API.
// The considered set is stripped before marshalling, so no DATA changes here: the default output
// carries the same findings it always did. It is a different answer (every subject looked at, passes
// included) and folding it in would change what every existing consumer receives, the same reason the
// verdict csv is a separate table rather than extra rows in the findings one. `--verdicts --format
// json` is where to ask for it.
//
// Not byte-identical, though. EmitUnpopulated means the new field still appears as `"verdicts": []`,
// which is inherent to adding a field to the response message rather than something stripping can
// undo. A consumer that rejects unknown keys sees one; a consumer reading `findings` is unaffected.
//
// It also keeps `results` honest: that command replays a written CheckResults document, which has no
// verdicts field, so emitting them here would make the two formats of one run disagree and the
// round-trip test says so.
func writeCheckDesignJSON(w io.Writer, resp *webapi.CheckDesignResponse) error {
	if len(resp.GetVerdicts()) > 0 {
		// A fresh message rather than a struct copy: a generated proto carries a MessageState with a
		// mutex in it, so copying one by value is what go vet's copylocks check exists to catch.
		resp = &webapi.CheckDesignResponse{Findings: resp.GetFindings(), Skipped: resp.GetSkipped()}
	}
	b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

func reviewCmd() *cobra.Command {
	var checklist, paramsDir, profilePath, intentPath, boardPath, format, renderDir, companion, conventions, resultsOut, urlBase string
	var coverage bool
	var ratifiedFloor float64
	var failOnOutcome string
	var minAnswered int
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
			// Parsed BEFORE anything is read, so a typo in a CI config fails in the first millisecond
			// rather than after a full run over a family of boards. The gate is otherwise applied last,
			// on every exit path below.
			gate, err := parseReviewGate(failOnOutcome, minAnswered)
			if err != nil {
				return err
			}
			// The checklist is configuration, so it travels as a value and where it came from stays the
			// caller's business (WS9-050). When the operator names none, the design's PROJECT is asked,
			// since a project declares its checklist the same way it declares its conventions and
			// profiles. The note says which one was chosen: a checklist nobody typed is not recoverable
			// from the outcomes it produced.
			man, checklistNote, err := reviewManifestFor(cmd.Context(), checklist, args)
			if err != nil {
				return err
			}
			noteChecklist(cmd.ErrOrStderr(), checklistNote)
			// The CLI is a thin client of the in-process ReviewService (WS9-048): it composes the
			// design-independent overlay config (catalog + profile index from --profile-path/--intent-path,
			// specs from --params) and constructs the service over a local no-containment loader, then
			// renders the proto response. Same service the web calls, so the two surfaces cannot drift.
			catalog, byName, err := composeReviewInputs(profilePath, intentPath)
			if err != nil {
				return err
			}
			// The supersession note is NOT written here, where check's equivalent used to be. This
			// catalog is the one the FLAGS built, and a project's own profiles supersede against the
			// catalog the RUN composes, so noting here described how the command line was spelled
			// rather than how the run was scored. It moves into the per-design loop below, which is
			// the only place a project has been resolved. Same defect and same shape as agni issue
			// 450, which PR 467 fixed for check and left standing here.
			// Reading the convention file is the CLI's job; the service takes the value.
			overlay := &webapi.OverlayConfig{}
			if conventions != "" {
				cfg, err := naming.Load(conventions)
				if err != nil {
					return err
				}
				overlay.Config = &webapi.AnalysisConfig{Conventions: cfg}
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
			ll := &localLoader{loader: newLoader()}
			svc := service.NewReviewService(ll, service.NewMemReviewStore(), catalog, byName, specs, env, "", cliProjects())
			// One create per design: a stored run is about ONE design, so the CLI's multi-design rollup is
			// several runs rather than one call. The loop is the rollup.
			var docs []*checkspb.CheckResults
			boardURI, err := cliArgURI(boardPath)
			if err != nil {
				return err
			}
			// Notes already written, so a rollup over several designs in ONE project says it once.
			// Per design is the correct SCOPE (two designs can resolve to projects that supersede
			// differently), and repeating an identical line per design is only noise.
			noted := map[string]bool{}
			for _, design := range args {
				parent, err := cliProjectParent(cmd.Context(), design)
				if err != nil {
					return err
				}
				// The catalog the run will be scored against, which is the flag-built one plus this
				// design's project's own rules. Composed here only to report what it superseded; the
				// service composes its own from the same inputs.
				if scored, _, err := withProjectRules(cmd.Context(), catalog, design, overlay); err == nil {
					var b strings.Builder
					noteSupersededRules(&b, scored)
					if line := b.String(); line != "" && !noted[line] {
						noted[line] = true
						fmt.Fprint(cmd.ErrOrStderr(), line)
					}
				}
				designURI, err := cliArgURI(design)
				if err != nil {
					return err
				}
				rv, err := svc.CreateReview(cmd.Context(), &webapi.CreateReviewRequest{
					// The run is stored under the design's project when it has one. It is resolved here
					// rather than inside CreateReview because the caller has already resolved this design
					// to compose its config, and a second resolution in the service could disagree with
					// the first — the run would then be filed under a project other than the one whose
					// rules scored it.
					Parent:   parent,
					Manifest: service.ManifestProto(man), DesignUri: designURI, BoardUri: boardURI, RatifiedFloor: ratifiedFloor,
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
				summary, err := renderReviewImages(reports, designSourcesOf(docs), renderDir, companion)
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
				if err := renderReviewResults(cmd.OutOrStdout(), doc, format, coverage); err != nil {
					return err
				}
				// The gate applies here too. This path returned early before, which is exactly the shape
				// that lets a flag work on two of three code paths and silently not on the third — and it
				// would have been the CI path, since a pipeline that gates is also the one archiving its
				// results document.
				return gateReview(cmd, gate, reports)
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
				case format == "html":
					// The checklist page, which is a different SHAPE from `check --format html` rather
					// than a filter of it: items in the team's own order, one row per question. It
					// carries every finding per item, where the markdown Detail cell caps at three,
					// which is what the cap's own comment says the web surface is for.
					meta, err := checklistMeta(cmd, ll, args[0], urlBase)
					if err != nil {
						return err
					}
					return rpt.ChecklistHTML(cmd.OutOrStdout(), buildChecklist(rep, meta))
				case format == "" || format == "markdown":
					out = review.RenderMarkdown(rep)
				default:
					return fmt.Errorf("review: unknown --format %q (want markdown, json or html)", format)
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
				case format == "html":
					// One page addresses one design: its title, its content hash and every link in it
					// name that design. Concatenating several would produce a document whose header
					// belongs to whichever ran last, so this refuses instead of guessing.
					return fmt.Errorf("review --format html takes one design (got %d); run it once per design", len(args))
				default:
					return fmt.Errorf("review: unknown --format %q (want markdown, json or html)", format)
				}
			}
			if err != nil {
				return err
			}
			if _, err := fmt.Fprint(cmd.OutOrStdout(), out); err != nil {
				return err
			}
			return gateReview(cmd, gate, reports)
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
	cmd.Flags().StringVar(&format, "format", "markdown", "per-item report format: markdown (Detail cell capped), json (full findings, for tooling), or html (the checklist as a self-contained page, every finding per item)")
	cmd.Flags().StringVar(&urlBase, "url-base", "", "base address of a running viewer (e.g. http://localhost:8080) so an --format html checklist links each finding to its proof. Subject to the same promise as `check --url-base`: the mount has to be one you DECLARED, and the server is asked whether it serves that name from the same root. Whenever links are withheld the reason is printed")
	cmd.Flags().StringVar(&renderDir, "render", "", "also write an annotated schematic SVG per design (each finding highlighted in place) to <dir>/<design-stem>/<sheet>.svg")
	cmd.Flags().StringVar(&companion, "companion", "", "geometry file (.eds) to draw the --render images on, joined to the netlist findings by net name; with one design only (else a sibling <stem>.eds is auto-detected per design)")
	cmd.Flags().StringVar(&resultsOut, "results-out", "", "also write the run as a self-contained check-result document (JSON) at this path; one design only. Render it later with `agni results`")
	cmd.Flags().StringVar(&failOnOutcome, "fail-on-outcome", "", "exit non-zero when any checklist ITEM sits at one of these outcomes (comma-separated, e.g. fail or fail,provisional). This is the coverage axis, not `check --fail-on`'s severity axis: it asks whether a question was answered, not how bad the answer was. Off by default")
	cmd.Flags().IntVar(&minAnswered, "min-answered", 0, "exit non-zero when fewer than N checklist items produced an ANSWER (pass, fail, provisional, or computed-n/a). Distinct from the covered count, which still counts an item whose rule is present but whose inputs are absent; that is the regression a severity gate cannot see. Off by default")
	return cmd
}

// gateReview applies the CI gate after the report has been rendered, so a tripped pipeline still gets
// its full report on stdout rather than only an error on stderr.
//
// SilenceUsage matches what `check --fail-on` does on a trip: a gate firing is the command working as
// asked, and dumping the usage text under it reads as a mistyped invocation.
func gateReview(cmd *cobra.Command, g reviewGate, reports []review.Report) error {
	if err := g.trip(reports); err != nil {
		cmd.SilenceUsage = true
		return err
	}
	return nil
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
	var renameApprox bool
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
			opts := diff.DefaultRenameOptions()
			opts.Enabled = renameApprox
			rep := diff.Designs(a, b, opts)
			w := cmd.OutOrStdout()
			switch format {
			case "json":
				return writeDiffJSON(w, rep)
			case "csv":
				return writeDiffCSV(w, rep)
			}
			fmt.Fprintf(w, "diff %s -> %s\n\n", args[0], args[1])
			fmt.Fprint(w, rep.Render(diffListLimit))
			return nil
		},
		// Validated ahead of the read, so a misspelled format fails before two designs are parsed.
		// This command used to accept any value and silently render text, so `--format jsn` produced
		// a human summary that a script would then fail to parse for reasons nothing explained.
		PreRunE: func(_ *cobra.Command, _ []string) error {
			switch format {
			case "text", "json", "csv":
				return nil
			}
			return fmt.Errorf("unknown --format %q (want: text, json, csv)", format)
		},
	}
	c.Flags().StringVar(&format, "format", "text",
		"output format: text (human summary), json (the DiffDesignsResponse wire shape the web API serves), or csv (one row per change)")
	c.Flags().BoolVar(&renameApprox, "rename-approx", false,
		"also pair a net that was renamed AND changed slightly, reported as renamed-approx with the evidence behind each pairing. Off by default: this ASSIGNS a best match among candidates rather than recovering a fact, so a gate reading the output should opt in")
	return c
}

func emitCmd() *cobra.Command {
	var format string
	c := &cobra.Command{
		Use:   "emit <in> [out]",
		Short: "Convert a design to IPC-2581 or an EDIF netlist (any input format; stdout if out omitted)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			out := ""
			if len(args) == 2 {
				out = args[1]
			}
			// Resolve the format BEFORE reading, so an unknown one fails on the flag rather than
			// after the design has been read and, when out names a file, after it has been created
			// and truncated.
			kind, err := emitFormat(format, out)
			if err != nil {
				return err
			}
			d, err := readDesign(args[0])
			if err != nil {
				return err
			}
			w := io.Writer(os.Stdout)
			if out != "" {
				f, err := os.Create(out)
				if err != nil {
					return err
				}
				defer f.Close()
				w = f
			}
			if kind == emitEDIF {
				return edif.WriteNetlist(w, d)
			}
			return ipc2581.Write(w, d)
		},
	}
	c.Flags().StringVar(&format, "format", "",
		"output format: ipc2581 or edif. Omitted, it follows the OUT file's extension (.edn, .edf and .edif are EDIF), and is ipc2581 when writing to stdout, which has no extension to read")
	return c
}

// The formats emit can write. Named so the resolver below and its caller cannot disagree on a
// spelling, which a bare string pair silently would.
const (
	emitEDIF    = "edif"
	emitIPC2581 = "ipc2581"
)

// emitFormat resolves which format an emit run writes, and NOTHING else. It deliberately returns the
// format's name rather than the writer itself: a writer value would put a raw *ir.Design in this
// signature, and C19 reserves that for a producer or an entry point, which format selection is
// neither. Choosing a format is a question about two strings and should not need to know the design
// type at all.
//
// A named format wins outright. Otherwise the OUT file's extension decides, which is the same
// dispatch the read side uses (readers/formats) and is what lets `agni emit board.kicad_sch
// board.edn` need no flag at all. Writing to stdout has no extension, so it falls back to IPC-2581,
// which is what this command emitted unconditionally before EDIF joined it.
//
// .eds is refused rather than treated as EDIF. It is a dual-capability format, a netlist AND the
// faithful schematic geometry beside it, so writing one from the netlist writer alone would produce
// a file that claims to carry a drawing and carries none. There is no schematic writer yet, and
// silently emitting half a document is worse than saying so.
func emitFormat(format, out string) (string, error) {
	if format == "" {
		switch strings.ToLower(filepath.Ext(out)) {
		case ".edn", ".edf", ".edif":
			format = emitEDIF
		case ".eds":
			return "", fmt.Errorf("cannot emit %s: .eds is an EDIF SCHEMATIC (a netlist plus its geometry) and only the netlist writer exists; write the netlist to .edn, or pass --format to override", out)
		default:
			format = emitIPC2581
		}
	}
	switch strings.ToLower(format) {
	case emitEDIF:
		return emitEDIF, nil
	case emitIPC2581:
		return emitIPC2581, nil
	}
	return "", fmt.Errorf("unknown emit format %q (have: %s, %s)", format, emitIPC2581, emitEDIF)
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
