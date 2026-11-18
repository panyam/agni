package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// PackStore keeps the packs a server holds (agni issue 1704). A pack is an AnalysisConfig, the shape
// `project.yaml` binds to, plus the files its relative paths name. Like DesignStore it is a port. The
// folder adapter is `internal/projects.DirPacks`, and a database adapter would write rows and blobs.
//
// Each method returns packs with name, uri, hash and update_time set. A pack's uri is the folder its
// files are read from, so config's relative paths resolve against it, and it is empty for a pack with
// no files.
//
// Errors:
//   - ErrInvalidArgument: a malformed id, or a file path that leaves the pack.
//   - ErrAlreadyExists: a create whose id is taken.
//   - ErrNotFound: a pack this store does not keep.
//   - ErrFailedPrecondition: a pack with files on a store that keeps no files.
//   - *BlobsMissingError: files whose contents neither blobs nor the store holds.
type PackStore interface {
	// Pack returns one pack by id.
	Pack(ctx context.Context, id string) (*webapi.Pack, error)
	// Packs returns every pack, ordered by name.
	Packs(ctx context.Context) ([]*webapi.Pack, error)
	// CreatePack keeps a new pack from p's title, config and files.
	CreatePack(ctx context.Context, id string, p *webapi.Pack, blobs BlobSource) (*webapi.Pack, error)
	// UpdatePack changes a pack. update edits a copy of the pack under the store's lock, and the store
	// keeps the copy's title, config and files.
	UpdatePack(ctx context.Context, id string, update func(*webapi.Pack) error, blobs BlobSource) (*webapi.Pack, error)
	// DeletePack removes a pack.
	DeletePack(ctx context.Context, id string) error
}

const packsNamePrefix = "packs/"

var errNoPackStore = fmt.Errorf("%w: this server keeps no packs; start it with --store", ErrFailedPrecondition)

// PackName builds a pack's resource name, "packs/{pack}".
func PackName(id string) string { return packsNamePrefix + id }

// PackID extracts the id from "packs/{pack}", with the containment checks of ProjectID.
func PackID(name string) (string, bool) {
	id, ok := strings.CutPrefix(name, packsNamePrefix)
	if !ok {
		return "", false
	}
	return id, validResourceID(id)
}

// PackHash names a pack's config and files, "sha256:<hex>". It hashes the config's deterministic
// encoding and each file's path and content hash, sorted by path. A file's contents are named by
// their hash, so a changed profile gives a changed pack hash. A profile changed in place under a
// `mount://` URI does not, since the pack holds only its URI.
func PackHash(p *webapi.Pack) string {
	h := sha256.New()
	b, _ := proto.MarshalOptions{Deterministic: true}.Marshal(p.GetConfig())
	h.Write(b)
	files := slices.Clone(p.GetFiles())
	slices.SortFunc(files, func(a, b *webapi.SnapshotFile) int { return strings.Compare(a.GetPath(), b.GetPath()) })
	for _, f := range files {
		fmt.Fprintf(h, "\x00%s\x00%s", f.GetPath(), f.GetSha256())
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// PackLookup finds a pack by its resource name, "packs/{pack}". An unknown name is ErrNotFound.
type PackLookup interface {
	GetPack(ctx context.Context, req *webapi.GetPackRequest) (*webapi.Pack, error)
}

// ResolvePacks looks up the packs names lists, in the order named. A name listed twice is
// INVALID_ARGUMENT, and a host with no lookup is FAILED_PRECONDITION. No names returns nil.
//
// OverlayFor layers the packs it returns, each as a tier of its own: a later pack's convention,
// intent and params replace an earlier one's, and its profiles, symbols and library accumulate, as a
// request's config layers over a project's.
func ResolvePacks(ctx context.Context, lookup PackLookup, names []string) ([]*webapi.Pack, error) {
	if len(names) == 0 {
		return nil, nil
	}
	if lookup == nil {
		return nil, fmt.Errorf("%w: this host keeps no packs, so it cannot read %s", ErrFailedPrecondition, strings.Join(names, ", "))
	}
	var read []*webapi.Pack
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			return nil, fmt.Errorf("%w: %s is named twice", ErrInvalidArgument, name)
		}
		seen[name] = true
		p, err := lookup.GetPack(ctx, &webapi.GetPackRequest{Name: name})
		if err != nil {
			return nil, err
		}
		read = append(read, p)
	}
	return read, nil
}

