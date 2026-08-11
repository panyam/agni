package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/artifact"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/internal/service"
)

// readAsNamed is the --as-named flag's binding. Like symbolPaths below it, it is a cobra persistent
// flag: bound once at startup, never mutated per run, which is the startup-DEFAULT shape CONSTRAINTS
// C22 permits. Nothing reads it except newDesignResolver, so the value reaches resolution as a
// FIELD on the resolver rather than as ambient state a deep call site consults.
var readAsNamed bool

// designResolver answers "which artifacts should this read open" for the CLI, as a client of
// ProjectService.
//
// It goes through the service rather than reading descriptors itself so there is ONE resolution
// path. A CLI that parsed its own descriptors would drift from the served one on exactly the
// questions that are invisible from outside: which companion supplies the board, whether an
// unresolved ref is an error, what a malformed descriptor does.
type designResolver struct {
	ws *cliWorkspace
	// asNamed disables descriptor-driven redirection: read exactly the artifact named, even when the
	// enclosing design declares it a companion view of a different entry.
	//
	// It exists because reading a companion AS a netlist is a legitimate DIAGNOSTIC operation, not
	// only a mistake. Checking that a schematic export and the netlist still describe the same design
	// means reading both as netlists and diffing them, which the redirect would otherwise prevent
	// (`examples/tutorial-project` does this in its check-views target).
	asNamed bool
}

// newDesignResolver reads the flag once and returns a configured resolver, the same shape as
// newLoader beside it. The store is built per resolution, because the CLI's tree root depends on the
// path being resolved; the service is the constant.
func newDesignResolver(ws *cliWorkspace) *designResolver {
	return &designResolver{ws: ws, asNamed: readAsNamed}
}

// designSource is which artifact each tier of a read opens, plus the line that says so.
//
// The fields are REFS resolved by the loader, not paths this type does arithmetic on. The engine
// used to take the path a user typed as the whole answer, which is why `warnEdsSibling` existed: a
// folder holding an OrCAD `.eds` schematic export beside the `.edn` netlist reads a different
// component count depending on which one a tool opens, and with nothing modelling "this folder is
// one design, and this file is its entry", the CLI could only print a warning telling the operator
// to go consult their own descriptor.
type designSource struct {
	service.DesignSources
	// Note is a line for stderr, empty when the named path was taken exactly as given. It is written
	// whenever an artifact the user did NOT name was read, because which file was read is not
	// recoverable from a component count or a findings list — the same reason the viewer's convention
	// bar names the active vocabulary instead of only offering a dropdown.
	Note string
}

// Resolve decides which artifacts a read should open, given the path a user named.
//
// The path is turned into a (mount, ref) pair against a tree rooted a bounded number of levels above
// it, then handed to ProjectService.ResolveDesign. Three outcomes:
//
//   - Resolves to nothing: read exactly what was named. This is every invocation that works today,
//     and it is the ordinary case for any folder with no descriptors.
//   - Resolves, and the named ref is the design's ENTRY or an undeclared sibling: read what was
//     named. Redirection is confined to files an operator explicitly listed as companions, because a
//     later revision of the netlist sits in the same folder and IS a legitimate analysis source; an
//     inferred rule would turn a diff of two revisions into a diff of one against itself.
//   - Resolves, and the named ref is a declared COMPANION: analysis reads the design's entry, while
//     the named artifact keeps whatever tier it alone supplies.
//
// Naming a design FOLDER resolves the same way and reads the declared entry, which is new
// capability: pointing agni at a design used to be an error, so there was no way to say "this
// design" rather than "this file".
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
	// The same ProjectService a server hosts, over a store rooted at this argument's mount. Same
	// code, different tree root, chosen by the client — which is the whole of the CLI/serve
	// difference now.
	svc := service.NewProjectService(projects.NewFSStore(projects.Tree{Mount: uri.Mount, FS: os.DirFS(root)}))
	resp, err := svc.ResolveDesign(ctx, &webapi.ResolveDesignRequest{Uri: uri.String()})
	if err != nil {
		return designSource{}, err
	}
	d := resp.GetDesign()
	if d == nil {
		if isDir {
			// Handing a directory to a reader can only produce an unsupported-extension message for
			// something that is not a file at all, so say what was actually missing.
			return designSource{}, fmt.Errorf("%s is a directory that declares no design: name a design file, or add a %s declaring which file is this design's entry", named, projects.DesignDescriptor)
		}
		plain.Note = edsSiblingNote(named)
		return plain, nil
	}
	namedIsTheDesign := isDir && uri.String() == d.GetUri()
	if !namedIsTheDesign && (r.asNamed || !service.IsCompanion(d, uri.String())) {
		return plain, nil
	}

	// Either the design itself was named, or one of its declared companions was.
	from := uri.String()
	if namedIsTheDesign {
		from = ""
	}
	tiers := service.SourcesFor(d, from)
	// The note is computed on REFS, before they become paths, so "did this tier come from the file
	// the user named" is a comparison of like with like. Comparing a ref against the path string the
	// user typed silently never matches, and the note then claims every tier was pulled in unasked.
	note := resolutionNote(named, uri.String(), d, tiers, namedIsTheDesign)

	return designSource{DesignSources: tiers, Note: note}, nil
}

