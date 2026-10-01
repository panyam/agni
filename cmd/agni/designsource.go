package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
)

// readAsNamed is the --as-named flag's binding, set once at startup and never mutated per run, like
// symbolPaths below it (the startup-DEFAULT shape C22 permits). Only newDesignResolver reads it, so
// it reaches resolution as a resolver FIELD rather than as ambient state.
var readAsNamed bool

// designResolver answers "which artifacts should this read open" for the CLI, as a client of
// ProjectService.
//
// It goes through the service rather than parsing descriptors itself so the CLI and a server give
// the same answer to which companion supplies the board, whether an unresolved ref is an error, and
// what a malformed descriptor does (agni issue 170).
type designResolver struct {
	ws *cliWorkspace
	// asNamed disables descriptor-driven redirection and reads exactly the artifact named, even a
	// declared companion. `examples/tutorial-project`'s check-views target uses it to diff a
	// schematic export against the netlist.
	asNamed bool
}

// newDesignResolver reads the flag once and returns a configured resolver, the same shape as
// newLoader beside it. The store is built per resolution rather than held here, because the CLI's
// tree root depends on the path being resolved.
func newDesignResolver(ws *cliWorkspace) *designResolver {
	return &designResolver{ws: ws, asNamed: readAsNamed}
}

// designSource is which artifact each tier of a read opens, plus the line that says so (agni issue
// 170). The fields are REFS resolved by the loader, not paths this type does arithmetic on.
type designSource struct {
	service.DesignSources
	// Note is a line for stderr, written whenever an artifact the user did NOT name was read and
	// empty when the named path was taken exactly as given.
	Note string
}

// Resolve decides which artifacts a read should open, given the path a user named.
//
// The path becomes an artifact URI through the workspace's mount table and goes to
// ProjectService.ResolveDesign over a store rooted at that mount. The outcomes:
//
//   - Resolves to nothing: read exactly what was named. A DIRECTORY that resolves to nothing is an
//     error.
//   - Names the design FOLDER or its ENTRY: read the entry, with the declared companions' tiers.
//   - Names an undeclared sibling: read exactly that file.
//   - Names a declared COMPANION: analysis reads the entry, while the named artifact keeps whatever
//     tier it alone supplies.
//
// Why companions are declared file by file is in
// docsite/content/architecture/projects-and-designs.md#the-netlist-is-the-source-the-rest-are-views
// (agni issues 170, 528).
func (r *designResolver) Resolve(ctx context.Context, named string) (designSource, error) {
	uri, err := r.ws.URI(named)
	if err != nil {
		return designSource{}, err
	}
	plain := designSource{DesignSources: service.DesignSources{NetlistURI: uri.String(), BoardURI: uri.String(), GeometryURI: uri.String()}}
	root, ok := mountRoot(r.ws, uri)
	if !ok {
		return plain, nil
	}
	isDir := false
	if fi, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(uri.Path))); statErr == nil {
		isDir = fi.IsDir()
	}
	// The same ProjectService a server hosts, over a store rooted at this argument's mount.
	svc := service.NewProjectService(projects.NewFSStore(projects.Tree{Mount: uri.Mount, FS: os.DirFS(root)}))
	resp, err := svc.ResolveDesign(ctx, &webapi.ResolveDesignRequest{Uri: uri.String()})
	if err != nil {
		return designSource{}, err
	}
	d := resp.GetDesign()
	if d == nil {
		if isDir {
			// A reader handed a directory would report an unsupported extension, so name what is missing.
			return designSource{}, fmt.Errorf("%s is a directory that declares no design: name a design file, or add a %s declaring which file is this design's entry", named, projects.DesignDescriptor)
		}
		plain.Note = edsSiblingNote(named)
		return plain, nil
	}
	// The decision itself is service.ResolveSources, shared with the served path (C32). Only the I/O
	// around it lives here.
	res := service.ResolveSources(d, uri.String(), isDir, r.asNamed)
	if !res.FromDeclaration {
		// An undeclared sibling, or --as-named on a companion, reads exactly what was named, so
		// there is no note. plain already holds the ref in all three tiers.
		return plain, nil
	}
	// Compare REFS, not the path string the user typed. A ref never matches a typed path, and the
	// note would then claim every tier was pulled in unasked.
	note := resolutionNote(named, uri.String(), d, res.DesignSources, res.NamedIsTheDesign, res.NamedIsTheEntry)

	return designSource{DesignSources: res.DesignSources, Note: note}, nil
}

