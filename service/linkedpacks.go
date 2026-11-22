package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/panyam/goapplib/storage/mounts"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// WithLinks lets the service link packs to project folders in mounts through readers (agni issue
// 1801), as DesignService.WithLinks links artifacts. uploads is the uploads mount, which a link may
// not name. src records the private source each link reads, and mountsOf finds the mounts a push
// names. Without readers, LinkPack and SyncPack are ErrFailedPrecondition.
func (s *PackService) WithLinks(readers MountReaders, uploads string, src SourceAccess, mountsOf MountsOf) *PackService {
	s.readers, s.uploadsMount, s.sources, s.mountsOf = readers, uploads, src, mountsOf
	return s
}

// WithGuard opens a pack read from a private source only for a caller whose own connection can read
// that source, as a design is opened (agni issue 1728). A pack g refuses is ErrPermissionDenied from
// GetPack and left out of ListPacks. A nil guard opens every pack. It also tells g which pack a file
// under the uploads mount's packs folder belongs to.
func (s *PackService) WithGuard(g *AccessGuard) *PackService {
	s.guard = g
	if g != nil {
		g.pack = func(ctx context.Context, id string) error {
			_, err := s.GetPack(ctx, &webapi.GetPackRequest{Name: PackName(id)})
			return err
		}
	}
	return s
}

// packLinked reports whether p was read from a mount by a link. Only a link records a read hash.
func packLinked(p *webapi.Pack) bool { return p.GetSource().GetReadHash() != "" }

// open returns ErrPermissionDenied when the caller may not read p's private source.
func (s *PackService) open(ctx context.Context, p *webapi.Pack) error {
	src := p.GetSource().GetAccessSource()
	if src == "" || s.guard == nil || ctx.Value(unguardedKey{}) != nil {
		return nil
	}
	if err := s.guard.Check(ctx, []string{src}); err != nil {
		if errors.Is(err, ErrPermissionDenied) {
			return fmt.Errorf("%w: %s opens only for who can read %s", ErrPermissionDenied, p.GetName(), src)
		}
		return err
	}
	return nil
}

// LinkPack reads a project folder in a mount into a pack, and records where it came from.
func (s *PackService) LinkPack(ctx context.Context, req *webapi.LinkPackRequest) (*webapi.Pack, error) {
	if s.store == nil {
		return nil, errNoPackStore
	}
	if s.readers == nil {
		return nil, fmt.Errorf("%w: this server links no packs; start it with --store and --uploads", ErrFailedPrecondition)
	}
	src := req.GetSource()
	if src.GetMount() == "" {
		return nil, fmt.Errorf("%w: the source names no mount", ErrInvalidArgument)
	}
	if src.GetMount() == s.uploadsMount {
		return nil, fmt.Errorf("%w: the mount %q holds uploaded files; ImportPackFiles puts a pack's files", ErrInvalidArgument, src.GetMount())
	}
	p, err := cleanSourcePath(src.GetPath())
	if err != nil {
		return nil, err
	}
	// The caller links only from a source they can read themselves, since the server's connection
	// reads it for them (agni issue 1728).
	if err := s.guard.Mount(ctx, src.GetMount()); err != nil {
		return nil, err
	}
	r, err := s.readers(ctx, src.GetMount())
	if err != nil {
		return nil, err
	}
	version, follow, err := r.Resolve(ctx, src.GetRef())
	if err != nil {
		return nil, err
	}
	return s.readLinked(ctx, r, req.GetPackId(), req.GetTitle(), &webapi.ArtifactSource{
		Mount: src.GetMount(), Path: p, Ref: src.GetRef(), Follow: follow, Version: version,
	})
}

