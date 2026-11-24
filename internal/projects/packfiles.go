package projects

import (
	"bytes"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// PackFromFiles is PackFromFolder over a folder whose paths are all of files, relative to it and
// slash-separated. `agni packs push` and ImportPackFiles read a folder this way.
func PackFromFiles(files []string, read func(string) ([]byte, error)) (id string, pack *webapi.Pack, kept []string, err error) {
	return PackFromFolder(ListFiles(files), read)
}

// ListFiles is a folder lister over files, a folder's paths relative to it and slash-separated. It
// returns the paths that equal rel or sit under it, in files' order.
func ListFiles(files []string) func(rel string) ([]string, error) {
	return func(rel string) ([]string, error) {
		var out []string
		for _, f := range files {
			if f == rel || strings.HasPrefix(f, rel+"/") {
				out = append(out, f)
			}
		}
		return out, nil
	}
}

// PackFromFolder reads the pack a project folder makes (agni issues 1697, 1746, 1871). list returns
// the folder's paths that equal a relative path or sit under it, relative to the folder and
// slash-separated, and none for a path the folder lacks. read returns one file's contents. Only the
// project.yaml at the folder's top is read, and only it and the paths its tiers name are listed, so
// a folder that also holds large designs is never listed whole.
//
// It returns the project's id and the pack, its config bound from project.yaml and its files the
// paths the config's profile, param, symbol, library and query tiers name, and the item files of
// any checklist written as items_dir (agni issue 1812). Each tier keeps a mount://
// URI as written, and a relative path names a file or a folder in the folder. kept lists those
// paths, sorted. A pack's files carry no hash here, since the caller knows its own. The folder's
// other files, such as its designs, are no part of the pack.
//
// A folder with no project.yaml, a project.yaml that extends another, or a tier naming a path
// outside the folder or among no files is service.ErrInvalidArgument.
func PackFromFolder(list func(rel string) ([]string, error), read func(string) ([]byte, error)) (id string, pack *webapi.Pack, kept []string, err error) {
	// within keeps what list gives that equals rel or sits under it, so a lister that returns more
	// can't widen a tier.
	known := map[string]bool{}
	within := func(rel string) ([]string, error) {
		if rel == "." || rel == "" {
			return nil, nil
		}
		got, err := list(rel)
		if err != nil {
			return nil, err
		}
		got = slices.DeleteFunc(got, func(f string) bool { return f != rel && !strings.HasPrefix(f, rel+"/") })
		for _, f := range got {
			known[f] = true
		}
		return got, nil
	}
	top, err := within(ProjectDescriptor)
	if err != nil {
		return "", nil, nil, err
	}
	if !slices.Contains(top, ProjectDescriptor) {
		return "", nil, nil, fmt.Errorf("%w: no %s at the top of the folder", service.ErrInvalidArgument, ProjectDescriptor)
	}
	raw, err := read(ProjectDescriptor)
	if err != nil {
		return "", nil, nil, err
	}
	id, project, src, err := ParseProject(bytes.NewReader(raw))
	if err != nil {
		return "", nil, nil, fmt.Errorf("%w: %s: %v", service.ErrInvalidArgument, ProjectDescriptor, err)
	}
	if src.Extends != "" {
		return "", nil, nil, fmt.Errorf("%w: %s extends %s, and a pack extends nothing; keep that one as its own pack and name both", service.ErrInvalidArgument, ProjectDescriptor, src.Extends)
	}
	seen := map[string]bool{}
	tier := func(refs []string) ([]string, error) {
		var out []string
		for _, r := range refs {
			if strings.Contains(r, "://") {
				out = append(out, r)
				continue
			}
			rel := path.Clean(strings.ReplaceAll(r, "\\", "/"))
			if rel == ".." || strings.HasPrefix(rel, "../") || path.IsAbs(rel) {
				return nil, fmt.Errorf("%w: %s leaves the project's folder; a pack holds only files beside its %s, so name it with a mount:// URI", service.ErrInvalidArgument, r, ProjectDescriptor)
			}
			named, err := within(rel)
			if err != nil {
				return nil, err
			}
			for _, f := range named {
				if !seen[f] {
					seen[f] = true
					kept = append(kept, f)
				}
			}
			if len(named) == 0 {
				return nil, fmt.Errorf("%w: %s names no file in the folder", service.ErrInvalidArgument, r)
			}
			out = append(out, rel)
		}
		return out, nil
	}
	cfg := project.GetConfig()
	under := func(dir string) ([]string, error) {
		got, err := within(dir)
		return slices.DeleteFunc(got, func(f string) bool { return f == dir }), err
	}
	itemFiles, err := ResolveChecklistDirs(cfg, src.ChecklistDirs, under, read)
	if err != nil {
		// Both wrapped, so a server still finds a file whose contents it lacks and asks for them.
		return "", nil, nil, fmt.Errorf("%w: %s: %w", service.ErrInvalidArgument, ProjectDescriptor, err)
	}
	// The item files go with the pack, so a server reading the pushed files reads the same checklist.
	for _, f := range itemFiles {
		if !known[f] {
			if _, err := within(f); err != nil {
				return "", nil, nil, err
			}
		}
		if known[f] && !seen[f] {
			seen[f] = true
			kept = append(kept, f)
		}
	}
	for _, t := range []struct {
		refs []string
		into *[]string
	}{{src.Profiles, &cfg.ProfileUris}, {src.Params, &cfg.ParamUris}, {src.Symbols, &cfg.SymbolPathUris}, {src.Lib, &cfg.LibraryUris}, {src.Queries, &cfg.QuerySetUris}} {
		if *t.into, err = tier(t.refs); err != nil {
			return "", nil, nil, err
		}
	}
	slices.Sort(kept)
	pack = &webapi.Pack{Name: service.PackName(id), Title: project.GetTitle(), Config: cfg}
	for _, f := range kept {
		pack.Files = append(pack.Files, &webapi.SnapshotFile{Path: f})
	}
	return id, pack, kept, nil
}
