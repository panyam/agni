package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
)

// cliMountSpecs holds the --mount flag values. It is a root PERSISTENT flag, so the same
// `name=path` form works on every subcommand, serve included.
var cliMountSpecs []string

// maxProjectWalk bounds how far above a named file the CLI looks for a project descriptor when it
// has to mint a mount. It stops short of the filesystem root so a stray `project.yaml` far up
// someone's home directory never becomes the authority a stored review is recorded under.
const maxProjectWalk = 4

// cliWorkspace is the CLI's mount table. A server is handed its mounts by an operator and the CLI
// is handed a PATH, so this turns each argument into an artifact URI whose authority is a mount
// this table can resolve.
//
// Three tiers, most explicit first:
//
//  1. An argument that already IS a URI is taken as written. Its authority must be a mount declared
//     with --mount, so a URI can say which mount you mean but never name a place no mount covers.
//  2. A path inside a DECLARED mount is addressed through it. Point the CLI at the same --mount a
//     server uses and the two produce identical URIs for the same design, so a stored review
//     created either way is directly comparable.
//  3. A path outside every declared mount gets a mount MINTED for it, rooted at the enclosing
//     project when one resolves and at the file's own directory otherwise.
//
// Tier 3 keeps `agni check some/board.edn` working with no configuration. Rooting it at the project
// rather than the filesystem root keeps a host path out of the URI, because a URI carrying
// `/Users/<someone>` would be recorded verbatim into every review document the run produced.
type cliWorkspace struct {
	mu       sync.Mutex
	declared []mounts.Mount
	minted   []mounts.Mount
}

// newCLIWorkspace parses the --mount flags once. A malformed spec is an error here rather than at
// first use, so a typo fails before any design is read.
func newCLIWorkspace() (*cliWorkspace, error) {
	declared, err := mounts.Parse(cliMountSpecs)
	if err != nil {
		return nil, err
	}
	return &cliWorkspace{declared: declared}, nil
}

// Mounts returns every mount this run can resolve: the declared ones first, then any minted along
// the way. Declared wins a name collision, matching mounts.Merge, since what an operator typed is
// the more specific intent.
func (w *cliWorkspace) Mounts() []mounts.Mount {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append(append([]mounts.Mount{}, w.declared...), w.minted...)
}

// URI turns one command-line argument into an artifact URI, applying the three tiers above.
//
// A path that does not exist is NOT an error here, because the reader already produces the
// not-found message a user knows.
func (w *cliWorkspace) URI(arg string) (artifact.URI, error) {
	if strings.HasPrefix(arg, artifact.Scheme+"://") {
		u, err := artifact.Parse(arg)
		if err != nil {
			return artifact.URI{}, err
		}
		if _, ok := mounts.Find(w.Mounts(), u.Mount); !ok {
			return artifact.URI{}, fmt.Errorf("%s names mount %q, which was not declared; pass --mount %s=<path>", arg, u.Mount, u.Mount)
		}
		return u, nil
	}
	abs, err := filepath.Abs(arg)
	if err != nil {
		return artifact.URI{}, err
	}
	if u, ok := w.inDeclared(abs); ok {
		return u, nil
	}
	return w.mint(abs)
}

// Declared reports whether name is a mount the OPERATOR named, through --mount or an agni.yaml, as
// opposed to one this run minted for an argument no declared mount covered.
//
// A server started from the same table resolves a declared name to the same root, while a minted
// one means nothing outside this process. Callers that turn a URI into something an OTHER process
// will follow have to know which they are holding.
//
// It reads w.declared rather than w.Mounts(), which merges in the minted ones and would answer yes
// to both.
func (w *cliWorkspace) Declared(name string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, ok := mounts.Find(w.declared, name)
	return ok
}