// readLinked reads src's folder at src.version into the pack id, creating or replacing it, and
// records src with the read's time and hash.
func (s *PackService) readLinked(ctx context.Context, r MountReader, id, title string, src *webapi.ArtifactSource) (*webapi.Pack, error) {
	files, _, err := readSource(ctx, s.blobs, r, src.GetVersion(), src.GetPath())
	if err != nil {
		return nil, err
	}
	src.ReadTime = timestamppb.New(s.clock())
	src.ReadHash = manifestHash(files)
	if s.sources != nil {
		src.AccessSource = s.sources.Guarding(src.GetMount())
	}
	return s.keepFolder(ctx, id, title, files, src)
}

// UnlinkPack makes a linked pack an uploaded one, keeping its config and files.
func (s *PackService) UnlinkPack(ctx context.Context, req *webapi.UnlinkPackRequest) (*webapi.Pack, error) {
	cur, err := s.GetPack(ctx, &webapi.GetPackRequest{Name: req.GetName()})
	if err != nil {
		return nil, err
	}
	if !packLinked(cur) {
		return cur, nil
	}
	id, _ := PackID(cur.GetName())
	return s.store.UpdatePack(ctx, id, func(p *webapi.Pack) error {
		// The files keep the private source they came from, so unlinking them never opens them to a
		// caller who can't read it (agni issue 1728).
		p.Source = nil
		if as := cur.GetSource().GetAccessSource(); as != "" {
			p.Source = &webapi.ArtifactSource{AccessSource: as}
		}
		return nil
	}, nil)
}

// SyncPack brings a followed pack to its source's current version.
func (s *PackService) SyncPack(ctx context.Context, req *webapi.SyncPackRequest) (*webapi.SyncPackResponse, error) {
	if s.readers == nil {
		return nil, fmt.Errorf("%w: this server links no packs; start it with --store and --uploads", ErrFailedPrecondition)
	}
	cur, err := s.GetPack(ctx, &webapi.GetPackRequest{Name: req.GetName()})
	if err != nil {
		return nil, err
	}
	if !packLinked(cur) {
		return nil, fmt.Errorf("%w: %s is uploaded, not linked, so it has nothing to sync from", ErrInvalidArgument, cur.GetName())
	}
	return s.sync(ctx, cur, "")
}

// sync moves a linked pack to what its source names now. A pinned pack, or one already at the
// source's version, is returned as it is. version, when set, is what the source was resolved to by
// the caller, so a pass over many packs resolves each mount and ref once.
func (s *PackService) sync(ctx context.Context, cur *webapi.Pack, version string) (*webapi.SyncPackResponse, error) {
	src := cur.GetSource()
	resp := &webapi.SyncPackResponse{Pack: cur, FromVersion: src.GetVersion(), ToVersion: src.GetVersion()}
	if !src.GetFollow() {
		return resp, nil
	}
	r, err := s.readers(ctx, src.GetMount())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cur.GetName(), err)
	}
	if version == "" {
		if version, _, err = r.Resolve(ctx, src.GetRef()); err != nil {
			return nil, fmt.Errorf("%s: %w", cur.GetName(), err)
		}
	}
	resp.ToVersion = version
	if version != "" && version == src.GetVersion() {
		return resp, nil
	}
	id, _ := PackID(cur.GetName())
	next := &webapi.ArtifactSource{Mount: src.GetMount(), Path: src.GetPath(), Ref: src.GetRef(), Follow: true, Version: version}
	files, _, err := readSource(ctx, s.blobs, r, version, src.GetPath())
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cur.GetName(), err)
	}
	// A mount with no history names no version, so the files say whether the pack moved.
	if manifestHash(files) == src.GetReadHash() && version == src.GetVersion() {
		return resp, nil
	}
	out, err := s.readLinked(ctx, r, id, cur.GetTitle(), next)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cur.GetName(), err)
	}
	resp.Pack, resp.Changed = out, true
	return resp, nil
}

// PackMove is what SourceChanged or PollSources did to one pack.
type PackMove struct {
	// Pack is "packs/{pack}".
	Pack string
	// From and To are the versions the pack held before and after.
	From, To string
	// Err is why the pack couldn't move.
	Err error
}

