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

// PackFromFiles reads the pack a project folder's files make (agni issues 1697, 1746). files are the
// folder's paths, relative to it and slash-separated, and read returns one file's contents. Only the
// project.yaml at the folder's top is read.
//
// It returns the project's id and the pack, its config bound from project.yaml and its files the
// paths the config's profile, param, symbol, library and query tiers name. Each tier keeps a mount://
// URI as written, and a relative path names a file or a folder among files. kept lists those paths,
// sorted. A pack's files carry no hash here, since the caller knows its own. The folder's other
// files, such as its designs, are no part of the pack.
//
// A folder with no project.yaml, a project.yaml that extends another, or a tier naming a path
// outside the folder or among no files is service.ErrInvalidArgument.
func PackFromFiles(files []string, read func(string) ([]byte, error)) (id string, pack *webapi.Pack, kept []string, err error) {
	if !slices.Contains(files, ProjectDescriptor) {
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
			n := 0
			for _, f := range files {
				if f == rel || strings.HasPrefix(f, rel+"/") {
					n++
					if !seen[f] {
						seen[f] = true
						kept = append(kept, f)
					}
				}
			}
			if n == 0 {
				return nil, fmt.Errorf("%w: %s names no file in the folder", service.ErrInvalidArgument, r)
			}
			out = append(out, rel)
		}
		return out, nil
	}
	cfg := project.GetConfig()
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