// inDeclared addresses an absolute path through a declared mount that contains it. The longest root
// wins, so nested mounts resolve to the most specific one rather than to whichever was typed first.
func (w *cliWorkspace) inDeclared(abs string) (artifact.URI, bool) {
	var best mounts.Mount
	var bestRel string
	for _, m := range w.Mounts() {
		rel, err := filepath.Rel(m.Root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if best.Name == "" || len(m.Root) > len(best.Root) {
			best, bestRel = m, rel
		}
	}
	if best.Name == "" {
		return artifact.URI{}, false
	}
	u, err := artifact.New(best.Name, filepath.ToSlash(bestRel))
	if err != nil {
		return artifact.URI{}, false
	}
	return u, true
}

// mint creates a mount for a path that no declared mount covers, rooted at the enclosing project
// when one resolves and at the file's own directory otherwise, then addresses the path through it.
//
// Rooting at the PROJECT makes the URI mean something off this machine.
// `mount://gateway/designs/gateway/gateway.edn` says which design of which project, and reads the
// same as the URI a server with that project mounted would produce.
func (w *cliWorkspace) mint(abs string) (artifact.URI, error) {
	root, name, err := projectRootAbove(abs)
	if err != nil {
		return artifact.URI{}, err
	}
	if root == "" {
		root = filepath.Dir(abs)
		if fi, err := os.Stat(abs); err == nil && fi.IsDir() {
			root = abs
		}
		name = "local"
	}
	w.mu.Lock()
	name = w.uniqueNameLocked(name, root)
	w.mu.Unlock()

	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return artifact.URI{}, err
	}
	return artifact.New(name, filepath.ToSlash(rel))
}

// uniqueNameLocked returns the mount name to use for root, reusing an existing mount with the same
// root and otherwise suffixing until the name is free and recording the new mount as minted. The
// caller holds w.mu.
//
// The suffix covers diffing two designs in different trees that share a project id, where two roots
// under one authority would make the second design unaddressable.
func (w *cliWorkspace) uniqueNameLocked(want, root string) string {
	all := append(append([]mounts.Mount{}, w.declared...), w.minted...)
	for _, m := range all {
		if m.Root == root {
			return m.Name
		}
	}
	name := want
	for n := 2; ; n++ {
		taken := false
		for _, m := range all {
			if m.Name == name {
				taken = true
				break
			}
		}
		if !taken {
			break
		}
		name = fmt.Sprintf("%s%d", want, n)
	}
	w.minted = append(w.minted, mounts.Mount{Name: name, Root: root})
	return name
}

