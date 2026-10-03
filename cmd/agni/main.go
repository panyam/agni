// Command agni is the CLI for the EDA tooling engine. It reads a design into the neutral IR and
// runs analyses over it. File I/O lives here at the edge and the logic lives in core/, readers/
// and service/ (CONSTRAINTS C1).
//
// Every analysis runs over the IR, so it is format-neutral. The reader is picked by file type
// (readDesign) and everything downstream is the same whatever the source format.
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
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/panyam/agni"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/diff"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/core/render"
	rpt "github.com/panyam/agni/core/report"
	"github.com/panyam/agni/core/review"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/opscmd"
	"github.com/panyam/agni/internal/version"
	"github.com/panyam/agni/readers/edif"
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/readers/ipc2581"
	"github.com/panyam/agni/service"
	_ "github.com/panyam/agni/stdlib/lib"           // registers the shipped derived relations (net.has_test_point, ...)
	"github.com/panyam/agni/stdlib/profiles"        // registers built-in "profile" rules; LoadDir adds overlay profiles
	_ "github.com/panyam/agni/stdlib/relations"     // registers the built-in EDB query relations (netlist/board/datasheet)
	_ "github.com/panyam/agni/stdlib/reviewquery"   // compiles a review manifest's inline query bindings as datalog
	_ "github.com/panyam/agni/stdlib/rules/builtin" // registers the built-in EE rule catalog (anonymous source)
	_ "github.com/panyam/agni/stdlib/rules/datalog" // registers the "dl" datalog-authored rule source
	"github.com/panyam/agni/stdlib/rules/intent"
)

// diffListLimit caps how many items each diff section prints.
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
		// `agni --version`. The same string internal/version stamps into a results document's
		// provenance. `agni version` adds the toolchain and platform.
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
	root.AddCommand(statsCmd(), checkCmd(), diffCmd(), renderCmd(), emitCmd(), validateCmd(), censusCmd(), serveCmd(), openCmd(), nativeCmd(), queryCmd(), traceCmd(), reviewCmd(), startCmd(), intakeCmd(), resultsCmd(), importResultsCmd(), opscmd.Healthcheck("agni", "localhost:8080"), opscmd.Version("agni"), paramsCmd(), tabulateCmd())
	return root
}

// symbolPaths holds the --symbol-path search directories for resolving xschem/gEDA symbols.
var symbolPaths []string

// envConfigWebDir holds the web_dir an agni.yaml named, "" when no file named one. applyEnvConfig
// fills it and resolveWebDir consults it. A package var for the same reason as cliMountSpecs, since
// the file is read once in PersistentPreRunE before any command knows it needs the value.
var envConfigWebDir string

// envConfigNativeTools holds the native_tools an agni.yaml named, the same way as envConfigWebDir.
// Only serve consumes it.
var envConfigNativeTools []string

// envSymbolPath names the environment variable that supplies --symbol-path when the flag is
// absent. It exists for the container image, where the symbol libraries ship at a fixed location
// and EVERY subcommand needs them. Overriding the image's CMD replaces the whole argument list, so
// without this `docker run <image> check board.kicad_sch` reads short with no error and reports
// fewer findings.
//
// Colon-separated, like PATH. The flag wins outright rather than appending, so an explicit
// --symbol-path is never widened by ambient configuration.
const envSymbolPath = "AGNI_SYMBOL_PATH"

// envWebDir names the environment variable that supplies --web-dir when neither the flag nor an
// agni.yaml names one.
//
// It serves an INSTALLED binary run from wherever the design lives, whose assets sit at a fixed
// absolute path. From a checkout the relative default already resolves, so two checkouts each serve
// their own assets. Precedence is the flag, then agni.yaml, then this.
const envWebDir = "AGNI_WEB_DIR"

// defaultWebDir is where the viewer's assets live relative to a repo checkout. `make serve`,
// `make demo` and the container's CMD rely on it (PR 457).
const defaultWebDir = "web"

