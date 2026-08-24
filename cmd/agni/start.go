package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/readers/formats"
)

// startCmd scaffolds a project around an existing design file.
//
// It is a CLI-side scaffolder and deliberately NOT a service rpc. ProjectService is read-only on
// purpose (C23's declared-identity case): a server mutating an operator's mount raises ownership,
// concurrency and provenance questions that nothing here needs answered. Writing two descriptors into
// a folder the operator named on the command line is a different act, and it stays on this side of
// that line.
func startCmd() *cobra.Command {
	var name, title string
	c := &cobra.Command{
		Use:   "start <design-file> [dir]",
		Short: "Scaffold a review project around an existing design file",
		Long: "Create a project from one design file, so every other command can stop taking flags. It " +
			"writes a project.yaml, a design.yaml naming the design and any companion views of it, a " +
			"conventions.yaml stub, and a review.yaml seeded from the shipped rule catalog — then copies " +
			"the design and its companions in. After it, `agni check <dir>` and `agni review <dir>` " +
			"resolve the project's config with no flags at all.\n\n" +
			"[dir] defaults to the current directory. Pointing it at a folder that already holds a " +
			"project.yaml ADDS the design to that project instead of creating a second one. Nothing is " +
			"overwritten: an existing file stops the command and is named.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			design := args[0]
			dir := "."
			if len(args) == 2 {
				dir = args[1]
			}
			return runStart(cmd.OutOrStdout(), design, dir, name, title)
		},
	}
	c.Flags().StringVar(&name, "name", "", "the project's declared id; defaults to the target directory's name (lowercased, with anything outside [a-z0-9._-] replaced by -)")
	c.Flags().StringVar(&title, "title", "", "the project's human-readable label; defaults to the id")
	return c
}

// idUnsafe matches every run of characters an AIP resource id may not carry. See projects.WriteProject
// for why the id shape is narrow: it becomes a path segment in "projects/{p}/designs/{d}".
var idUnsafe = regexp.MustCompile(`[^a-z0-9._-]+`)

// deriveID turns a folder or file name into a legal resource id, or "" when nothing usable survives.
//
// It sanitizes rather than rejecting, because the common case is a folder named "Sample Board" and
// failing on it would send an operator to --name to type almost the same string back. The result is
// printed, so a silent rename is not silent.
func deriveID(s string) string {
	id := idUnsafe.ReplaceAllString(strings.ToLower(s), "-")
	id = strings.Trim(id, "-._")
	if id == "" || !('a' <= id[0] && id[0] <= 'z' || '0' <= id[0] && id[0] <= '9') {
		return ""
	}
	return id
}

