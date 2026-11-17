package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// DesignReader reads designs that stand alone, each by its id (agni issue 1693). A design ties to no
// project. It is its files plus the record its `design.yaml` holds, and a project lists designs by
// reference (agni issue 1694). A mounted folder is a DesignReader and nothing more, so its designs
// stay read-only.
//
// A design's name on the wire is "designs/{design}". Each method returns designs with that name set.
type DesignReader interface {
	// Design returns one design by id. An id naming nothing is ErrNotFound.
	Design(ctx context.Context, id string) (*webapi.Design, error)
	// Designs returns every design, ordered by name.
	Designs(ctx context.Context) ([]*webapi.Design, error)
}

// DesignStore is a DesignReader that also keeps the designs a server's users create. A design
// arrives as its FILES: its `design.yaml` and whatever else it holds, such as a netlist and a board.
// The files are a manifest of paths and hashes, and blobs supplies their contents, so nothing here
// names a host path. Each adapter decides how it keeps them. The folder adapter
// (`internal/projects.DirStore`) writes one folder per design, and a database adapter would write
// rows and blobs.
//
// The id comes from the call. A `design.yaml` that names a different id is ErrInvalidArgument, and
// one that names none takes the call's. A stored design stands alone, so an `extends` in its
// `design.yaml` is ErrInvalidArgument.
//
// Errors:
//   - ErrInvalidArgument: a malformed id, a path that leaves the design's folder, a missing or
//     unparseable `design.yaml`, or a file whose contents do not match its hash.
//   - ErrAlreadyExists: a create whose id is taken.
//   - ErrNotFound: a replace or a delete naming nothing this store keeps.
//   - *BlobsMissingError: files whose contents blobs does not hold, listing their hashes, so the
//     caller sends them with UploadBlobs and asks again.
//
// A replace swaps the whole file set at once, so a reader sees the old files or the new ones and
// never a mix.
type DesignStore interface {
	DesignReader
	CreateDesign(ctx context.Context, id string, files []*webapi.SnapshotFile, blobs BlobSource) (*webapi.Design, error)
	ReplaceDesign(ctx context.Context, id string, files []*webapi.SnapshotFile, blobs BlobSource) (*webapi.Design, error)
	DeleteDesign(ctx context.Context, id string) error
}

// BlobSource returns a file's contents by its hash, "sha256:<hex>" as SnapshotFile.sha256 writes it.
// A hash it does not hold answers ok=false, and the store then reports a *BlobsMissingError.
type BlobSource func(ctx context.Context, sha256 string) (b []byte, ok bool)

const designsNamePrefix = "designs/"

// StandaloneDesignName builds a standalone design's resource name, "designs/{design}". Agni issue
// 1694 moves the wire to this form, and the nested `projects/{p}/designs/{d}` of DesignName goes.
func StandaloneDesignName(id string) string { return designsNamePrefix + id }

// StandaloneDesignID extracts the id from "designs/{design}", with the containment checks of
// ProjectID.
func StandaloneDesignID(name string) (string, bool) {
	id, ok := strings.CutPrefix(name, designsNamePrefix)
	if !ok {
		return "", false
	}
	return id, validResourceID(id)
}

// DesignCatalog maps each design id to the store that holds it, so one server can keep designs in a
// folder, in object storage and in a database behind one interface. A design id is unique per
// server, and the catalog refuses a create whose id any store already holds.
//
// It writes through one DesignStore and reads through it and every mounted reader. A write naming a
// design that only a reader holds is ErrFailedPrecondition, since a mounted folder is read-only. With
// no store, every write is ErrFailedPrecondition.
//
// Two stores holding one id is an error on any read that meets it. Picking the first could answer a
// question about one design with another's files.
type DesignCatalog struct {
	store   DesignStore
	mounted []DesignReader
}

// NewDesignCatalog returns a catalog writing through store, which may be nil, and reading through
// store and each of mounted.
func NewDesignCatalog(store DesignStore, mounted ...DesignReader) *DesignCatalog {
	return &DesignCatalog{store: store, mounted: mounted}
}

