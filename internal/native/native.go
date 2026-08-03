// Package native shells out to a format's own CLI to produce a golden reference render
// (SVG), to validate the WebGL/SVG paths against. This is a deliberate external-process
// surface, so it is gated three ways: a tool must be registered for the file's extension,
// its name must be explicitly allowlisted by the operator (--enable-native <tool>), and its
// binary must be on PATH. Tools are invoked with exec (no shell), on an already-mount-safe
// absolute path, into a temp output dir, under a timeout.
package native

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/panyam/agni/readers/geda"
	"github.com/panyam/agni/readers/xschem"
)

// Sentinel errors, mapped to Connect codes by the caller.
var (
	ErrNoTool     = errors.New("no native renderer for this format")
	ErrNotEnabled = errors.New("native renderer not enabled")
	ErrNotFound   = errors.New("native renderer binary not found on PATH")
)

// nativeTimeout bounds a single external render; large schematics are the reason native is
// slow, but a hung tool must not wedge the server.
const nativeTimeout = 60 * time.Second

// nativeRenderer describes one external tool that renders a file's format to SVG.
type nativeRenderer struct {
	tool string // allowlist key the operator enables (also the binary looked up on PATH)
	// args builds the command arguments (after the binary) to write SVG(s) into outDir for
	// the given 1-based page of absPath.
	args func(absPath, outDir string, page int) []string
}

// nativeByExt maps a lowercase extension to its native renderer via kicad-cli: KiCad
// schematics (per page) and boards (a fixed overview layer set). Note the two are different
// views — the viewer draws a board only as a netlist auto-layout, so NATIVE on a .kicad_pcb
// is the real board rather than a like-for-like of the WebGL/SVG grid. EDIF (.eds) has no
// open native CLI, so it is absent. The shared .sch extension is not here: it is resolved by
// nativeRendererFor, which sniffs the header to pick xschem vs gEDA.
var nativeByExt = map[string]nativeRenderer{
	".kicad_sch": kicadSch,
	".kicad_pro": kicadSch,
	".kicad_pcb": kicadPcb,
}

// xschemNative exports an xschem schematic to SVG headlessly (--no_x). xschem writes the SVG as
// plot.svg in the process working directory, which runRender sets to outDir, so exactly one SVG
// lands there. It renders the whole sheet (no page selection), so page is ignored. Symbol
// resolution uses the tool's own XSCHEM_LIBRARY_PATH environment.
var xschemNative = nativeRenderer{
	tool: "xschem",
	args: func(absPath, _ string, _ int) []string {
		return []string{"--no_x", "--quit", "--svg", absPath}
	},
}

// gedaNative exports a gEDA gschem schematic to SVG via Lepton EDA's lepton-cli, which infers
// the format from the .svg output extension. It renders the whole sheet, so page is ignored.
var gedaNative = nativeRenderer{
	tool: "lepton-cli",
	args: func(absPath, outDir string, _ int) []string {
		return []string{"export", "-o", filepath.Join(outDir, "sheet.svg"), absPath}
	},
}

// nativeRendererFor resolves the native renderer for a file. Most formats key off the
// extension; the shared .sch extension is disambiguated by sniffing the header (xschem opens
// with "v {xschem", gEDA with "v <date>"), the same rule readDesign uses. A .sch whose header
// matches neither, or an unreadable file, has no native renderer.
func nativeRendererFor(absPath string) (nativeRenderer, bool) {
	if lowerExt(absPath) == ".sch" {
		head, err := readHead(absPath, 256)
		if err != nil {
			return nativeRenderer{}, false
		}
		switch {
		case xschem.IsXschem(head):
			return xschemNative, true
		case geda.IsGeda(head):
			return gedaNative, true
		default:
			return nativeRenderer{}, false
		}
	}
	r, ok := nativeByExt[lowerExt(absPath)]
	return r, ok
}

// readHead reads up to n leading bytes of a file for format sniffing.
func readHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	m, err := f.Read(buf)
	if err != nil && m == 0 {
		return nil, err
	}
	return buf[:m], nil
}

// pcbLayers is the overview layer set for a native board render. Kept minimal and
// version-stable (copper + board outline); richer layers are a later refinement.
const pcbLayers = "F.Cu,B.Cu,Edge.Cuts"

var kicadSch = nativeRenderer{
	tool: "kicad-cli",
	args: func(absPath, outDir string, page int) []string {
		return []string{"sch", "export", "svg", "--pages", strconv.Itoa(page), "--output", outDir, absPath}
	},
}

var kicadPcb = nativeRenderer{
	tool: "kicad-cli",
	// A board is one drawing (no page selection). --mode-single writes a single combined SVG,
	// and in that mode --output is the full file path, so point it at a file inside outDir.
	args: func(absPath, outDir string, _ int) []string {
		return []string{"pcb", "export", "svg", "--mode-single", "--layers", pcbLayers, "--output", filepath.Join(outDir, "board.svg"), absPath}
	},
}

// Available reports whether NATIVE can be served for absPath: a renderer is registered
// (by extension, or by sniffing a .sch), its tool is enabled, and its binary is installed.
func Available(absPath string, enabled map[string]bool) bool {
	r, ok := nativeRendererFor(absPath)
	if !ok || !enabled[r.tool] {
		return false
	}
	_, err := exec.LookPath(r.tool)
	return err == nil
}

// Cache memoizes rendered SVGs by (path, mtime, page) so navigating sheets does not
// re-shell for an unchanged file.
type Cache struct {
	mu sync.Mutex
	m  map[string]string
}

