package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/artifact"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/internal/service"
)

// cliMountSpecs holds the --mount flag values. It is a root PERSISTENT flag, so the same
// `name=path` form works on every subcommand and on serve, rather than serve owning a concept the
// rest of the CLI could not express.
var cliMountSpecs []string

// maxProjectWalk bounds how far above a named file the CLI looks for a project descriptor when it
// has to mint a mount. It is a bound rather than a walk to the filesystem root because a stray
// `project.yaml` far up someone's home directory should never silently become the authority a
// stored review is recorded under.
const maxProjectWalk = 4

// cliWorkspace is the CLI's mount table, and the reason the CLI is no longer a special case.
//
// A server is handed its mounts by an operator. The CLI is handed a PATH, which is the whole
// ergonomic difference between them, so this turns one into the other: an argument becomes an
// artifact URI, and the authority it names is a mount this table can resolve.
//
// Three tiers, most explicit first:
//
//  1. An argument that already IS a URI is taken as written. Its authority must be a mount that was
//     declared with --mount, so being explicit is a way to say which mount you mean, never a way to
//     name a place no mount covers.
//  2. A path that falls inside a DECLARED mount is addressed through it. This is the tier worth
//     having: point the CLI at the same --mount a server uses and the two produce identical URIs for
//     the same design, so a stored review created either way is directly comparable.
//  3. A path outside every declared mount gets a mount MINTED for it, rooted at the enclosing
//     project when one resolves and at the file's own directory otherwise.
//
// Tier 3 is what keeps `agni check some/board.edn` working with no configuration, and rooting it at
// the project rather than at the filesystem root is what keeps a host path out of the URI. A URI
// carrying `/Users/<someone>` would be recorded verbatim into every review document the run
// produced, which is neither portable nor anyone's business.
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
// the way. Declared wins a name collision, matching mounts.Merge's rule that what an operator typed
// is the more specific intent.
func (w *cliWorkspace) Mounts() []mounts.Mount {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append(append([]mounts.Mount{}, w.declared...), w.minted...)
}