// resolutionNote is the stderr line naming every artifact that was read but not asked for.
//
// It lists the board and sheet tiers whenever they came from somewhere other than the entry AND
// somewhere other than what the caller named, in both the design-named and companion-named cases. A
// design's declared board is the design's board whichever of its views you point at, so pointing at
// the schematic still runs the board-tier rules against the declared board, which is more than was
// asked for and therefore has to be said.
func resolutionNote(named, ref string, d *webapi.Design, tiers service.DesignSources, namedIsTheDesign bool) string {
	// The descriptor is named by the DESIGN's own mount-relative path, not by joining onto whatever
	// the caller typed. The caller may have typed a path or a URI, and filepath.Join on a URI mangles
	// its scheme; the design already knows where it lives, in one spelling, in all three tiers.
	descriptor := path.Join(uriPath(d.GetUri()), projects.DesignDescriptor)
	var note string
	if namedIsTheDesign {
		note = fmt.Sprintf("note: reading %s (the entry %s declares)", path.Base(d.GetEntryUri()), descriptor)
	} else {
		note = fmt.Sprintf("note: %s is a companion view declared by %s; analysis reads %s (the design's entry)",
			path.Base(uriPath(named)), descriptor, path.Base(d.GetEntryUri()))
	}
	var extra []string
	if g := tiers.GeometryURI; g != tiers.NetlistURI && g != ref {
		extra = append(extra, "sheets from "+path.Base(g))
	}
	if b := tiers.BoardURI; b != tiers.NetlistURI && b != ref {
		extra = append(extra, "board geometry from "+path.Base(b))
	}
	if len(extra) > 0 {
		note += ", with " + strings.Join(extra, " and ")
	}
	if !namedIsTheDesign {
		note += ". Pass --as-named to read the file itself"
	}
	return note + ".\n"
}

// edsSiblingNote is the fallback advice for a folder that declares no design: reading an EDIF
// SCHEMATIC-geometry (.eds) export while the sibling NETLIST (.edn) exists. The two read different
// component counts — the .eds reflects what the schematic DRAWS, the .edn is authoritative for
// component identity — so a count taken off the .eds is a silent footgun (it read test_point 565
// against the .edn's 1385 on one real board). Empty when there is no .eds, no sibling .edn, or the
// input already is the .edn.
//
// It survives as ADVICE only where there is no declaration to act on. A design that names its entry
// gets the redirect above instead, which is the same guardrail with the guess removed: it names the
// file the operator actually declared, and it fires for a `.kicad_pcb` or an IPC-2581 companion just
// as readily as for a `.eds`, where this heuristic only ever knew one stem-matched pair.
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
// unknown, which for the CLI means the argument named a path that does not exist: the workspace
// mints a mount for anything real, so an unmintable argument is one nothing could be rooted at.
func mountRoot(ws *cliWorkspace, uri artifact.URI) (string, bool) {
	m, ok := mounts.Find(ws.Mounts(), uri.Mount)
	if !ok {
		return "", false
	}
	return m.Root, true
}

// uriPath is the mount-relative path of a URI, or the string unchanged when it is not one. It exists
// so a message can name a file the same way whether the caller typed a path or a URI.
func uriPath(s string) string {
	if u, err := artifact.Parse(s); err == nil {
		return u.Path
	}
	return filepath.ToSlash(s)
}