// SourceChanged moves every linked pack that follows the ref c names in a mount of c.Source, as a
// push webhook reports it (agni issue 1801). It follows DesignService.SourceChanged's rule: a pack
// whose link names c.Ref, or names no ref when c.Ref is the default branch. The source is resolved
// again, so a late or forged delivery can't move a pack backward. It runs as the server, so it sees
// every pack.
func (s *PackService) SourceChanged(ctx context.Context, c mounts.Change) ([]PackMove, error) {
	if s.readers == nil || s.mountsOf == nil || s.store == nil {
		return nil, nil
	}
	if c.Deleted || c.Ref == "" {
		return nil, nil
	}
	names, err := s.mountsOf(ctx, c.Source)
	if err != nil {
		return nil, err
	}
	return s.catchUp(unguarded(ctx), func(src *webapi.ArtifactSource) bool {
		follows := src.GetRef() == c.Ref || src.GetRef() == "" && c.Default && c.RefKind == mounts.RefBranch
		seen := c.Version != "" && src.GetVersion() == c.Version
		return slices.Contains(names, src.GetMount()) && follows && !seen
	})
}

// PollSources makes one pass over every linked pack that follows a branch or a folder, as
// DesignService.PollSources does for designs.
func (s *PackService) PollSources(ctx context.Context) ([]PackMove, error) {
	if s.readers == nil || s.store == nil {
		return nil, nil
	}
	return s.catchUp(unguarded(ctx), func(*webapi.ArtifactSource) bool { return true })
}

// catchUp resolves each followed pack's mount and ref that want takes, once per pair, and syncs the
// packs that are behind.
func (s *PackService) catchUp(ctx context.Context, want func(*webapi.ArtifactSource) bool) ([]PackMove, error) {
	all, err := s.store.Packs(ctx)
	if err != nil {
		return nil, err
	}
	type key struct{ mount, ref string }
	type answer struct {
		version string
		err     error
	}
	answers := map[key]answer{}
	var out []PackMove
	for _, p := range all {
		src := p.GetSource()
		if !packLinked(p) || !src.GetFollow() || !want(src) {
			continue
		}
		k := key{src.GetMount(), src.GetRef()}
		ans, ok := answers[k]
		if !ok {
			r, err := s.readers(ctx, src.GetMount())
			if err == nil {
				ans.version, _, err = r.Resolve(ctx, src.GetRef())
			}
			ans.err = err
			answers[k] = ans
		}
		if ans.err != nil {
			out = append(out, PackMove{Pack: p.GetName(), From: src.GetVersion(), Err: ans.err})
			continue
		}
		if ans.version != "" && ans.version == src.GetVersion() {
			continue
		}
		resp, err := s.sync(ctx, p, ans.version)
		if err != nil {
			out = append(out, PackMove{Pack: p.GetName(), From: src.GetVersion(), To: ans.version, Err: err})
			continue
		}
		if resp.GetChanged() {
			out = append(out, PackMove{Pack: p.GetName(), From: resp.GetFromVersion(), To: resp.GetToVersion()})
		}
	}
	return out, nil
}

// LinkedTo names the packs linked to the mount named mount, sorted, so a mount with packs reading
// from it is not deleted under them.
func (s *PackService) LinkedTo(ctx context.Context, mount string) ([]string, error) {
	if s.store == nil || mount == s.uploadsMount {
		return nil, nil
	}
	all, err := s.store.Packs(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range all {
		if packLinked(p) && p.GetSource().GetMount() == mount {
			out = append(out, p.GetName())
		}
	}
	slices.Sort(out)
	return out, nil
}

// PackVersions maps each linked pack among packs that holds a version to it, for a report.
func PackVersions(packs []*webapi.Pack) map[string]string {
	var out map[string]string
	for _, p := range packs {
		if v := p.GetSource().GetVersion(); packLinked(p) && v != "" {
			if out == nil {
				out = map[string]string{}
			}
			out[p.GetName()] = v
		}
	}
	return out
}