// resolutionNote is the stderr line naming every artifact that was read but not asked for.
//
// It lists the board and sheet tiers whenever they came from somewhere other than the entry AND
// other than what the caller named. A design's declared board is its board whichever view you point
// at, so pointing at the schematic still runs board-tier rules against that board.
func resolutionNote(named, ref string, d *webapi.Design, tiers service.DesignSources, namedIsTheDesign, namedIsTheEntry bool) string {
	// Built from the DESIGN's own mount-relative path, since filepath.Join onto a typed URI mangles
	// its scheme.
	descriptor := path.Join(uriPath(d.GetUri()), projects.DesignDescriptor)
	var extra []string
	if g := tiers.GeometryURI; g != tiers.NetlistURI && g != ref {
		extra = append(extra, "sheets from "+path.Base(g))
	}
	if b := tiers.BoardURI; b != tiers.NetlistURI && b != ref {
		extra = append(extra, "board geometry from "+path.Base(b))
	}
	// Naming the entry read exactly the file asked for, so only companion tiers get a line, and a
	// run that picked up no companion prints nothing.
	if namedIsTheEntry {
		if len(extra) == 0 {
			return ""
		}
		return fmt.Sprintf("note: reading %s with %s (declared by %s). Pass --as-named for the file alone.\n",
			path.Base(uriPath(named)), strings.Join(extra, " and "), descriptor)
	}
	var note string
	if namedIsTheDesign {
		note = fmt.Sprintf("note: reading %s (the entry %s declares)", path.Base(d.GetEntryUri()), descriptor)
	} else {
		note = fmt.Sprintf("note: %s is a companion view declared by %s; analysis reads %s (the design's entry)",
			path.Base(uriPath(named)), descriptor, path.Base(d.GetEntryUri()))
	}
	if len(extra) > 0 {
		note += ", with " + strings.Join(extra, " and ")
	}
	if !namedIsTheDesign {
		note += ". Pass --as-named to read the file itself"
	}
	return note + ".\n"
}

// edsSiblingNote is the fallback advice for a folder that declares no design, when reading an EDIF
// SCHEMATIC-geometry (.eds) export while the sibling NETLIST (.edn) exists. The .eds reflects what
// the schematic DRAWS and the .edn is authoritative for component identity, so counts differ (565
// test points off the .eds against the .edn's 1385, on one real board). Empty when there is no .eds,
// no sibling .edn, or the input already is the .edn.
//
// A design that declares its entry gets Resolve's redirect instead (agni issue 170).
func edsSiblingNote(path string) string {
	if !strings.EqualFold(filepath.Ext(path), ".eds") {
		return ""
	}
	stem := strings.TrimSuffix(path, filepath.Ext(path))
	for _, ext := range []string{".edn", ".EDN"} {
		if _, err := os.Stat(stem + ext); err == nil {
			return fmt.Sprintf("warning: %s is an EDIF schematic-geometry (.eds) export; component counts may be lower than the netlist. The sibling %s (.edn netlist) is authoritative for component identity — declare it as this design's entry in a %s.\n",
				filepath.Base(path), filepath.Base(stem+ext), projects.DesignDescriptor)
		}
	}
	return ""
}

// mountRoot returns the host root a URI's mount resolves to. It reports false when the mount is
// unknown, which for the CLI means the argument named a path that does not exist, since the
// workspace mints a mount for anything real.
func mountRoot(ws *cliWorkspace, uri artifact.URI) (string, bool) {
	m, ok := mounts.Find(ws.Mounts(), uri.Mount)
	if !ok {
		return "", false
	}
	return m.Root, true
}

// uriPath is the mount-relative path of a URI, or the string slash-normalized when it is not one, so
// a message names a file the same way whether the caller typed a path or a URI.
func uriPath(s string) string {
	if u, err := artifact.Parse(s); err == nil {
		return u.Path
	}
	return filepath.ToSlash(s)
}
