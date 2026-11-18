package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
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
	// Designs returns every design, ordered by name. A reader of folders it does not own may return
	// two designs with one name, when two folders declare one id. The catalog reports such a name as
	// unreachable.
	Designs(ctx context.Context) ([]*webapi.Design, error)
}

// DesignStore is a DesignReader that also keeps the designs a server's users create (agni issues
// 1693, 1696). A design is its record, the title and series it carries, plus its artifacts. An
// artifact is one source of the design, such as a schematic with its sheets, a board or a BOM, and it
// arrives as FILES: a manifest of paths and hashes, with blobs supplying the contents. Nothing here
// names a host path. Each adapter decides how it keeps a design. The folder adapter
// (`internal/projects.DirStore`) writes one folder per design and one subfolder per artifact, and a
// database adapter would write rows and blobs.
//
// The store reads each artifact's files to find its type and entry, and from those which artifact
// analysis reads. A write that leaves the design ambiguous is refused, so a stored design always
// reads one way.
//
// Errors:
//   - ErrInvalidArgument: a malformed id, a path that leaves its artifact, a file whose contents do
//     not match its hash, or an artifact whose files make more than one design.
//   - ErrAlreadyExists: a create whose id is taken.
//   - ErrNotFound: a design or an artifact this store does not keep, unless the write allows it
//     missing.
//   - ErrFailedPrecondition: a write leaving two netlists, two schematics and no netlist, or two
//     boards.
//   - *BlobsMissingError: files whose contents blobs does not hold, listing their hashes, so the
//     caller sends them with UploadBlobs and asks again.
//
// Every write changes the design at once, so a reader sees it before the write or after it and never
// a mix.
type DesignStore interface {
	DesignReader
	// CreateDesign makes a design with no artifacts.
	CreateDesign(ctx context.Context, id string, rec DesignRecord) (*webapi.Design, error)
	// UpdateDesign changes a design's record. update edits the record in place, under the store's lock.
	UpdateDesign(ctx context.Context, id string, update func(*DesignRecord)) (*webapi.Design, error)
	// DeleteDesign removes a design and its artifacts.
	DeleteDesign(ctx context.Context, id string) error
	// Artifacts returns a design's artifacts, ordered by name.
	Artifacts(ctx context.Context, id string) ([]*webapi.Artifact, error)
	// WriteArtifacts replaces or creates the artifacts writes name, all in one write, and returns them
	// in the order of writes.
	WriteArtifacts(ctx context.Context, id string, writes []ArtifactWrite, blobs BlobSource) ([]*webapi.Artifact, error)
	// DeleteArtifact removes one artifact.
	DeleteArtifact(ctx context.Context, id, artifact string) error
}

// DesignRecord is what a design says about itself apart from its artifacts. It is the part of
// `design.yaml` a caller sets.
type DesignRecord struct {
	Title            string
	Series           string
	FlattenHierarchy bool
	// Intent is the design's declared architecture (agni issue 1704), as `intent:` in a design.yaml.
	// Nil declares none. A pack's intent, or a request's, replaces it for one read.
	Intent *configpb.DesignIntent
}

// ArtifactWrite is one artifact's new files. AllowMissing creates the artifact when the design has
// none of that id (AIP-134).
type ArtifactWrite struct {
	ID           string
	Files        []*webapi.SnapshotFile
	AllowMissing bool
}

// BlobSource returns a file's contents by its hash, "sha256:<hex>" as SnapshotFile.sha256 writes it.
// A hash it does not hold answers ok=false, and the store then reports a *BlobsMissingError.
type BlobSource func(ctx context.Context, sha256 string) (b []byte, ok bool)

const designsNamePrefix = "designs/"

// StandaloneDesignName builds a standalone design's resource name, "designs/{design}". Agni issue
// 1694 moves the wire to this form, and the nested `projects/{p}/designs/{d}` of DesignName goes.
func StandaloneDesignName(id string) string { return designsNamePrefix + id }

const artifactsSegment = "/artifacts/"

// ArtifactName builds an artifact's resource name, "designs/{design}/artifacts/{artifact}".
func ArtifactName(design, artifact string) string {
	return StandaloneDesignName(design) + artifactsSegment + artifact
}

// ArtifactID splits "designs/{design}/artifacts/{artifact}" into its two ids.
func ArtifactID(name string) (design, artifact string, ok bool) {
	parent, a, found := strings.Cut(name, artifactsSegment)
	if !found {
		return "", "", false
	}
	d, ok := StandaloneDesignID(parent)
	if !ok || !validResourceID(a) {
		return "", "", false
	}
	return d, a, true
}

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
// Two places holding one id leave that id unreachable. Designs lists it in unreachable rather than
// failing the whole listing, and Design answers ErrFailedPrecondition. Picking either could answer a
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

