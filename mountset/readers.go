package mountset

import (
	"context"
	"fmt"

	"github.com/panyam/goapplib/storage/mounts"

	"github.com/panyam/agni/service"
)

// Readers is the service.MountReaders a linked artifact reads through (agni issues 1723, 1759). Each
// reader is the registry's ResolveAt, ListAt and FetchAt for one mount. A GitHub mount lists a
// commit's files with their git blob ids and downloads only what the blobs lack. A folder from the
// flags is read as it is now. The uploads mount holds uploaded files and can't be linked.
func (t *Table) Readers(ctx context.Context, name string) (service.MountReader, error) {
	if t.upMount != "" && name == t.upMount {
		return nil, fmt.Errorf("%w: the mount %q holds uploaded files; UpdateArtifact puts them", service.ErrInvalidArgument, name)
	}
	if _, err := t.reg.Get(ctx, name); err != nil {
		return nil, Err(err)
	}
	return reader{t.reg, name}, nil
}

type reader struct {
	reg  *mounts.Registry
	name string
}

func (r reader) Resolve(ctx context.Context, ref string) (string, bool, error) {
	v, moves, err := r.reg.ResolveAt(viewing(ctx), r.name, ref)
	return v, moves, Err(err)
}

func (r reader) List(ctx context.Context, version, under string) ([]service.MountFile, error) {
	files, err := r.reg.ListAt(viewing(ctx), r.name, version, under)
	if err != nil {
		return nil, Err(err)
	}
	out := make([]service.MountFile, len(files))
	for i, f := range files {
		out[i] = service.MountFile{Path: f.Path, ID: f.ID, Size: f.Size}
	}
	return out, nil
}

func (r reader) Fetch(ctx context.Context, version string, f service.MountFile) (string, bool, error) {
	h, fetched, err := r.reg.FetchAt(viewing(ctx), r.name, version, mounts.File{Path: f.Path, ID: f.ID, Size: f.Size})
	return string(h), fetched, Err(err)
}
