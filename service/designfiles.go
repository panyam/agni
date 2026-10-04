package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// FileReader reads one file inside a mount. It is the port ListDesignFiles sizes and hashes files
// through, separate from Workspace so the hosts that only browse (the datasheet service) need not
// read.
type FileReader interface {
	ReadFile(ctx context.Context, uri artifact.URI) ([]byte, error)
}

// ProjectDescriptorName is the file that makes a folder a project. internal/projects owns the name
// and a test there holds this copy to it, since this package cannot import that one.
const ProjectDescriptorName = "project.yaml"

// DesignFilesMaxBytes bounds a design's file set. A set over it is refused rather than truncated,
// because a partial set analyses as a design missing its board or its profiles and reads clean. It
// sits above the largest set the in-browser engine was measured to handle (a 97 MB KiCad board in
// agni issue 852), so the page's own threshold, not this bound, decides which designs go to the
// browser.
const DesignFilesMaxBytes = 256 << 20

// designFilesMaxDirs bounds the walk, for a design folder that turns out to hold a vendored tree.
const designFilesMaxDirs = 2000

// WithDesignFiles enables ListDesignFiles, which needs the project resolver to find a design's
// project config and a reader to size and hash its files. Without it the rpc reports that this host
// does not offer it.
func (s *WorkspaceService) WithDesignFiles(projects *ProjectResolver, files FileReader) *WorkspaceService {
	s.projects, s.files = projects, files
	return s
}

// ListDesignFiles lists every file a design's analysis reads (agni issue 853): the design's folder,
// recursively, which holds its entry, companions, revisions and descriptor; the project's
// descriptor; and every directory the project's config names (profiles, params, symbols, lib). It
// is what a host that analyses the design somewhere else, the in-browser engine first, fetches.
//
// Every listed path is in the design's own mount. A config URI naming another mount is refused,
// since a client mounting the files under one name would resolve it to nothing and analyse the
// design without that tier.
func (s *WorkspaceService) ListDesignFiles(ctx context.Context, req *webapi.ListDesignFilesRequest) (*webapi.ListDesignFilesResponse, error) {
	if s.files == nil {
		return nil, fmt.Errorf("%w: this host does not list design files", ErrInvalidArgument)
	}
	u, err := ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	var dirs, singles []string
	isDir, err := s.isDir(ctx, u)
	if err != nil {
		return nil, err
	}
	designDir := u.Path
	if !isDir {
		designDir = parentPath(u.Path)
	}
	if s.projects != nil && s.projects.Store != nil {
		design, project, err := s.projects.Store.ResolveDesign(ctx, u)
		if err != nil {
			return nil, ClassifyLoadErr(err)
		}
		if design != nil {
			du, err := artifact.Parse(design.GetUri())
			if err != nil {
				return nil, err
			}
			designDir = du.Path
		}
		if project != nil {
			pu, err := artifact.Parse(project.GetUri())
			if err != nil {
				return nil, err
			}
			singles = append(singles, joinPath(pu.Path, ProjectDescriptorName))
			cfg := project.GetConfig()
			for _, list := range [][]string{cfg.GetProfileUris(), cfg.GetParamUris(), cfg.GetSymbolPathUris(), cfg.GetLibraryUris()} {
				for _, raw := range list {
					cu, err := artifact.Parse(raw)
					if err != nil {
						return nil, err
					}
					if cu.Mount != u.Mount {
						return nil, fmt.Errorf("%w: project config %s is in another mount than the design", ErrInvalidArgument, raw)
					}
					dirs = append(dirs, cu.Path)
				}
			}
		}
	}
	dirs = append([]string{designDir}, dirs...)

	seen := map[string]bool{}
	var paths []string
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	budget := designFilesMaxDirs
	for _, d := range dirs {
		if err := s.walkFiles(ctx, u.Mount, d, add, &budget); err != nil {
			return nil, err
		}
	}
	for _, p := range singles {
		add(p)
	}
	sort.Strings(paths)

	resp := &webapi.ListDesignFilesResponse{Mount: u.Mount}
	for _, p := range paths {
		fu, err := artifact.New(u.Mount, p)
		if err != nil {
			return nil, err
		}
		b, err := s.files.ReadFile(ctx, fu)
		if err != nil {
			// A descriptor or config directory the project names but the tree lacks is the project's
			// error, and the analysis reports it when it composes. Listing what exists keeps the two
			// hosts failing the same way.
			continue
		}
		resp.TotalSize += int64(len(b))
		if resp.TotalSize > DesignFilesMaxBytes {
			return nil, fmt.Errorf("%w: the design's files exceed %d MB", ErrResourceExhausted, DesignFilesMaxBytes>>20)
		}
		sum := sha256.Sum256(b)
		resp.Files = append(resp.Files, &webapi.DesignFile{Path: p, Size: int64(len(b)), Sha256: "sha256:" + hex.EncodeToString(sum[:])})
	}
	return resp, nil
}

// isDir reports whether u names a directory, by listing its parent.
func (s *WorkspaceService) isDir(ctx context.Context, u artifact.URI) (bool, error) {
	if u.Path == "" {
		return true, nil
	}
	pu, err := artifact.New(u.Mount, parentPath(u.Path))
	if err != nil {
		return false, err
	}
	entries, err := s.ws.ListDir(ctx, pu)
	if err != nil {
		return false, fmt.Errorf("%w: %s", ErrNotFound, err)
	}
	base := path.Base(u.Path)
	for _, e := range entries {
		if e.Name == base {
			return e.IsDir, nil
		}
	}
	return false, fmt.Errorf("%w: %s", ErrNotFound, u)
}

// walkFiles adds every file under dir, depth-first, spending one unit of budget per directory.
func (s *WorkspaceService) walkFiles(ctx context.Context, mount, dir string, add func(string), budget *int) error {
	if *budget <= 0 {
		return fmt.Errorf("%w: the design's folder holds more than %d directories", ErrResourceExhausted, designFilesMaxDirs)
	}
	*budget--
	du, err := artifact.New(mount, dir)
	if err != nil {
		return err
	}
	entries, err := s.ws.ListDir(ctx, du)
	if err != nil {
		// A config directory the project names but the tree lacks; see ListDesignFiles.
		return nil
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name, ".") {
			continue
		}
		p := joinPath(dir, e.Name)
		if e.IsDir {
			if err := s.walkFiles(ctx, mount, p, add, budget); err != nil {
				return err
			}
			continue
		}
		add(p)
	}
	return nil
}

func parentPath(p string) string {
	if d := path.Dir(p); d != "." {
		return d
	}
	return ""
}

func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}