// Designs returns every design in every store, ordered by name. Names held twice are left out and
// returned in unreachable, ordered.
func (c *DesignCatalog) Designs(ctx context.Context) (designs []*webapi.Design, unreachable []string, err error) {
	count := map[string]int{}
	var all []*webapi.Design
	for _, r := range c.readers() {
		ds, err := r.Designs(ctx)
		if err != nil {
			return nil, nil, err
		}
		for _, d := range ds {
			d.ReadOnly = c.store == nil || r != DesignReader(c.store)
			count[d.GetName()]++
			all = append(all, d)
		}
	}
	for _, d := range all {
		if count[d.GetName()] == 1 {
			designs = append(designs, d)
		}
	}
	for name, n := range count {
		if n > 1 {
			unreachable = append(unreachable, name)
		}
	}
	sort.Slice(designs, func(i, j int) bool { return designs[i].GetName() < designs[j].GetName() })
	sort.Strings(unreachable)
	return designs, unreachable, nil
}

// CreateDesign adds a design with no artifacts to the store, under an id no store holds.
func (c *DesignCatalog) CreateDesign(ctx context.Context, id string, rec DesignRecord) (*webapi.Design, error) {
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
	return c.store.CreateDesign(ctx, id, rec)
}

// UpdateDesign changes the record of a design the store holds.
func (c *DesignCatalog) UpdateDesign(ctx context.Context, name string, update func(*DesignRecord)) (*webapi.Design, error) {
	id, err := c.writable(ctx, name)
	if err != nil {
		return nil, err
	}
	return c.store.UpdateDesign(ctx, id, update)
}

// DeleteDesign removes a design the store holds.
func (c *DesignCatalog) DeleteDesign(ctx context.Context, name string) error {
	id, err := c.writable(ctx, name)
	if err != nil {
		return err
	}
	return c.store.DeleteDesign(ctx, id)
}

// Artifacts returns the artifacts of the design named "designs/{design}". A mounted design has none.
func (c *DesignCatalog) Artifacts(ctx context.Context, name string) ([]*webapi.Artifact, error) {
	_, r, err := c.find(ctx, name)
	if err != nil {
		return nil, err
	}
	if c.store == nil || r != DesignReader(c.store) {
		return nil, nil
	}
	id, _ := StandaloneDesignID(name)
	return c.store.Artifacts(ctx, id)
}

// WriteArtifacts writes artifacts of a design the store holds, all at once.
func (c *DesignCatalog) WriteArtifacts(ctx context.Context, name string, writes []ArtifactWrite, blobs BlobSource) ([]*webapi.Artifact, error) {
	id, err := c.writable(ctx, name)
	if err != nil {
		return nil, err
	}
	return c.store.WriteArtifacts(ctx, id, writes, blobs)
}

// DeleteArtifact removes the artifact named "designs/{design}/artifacts/{artifact}".
func (c *DesignCatalog) DeleteArtifact(ctx context.Context, name string) error {
	d, a, ok := ArtifactID(name)
	if !ok {
		return fmt.Errorf("%w: %q is not an artifact resource name, designs/{design}/artifacts/{artifact}", ErrInvalidArgument, name)
	}
	id, err := c.writable(ctx, StandaloneDesignName(d))
	if err != nil {
		return err
	}
	return c.store.DeleteArtifact(ctx, id, a)
}

// holder returns a design and whether the store holds it, rather than a mounted reader.
func (c *DesignCatalog) holder(ctx context.Context, name string) (*webapi.Design, bool, error) {
	d, r, err := c.find(ctx, name)
	if err != nil {
		return nil, false, err
	}
	return d, c.store != nil && r == DesignReader(c.store), nil
}

var errNoStore = fmt.Errorf("%w: this server keeps no designs; start it with --store", ErrFailedPrecondition)

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
			return nil, nil, fmt.Errorf("%w: two stores hold %s; a design id is unique per server", ErrFailedPrecondition, name)
		}
		found, holder = d, r
	}
	if found != nil {
		found.ReadOnly = c.store == nil || holder != DesignReader(c.store)
	}
	if found == nil {
		return nil, nil, fmt.Errorf("%w: no design %s", ErrNotFound, name)
	}
	return found, holder, nil
}
