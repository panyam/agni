package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/panyam/agni/internal/artifact"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/projects"
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
			declared, _, parseErr := projects.ParseProject(f)
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