// NewCache returns an empty render cache; one per server.
func NewCache() *Cache { return &Cache{m: map[string]string{}} }

// Render renders one page of absPath to SVG using its registered, enabled tool. page is
// 1-based. It returns ErrNoTool / ErrNotEnabled / ErrNotFound for the gate failures so the
// caller can map them to Connect codes.
func (c *Cache) Render(ctx context.Context, absPath string, page int, enabled map[string]bool) (string, error) {
	r, ok := nativeRendererFor(absPath)
	if !ok {
		return "", ErrNoTool
	}
	if !enabled[r.tool] {
		return "", ErrNotEnabled
	}
	bin, err := exec.LookPath(r.tool)
	if err != nil {
		return "", ErrNotFound
	}

	key, ok := cacheKey(absPath, page)
	if ok {
		c.mu.Lock()
		svg, hit := c.m[key]
		c.mu.Unlock()
		if hit {
			return svg, nil
		}
	}

	svg, err := runRender(ctx, r, bin, absPath, page)
	if err != nil {
		return "", err
	}
	if ok {
		c.mu.Lock()
		c.m[key] = svg
		c.mu.Unlock()
	}
	return svg, nil
}

// RenderFile renders one page (1-based) of absPath to SVG using its registered native tool,
// for direct CLI use. Unlike Cache.Render it does NOT consult the operator allowlist: the
// allowlist is a server guard against a shared deployment shelling out on a request, whereas
// invoking the CLI is itself the operator's consent. The other two gates still apply, so it
// returns ErrNoTool (no renderer for this format) or ErrNotFound (binary not on PATH). No
// caching — a one-shot CLI render has nothing to reuse.
func RenderFile(ctx context.Context, absPath string, page int) (string, error) {
	r, ok := nativeRendererFor(absPath)
	if !ok {
		return "", ErrNoTool
	}
	bin, err := exec.LookPath(r.tool)
	if err != nil {
		return "", ErrNotFound
	}
	return runRender(ctx, r, bin, absPath, page)
}

// runRender execs one native render into a fresh temp dir and returns the single SVG it wrote.
func runRender(ctx context.Context, r nativeRenderer, bin, absPath string, page int) (string, error) {
	outDir, err := os.MkdirTemp("", "agni-native-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(outDir)

	ctx, cancel := context.WithTimeout(ctx, nativeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, r.args(absPath, outDir, page)...)
	// Run in outDir so a tool that writes to its working directory (xschem emits plot.svg
	// there) lands its output where readOnlySVG looks; tools that take an explicit --output
	// path (kicad-cli, lepton-cli) are unaffected.
	cmd.Dir = outDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s: %w: %s", r.tool, err, string(out))
	}
	return readOnlySVG(outDir)
}

// guiToolFor returns the GUI application (macOS app name) or binary that natively edits
// absPath, or ("", false). KiCad opens all of its file types; the shared .sch extension is
// sniffed for xschem vs Lepton the same way nativeRendererFor does, but resolves to the GUI
// binary (lepton-schematic, not the lepton-cli exporter).
func guiToolFor(absPath string) (string, bool) {
	switch lowerExt(absPath) {
	case ".kicad_sch", ".kicad_pro", ".kicad_pcb":
		return "KiCad", true
	case ".sch":
		head, err := readHead(absPath, 256)
		if err != nil {
			return "", false
		}
		switch {
		case xschem.IsXschem(head):
			return "xschem", true
		case geda.IsGeda(head):
			return "lepton-schematic", true
		}
	}
	return "", false
}

// OpenArgs returns the command (binary + args) that opens absPath in its native GUI tool on
// the current platform, or ErrNoTool when no native GUI is known for the format/platform. It
// launches nothing — the caller execs the result, so the command is deterministic and unit
// testable. On macOS the KiCad app is launched via `open -a`; elsewhere, and for the X11
// schematic editors, the binary is invoked directly (a missing binary surfaces at exec time).
func OpenArgs(absPath string) (bin string, args []string, err error) {
	tool, ok := guiToolFor(absPath)
	if !ok {
		return "", nil, ErrNoTool
	}
	switch runtime.GOOS {
	case "darwin":
		if tool == "KiCad" {
			return "open", []string{"-a", "KiCad", absPath}, nil
		}
		return tool, []string{absPath}, nil
	case "linux":
		bin := tool
		if tool == "KiCad" {
			bin = "kicad"
		}
		return bin, []string{absPath}, nil
	default:
		return "", nil, ErrNoTool
	}
}

// readOnlySVG returns the single .svg the tool wrote into dir. Native renders one page into a
// fresh dir, so exactly one file is expected; anything else is an error worth surfacing.
func readOnlySVG(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var svgs []string
	for _, e := range entries {
		if !e.IsDir() && lowerExt(e.Name()) == ".svg" {
			svgs = append(svgs, e.Name())
		}
	}
	if len(svgs) != 1 {
		return "", fmt.Errorf("native render produced %d svg files, want 1", len(svgs))
	}
	b, err := os.ReadFile(filepath.Join(dir, svgs[0]))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// cacheKey is path|mtime|page; ok is false when the file cannot be stat'd (then skip caching).
func cacheKey(absPath string, page int) (string, bool) {
	fi, err := os.Stat(absPath)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("%s|%d|%d", absPath, fi.ModTime().UnixNano(), page), true
}

func lowerExt(name string) string {
	return strings.ToLower(filepath.Ext(name))
}