// packConfigURIs returns a copy of p's config with every relative path rewritten under p's uri.
func packConfigURIs(p *webapi.Pack) (*webapi.AnalysisConfig, error) {
	cfg := proto.Clone(p.GetConfig()).(*webapi.AnalysisConfig)
	if cfg == nil {
		cfg = &webapi.AnalysisConfig{}
	}
	rewrite := func(refs []string) ([]string, error) {
		out := make([]string, 0, len(refs))
		for _, r := range refs {
			if strings.Contains(r, "://") {
				out = append(out, r)
				continue
			}
			if p.GetUri() == "" {
				return nil, fmt.Errorf("%w: %s names %q, and holds no files for it to name", ErrFailedPrecondition, p.GetName(), r)
			}
			base, err := ParseArtifactURI(p.GetUri())
			if err != nil {
				return nil, err
			}
			u, err := base.Join(r)
			if err != nil {
				return nil, fmt.Errorf("%w: %s: %v", ErrInvalidArgument, p.GetName(), err)
			}
			out = append(out, u.String())
		}
		return out, nil
	}
	var err error
	for _, refs := range []*[]string{&cfg.ProfileUris, &cfg.ParamUris, &cfg.SymbolPathUris, &cfg.LibraryUris, &cfg.QuerySetUris} {
		if *refs, err = rewrite(*refs); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// checkPack refuses a pack whose config cannot be read the way a pack is read: one that extends
// another, or names a relative path no file of its own sits at.
func checkPack(p *webapi.Pack) error {
	cfg := p.GetConfig()
	if cfg.GetExtends() != "" {
		return fmt.Errorf("%w: a pack extends nothing; name the packs to layer on the snapshot instead (extends %q)", ErrInvalidArgument, cfg.GetExtends())
	}
	var paths []string
	for _, f := range p.GetFiles() {
		paths = append(paths, path.Clean(f.GetPath()))
	}
	for _, refs := range [][]string{cfg.GetProfileUris(), cfg.GetParamUris(), cfg.GetSymbolPathUris(), cfg.GetLibraryUris(), cfg.GetQuerySetUris()} {
		for _, r := range refs {
			if strings.Contains(r, "://") {
				continue
			}
			clean := path.Clean(r)
			if clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
				return fmt.Errorf("%w: %q leaves the pack; name a path among its files or a mount:// URI", ErrInvalidArgument, r)
			}
			if !slices.ContainsFunc(paths, func(f string) bool { return clean == "." || f == clean || strings.HasPrefix(f, clean+"/") }) {
				return fmt.Errorf("%w: %q names no file among the pack's files", ErrInvalidArgument, r)
			}
		}
	}
	return nil
}

// PackService serves the pack resource, "packs/{pack}", over a PackStore (agni issue 1704). It does
// no I/O itself. It validates requests and applies AIP-158 pagination.
type PackService struct {
	store PackStore
	blobs BlobSource
}

// NewPackService returns a PackService over store, reading uploaded file contents from blobs. A nil
// store keeps no packs: a read answers NOT_FOUND and a write FAILED_PRECONDITION naming --store.
func NewPackService(store PackStore, blobs BlobSource) *PackService {
	if blobs == nil {
		blobs = func(context.Context, string) ([]byte, bool) { return nil, false }
	}
	return &PackService{store: store, blobs: blobs}
}

// CreatePack keeps a new pack.
func (s *PackService) CreatePack(ctx context.Context, req *webapi.CreatePackRequest) (*webapi.Pack, error) {
	if s.store == nil {
		return nil, errNoPackStore
	}
	if !newIDPattern.MatchString(req.GetPackId()) {
		return nil, fmt.Errorf("%w: pack id %q must be lowercase letters, digits, '.', '_' or '-', starting with a letter or digit", ErrInvalidArgument, req.GetPackId())
	}
	if err := checkPack(req.GetPack()); err != nil {
		return nil, err
	}
	return s.store.CreatePack(ctx, req.GetPackId(), req.GetPack(), s.blobs)
}

// GetPack returns one pack by resource name. A malformed name is INVALID_ARGUMENT, and a pack the
// server lacks is NOT_FOUND.
func (s *PackService) GetPack(ctx context.Context, req *webapi.GetPackRequest) (*webapi.Pack, error) {
	id, ok := PackID(req.GetName())
	if !ok {
		return nil, fmt.Errorf("%w: %q is not a pack resource name, packs/{pack}", ErrInvalidArgument, req.GetName())
	}
	if s.store == nil {
		return nil, fmt.Errorf("%w: no pack %s; this server keeps no packs (start it with --store)", ErrNotFound, req.GetName())
	}
	return s.store.Pack(ctx, id)
}

// ListPacks returns the packs this server holds, ordered by name.
func (s *PackService) ListPacks(ctx context.Context, req *webapi.ListPacksRequest) (*webapi.ListPacksResponse, error) {
	if s.store == nil {
		return &webapi.ListPacksResponse{}, nil
	}
	all, err := s.store.Packs(ctx)
	if err != nil {
		return nil, err
	}
	page, next := paginate(len(all), req.GetPageSize(), req.GetPageToken(), func(i int) string { return all[i].GetName() })
	resp := &webapi.ListPacksResponse{NextPageToken: next}
	for _, i := range page {
		resp.Packs = append(resp.Packs, all[i])
	}
	return resp, nil
}

// UpdatePack changes a pack's title, config or files, as update_mask names.
func (s *PackService) UpdatePack(ctx context.Context, req *webapi.UpdatePackRequest) (*webapi.Pack, error) {
	if s.store == nil {
		return nil, errNoPackStore
	}
	in := req.GetPack()
	id, ok := PackID(in.GetName())
	if !ok {
		return nil, fmt.Errorf("%w: %q is not a pack resource name, packs/{pack}", ErrInvalidArgument, in.GetName())
	}
	paths := req.GetUpdateMask().GetPaths()
	if len(paths) == 0 {
		paths = []string{"title", "config", "files"}
	}
	for _, p := range paths {
		if p != "title" && p != "config" && p != "files" {
			return nil, fmt.Errorf("%w: update_mask path %q; a pack's title, config and files can be changed", ErrInvalidArgument, p)
		}
	}
	return s.store.UpdatePack(ctx, id, func(p *webapi.Pack) error {
		for _, f := range paths {
			switch f {
			case "title":
				p.Title = in.GetTitle()
			case "config":
				p.Config = in.GetConfig()
			case "files":
				p.Files = in.GetFiles()
			}
		}
		return checkPack(p)
	}, s.blobs)
}

// DeletePack removes a pack.
func (s *PackService) DeletePack(ctx context.Context, req *webapi.DeletePackRequest) error {
	id, ok := PackID(req.GetName())
	if !ok {
		return fmt.Errorf("%w: %q is not a pack resource name, packs/{pack}", ErrInvalidArgument, req.GetName())
	}
	if s.store == nil {
		return fmt.Errorf("%w: no pack %s", ErrNotFound, req.GetName())
	}
	return s.store.DeletePack(ctx, id)
}