// projectRootAbove walks up from a path looking for a project descriptor, returning the folder
// holding it and the project's declared id. It returns ("", "", nil) when there is none within
// maxProjectWalk levels, and an error when one EXISTS and does not parse.
//
// The declared id becomes the mount NAME, so a design's URI is the same whether it was reached
// through the CLI or through a server.
//
// A parse error must never read as "no project here". This function decides where the mount is
// ROOTED, and answering "none" for a broken descriptor roots the mount at the design's own folder,
// leaving the descriptor OUTSIDE the mount where nothing downstream can see it. The run then
// composes against the built-in vocabulary and reports an authoritative-looking answer (agni issue
// 312; issue 306 measured 40 findings a project's own lexicon would not have raised and 95 it
// would have). See
// docsite/content/architecture/projects-and-designs.md#resolution-is-an-interface-not-a-path-convention.
func projectRootAbove(abs string) (root, id string, err error) {
	dir := abs
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		dir = filepath.Dir(abs)
	}
	for range maxProjectWalk + 1 {
		f, openErr := os.Open(filepath.Join(dir, projects.ProjectDescriptor))
		if openErr == nil {
			declared, _, _, parseErr := projects.ParseProject(f)
			f.Close()
			if parseErr != nil {
				// Name the DIRECTORY, since ParseProject already names the descriptor file and the
				// walk can pass several.
				return "", "", fmt.Errorf("%s: %w", dir, parseErr)
			}
			return dir, declared, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", "", nil
}

// cliWS is the workspace for this run, built once on first use.
//
// It is package-level like the flag variables, because a CLI process serves exactly one invocation
// and "the mounts for this run" is process state. Nothing mutates it after the first call and no
// request path reads it, which is the startup-default shape CONSTRAINTS C22 allows.
var (
	cliWSOnce sync.Once
	cliWSVal  *cliWorkspace
	cliWSErr  error
)

// workspace returns this run's mount table, parsing --mount the first time it is asked.
func workspace() (*cliWorkspace, error) {
	cliWSOnce.Do(func() { cliWSVal, cliWSErr = newCLIWorkspace() })
	return cliWSVal, cliWSErr
}

// cliArgURI turns a command-line argument into an artifact URI string for a request literal.
//
// A path that does not exist is not an error here, per cliWorkspace.URI. A failure to MINT (a
// governing project descriptor that does not parse) IS returned, because passing the raw argument
// through would leave the service no mount to resolve it against, and it would compose as though
// the design belonged to no project (agni issue 312).
func cliArgURI(arg string) (string, error) {
	if arg == "" {
		// An unsupplied optional flag stays unsupplied. Without this, filepath.Abs("") resolves to the
		// working directory and an absent --board-path arrives as a URI naming a real folder, which the
		// service then reads as "a board was supplied" and fails on a directory that is not one.
		return "", nil
	}
	ws, err := workspace()
	if err != nil {
		return "", err
	}
	u, err := ws.URI(arg)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// cliProjects is the CLI's project resolver: the same filesystem-backed store and config loader a
// server uses, over the mounts this run has (declared with --mount, or minted per argument).
//
// It goes through the same code serve does, so a design checked from the terminal and in the
// browser compose the same config and agree about what a board was measured against.
func cliProjects() *service.ProjectResolver {
	return &service.ProjectResolver{Store: cliProjectStore{}, Config: cliProjectConfig{}}
}

// cliProjectStore and cliProjectConfig read the run's mounts at CALL time rather than holding a
// snapshot.
//
// The CLI mints a mount lazily, when an argument is first turned into a URI, and the services are
// constructed BEFORE the first argument is resolved. A resolver built from `ws.Mounts()` at
// construction holds an empty list forever, and every design silently resolves to no project,
// which looks exactly like a design that has none.
type cliProjectStore struct{}

func (cliProjectStore) store() (*projects.FSStore, error) {
	ws, err := workspace()
	if err != nil {
		return nil, err
	}
	return projects.NewFSStore(projectTrees(ws.Mounts())...), nil
}

func (c cliProjectStore) Project(ctx context.Context, name string) (*webapi.Project, error) {
	s, err := c.store()
	if err != nil {
		return nil, err
	}
	return s.Project(ctx, name)
}

func (c cliProjectStore) Projects(ctx context.Context) ([]*webapi.Project, error) {
	s, err := c.store()
	if err != nil {
		return nil, err
	}
	return s.Projects(ctx)
}

func (c cliProjectStore) Design(ctx context.Context, name string) (*webapi.Design, error) {
	s, err := c.store()
	if err != nil {
		return nil, err
	}
	return s.Design(ctx, name)
}

func (c cliProjectStore) Designs(ctx context.Context, parent string) ([]*webapi.Design, error) {
	s, err := c.store()
	if err != nil {
		return nil, err
	}
	return s.Designs(ctx, parent)
}

func (c cliProjectStore) ResolveDesign(ctx context.Context, uri artifact.URI) (*webapi.Design, *webapi.Project, error) {
	s, err := c.store()
	if err != nil {
		return nil, nil, err
	}
	return s.ResolveDesign(ctx, uri)
}

type cliProjectConfig struct{}

func (cliProjectConfig) ResolveConfig(ctx context.Context, cfg *webapi.AnalysisConfig, namespace string) (service.ResolvedConfig, error) {
	ws, err := workspace()
	if err != nil {
		return service.ResolvedConfig{}, err
	}
	return (&osProjectConfig{mounts: ws.Mounts()}).ResolveConfig(ctx, cfg, namespace)
}

// withProjectRules splices the rules a design's project supplies onto a catalog, for the CLI's own
// facet resolution, and returns the composed Overlay beside it.
//
// The CLI resolves `--rule` and `--tag` to rule NAMES before calling the service, so its local
// catalog has to span the same name space the run will. Without this a project's own rule is
// unselectable (`--rule gateway/signal-net-naming` reports "no rules selected"), and the unfiltered
// case sends the local catalog's full name list as an explicit selection, which silently EXCLUDES
// every project rule from the run.
//
// Both results come from one service.OverlayFor call, so the CLI and the service cannot disagree.
// Composing separately produced a duplicate-source error for `--conventions` naming the file the
// project already declares (see service/overlay.go). A caller writing a results document records
// the Overlay's tiers, the ones the RUN attached, rather than the ones its own flags named.
//
// A design with no project still gets the request's own config. A descriptor that exists and does
// not parse is an error (see ProjectResolver.Overlay).
func withProjectRules(ctx context.Context, base *check.Catalog, arg string, req *webapi.OverlayConfig) (*check.Catalog, service.Overlay, error) {
	r := cliProjects()
	// Do not return early when no project resolves. The REQUEST's own config still has to reach this
	// catalog, or `--rule <config>/<rule>` selects nothing and the empty selection silently runs the
	// whole catalog instead of the one rule asked for.
	d, p, err := cliResolveProject(ctx, arg)
	if err != nil {
		return nil, service.Overlay{}, err
	}
	ov, err := service.OverlayFor(ctx, r.Config, r.Store, p, d, req, service.Overlay{}, "")
	if err != nil {
		return nil, service.Overlay{}, err
	}
	cat, err := ov.Catalog(base)
	return cat, ov, err
}

// cliResolveProject returns the design and project governing a command-line argument. Both are nil
// when the argument belongs to neither, and it errors only when something that EXISTS fails to
// parse.
//
// Absent config is ordinary (most files on a mounted folder belong to no project) and malformed
// config is not. A caller that flattens the two runs against the built-in vocabulary and reports an
// authoritative-looking answer, so every CLI caller draws the line here (issue 312).
//
// An argument that names no existing path is not an error, and is left to the reader's not-found
// message.
//
// The ErrNotFound case is DEFENSIVE and no test reaches it. A store reports it for a mount it does
// not have, which this CLI cannot produce because the workspace and the store share one mount list
// and URI registers the mount before this call names it. It stays because ProjectStore is an
// interface, and an unknown mount means nothing to resolve against rather than something broken.
func cliResolveProject(ctx context.Context, arg string) (*webapi.Design, *webapi.Project, error) {
	ws, err := workspace()
	if err != nil {
		return nil, nil, err
	}
	u, err := ws.URI(arg)
	if err != nil {
		return nil, nil, err
	}
	d, p, err := cliProjects().Store.ResolveDesign(ctx, u)
	switch {
	case err == nil:
		return d, p, nil
	case errors.Is(err, service.ErrNotFound):
		return nil, nil, nil
	default:
		return nil, nil, err
	}
}

// cliProjectParent is the project resource name a design's review should be stored under, empty when
// the design belongs to none.
//
// Empty is a real answer. Reviewing a loose file is the ordinary case on a mounted folder, and such
// a run is stored unparented, since a synthetic parent would assert ownership that does not exist.
// A descriptor that exists and does not parse is an error instead, because the design does belong
// to a project and filing its run under none would record the wrong provenance.
func cliProjectParent(ctx context.Context, arg string) (string, error) {
	_, p, err := cliResolveProject(ctx, arg)
	if err != nil {
		return "", err
	}
	return p.GetName(), nil
}

// cliProjectChecklist reports the review manifest a design's project declares, and the project it
// came from.
//
// It returns THREE distinguishable states, because the caller has something different to say for
// each:
//
//	("", "")            the design belongs to no project
//	("", "projects/x")  it belongs to one, and that project declares no checklist
//	("mount://…", "projects/x")  it belongs to one that declares this checklist
//
// Collapsing the middle case into the first would tell an operator with a real project to "pass
// --checklist" when the fix is a `checklist:` line in the project.yaml they already have.
//
// A descriptor that exists and does not PARSE is returned as an error, since the fix there is one
// edit to that descriptor and not --checklist either.
func cliProjectChecklist(ctx context.Context, arg string) (uri, project string, err error) {
	_, p, err := cliResolveProject(ctx, arg)
	if err != nil {
		return "", "", err
	}
	return p.GetConfig().GetChecklistUri(), p.GetName(), nil
}

// relName returns the name provenance records for an absolute host path, which is its path within
// the mount that contains it, or its base name when no mount does. A relative path comes back
// unchanged apart from slashes.
//
// It is the read-time twin of inDeclared, with the longest root winning, so it agrees with the name
// the browser would see.
//
// Only the TAIL is kept, without the mount name, because a minted mount name is local to this
// process and a stored document carrying it would name something the reader cannot resolve. The
// tail is what `CheckReport.source` promises.
//
// The base-name fallback covers a file outside every mount, such as a symbol library found through
// --symbol-path. It drops the directory so a host path is never published.
func (w *cliWorkspace) relName(abs string) string {
	if !filepath.IsAbs(abs) {
		return filepath.ToSlash(abs)
	}
	best := ""
	bestRel := ""
	for _, m := range w.Mounts() {
		rel, err := filepath.Rel(m.Root, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if best == "" || len(m.Root) > len(best) {
			best, bestRel = m.Root, rel
		}
	}
	if best == "" {
		return filepath.Base(abs)
	}
	return filepath.ToSlash(bestRel)
}