func (c *DesignCatalog) readers() []DesignReader {
	if c.store == nil {
		return c.mounted
	}
	return append([]DesignReader{c.store}, c.mounted...)
}

// Design returns one design by resource name, "designs/{design}".
func (c *DesignCatalog) Design(ctx context.Context, name string) (*webapi.Design, error) {
	d, _, err := c.find(ctx, name)
	return d, err
}

// Designs returns every design in every store, ordered by name.
func (c *DesignCatalog) Designs(ctx context.Context) ([]*webapi.Design, error) {
	var out []*webapi.Design
	seen := map[string]bool{}
	for _, r := range c.readers() {
		ds, err := r.Designs(ctx)
		if err != nil {
			return nil, err
		}
		for _, d := range ds {
			if seen[d.GetName()] {
				return nil, fmt.Errorf("two stores hold %s; a design id is unique per server", d.GetName())
			}
			seen[d.GetName()] = true
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetName() < out[j].GetName() })
	return out, nil
}

// CreateDesign adds a design to the store, under an id no store holds.
func (c *DesignCatalog) CreateDesign(ctx context.Context, id string, files []*webapi.SnapshotFile, blobs BlobSource) (*webapi.Design, error) {
	if c.store == nil {
		return nil, errNoStore
	}
	name := StandaloneDesignName(id)
	if _, ok := StandaloneDesignID(name); !ok {
		return nil, fmt.Errorf("%w: %q is not a design id", ErrInvalidArgument, id)
	}
	for _, r := range c.mounted {
		if _, err := r.Design(ctx, id); err == nil {
			return nil, fmt.Errorf("%w: %s is declared in a mounted folder", ErrAlreadyExists, name)
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return c.store.CreateDesign(ctx, id, files, blobs)
}

// ReplaceDesign swaps the files of a design the store holds.
func (c *DesignCatalog) ReplaceDesign(ctx context.Context, name string, files []*webapi.SnapshotFile, blobs BlobSource) (*webapi.Design, error) {
	id, err := c.writable(ctx, name)
	if err != nil {
		return nil, err
	}
	return c.store.ReplaceDesign(ctx, id, files, blobs)
}

// DeleteDesign removes a design the store holds.
func (c *DesignCatalog) DeleteDesign(ctx context.Context, name string) error {
	id, err := c.writable(ctx, name)
	if err != nil {
		return err
	}
	return c.store.DeleteDesign(ctx, id)
}

var errNoStore = fmt.Errorf("%w: this server keeps no designs; start it with --store-dir", ErrFailedPrecondition)

// writable returns the id of a design the store holds, refusing one only a mounted reader holds.
func (c *DesignCatalog) writable(ctx context.Context, name string) (string, error) {
	if c.store == nil {
		return "", errNoStore
	}
	_, r, err := c.find(ctx, name)
	if err != nil {
		return "", err
	}
	if r != DesignReader(c.store) {
		return "", fmt.Errorf("%w: %s is declared in a mounted folder, which is read-only", ErrFailedPrecondition, name)
	}
	id, _ := StandaloneDesignID(name)
	return id, nil
}

// find returns a design and the reader holding it. Two readers holding it is an error.
func (c *DesignCatalog) find(ctx context.Context, name string) (*webapi.Design, DesignReader, error) {
	id, ok := StandaloneDesignID(name)
	if !ok {
		return nil, nil, fmt.Errorf("%w: %q is not a design resource name, designs/{design}", ErrInvalidArgument, name)
	}
	var found *webapi.Design
	var holder DesignReader
	for _, r := range c.readers() {
		d, err := r.Design(ctx, id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if found != nil {
			return nil, nil, fmt.Errorf("two stores hold %s; a design id is unique per server", name)
		}
		found, holder = d, r
	}
	if found == nil {
		return nil, nil, fmt.Errorf("%w: no design %s", ErrNotFound, name)
	}
	return found, holder, nil
}