// applyEnvConfig fills the tier-1 flags from the nearest agni.yaml, for the ones the operator did not
// pass. It runs once, before any command.
//
// A passed flag WINS OUTRIGHT rather than merging, as --symbol-path does with its environment
// variable, because an operator who named a mount table is answering for the whole table.
//
// The file used is announced on stderr, since a mount table nobody typed cannot be recovered from
// the run's output.
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
// The source is returned rather than logged so the caller can announce only the values nobody typed.
// source is "" for the flag and the built-in default, "agni.yaml" or envWebDir otherwise.
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
// --symbol-path values (or the envSymbolPath fallback).
func newLoader() *formats.Loader {
	l := &formats.Loader{SymbolPaths: resolveSymbolPaths(os.Getenv)}
	// The CLI opens a design by absolute host path, so without this every locator would record this
	// machine's directory layout and `--format json` and `--results-out` would publish it (agni issue
	// 501). The lookup is deferred because the argument's mount is minted after the loader is built.
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
// design's descriptor has decided which file that is (designResolver.Resolve).
func readDesign(path string) (*ir.Design, error) {
	d, _, err := readDesignWithConfig(path)
	return d, err
}

// readDesignWithConfig is readDesign plus the resolved overlay, for a command that needs a config
// tier the READ itself does not consume, such as the datasheet corpus a project declares (agni issue
// 474). Take the tier from here rather than resolving the project again in the caller, because a
// second resolution can disagree with the one the design was read under.
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
	// The design's PROJECT config reaches the read here for every command no service mediates
	// (stats, diff, emit, render, intake, profilediag). Net roles are resolved at ingestion, so
	// without it the tutorial project reads one rail where its own vocabulary sees four (agni issue
	// 228). A descriptor that does not parse is fatal and a design with none reads under the
	// defaults (see cliResolveProject).
	ov, err := designOverlay(ctx, path)
	if err != nil {
		return nil, service.Overlay{}, err
	}
	netlist := localOf(src.NetlistURI)
	d, err := readerFor(newLoader(), ov.ReadOptions()...).ReadDesign(netlist)
	if err != nil {
		return nil, service.Overlay{}, err
	}
	fmt.Fprint(os.Stderr, hierarchyNote(netlist, d.GetInputDiagnostics().GetUnexpandedHierarchy()))
	return d, ov, nil
}

// designReadOptions composes the per-read config a design's project supplies, which is its naming
// vocabulary and the symbol libraries it declares.
//
// A design in NO project returns no options and no error, which is the ordinary case for a loose
// file. A descriptor that exists and does not PARSE is an error, since reading under the defaults
// would answer a different question without saying so.
func designReadOptions(ctx context.Context, path string) ([]service.ReadOption, error) {
	ov, err := designOverlay(ctx, path)
	if err != nil {
		return nil, err
	}
	return ov.ReadOptions(), nil
}

// designOverlay resolves the whole overlay a design's project supplies, of which the read consumes
// only part. Same contract as designReadOptions above.
func designOverlay(ctx context.Context, path string) (service.Overlay, error) {
	ws, err := workspace()
	if err != nil {
		return service.Overlay{}, err
	}
	// A path that names nothing is left to the reader (cliWorkspace.URI), so an error here is a
	// failure to MINT, meaning a governing descriptor that does not parse. Swallowing it would read
	// under the built-in vocabulary (agni issue 312).
	u, err := ws.URI(path)
	if err != nil {
		return service.Overlay{}, err
	}
	return cliProjects().Overlay(ctx, u, &webapi.OverlayConfig{}, "")
}

// noteSource writes a resolution note to w, if there is one. Notes go to stderr so a redirect never
// contaminates a `--format json` document on stdout.
func noteSource(w io.Writer, src designSource) {
	if src.Note != "" {
		fmt.Fprint(w, src.Note)
	}
}

// readModel builds the check Model for a path, which is the netlist IR plus the board geometry when
// the design has a board (WS3-008). A board that exists and fails to parse fails the run, since
// checking without it would pass board rules that never ran.
func readModel(path string) (check.Model, error) {
	m, err := readModelWithParams(path, "")
	return m, err
}

// readModelWithParams also attaches the params tier (WS10-003) from a directory of PartSpec
// textprotos when paramsDir is non-empty. A bad corpus fails the run (param.LoadSet is
// all-or-nothing), since a smaller corpus would report false passes.
//
// The netlist and board tiers come from the design descriptor (designResolver.Resolve), so they can
// be DIFFERENT files, with a declared board companion supplying the copper (C21). check, query and review
// build their models through the services' BuildModel instead, which is where --board-path (WS3-089)
// applies.
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
		return check.NewModel(d, check.WithBoard(bg)), nil
	}
	specs, err := param.LoadSet(os.DirFS(paramsDir))
	if err != nil {
		return nil, fmt.Errorf("--params %s: %w", paramsDir, err)
	}
	return check.NewModel(d, check.WithBoard(bg), check.WithParamProvider(specs)), nil
}