// companionsOf returns the sibling files that are VIEWS of the named design: same stem, different
// extension, and carrying geometry or a board.
//
// The two halves of that rule each prevent a specific wrong answer. Same-stem is what keeps a later
// revision out: `gateway-rev-b.edn` beside `gateway.edn` is a legitimate analysis source in its own
// right, and declaring it a companion would turn a diff of two revisions into a diff of one against
// itself — which is exactly why the descriptor format declares companions file by file rather than
// inferring them from "everything beside the entry".
//
// Carrying geometry or a board is what keeps a second ENCODING out. A `.edf` beside a `.edn` is
// another netlist, not a view; declaring it would be inert at best and misleading at worst. The test
// is asked of the format registry rather than of a list of extensions here, so a reader added later
// is classified by what it can do.
func companionsOf(designPath string) ([]string, error) {
	dir := filepath.Dir(designPath)
	base := filepath.Base(designPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || e.Name() == base {
			continue
		}
		n := e.Name()
		if strings.TrimSuffix(n, filepath.Ext(n)) != stem {
			continue
		}
		if formats.HasFaithful(n) || formats.HasBoard(n) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out, nil
}

// runStart does the work, taking every input explicitly so it is testable without a cobra command.
func runStart(w io.Writer, designPath, dir, name, title string) error {
	if !formats.HasNetlist(designPath) {
		return fmt.Errorf("%s carries no netlist, so it cannot be a design's entry (netlist formats: %s)",
			designPath, strings.Join(formats.NetlistExts(), ", "))
	}
	if _, err := os.Stat(designPath); err != nil {
		return err
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	companions, err := companionsOf(designPath)
	if err != nil {
		return err
	}

	base := filepath.Base(designPath)
	designID := deriveID(strings.TrimSuffix(base, filepath.Ext(base)))
	if designID == "" {
		return fmt.Errorf("cannot derive a design id from %q; rename the file to start with a letter or digit", base)
	}

	// An existing project.yaml means ADD a design rather than create a second project. Scaffolding a
	// nested project would give one tree two config scopes, and the design would resolve to whichever
	// the store walked into first.
	existing, adopted := existingProjectID(root)
	projectID := name
	if projectID == "" {
		projectID = existing
	}
	if projectID == "" {
		projectID = deriveID(filepath.Base(root))
	}
	if projectID == "" {
		return fmt.Errorf("cannot derive a project id from %q; pass --name", root)
	}
	if adopted && name != "" && name != existing {
		return fmt.Errorf("%s already declares the project %q; --name %q would rename it, which this command will not do",
			filepath.Join(root, projects.ProjectDescriptor), existing, name)
	}
	if title == "" {
		title = projectID
	}

	designDir := filepath.Join(root, "designs", designID)
	// Every write is checked BEFORE any write happens, so a refusal leaves nothing half-created.
	planned := []string{filepath.Join(designDir, projects.DesignDescriptor), filepath.Join(designDir, base)}
	for _, c := range companions {
		planned = append(planned, filepath.Join(designDir, c))
	}
	if !adopted {
		planned = append(planned,
			filepath.Join(root, projects.ProjectDescriptor),
			filepath.Join(root, "conventions.yaml"),
			filepath.Join(root, "review.yaml"))
	}
	for _, p := range planned {
		if _, err := os.Stat(p); err == nil {
			rel, _ := filepath.Rel(root, p)
			return fmt.Errorf("%s already exists; agni start will not overwrite it", rel)
		}
	}

	if err := os.MkdirAll(designDir, 0o755); err != nil {
		return err
	}
	if !adopted {
		if err := writeDescriptorFile(filepath.Join(root, projects.ProjectDescriptor), func(f io.Writer) error {
			return projects.WriteProject(f, projectHeader, projectID, title, nil)
		}); err != nil {
			return err
		}
		if err := writeStub(filepath.Join(root, "conventions.yaml"), conventionsStub(projectID)); err != nil {
			return err
		}
		if err := writeStub(filepath.Join(root, "review.yaml"), seededChecklist(title)); err != nil {
			return err
		}
	}

	// Copy before writing the descriptor, so a descriptor never names a file that is not there yet.
	origin, _ := filepath.Abs(designPath)
	if err := copyFile(designPath, filepath.Join(designDir, base)); err != nil {
		return err
	}
	srcDir := filepath.Dir(designPath)
	for _, c := range companions {
		if err := copyFile(filepath.Join(srcDir, c), filepath.Join(designDir, c)); err != nil {
			return err
		}
	}
	if err := writeDescriptorFile(filepath.Join(designDir, projects.DesignDescriptor), func(f io.Writer) error {
		return projects.WriteDesign(f, designHeader(origin), designID, designID, base, companions)
	}); err != nil {
		return err
	}

	report(w, root, projectID, designID, base, companions, adopted)
	return nil
}

// existingProjectID returns the id declared by a project.yaml at root, and whether there was one.
func existingProjectID(root string) (string, bool) {
	f, err := os.Open(filepath.Join(root, projects.ProjectDescriptor))
	if err != nil {
		return "", false
	}
	defer f.Close()
	id, _, _, err := projects.ParseProject(f)
	if err != nil {
		return "", false
	}
	return id, true
}

func writeDescriptorFile(path string, emit func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return emit(f)
}

func writeStub(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

// report tells the operator what was created and, load-bearingly, what was COPIED and from where.
//
// The copy is the one thing about this command that can surprise someone later: the project owns its
// own copy of the design from here on, so editing the original leaves the project checking a stale
// file with nothing to say so.
func report(w io.Writer, root, projectID, designID, entry string, companions []string, adopted bool) {
	rel := func(p ...string) string {
		return filepath.Join(append([]string{filepath.Base(root)}, p...)...)
	}
	if adopted {
		fmt.Fprintf(w, "Added design %q to the existing project %q.\n\n", designID, projectID)
	} else {
		fmt.Fprintf(w, "Created project %q.\n\n", projectID)
		fmt.Fprintf(w, "  %s\n", rel(projects.ProjectDescriptor))
		fmt.Fprintf(w, "  %s        (stub — your team's naming vocabulary)\n", rel("conventions.yaml"))
		fmt.Fprintf(w, "  %s             (seeded from the shipped catalog — edit it)\n", rel("review.yaml"))
	}
	fmt.Fprintf(w, "  %s\n", rel("designs", designID, projects.DesignDescriptor))
	fmt.Fprintf(w, "  %s   (copied)\n", rel("designs", designID, entry))
	for _, c := range companions {
		fmt.Fprintf(w, "  %s   (copied, declared as a companion view)\n", rel("designs", designID, c))
	}
	fmt.Fprintf(w, "\nThe project owns these copies. Edits to the original files do not reach it.\n")
	fmt.Fprintf(w, "\nNext:\n  agni check %s\n  agni review %s\n",
		filepath.Join(filepath.Base(root), "designs", designID),
		filepath.Join(filepath.Base(root), "designs", designID))
}

const projectHeader = `Generated by ` + "`agni start`" + `. Edit freely.

What makes this folder a project is this file: a declared name, and the fact that the config
beside it belongs to every design under designs/ rather than to any one of them.

The config files are found by their conventional names (conventions.yaml, profiles/, params/,
review.yaml), so none of them is declared here. Declare one only to depart from that: a
conventions file shared with another project, a differently-named checklist, or "" to opt out.`

// designHeader records where the copied files came from. It is a COMMENT rather than a field because
// Design has no place for provenance, and adding one to carry a scaffolder's note would be a schema
// change made for a message.
func designHeader(origin string) string {
	return `Generated by ` + "`agni start`" + ` from ` + origin + `.

entry names the file ANALYSIS reads. companions are views of that same design — a schematic to
draw it, a board to locate findings on — never a second source of components to reconcile
against the entry.

Companions are listed file by file on purpose. A later revision of the entry sits in the same
folder and is a legitimate analysis source of its own, so anything inferred from "everything
beside the entry" would turn a diff of two revisions into a diff of one against itself.`
}

// conventionsStub is a VALID naming config, not an empty file. A project declaring a config tier that
// fails to load is an error rather than a skip (osProjectConfig), so a stub that did not parse would
// make every command on the scaffolded project fail at the config it was supposed to give them.
func conventionsStub(projectID string) string {
	return `# Your team's naming vocabulary, composed into every run on this project.
#
# It carries two halves, wired differently:
#   lexicon: teaches the engine which net names are this project's rails, grounds and supply pins.
#            It reaches the design READ, so every rule sees the roles it implies.
#   rules:   adds catalog rules, namespaced ` + projectID + `/<rule name>.
#
# Both are optional. As written this declares a name and nothing else, which changes nothing.
name: ` + projectID + `
#
# lexicon:
#   net:
#     rail:
#       patterns: ["_[0-9]V[0-9]$"]
#   pin:
#     supply:
#       patterns: ["^PWRIN"]
#
# rules:
#   - name: signal-net-naming
#     severity: warning
#     why: "signal nets are UPPER_SNAKE"
#     allow: ["^[A-Z][A-Z0-9_]*$"]
`
}

// checklistAreas is the seeded checklist's shape: one area per rule CATEGORY the shipped catalog
// carries, so a first run answers something real rather than reporting an empty checklist.
//
// A tag binding is used rather than a rule binding on purpose. It survives the catalog growing: a rule
// added to a category later joins the item that already covers it, where an item naming one rule would
// silently stay as narrow as the day it was written.
var checklistAreas = []struct {
	category, area, title string
}{
	{"power", "Power", "the power tree holds up under the shipped power rules"},
	{"connectivity", "Connectivity", "nothing is left unconnected or wrongly driven"},
	{"integrity", "Signal integrity", "signals are terminated and referenced"},
	{"datasheet", "Part limits", "no part is operated outside its datasheet limits"},
	{"board", "Board", "the layout meets the fabrication floor"},
	{"naming", "House style", "names follow this project's conventions"},
}

// seededChecklist renders the starter manifest, keeping only the categories the running catalog
// actually has. Emitting an item for a category with no rules would score not-automated forever and
// teach a new reader that the outcome means something is broken.
func seededChecklist(title string) string {
	have := map[string]bool{}
	for _, r := range check.DefaultCatalog().Rules() {
		have[r.Tags["category"]] = true
	}
	var b strings.Builder
	b.WriteString("# Generated by `agni start`, seeded from the shipped rule catalog. Edit freely.\n")
	b.WriteString("#\n")
	b.WriteString("# This is your team's review checklist: the questions you ask of every board, bound to\n")
	b.WriteString("# the rules that can answer them. Items nothing can answer still belong here — give them\n")
	b.WriteString("# a `note:` and they stay visible as work for a human instead of falling off the list.\n")
	fmt.Fprintf(&b, "name: %s review\nareas:\n", title)
	n := 0
	for _, a := range checklistAreas {
		if !have[a.category] {
			continue
		}
		n++
		fmt.Fprintf(&b, "  - name: %s\n    items:\n", a.area)
		fmt.Fprintf(&b, "      - {id: %q, title: %s, tag: \"category=%s\"}\n", fmt.Sprintf("%s%d", strings.ToUpper(a.category[:1]), n), a.title, a.category)
	}
	b.WriteString("  - name: Manual\n    items:\n")
	b.WriteString("      - {id: \"M1\", title: the assembly drawing lists a torque spec for every fastener,\n")
	b.WriteString("         note: manual review against the mechanical drawing package}\n")
	return b.String()
}