// URI turns one command-line argument into an artifact URI, applying the three tiers above.
//
// A path that does not exist is NOT an error here. The reader produces the not-found message a user
// already knows, and inventing a second one at this layer would only mean two ways to be told the
// same thing.
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
// Rooting at the PROJECT is what makes the resulting URI mean something outside this machine:
// `mount://gateway/designs/gateway/gateway.edn` says which design of which project, and reads the
// same as the URI a server with that project mounted would produce.
func (w *cliWorkspace) mint(abs string) (artifact.URI, error) {
	root, name := projectRootAbove(abs)
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
// root and otherwise suffixing until the name is free.
//
// The suffixing matters for the case that motivated per-argument mounts at all: diffing two designs
// that live in different trees and happen to share a project id. Two roots under one authority would
// make the second design unaddressable.
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
// holding it and the project's declared id. It returns ("", "") when there is none within
// maxProjectWalk levels.
//
// The declared id becomes the mount NAME, which is the point: a project already has an
// operator-chosen identity, and inventing a second one for the same thing would mean a design's URI
// depended on whether it was reached through the CLI or through a server.
func projectRootAbove(abs string) (root, id string) {
	dir := abs
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		dir = filepath.Dir(abs)
	}
	for range maxProjectWalk + 1 {
		f, err := os.Open(filepath.Join(dir, projects.ProjectDescriptor))
		if err == nil {
			declared, _, _, parseErr := projects.ParseProject(f)
			f.Close()
			if parseErr == nil {
				return dir, declared
			}
			// A malformed descriptor is not this function's to report: the design read that follows
			// will surface it with the context a user can act on. Minting a plain mount here keeps
			// the addressing sane in the meantime.
			return "", ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", ""
}

// cliWS is the workspace for this run, built once on first use.
//
// It is package-level for the same reason the flag variables are: a CLI process serves exactly one
// invocation, so "the mounts for this run" is genuinely process state rather than ambient config
// standing in for a parameter. Nothing mutates it after the first call, and no request path reads
// it, which is the CONSTRAINTS C22 startup-default shape.
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
// It swallows the error deliberately. Every caller is building a request whose service will parse
// this string and classify a bad one with the context that matters (which rpc, which field); a
// second error raised here would be the same failure reported twice, in the less useful place. An
// unmintable argument passes through unchanged and fails at that parse.
func cliArgURI(arg string) string {
	if arg == "" {
		// An unsupplied optional flag stays unsupplied. Without this, filepath.Abs("") resolves to the
		// working directory and an absent --board-path arrives as a URI naming a real folder, which the
		// service then reads as "a board was supplied" and fails on a directory that is not one.
		return ""
	}
	ws, err := workspace()
	if err != nil {
		return arg
	}
	u, err := ws.URI(arg)
	if err != nil {
		return arg
	}
	return u.String()
}

// cliProjects is the CLI's project resolver: the same filesystem-backed store and config loader a
// server uses, over the mounts this run has (declared with --mount, or minted per argument).
//
// The CLI resolves projects for the same reason serve does, and through the same code. A design
// checked from the terminal and the same design checked in the browser have to compose the same
// config, or the two surfaces disagree about what a board was measured against — which is the drift
// this whole workstream is closing, not one to reintroduce at the CLI edge.
func cliProjects() *service.ProjectResolver {
	return &service.ProjectResolver{Store: cliProjectStore{}, Config: cliProjectConfig{}}
}

// cliProjectStore and cliProjectConfig read the run's mounts at CALL time rather than holding a
// snapshot.
//
// That is not fussiness. The CLI mints a mount lazily, when an argument is first turned into a URI,
// and the services are constructed BEFORE the first argument is resolved. A resolver built from
// `ws.Mounts()` at construction therefore holds an empty list forever, and every design silently
// resolves to no project — a failure that looks exactly like a design which genuinely has none.
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
// facet resolution.
//
// The CLI resolves `--rule` and `--tag` to rule NAMES before calling the service, so its local
// catalog has to span the same name space the run will. Without this a project's own rule is
// unselectable — `--rule gateway/signal-net-naming` reports "no rules selected" for a rule that
// would have run — and, worse, the unfiltered case sends the local catalog's full name list as an
// explicit selection, which silently EXCLUDES every project rule from the run.
//
// It goes through service.OverlayFor rather than splicing by hand, so the CLI and the service cannot
// disagree about the result. Composing them separately is what produced a duplicate-source error the
// moment an operator passed `--conventions` for the file their project already declares: two code
// paths, one adding what the other replaced.
//
// A design that resolves to no project returns the catalog unchanged, and a resolution failure is
// not fatal: the run still has its own composition, and failing the whole command because some
// unrelated descriptor is malformed would be worse than listing one fewer rule.
// It returns the composed Overlay alongside the catalog so a caller writing a results document records
// the tiers the RUN had attached rather than the ones its own flags named. Both come from the one
// OverlayFor call for the same reason the catalog does: a second composition to answer "were profiles
// attached" could disagree with the first.
func withProjectRules(ctx context.Context, base *check.Catalog, arg string, req *webapi.OverlayConfig) (*check.Catalog, service.Overlay, error) {
	r := cliProjects()
	// A project may not resolve, and that is fine — but the REQUEST's own config still has to reach
	// this catalog. Bailing out early on a miss dropped `--conventions` from facet resolution, so
	// `--rule <config>/<rule>` selected nothing and the empty selection silently ran the whole
	// catalog instead of the one rule asked for.
	// A design with no descriptor resolves to nothing and runs on the base catalog. One whose
	// descriptor exists and does not PARSE fails here instead, because the rules this run would
	// otherwise compose are not the rules the project declared (see ProjectResolver.Overlay).
	var p *webapi.Project
	var d *webapi.Design
	if ws, err := workspace(); err == nil {
		if u, err := ws.URI(arg); err == nil {
			design, resolved, err := r.Store.ResolveDesign(ctx, u)
			switch {
			case err == nil:
				p, d = resolved, design
			case errors.Is(err, service.ErrNotFound):
				// Unknown mount: nothing to resolve against, same as having no descriptor.
			default:
				return nil, service.Overlay{}, err
			}
		}
	}
	ov, err := service.OverlayFor(ctx, r.Config, r.Store, p, d, req, service.Overlay{}, "")
	if err != nil {
		return nil, service.Overlay{}, err
	}
	cat, err := ov.Catalog(base)
	return cat, ov, err
}

// cliProjectParent is the project resource name a design's review should be stored under, empty when
// the design belongs to none.
//
// Empty is a real answer rather than a failure. Reviewing a loose file is the ordinary case on a
// mounted folder, and such a run is stored unparented — giving it a synthetic parent would assert an
// ownership that does not exist.
func cliProjectParent(ctx context.Context, arg string) string {
	ws, err := workspace()
	if err != nil {
		return ""
	}
	u, err := ws.URI(arg)
	if err != nil {
		return ""
	}
	_, p, err := cliProjects().Store.ResolveDesign(ctx, u)
	if err != nil || p == nil {
		return ""
	}
	return p.GetName()
}

// cliProjectChecklist reports the review manifest a design's project declares, and the project it
// came from.
//
// It returns THREE distinguishable states rather than a single "found or not", because the caller
// has three different things to say:
//
//	("", "")            the design belongs to no project
//	("", "projects/x")  it belongs to one, and that project declares no checklist
//	("mount://…", "projects/x")  it belongs to one that declares this checklist
//
// Collapsing the middle case into the first would tell an operator with a real project to "pass
// --checklist" when the actionable fix is a `checklist:` line in the project.yaml they already have.
//
// A resolution failure reads as "no project", on the same terms as cliProjectParent: reviewing a
// loose file is ordinary, and failing the whole command because some unrelated descriptor on the
// mount is malformed would be worse than asking for the flag.
func cliProjectChecklist(ctx context.Context, arg string) (uri, project string) {
	ws, err := workspace()
	if err != nil {
		return "", ""
	}
	u, err := ws.URI(arg)
	if err != nil {
		return "", ""
	}
	_, p, err := cliProjects().Store.ResolveDesign(ctx, u)
	if err != nil || p == nil {
		return "", ""
	}
	return p.GetConfig().GetChecklistUri(), p.GetName()
}