func statsCmd() *cobra.Command {
	var format string
	var maskPaths, nets, refs []string
	c := &cobra.Command{
		Use:   "stats <file>",
		Short: "Print component/section/net counts for one design",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch format {
			case "json":
				return statsJSON(cmd, args[0], maskPaths, nets, refs)
			case "text":
			default:
				return fmt.Errorf("--format %s: want text or json", format)
			}
			if len(maskPaths)+len(nets)+len(refs) > 0 {
				return fmt.Errorf("--mask, --net and --ref select what --format json carries; the text summary takes none")
			}
			d, err := readDesign(args[0])
			if err != nil {
				return err
			}
			// Components are grouped by ref_des in the IR, each holding one section per source instance.
			sectionsTotal, multi := 0, 0
			for _, c := range d.Components {
				sectionsTotal += len(c.Sections)
				if len(c.Sections) > 1 {
					multi++
				}
			}
			// Through the command's writer rather than os.Stdout, or a test's cmd.SetOut buffer stays
			// empty while the text still reaches the process stdout.
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
			// The counts above cover what the read EXTRACTED, which on a hierarchical design is the top
			// level alone. Said here as well as in the stderr note (agni issue 707).
			if blocks := d.GetInputDiagnostics().GetUnexpandedHierarchy(); len(blocks) > 0 {
				fmt.Fprintf(w, "not extracted:       %s (counts above are the top %s only)\n",
					hierarchySummary(blocks), hierarchyNoun(blocks))
			}
			// Physical tier, shown only when a reader populated it (IPC-2581, KiCad PCB).
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
	c.Flags().StringVar(&format, "format", "text", "text (the summary above) or json (the GetDesignResponse the GetDesign rpc returns)")
	c.Flags().StringSliceVar(&maskPaths, "mask", nil, "with --format json, the read_mask: which fields to carry, over GetDesignResponse. `design` is the whole IR, `design.nets` or `design.components.mpn` parts of it, `*` everything. Repeatable or comma-separated; unset is the summary alone")
	c.Flags().StringArrayVar(&nets, "net", nil, "with --format json and a mask selecting design, keep only this net in design.nets (repeatable)")
	c.Flags().StringArrayVar(&refs, "ref", nil, "with --format json and a mask selecting design, keep only this component in design.components (repeatable)")
	return c
}

// statsJSON answers `stats --format json` with the GetDesignResponse the GetDesign rpc returns,
// through the same in-process DesignService the server runs (C31), so --mask, --net and --ref mean
// what read_mask, nets and ref_des mean on the wire (agni issue 836).
func statsJSON(cmd *cobra.Command, design string, maskPaths, nets, refs []string) error {
	uri, err := cliArgURI(design)
	if err != nil {
		return err
	}
	req := &webapi.GetDesignRequest{Uri: string(uri), AsNamed: readAsNamed, Nets: nets, RefDes: refs}
	if len(maskPaths) > 0 {
		req.ReadMask = &fieldmaskpb.FieldMask{Paths: maskPaths}
	}
	svc := service.NewDesignService(&localLoader{loader: newLoader()}, nil, render.Style{}, cliProjects())
	resp, err := svc.GetDesign(cmd.Context(), req)
	if err != nil {
		return err
	}
	out, err := protoJSON(resp)
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write([]byte(out))
	return err
}

func checkCmd() *cobra.Command {
	var ruleNames, tagPairs []string
	var format, failOn, paramsDir, conventions, profilePath, intentPath, resultsOut, boardPath string
	var verdicts bool
	var serverVal, outPath string
	var orderBy []string
	var srvSpec serverSpec
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
			// Before redirectOut, so a --server this process cannot honour refuses the run before the
			// output file is created (agni issue 637).
			var err error
			if srvSpec, err = resolveServer(serverVal); err != nil {
				return err
			}
			closeOut, err := redirectOut(cmd, outPath)
			if err != nil {
				return err
			}
			defer closeOut()
			switch format {
			case "text", "json", "csv", "markdown", "report":
			case "html":
				// html is the verdict REPORT and has no findings-only form, so it turns --verdicts on
				// rather than refusing.
				verdicts = true
			default:
				return fmt.Errorf("unknown --format %q (want: text, json, csv, markdown, report, html)", format)
			}
			if len(orderBy) > 0 && format != "csv" {
				return fmt.Errorf("--order-by orders a csv table; --format %s has none", format)
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
			// Ad-hoc sources (overlay profiles, design intent) compose onto the catalog. CatalogWith
			// keeps the built-ins AND any RegisterSource'd suites, where NewCatalog would drop the latter.
			overlay := &webapi.OverlayConfig{}
			var extra []check.RuleSource
			// The CLI reads the --conventions file and sends the convention as a VALUE on the request,
			// which the service composes (WS3-102).
			if conventions != "" {
				cfg, err := naming.Load(conventions)
				if err != nil {
					return err
				}
				overlay.Config = &webapi.AnalysisConfig{Conventions: cfg}
			}
			if profilePath != "" {
				// Naming the directory this design's project ALREADY composes would load every profile
				// twice and report each profile finding twice (agni issue 450). Refused, as --conventions
				// refuses a duplicate source one layer down.
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
			// --intent-path rides the request as a VALUE, as the design's own intent: section does, so the
			// model sees which connectors it declares internal and a design that already declares
			// intent has it replaced rather than compiled twice (agni issue 831).
			if intentPath != "" {
				di, err := intent.LoadFileProto(intentPath)
				if err != nil {
					return err
				}
				if overlay.Config == nil {
					overlay.Config = &webapi.AnalysisConfig{}
				}
				overlay.Config.Intent = di
			}
			if len(extra) > 0 {
				catalog = check.CatalogWith(extra...)
			}
			// Resolve the --rule/--tag facets to rule NAMES against the catalog the RUN will use, which
			// adds the request's convention and the project's own rules. CheckDesign and GetCheckReport
			// select by name, so without the project's rules `--rule gateway/signal-net-naming` would
			// report "no rules selected" for a rule that runs.
			resolveAgainst, runOverlay, err := withProjectRules(cmd.Context(), catalog, args[0], overlay)
			// Noted against the run's catalog rather than the flag-built one, so a project's own
			// supersessions are reported too (C25, agni issue 450).
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
			// Thin client of the in-process CheckService, the same service the web check panel runs
			// (WS9-048). The catalog and datasheet corpus are injected, while --conventions rides the
			// request so its lexicon reaches the design read without touching process state.
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
			// Addressed once, since every request below names the same two artifacts.
			designURI, err := cliArgURI(args[0])
			if err != nil {
				return err
			}
			boardURI, err := cliArgURI(boardPath)
			if err != nil {
				return err
			}
			var failFindings []*checkspb.Finding
			// --results-out takes one path through the service for every format (WS3-103). The document
			// holds findings in run order and the terminal output is rendered FROM it, so the file and
			// the terminal show the same artifact.
			if resultsOut != "" {
				resp, err := svc.CheckDesign(ctx, &webapi.CheckDesignRequest{Uri: designURI, Rules: names, Overlay: overlay, BoardUri: boardURI, AsNamed: readAsNamed})
				if err != nil {
					return err
				}
				// Provenance comes off the composed overlay, not the flags, or a project declaring
				// conventions, profiles and params records `run: {}` when no flag was passed. The flag
				// values are the DEPLOYMENT half of the union and the overlay adds the project's half.
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
				// The document has no field for a considered set (OUT_OF_SCOPE.md), so the coverage
				// line comes from the live response. A later replay of the document cannot reproduce it.
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
				rresp, err := svc.GetCheckReport(ctx, &webapi.GetCheckReportRequest{Uri: designURI, Rules: names, Overlay: overlay, BoardUri: boardURI, AsNamed: readAsNamed})
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
			default: // text, json and csv all need the raw findings
				resp, err := svc.CheckDesign(ctx, &webapi.CheckDesignRequest{Uri: designURI, Rules: names, Overlay: overlay, BoardUri: boardURI, AsNamed: readAsNamed})
				if err != nil {
					return err
				}
				// --verdicts selects the CONSIDERED SET instead of the violations, which is what each
				// rule concluded about every subject it looked at, passes included. --fail-on still reads
				// the findings, since a pass is not a gate condition.
				if verdicts {
					// viewerLinkMeta decides whether rows carry a viewer link, and emits none rather
					// than a guessed one (issue 392). Both halves of a link name the ENTRY rather than
					// the argument and come from one resolution (agni issue 489).
					meta := viewerLinkMeta(cmd, ctx, ll, designURI, srvSpec)
					meta.Generated = time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
					switch format {
					case "csv":
						if err := writeVerdictCSV(cmd.OutOrStdout(), resp.GetVerdicts(), meta, orderBy...); err != nil {
							return err
						}
					case "json":
						if err := marshalCheckDesign(cmd.OutOrStdout(), resp); err != nil {
							return err
						}
					case "html":
						// resolveAgainst, NOT catalog, because the report has to read the catalog the RUN
						// used. `catalog` lacks the project's rules and the --conventions rules, which would
						// then render under a bare name with no summary, impact or remedy (agni issue 411).
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
					if err := writeCheckCSV(cmd.OutOrStdout(), resp.GetFindings(), orderBy...); err != nil {
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
	serverFlag(cmd, &serverVal)
	withSelfServer(cmd, &srvSpec)
	outFileFlag(cmd, &outPath)
	cmd.Flags().StringSliceVar(&orderBy, "order-by", nil, "order the csv rows by these columns, comma-separated, a leading - for descending: --order-by=rule,-subject. A column sorts by its type, so subject puts R2 before R10. csv only, since it orders the projected table the TableService serves (agni issue 862)")
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

// writeCheckText prints the default per-rule summary plus the first findings (reports and tooling
// use --format markdown/report/json).
//
// It closes with a COVERAGE line by default, since a findings list cannot say what was examined. The
// per-subject rows stay behind --verdicts, being six times the volume here and far more on a real
// board.
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

// writeCoverage states what the run looked at, in the terms core/report uses for the HTML report.
//
// The findings-only count is over rules that REPORTED SOMETHING without stating a considered set,
// not over every rule that ran. On the tutorial board 84 rules run and 22 state a considered set, and
// most of the other 62 had no subject in scope, so counting them would invent a coverage hole.
//
// NOT_CONSIDERED is counted apart from the judged outcomes, since it means the rule was willing to
// judge and an input was missing.
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

// writeCheckDesignJSON emits the CheckDesign response as protojson, the wire shape the RPC returns
// (C31). The response is already sheet-annotated by the service (WS9-048), so it is marshalled
// verbatim.
//
// The considered set is stripped first, since `--verdicts --format json` is where to ask for it and
// `agni results` replays a CheckResults document that has no verdicts field. EmitUnpopulated still
// prints `"verdicts": []`, which a consumer rejecting unknown keys will see.
func writeCheckDesignJSON(w io.Writer, resp *webapi.CheckDesignResponse) error {
	if len(resp.GetVerdicts()) > 0 {
		// A fresh message rather than a struct copy, because a generated proto carries a mutex and
		// go vet's copylocks check rejects copying one by value.
		resp = &webapi.CheckDesignResponse{Findings: resp.GetFindings(), Skipped: resp.GetSkipped()}
	}
	return marshalCheckDesign(w, resp)
}

// marshalCheckDesign prints the response as it stands. `--verdicts --format json` calls it with the
// considered set in place, so that output is the whole CheckDesign response a server returns (agni
// issue 822), and a client reading verdicts through the CLI parses the same message.
func marshalCheckDesign(w io.Writer, resp *webapi.CheckDesignResponse) error {
	b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

func reviewCmd() *cobra.Command {
	var checklist, paramsDir, profilePath, intentPath, boardPath, format, renderDir, companion, conventions, resultsOut, serverVal, reviewOutPath string
	var srvSpec serverSpec
	var coverage bool
	var ratifiedFloor float64
	var failOnOutcome string
	var minAnswered int
	var reviewLibs, orderBy []string
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
			// This and the --server check run before redirectOut, so a run that cannot produce what
			// was asked for refuses before the output file is created (agni issue 637).
			if coverageShadowsFormat(cmd, coverage) {
				return fmt.Errorf("review: --coverage emits the per-area rollup, which renders as markdown "+
					"only, so --format %q would be discarded. Pass one or the other. The per-item page "+
					"(--format html on its own) already carries the same rollup in its header", format)
			}
			var err error
			if srvSpec, err = resolveServer(serverVal); err != nil {
				return err
			}
			closeOut, err := redirectOut(cmd, reviewOutPath)
			if err != nil {
				return err
			}
			defer closeOut()
			// Parsed BEFORE anything is read, so a typo in a CI config fails before a full run. The gate
			// itself applies last, on every exit path below.
			gate, err := parseReviewGate(failOnOutcome, minAnswered)
			if err != nil {
				return err
			}
			// The checklist travels as a value and where it came from is the caller's business
			// (WS9-050). With no --checklist the design's PROJECT supplies one, and the note names it,
			// since a checklist nobody typed cannot be recovered from the outcomes.
			man, checklistNote, err := reviewManifestFor(cmd.Context(), checklist, cmd.InOrStdin(), args)
			if err != nil {
				return err
			}
			noteChecklist(cmd.ErrOrStderr(), checklistNote)
			// A thin client of the in-process ReviewService, the same service the web calls (WS9-048).
			// The CLI composes the design-independent inputs and renders the proto response.
			catalog, byName, err := composeReviewInputs(profilePath)
			if err != nil {
				return err
			}
			// The supersession note is written per design in the loop below, against the catalog the
			// run composes rather than this flag-built one (agni issue 450).
			overlay := &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{}}
			if conventions != "" {
				cfg, err := naming.Load(conventions)
				if err != nil {
					return err
				}
				overlay.Config.Conventions = cfg
			}
			// --lib rides the request as values, as it does for `query` (agni issues 779, 788), so an
			// inline checklist query can name its members.
			if err := addLibraries(overlay.Config, reviewLibs); err != nil {
				return err
			}
			// --intent-path rides the request as a value, as it does for `check` (agni issue 831).
			if intentPath != "" {
				di, err := intent.LoadFileProto(intentPath)
				if err != nil {
					return err
				}
				overlay.Config.Intent = di
			}
			var specs param.ParamProvider
			if paramsDir != "" {
				set, err := param.LoadSet(os.DirFS(paramsDir))
				if err != nil {
					return fmt.Errorf("--params %s: %w", paramsDir, err)
				}
				specs = set
			}
			// The service creates review RESOURCES (WS9-053), here over an IN-MEMORY store so `agni
			// review` leaves no files behind. --results-out is the explicit way to write one.
			env := service.ReviewEnv{ProducerVersion: version.Version(), Profiles: profilePath != "", Intent: intentPath != ""}
			ll := &localLoader{loader: newLoader()}
			svc := service.NewReviewService(ll, service.NewMemReviewStore(), catalog, byName, specs, env, "", cliProjects())
			// One create per design, since a stored run is about ONE design. The loop is the rollup.
			var docs []*checkspb.CheckResults
			var reviews []*webapi.Review
			boardURI, err := cliArgURI(boardPath)
			if err != nil {
				return err
			}
			// Notes already written. Two designs can resolve to projects that supersede differently, so
			// the note is per design, and an identical line is printed once.
			noted := map[string]bool{}
			for _, design := range args {
				parent, err := cliProjectParent(cmd.Context(), design)
				if err != nil {
					return err
				}
				// The flag-built catalog plus this design's project rules, composed here only to report
				// what it superseded. The service composes its own from the same inputs.
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
					// Stored under the design's project when it has one. Resolved here rather than in
					// CreateReview, since a second resolution could disagree and file the run under a
					// project other than the one whose rules scored it.
					Parent:   parent,
					Manifest: service.ManifestProto(man), DesignUri: designURI, BoardUri: boardURI, RatifiedFloor: ratifiedFloor,
					AsNamed: readAsNamed,
					// --conventions rides the REQUEST as a value and the service composes it (WS3-102).
					Overlay: overlay,
				})
				if err != nil {
					return err
				}
				docs = append(docs, rv.GetResults())
				// The CLI's store lives as long as the command, so the name it assigned means nothing
				// once the command exits. The run is emitted unnamed, as `agni results` emits a document.
				reviews = append(reviews, service.ReviewOf("", rv.GetResults()))
			}
			// Map the stored documents back to the Go view-model, the CLI analogue of the web tier's
			// reportFromWire.
			reports := reportsFromDocs(docs)
			// --render bakes each design's findings into annotated schematic SVGs (WS7-043). The summary
			// goes to stderr so stdout stays a clean report.
			if renderDir != "" {
				summary, err := renderReviewImages(reports, designSourcesOf(docs), renderDir, companion)
				if err != nil {
					return err
				}
				fmt.Fprint(cmd.ErrOrStderr(), summary)
			}
			// --results-out writes the per-design review document (WS3-103) and renders FROM it, as
			// `check --results-out` does. A document is about one design, so a rollup is refused.
			if resultsOut != "" {
				if len(reports) != 1 {
					return fmt.Errorf("--results-out writes one design's document; got %d designs (run it per design)", len(reports))
				}
				// The service already built the whole document, snapshot and catalog included, so this
				// writes what the run recorded rather than assembling a second one (WS9-053).
				doc := docs[0]
				if err := writeResults(resultsOut, doc); err != nil {
					return err
				}
				if err := renderReviewResults(cmd.OutOrStdout(), "", doc, format, coverage); err != nil {
					return err
				}
				// The gate applies on this path too, which is the one a CI pipeline archiving its
				// results document takes (agni issue 199).
				return gateReview(cmd, gate, reports)
			}
			// --coverage is the rollup shortcut and --format picks the surface. JSON carries the FULL
			// finding list per item, where markdown caps the Detail cell.
			var out string
			if len(reports) == 1 {
				rep := reports[0]
				switch {
				case coverage:
					out = review.RenderCoverageMarkdown(rep)
				case format == "json":
					// The Review the rpc returned, summary included (C31, agni issue 734).
					out, err = protoJSON(reviews[0])
				case format == "csv":
					// The review table the TableService projects, one row per item (agni issue 862).
					return writeTableCSV(cmd.OutOrStdout(), service.ReviewTables(reviews[0])[0], orderBy)
				case format == "html":
					// The checklist page, items in the manifest's order with one row per question and
					// every finding per item (markdown caps the Detail cell at three).
					meta, err := checklistMeta(cmd, ll, args[0], srvSpec)
					if err != nil {
						return err
					}
					return rpt.ChecklistHTML(cmd.OutOrStdout(), buildChecklist(rep, meta))
				case format == "" || format == "markdown":
					out = review.RenderMarkdown(rep)
				default:
					return fmt.Errorf("review: unknown --format %q (want markdown, json, csv or html)", format)
				}
			} else {
				agg := review.Aggregate{Manifest: man.Name, Reports: reports}
				switch {
				case coverage:
					out = review.RenderAggregateCoverageMarkdown(agg)
				case format == "json":
					// One Review per design, as ListReviews answers. The rollup the markdown draws is
					// derivable from it, so it has no message of its own.
					out, err = protoJSON(&webapi.ListReviewsResponse{Reviews: reviews})
				case format == "" || format == "markdown":
					out = review.RenderAggregateMarkdown(agg)
				case format == "html":
					// One page addresses one design, since its title, content hash and every link name
					// that design.
					return fmt.Errorf("review --format html takes one design (got %d); run it once per design", len(args))
				case format == "csv":
					// A csv is one table, and a rollup is one per design.
					return fmt.Errorf("review --format csv takes one design (got %d); run it once per design", len(args))
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
	cmd.Flags().StringVar(&checklist, "checklist", "", "review manifest (YAML, or - for stdin) declaring review areas and their checklist items")
	cmd.Flags().StringArrayVar(&reviewLibs, "lib", nil, "a directory of derived-relation modules (<module.path>.dl, optional docs/<member.path>.md) sent with the run, beside any the design's project carries, so a checklist's inline query can call them. Repeatable")
	cmd.Flags().StringVar(&paramsDir, "params", "", "directory of seeded PartSpec textprotos; enables datasheet-backed rules")
	cmd.Flags().StringVar(&profilePath, "profile-path", "", "directory of YAML interface-profile declarations added to the catalog")
	cmd.Flags().StringVar(&intentPath, "intent-path", "", "a YAML design-intent declaration (expected modules, voltage domains); its rules join the catalog so intent-bound items resolve")
	cmd.Flags().StringVar(&conventions, "conventions", "", "an operator naming-convention config (YAML); its rules join the catalog namespaced as <config name>/<rule name>, and its lexicon teaches the run which net names are this project's power rails, grounds, and feedback nodes")
	cmd.Flags().StringVar(&boardPath, "board-path", "", "a board-geometry file (.kicad_pcb / IPC-2581 .xml|.cvg) attached to the netlist design so board-tier DRC items resolve pass/fail instead of not-applicable")
	cmd.Flags().BoolVar(&coverage, "coverage", false, "emit a per-area coverage rollup (covered/pass/fail/provisional/needs-intent/needs-data/computed-n-a/n-a/not-automated) instead of the per-item report. Markdown only, so it refuses an explicit --format; the --format html page already carries the same rollup in its header")
	cmd.Flags().Float64Var(&ratifiedFloor, "ratified-floor", 0, "datasheet-confidence floor for a trustworthy finding; a fail whose findings are all mock or below this is 'provisional'. 0 uses the default (0.9)")
	cmd.Flags().StringVar(&format, "format", "markdown", "per-item report format: markdown (Detail cell capped), json (full findings, for tooling), html (the checklist as a self-contained page, every finding per item), or csv (one row per item, the table the TableService projects)")
	cmd.Flags().StringSliceVar(&orderBy, "order-by", nil, "order the csv rows by these columns, comma-separated, a leading - for descending: --order-by=outcome,id. csv only (agni issue 862)")
	serverFlag(cmd, &serverVal)
	withSelfServer(cmd, &srvSpec)
	outFileFlag(cmd, &reviewOutPath)
	cmd.Flags().StringVar(&renderDir, "render", "", "also write an annotated schematic SVG per design (each finding highlighted in place) to <dir>/<design-stem>/<sheet>.svg")
	cmd.Flags().StringVar(&companion, "companion", "", "geometry file (.eds) to draw the --render images on, joined to the netlist findings by net name; with one design only (else a sibling <stem>.eds is auto-detected per design)")
	cmd.Flags().StringVar(&resultsOut, "results-out", "", "also write the run as a self-contained check-result document (JSON) at this path; one design only. Render it later with `agni results`")
	cmd.Flags().StringVar(&failOnOutcome, "fail-on-outcome", "", "exit non-zero when any checklist ITEM sits at one of these outcomes (comma-separated, e.g. fail or fail,provisional). This is the coverage axis, not `check --fail-on`'s severity axis: it asks whether a question was answered, not how bad the answer was. Off by default")
	cmd.Flags().IntVar(&minAnswered, "min-answered", 0, "exit non-zero when fewer than N checklist items produced an ANSWER (pass, fail, provisional, or computed-n/a). Distinct from the covered count, which still counts an item whose rule is present but whose inputs are absent; that is the regression a severity gate cannot see. Off by default")
	return cmd
}

// coverageShadowsFormat reports whether --coverage would silently throw away a --format the caller
// asked for.
//
// --coverage emits the per-area rollup as markdown alone, and the dispatch tests it FIRST, so any
// other --format would be dropped. A coverage JSON would need its own wire message (C31), and the
// per-item html page already carries the rollup in its header band.
//
// It asks Changed rather than comparing against the default, because --format defaults to markdown
// on review and to "" on results.
func coverageShadowsFormat(cmd *cobra.Command, coverage bool) bool {
	if !coverage || !cmd.Flags().Changed("format") {
		return false
	}
	return cmd.Flags().Lookup("format").Value.String() != "markdown"
}

// gateReview applies the CI gate after the report has been rendered, so a tripped pipeline still gets
// its full report on stdout rather than only an error on stderr.
//
// SilenceUsage matches `check --fail-on`, since a gate firing is the command working as asked and
// usage text under it reads as a mistyped invocation.
func gateReview(cmd *cobra.Command, g reviewGate, reports []review.Report) error {
	if err := g.trip(reports); err != nil {
		cmd.SilenceUsage = true
		return err
	}
	return nil
}

// composeReviewInputs builds the design-INDEPENDENT review inputs shared by the CLI (reviewCmd) and
// the served ReviewService. That is the rule catalog with the profiles at profilePath spliced in, and
// the by-Name profile index the presence check reads. Intent is per design, so it arrives on the
// request rather than here.
func composeReviewInputs(profilePath string) (*check.Catalog, map[string][]profiles.Profile, error) {
	overlay, err := loadOverlayProfiles(profilePath)
	if err != nil {
		return nil, nil, err
	}
	return composeReviewInputsFrom(overlay)
}

// loadOverlayProfiles reads the overlay interface profiles at path, or nil for an empty path. serve
// feeds several catalogs from one --profile-path and reads the directory once through this.
func loadOverlayProfiles(profilePath string) ([]profiles.Profile, error) {
	if profilePath == "" {
		return nil, nil
	}
	return profiles.LoadDir(profilePath)
}

// composeReviewInputsFrom is composeReviewInputs over profiles that are already loaded, plus any
// extra sources the caller composed itself, with agni.New composing the catalog (C22).
//
// extra carries serve's --conventions, a startup DEPLOYMENT default rather than a per-request value
// (WS3-109). Compose through here rather than a second CatalogWith, which would drop the profile
// sources.
func composeReviewInputsFrom(overlay []profiles.Profile, extra ...check.RuleSource) (*check.Catalog, map[string][]profiles.Profile, error) {
	e, err := newEngine(overlay, extra)
	if err != nil {
		return nil, nil, err
	}
	return e.Catalog(), e.ProfileIndex(), nil
}

// newEngine composes the engine the CLI runs on. Every command that runs rules goes through it, so
// agni.New checks the four registration hooks once.
func newEngine(overlay []profiles.Profile, extra []check.RuleSource, more ...agni.Option) (*agni.Engine, error) {
	opts := []agni.Option{
		agni.WithProfiles(overlay),
		agni.WithSources(extra...),
		agni.WithProducerVersion(version.Version()),
	}
	opts = append(opts, more...)
	return agni.New(opts...)
}

func diffCmd() *cobra.Command {
	var format string
	var orderBy []string
	var renameApprox, includeEqual bool
	c := &cobra.Command{
		Use:   "diff <old> <new>",
		Short: "Structural diff between two revisions of a design (over the IR)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// json is the message DiffDesigns returns, so it goes through the service (C31, agni
			// issues 737 and 817): the CLI is a client of the rpc rather than a second composition
			// of it, and the sheet and placement maps the service fills are in its output too.
			if len(orderBy) > 0 && format != "csv" {
				return fmt.Errorf("--order-by orders a csv table; --format %s has none", format)
			}
			if format == "json" {
				return diffViaService(cmd, args[0], args[1], renameApprox, includeEqual)
			}
			// csv is the diff table the TableService projects from the same response (agni 862).
			if format == "csv" {
				resp, err := diffResponse(cmd, args[0], args[1], renameApprox, includeEqual)
				if err != nil {
					return err
				}
				return writeDiffCSV(cmd.OutOrStdout(), resp, orderBy)
			}
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
			fmt.Fprintf(w, "diff %s -> %s\n\n", args[0], args[1])
			fmt.Fprint(w, rep.Render(diffListLimit))
			if includeEqual {
				fmt.Fprintf(w, "\nUnchanged nets: %d\n", len(rep.Equal))
			}
			return nil
		},
		// Validated ahead of the read, so a misspelled format fails before two designs are parsed
		// rather than falling through to text (agni issue 372).
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
	c.Flags().StringSliceVar(&orderBy, "order-by", nil, "order the csv rows by these columns, comma-separated, a leading - for descending: --order-by=change_class,subject. A column sorts by its type, so subject puts R2 before R10. csv only (agni issue 862)")
	c.Flags().BoolVar(&includeEqual, "include-equal", false,
		"also report the nets that did not change: as kind equal in --format json, as net-equal rows in csv, and as a count in text. Off by default")
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
			// Resolve the format BEFORE reading, so an unknown one fails before out is created and
			// truncated.
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

// The formats emit can write.
const (
	emitEDIF    = "edif"
	emitIPC2581 = "ipc2581"
)

// emitFormat resolves which format an emit run writes. It returns the format's name rather than a
// writer, since a writer would put a raw *ir.Design in this signature and C19 reserves that for a
// producer or an entry point.
//
// A named format wins outright. Otherwise the OUT file's extension decides, as on the read side
// (readers/formats), and stdout falls back to IPC-2581.
//
// .eds is refused. It carries a netlist AND the schematic geometry, and there is no schematic writer,
// so the netlist writer alone would produce a file claiming a drawing it lacks.
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
