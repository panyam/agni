package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// MountFile is one file a MountReader lists. Path is relative to the mount's root, with forward
// slashes. ID is the reader's own name for the file's contents, such as a git blob id, and is empty
// when the reader has none. Size is the file's size in bytes.
type MountFile struct {
	Path string
	ID   string
	Size int64
}

// MountReader reads one mount's files at a version, for linked artifacts (agni issue 1723). It
// classifies its errors with this package's sentinels: ErrNotFound for a ref or path the mount
// lacks, ErrUnavailable for a source it can't reach, and ErrResourceExhausted for one over its caps.
type MountReader interface {
	// Resolve names the version ref reads as now, a commit SHA on a mount with history and "" on one
	// without. follow is true when a later sync should move to a newer version: an empty ref, a
	// branch, or a mount with no history. A tag or a commit is pinned.
	Resolve(ctx context.Context, ref string) (version string, follow bool, err error)
	// List returns the files at version that are under, or are, the path under, sorted by path. An
	// empty under is the mount's root. A path naming no file or folder returns none.
	List(ctx context.Context, version, under string) ([]MountFile, error)
	// Fetch keeps f's contents in the server's blobs and returns their hash, "sha256:<hex>". fetched
	// is true when the reader had to download them because the server held no copy.
	Fetch(ctx context.Context, version string, f MountFile) (sha string, fetched bool, err error)
}

// MountReaders returns the reader for the mount named mount. An unknown mount is ErrNotFound, and one
// that can't be linked, such as the uploads mount, is ErrInvalidArgument.
type MountReaders func(ctx context.Context, mount string) (MountReader, error)

// WithLinks lets the service link artifacts to mounts through readers (agni issue 1723). uploads is
// the name of the server's uploads mount, which an uploaded artifact's source names and a link may
// not. Without it, LinkArtifact and SyncDesign are ErrFailedPrecondition.
func (s *DesignService) WithLinks(readers MountReaders, uploads string) *DesignService {
	s.readers, s.uploadsMount = readers, uploads
	return s
}

// noLinks is why a server without readers refuses a link or a sync.
const noLinks = "this server links no artifacts; start it with --store and --uploads"

// linked reports whether a was read from a mount by a link. Only a link records a read hash, so an
// uploaded artifact is never linked, whichever mount its store names for it.
func (s *DesignService) linked(a *webapi.Artifact) bool {
	return a.GetSource().GetReadHash() != ""
}

// LinkArtifact points an artifact at a path in a mount and reads its files.
func (s *DesignService) LinkArtifact(ctx context.Context, req *webapi.LinkArtifactRequest) (*webapi.Artifact, error) {
	if s.readers == nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedPrecondition, noLinks)
	}
	d, id, ok := ArtifactID(req.GetName())
	if !ok || !newIDPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: %q is not an artifact resource name, designs/{design}/artifacts/{artifact}, with an id of lowercase letters, digits, '.', '_' or '-'", ErrInvalidArgument, req.GetName())
	}
	src := req.GetSource()
	if src.GetMount() == "" {
		return nil, fmt.Errorf("%w: the source names no mount", ErrInvalidArgument)
	}
	if src.GetMount() == s.uploadsMount {
		return nil, fmt.Errorf("%w: the mount %q holds uploaded files; UpdateArtifact puts them", ErrInvalidArgument, src.GetMount())
	}
	p, err := cleanSourcePath(src.GetPath())
	if err != nil {
		return nil, err
	}
	// The caller links only from a source they can read themselves, since the server's connection
	// reads it for them (agni issue 1728).
	if err := s.catalog.guard.Mount(ctx, src.GetMount()); err != nil {
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
	if p, err = s.entryOf(ctx, r, version, p); err != nil {
		return nil, err
	}
	files, _, decl, err := s.readEntry(ctx, r, version, p)
	if err != nil {
		return nil, err
	}
	at := &webapi.ArtifactSource{
		Mount: src.GetMount(), Path: p, Ref: src.GetRef(), Follow: follow, Version: version,
		ReadTime: timestamppb.New(s.clock()), ReadHash: manifestHash(files),
	}
	writes := []ArtifactWrite{{ID: id, Files: files, AllowMissing: req.GetAllowMissing(), Source: at}}
	if decl != nil && len(decl.Companions) > 0 {
		have, err := s.catalog.Artifacts(ctx, StandaloneDesignName(d))
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		more, err := s.companionWrites(ctx, r, at, id, decl, have, writes)
		if err != nil {
			return nil, err
		}
		writes = append(writes, more...)
	}
	out, err := s.catalog.WriteArtifacts(ctx, StandaloneDesignName(d), writes, s.blobs)
	if err != nil {
		return nil, err
	}
	if err := s.keepDeclared(ctx, StandaloneDesignName(d), id, decl); err != nil {
		return nil, err
	}
	return out[0], nil
}

// entryOf returns the path to link for p. When p is a folder whose design.yaml names an entry, it is
// that entry, so a link of the folder links the design as the descriptor declares it and its
// companions follow (agni issue 1873). Otherwise it is p.
func (s *DesignService) entryOf(ctx context.Context, r MountReader, version, p string) (string, error) {
	listed, err := r.List(ctx, version, p)
	if err != nil || len(listed) == 0 || (len(listed) == 1 && listed[0].Path == p) {
		return p, err
	}
	desc := path.Join(p, DesignDescriptorName)
	i := slices.IndexFunc(listed, func(f MountFile) bool { return f.Path == desc })
	if i < 0 {
		return p, nil
	}
	h, _, err := r.Fetch(ctx, version, listed[i])
	if err != nil {
		return "", err
	}
	raw, ok := s.blobs(ctx, h)
	if !ok {
		return p, nil
	}
	var d struct {
		Entry string `yaml:"entry"`
	}
	if yaml.Unmarshal(raw, &d) != nil || d.Entry == "" {
		return p, nil
	}
	entry := path.Clean(d.Entry)
	if entry == ".." || strings.HasPrefix(entry, "../") || path.IsAbs(entry) {
		return "", fmt.Errorf("%w: %s names the entry %s outside its folder", ErrInvalidArgument, desc, d.Entry)
	}
	return path.Join(p, entry), nil
}

// companionWrites links each companion decl declares beside the entry linked at entry, on the same
// mount and ref, as an artifact of its own with the id push gives it (agni issue 1873). Each one
// names entryID as its companion_of (agni issue 1890). A companion is left out when the design
// already has an artifact of its id, or one linked to its path, or when pending writes take its id,
// so a link a person made, or an upload, is never replaced. Each write creates its artifact, except
// that an artifact linked to the companion's path and ref with no companion_of is stamped with it.
// A pending write of that artifact gets the stamp in place, and otherwise a write keeps its files.
func (s *DesignService) companionWrites(ctx context.Context, r MountReader, entry *webapi.ArtifactSource, entryID string, decl *entryDecl, have []*webapi.Artifact, pending []ArtifactWrite) ([]ArtifactWrite, error) {
	dir := companionDir(entry)
	taken := map[string]bool{}
	for _, w := range pending {
		taken[w.ID] = true
	}
	var out []ArtifactWrite
	for _, c := range decl.Companions {
		p := path.Join(dir, c.Path)
		if a := s.unstamped(entry, entryID, p, have); a != nil {
			_, id, _ := ArtifactID(a.GetName())
			i := slices.IndexFunc(pending, func(w ArtifactWrite) bool { return w.ID == id })
			switch {
			case i >= 0 && pending[i].Source != nil:
				pending[i].Source.CompanionOf = entryID
			case i < 0:
				out = append(out, stampWrite(a, entryID))
				taken[id] = true
			}
			continue
		}
		held := slices.ContainsFunc(have, func(a *webapi.Artifact) bool {
			_, id, _ := ArtifactID(a.GetName())
			src := a.GetSource()
			return id == c.ID || (s.linked(a) && src.GetMount() == entry.GetMount() && src.GetPath() == p)
		})
		if held || taken[c.ID] {
			continue
		}
		files, _, _, err := s.readEntry(ctx, r, entry.GetVersion(), p)
		if err != nil {
			return nil, fmt.Errorf("companion %s: %w", c.Path, err)
		}
		taken[c.ID] = true
		out = append(out, ArtifactWrite{ID: c.ID, Files: files, AllowMissing: true, Source: &webapi.ArtifactSource{
			Mount: entry.GetMount(), Path: p, Ref: entry.GetRef(), Follow: entry.GetFollow(), Version: entry.GetVersion(),
			ReadTime: entry.GetReadTime(), ReadHash: manifestHash(files), CompanionOf: entryID,
		}})
	}
	return out, nil
}

// companionDir is the folder of the entry linked at entry, "" at the mount's root. A design.yaml's
// companion paths are relative to it.
func companionDir(entry *webapi.ArtifactSource) string {
	if dir := path.Dir(entry.GetPath()); dir != "." {
		return dir
	}
	return ""
}

// unstamped returns the artifact of have linked to p on entry's mount and ref with no companion_of,
// other than entryID itself, or nil. A link of a declared entry before agni issue 1890 left its
// companions so.
func (s *DesignService) unstamped(entry *webapi.ArtifactSource, entryID, p string, have []*webapi.Artifact) *webapi.Artifact {
	for _, a := range have {
		_, id, _ := ArtifactID(a.GetName())
		src := a.GetSource()
		if id != entryID && s.linked(a) && src.GetCompanionOf() == "" && src.GetMount() == entry.GetMount() && src.GetPath() == p && src.GetRef() == entry.GetRef() {
			return a
		}
	}
	return nil
}

// stampWrite writes a again with its files and source, naming entryID as its companion_of.
func stampWrite(a *webapi.Artifact, entryID string) ArtifactWrite {
	_, id, _ := ArtifactID(a.GetName())
	src := proto.Clone(a.GetSource()).(*webapi.ArtifactSource)
	src.CompanionOf = entryID
	return ArtifactWrite{ID: id, Files: a.GetFiles(), AllowMissing: true, Source: src}
}

// stampCompanions names entryID as the companion_of of each artifact of design that a link of the
// entry before agni issue 1890 followed, so a bootstrap after an upgrade counts it as named. It
// reads the entry's design.yaml at the version the entry holds, and only when design has a linked
// artifact in the entry's folder on its mount with no companion_of. It reports whether it wrote.
func (s *DesignService) stampCompanions(ctx context.Context, design, entryID string) (bool, error) {
	if s.readers == nil {
		return false, nil
	}
	all, err := s.catalog.Artifacts(ctx, design)
	if err != nil {
		return false, err
	}
	i := slices.IndexFunc(all, func(a *webapi.Artifact) bool { _, id, _ := ArtifactID(a.GetName()); return id == entryID })
	if i < 0 || !s.linked(all[i]) {
		return false, nil
	}
	entry := all[i].GetSource()
	dir := companionDir(entry)
	candidate := slices.ContainsFunc(all, func(a *webapi.Artifact) bool {
		p := a.GetSource().GetPath()
		return a != all[i] && s.linked(a) && a.GetSource().GetCompanionOf() == "" && a.GetSource().GetMount() == entry.GetMount() && path.Dir(p) == path.Dir(entry.GetPath())
	})
	if !candidate {
		return false, nil
	}
	r, err := s.readers(ctx, entry.GetMount())
	if err != nil {
		return false, err
	}
	_, _, decl, err := s.readEntry(ctx, r, entry.GetVersion(), entry.GetPath())
	if err != nil || decl == nil {
		return false, err
	}
	var writes []ArtifactWrite
	for _, c := range decl.Companions {
		if a := s.unstamped(entry, entryID, path.Join(dir, c.Path), all); a != nil {
			writes = append(writes, stampWrite(a, entryID))
		}
	}
	if len(writes) == 0 {
		return false, nil
	}
	_, err = s.catalog.WriteArtifacts(ctx, design, writes, s.blobs)
	return err == nil, err
}

// keepDeclared stores what a linked entry's design.yaml declares, as ImportDesignFiles does for a
// pushed folder. The intent and flatten_hierarchy become the design's own (agni issues 1854, 1887),
// and a descriptor with no intent: section or flatten_hierarchy line leaves the kept one, so it
// never clears a value set with UpdateDesign.
// The artifact id becomes the design's Entry, so the design reads the entry the descriptor names
// rather than a schematic linked beside a board (agni issue 1858). A nil decl changes nothing.
func (s *DesignService) keepDeclared(ctx context.Context, design, id string, decl *entryDecl) error {
	if decl == nil {
		return nil
	}
	_, err := s.catalog.UpdateDesign(ctx, design, func(r *DesignRecord) {
		declared{flatten: decl.Flatten}.keep(r, decl.Intent)
		r.Entry = id
	})
	return err
}

// UnlinkArtifact makes a linked artifact an uploaded one, keeping its files.
func (s *DesignService) UnlinkArtifact(ctx context.Context, req *webapi.UnlinkArtifactRequest) (*webapi.Artifact, error) {
	a, err := s.GetArtifact(ctx, &webapi.GetArtifactRequest{Name: req.GetName()})
	if err != nil {
		return nil, err
	}
	if !s.linked(a) {
		return a, nil
	}
	d, id, _ := ArtifactID(a.GetName())
	// The files keep the private source they came from, so unlinking them never opens them to a
	// caller who can't read it (agni issue 1728).
	var keep *webapi.ArtifactSource
	if as := a.GetSource().GetAccessSource(); as != "" {
		keep = &webapi.ArtifactSource{Mount: s.uploadsMount, AccessSource: as}
	}
	out, err := s.catalog.WriteArtifacts(ctx, StandaloneDesignName(d), []ArtifactWrite{{ID: id, Files: a.GetFiles(), Source: keep}}, s.blobs)
	if err != nil {
		return nil, err
	}
	return out[0], nil
}

// SyncDesign brings a design's followed artifacts to their sources' current versions in one write.
func (s *DesignService) SyncDesign(ctx context.Context, req *webapi.SyncDesignRequest) (*webapi.SyncDesignResponse, error) {
	if s.readers == nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedPrecondition, noLinks)
	}
	all, err := s.catalog.Artifacts(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	byID := map[string]*webapi.Artifact{}
	for _, a := range all {
		_, id, _ := ArtifactID(a.GetName())
		byID[id] = a
	}
	var picked []*webapi.Artifact
	if len(req.GetArtifacts()) == 0 {
		for _, a := range all {
			if s.linked(a) {
				picked = append(picked, a)
			}
		}
	} else {
		for _, id := range req.GetArtifacts() {
			a, ok := byID[id]
			switch {
			case !ok:
				return nil, fmt.Errorf("%w: no artifact %s", ErrNotFound, ArtifactName(strings.TrimPrefix(req.GetName(), designsNamePrefix), id))
			case !s.linked(a):
				return nil, fmt.Errorf("%w: %s is uploaded, not linked, so it has nothing to sync from", ErrInvalidArgument, a.GetName())
			}
			if !slices.Contains(picked, a) {
				picked = append(picked, a)
			}
		}
	}
	slices.SortFunc(picked, func(a, b *webapi.Artifact) int { return strings.Compare(a.GetName(), b.GetName()) })

	// One reader and one resolved version per mount and ref, so artifacts from one repo move to one
	// commit together.
	type key struct{ mount, ref string }
	readers := map[key]MountReader{}
	versions := map[key]string{}
	resp := &webapi.SyncDesignResponse{}
	var writes []ArtifactWrite
	// downloaded holds every file this sync downloaded, by hash. Proposing one artifact can download
	// a file another artifact holds, so each row counts the downloads among its own files.
	downloaded := map[string]bool{}
	type rowFiles struct {
		row   *webapi.ArtifactSync
		files []*webapi.SnapshotFile
	}
	var rows []rowFiles
	now := timestamppb.New(s.clock())
	// decl is what the design.yaml beside a rewritten entry declares now, kept after the write for
	// the artifact entryID.
	var decl *entryDecl
	var entryID string
	var entryReader MountReader
	var entrySrc *webapi.ArtifactSource
	for _, a := range picked {
		src := a.GetSource()
		row := &webapi.ArtifactSync{Artifact: a.GetName(), FromVersion: src.GetVersion(), ToVersion: src.GetVersion()}
		resp.Artifacts = append(resp.Artifacts, row)
		if !src.GetFollow() {
			continue
		}
		k := key{src.GetMount(), src.GetRef()}
		r, ok := readers[k]
		if !ok {
			if r, err = s.readers(ctx, src.GetMount()); err != nil {
				return nil, fmt.Errorf("%s: %w", a.GetName(), err)
			}
			v, _, err := r.Resolve(ctx, src.GetRef())
			if err != nil {
				return nil, fmt.Errorf("%s: %w", a.GetName(), err)
			}
			readers[k], versions[k] = r, v
		}
		version := versions[k]
		row.ToVersion = version
		if version != "" && version == src.GetVersion() {
			continue
		}
		if req.GetValidateOnly() && version != "" {
			row.Changed = true
			continue
		}
		// A mount with no history names no version, so the files are read to see whether they moved.
		files, got, declared, err := s.readEntry(ctx, r, version, src.GetPath())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", a.GetName(), err)
		}
		for _, h := range got {
			downloaded[h] = true
		}
		rows = append(rows, rowFiles{row, files})
		hash := manifestHash(files)
		row.Changed = hash != src.GetReadHash()
		if !row.Changed && version == src.GetVersion() {
			continue
		}
		_, id, _ := ArtifactID(a.GetName())
		next := &webapi.ArtifactSource{Mount: src.GetMount(), Path: src.GetPath(), Ref: src.GetRef(), Follow: true, Version: version, ReadTime: now, ReadHash: hash, CompanionOf: src.GetCompanionOf()}
		writes = append(writes, ArtifactWrite{ID: id, Files: files, Source: next})
		if declared != nil {
			decl, entryID, entryReader, entrySrc = declared, id, r, next
		}
	}
	for _, rf := range rows {
		for _, f := range rf.files {
			if downloaded[f.GetSha256()] {
				rf.row.FilesFetched++
				rf.row.BytesFetched += f.GetSize()
			}
		}
	}
	if len(writes) == 0 || req.GetValidateOnly() {
		return resp, nil
	}
	// A companion the descriptor declares now and the design lacks is linked with the entry, so a
	// companion added to design.yaml reaches the server on the next sync (agni issue 1873).
	if decl != nil && len(decl.Companions) > 0 {
		more, err := s.companionWrites(ctx, entryReader, entrySrc, entryID, decl, all, writes)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ArtifactName(strings.TrimPrefix(req.GetName(), designsNamePrefix), entryID), err)
		}
		for _, w := range more {
			resp.Artifacts = append(resp.Artifacts, &webapi.ArtifactSync{
				Artifact: ArtifactName(strings.TrimPrefix(req.GetName(), designsNamePrefix), w.ID), ToVersion: w.Source.GetVersion(), Changed: true,
			})
		}
		writes = append(writes, more...)
	}
	if _, err := s.catalog.WriteArtifacts(ctx, req.GetName(), writes, s.blobs); err != nil {
		return nil, err
	}
	if err := s.keepDeclared(ctx, req.GetName(), entryID, decl); err != nil {
		return nil, err
	}
	return resp, nil
}

// LinkedTo names the stored artifacts whose source is the mount named mount, sorted, so a mount
// with artifacts reading from it is not deleted under them.
func (s *DesignService) LinkedTo(ctx context.Context, mount string) ([]string, error) {
	if mount == s.uploadsMount {
		return nil, nil
	}
	// Every design counts, including ones the caller can't open, or a mount would be deleted under
	// a design its deleter can't see.
	ctx = unguarded(ctx)
	designs, _, err := s.catalog.Designs(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, d := range designs {
		arts, err := s.catalog.Artifacts(ctx, d.GetName())
		if err != nil {
			// A mounted design has no artifacts and no record to read them from.
			continue
		}
		for _, a := range arts {
			if s.linked(a) && a.GetSource().GetMount() == mount {
				out = append(out, a.GetName())
			}
		}
	}
	slices.Sort(out)
	return out, nil
}

// refuseLinked refuses a write of uploaded files over any of ids that is linked, so a sync never
// overwrites what a person put by hand, and a put never hides a link.
func (s *DesignService) refuseLinked(ctx context.Context, design string, ids []string) error {
	all, err := s.catalog.Artifacts(ctx, design)
	if err != nil {
		// The write itself reports a missing or mounted design.
		return nil
	}
	for _, a := range all {
		_, id, _ := ArtifactID(a.GetName())
		if slices.Contains(ids, id) && s.linked(a) {
			src := a.GetSource()
			return fmt.Errorf("%w: %s is linked to %s; UnlinkArtifact it before putting files", ErrFailedPrecondition, a.GetName(), SourceString(src))
		}
	}
	return nil
}

// SourceString writes a source the way `agni designs link` takes it, "jetson/hw/board@1.1.2".
func SourceString(src *webapi.ArtifactSource) string {
	out := src.GetMount()
	if src.GetPath() != "" {
		out += "/" + src.GetPath()
	}
	if src.GetRef() != "" {
		out += "@" + src.GetRef()
	}
	return out
}

// readSource reads the files a link at p takes, at version, keeping each in the blobs. A path
// naming a folder takes every file under it, with paths relative to it. A path naming a file that
// is a design's entry or companion takes the files the design's artifact holds, grouped as
// PlanArtifacts groups a folder, so a KiCad root sheet brings its child sheets. Any other file is
// taken alone. It also returns the hashes of the files it downloaded, which can include files read
// only to find the design.
func (s *DesignService) readSource(ctx context.Context, r MountReader, version, p string) ([]*webapi.SnapshotFile, []string, error) {
	return readSource(ctx, s.blobs, r, version, p)
}

// entryDecl is what the design.yaml beside a linked path declares when it names that path as its
// entry. Intent is nil when it declares none.
type entryDecl struct {
	Intent *configpb.DesignIntent
	// Flatten is the descriptor's flatten_hierarchy, nil when it has no such line (agni issue 1887).
	Flatten *bool
	// Companions are the files the descriptor's companions: names, each with the artifact id
	// PlanArtifacts gives it, so a link of the entry links them as push keeps them (agni issue 1873).
	Companions []declaredCompanion
}

// declaredCompanion is one companion a design.yaml declares. Path is relative to the descriptor's
// folder.
type declaredCompanion struct {
	ID, Path string
}

// readEntry is readSource, plus what the design.yaml declares when p is its entry, nil otherwise.
func (s *DesignService) readEntry(ctx context.Context, r MountReader, version, p string) ([]*webapi.SnapshotFile, []string, *entryDecl, error) {
	return readLinkedSource(ctx, s.blobs, r, version, p)
}

// readSource is DesignService.readSource over blobs, so a linked pack reads its folder the same way.
func readSource(ctx context.Context, blobs BlobSource, r MountReader, version, p string) ([]*webapi.SnapshotFile, []string, error) {
	files, fetched, _, err := readLinkedSource(ctx, blobs, r, version, p)
	return files, fetched, err
}

// readLinkedSource is readSource, plus the intent the design.yaml beside p declares when p is that
// descriptor's entry, so a link keeps the intent a push of the folder keeps (agni issue 1854). The
// intent is nil when p is no descriptor's entry.
func readLinkedSource(ctx context.Context, blobs BlobSource, r MountReader, version, p string) ([]*webapi.SnapshotFile, []string, *entryDecl, error) {
	listed, err := r.List(ctx, version, p)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(listed) == 0 {
		at := ""
		if version != "" {
			at = " at " + version
		}
		return nil, nil, nil, fmt.Errorf("%w: the mount has no file or folder %q%s", ErrNotFound, p, at)
	}
	var fetched []string
	hashes := map[string]string{}
	fetch := func(f MountFile) (string, error) {
		if h, ok := hashes[f.Path]; ok {
			return h, nil
		}
		h, got, err := r.Fetch(ctx, version, f)
		if err != nil {
			return "", err
		}
		if got {
			fetched = append(fetched, h)
		}
		hashes[f.Path] = h
		return h, nil
	}
	take := func(f MountFile, rel string) (*webapi.SnapshotFile, error) {
		h, err := fetch(f)
		if err != nil {
			return nil, err
		}
		return &webapi.SnapshotFile{Path: rel, Sha256: h, Size: f.Size}, nil
	}
	var out []*webapi.SnapshotFile
	if len(listed) > 1 || listed[0].Path != p {
		for _, f := range listed {
			rel := f.Path
			if p != "" {
				rel = strings.TrimPrefix(f.Path, p+"/")
			}
			sf, err := take(f, rel)
			if err != nil {
				return nil, nil, nil, err
			}
			out = append(out, sf)
		}
		return out, fetched, nil, nil
	}
	dir := path.Dir(p)
	if dir == "." {
		dir = ""
	}
	siblings, err := r.List(ctx, version, dir)
	if err != nil {
		return nil, nil, nil, err
	}
	byRel := map[string]MountFile{}
	var rels []string
	for _, f := range siblings {
		rel := f.Path
		if dir != "" {
			rel = strings.TrimPrefix(f.Path, dir+"/")
		}
		byRel[rel] = f
		rels = append(rels, rel)
	}
	var readErr error
	read := func(rel string) ([]byte, error) {
		f, ok := byRel[rel]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, rel)
		}
		h, err := fetch(f)
		if err != nil {
			readErr = err
			return nil, err
		}
		b, _ := blobs(ctx, h)
		return b, nil
	}
	_, group := artifactGroup(rels, read, path.Base(p))
	if readErr != nil {
		return nil, nil, nil, readErr
	}
	if group == nil {
		sf, err := take(listed[0], path.Base(p))
		if err != nil {
			return nil, nil, nil, err
		}
		return []*webapi.SnapshotFile{sf}, fetched, nil, nil
	}
	for _, pf := range group {
		sf, err := take(byRel[pf.Folder], pf.Rel)
		if err != nil {
			return nil, nil, nil, err
		}
		out = append(out, sf)
	}
	slices.SortFunc(out, func(a, b *webapi.SnapshotFile) int { return strings.Compare(a.GetPath(), b.GetPath()) })
	decl, err := entryDeclared(rels, read, path.Base(p))
	if err != nil {
		return nil, nil, nil, err
	}
	return out, fetched, decl, nil
}

// entryDeclared is what the design.yaml among a folder's files declares, when that descriptor names
// target as its entry. A companion such as the board is linked on its own, so its link reads
// nothing. A folder with no descriptor, or one naming another entry, gives nil.
func entryDeclared(files []string, read func(string) ([]byte, error), target string) (*entryDecl, error) {
	if !slices.Contains(files, DesignDescriptorName) {
		return nil, nil
	}
	d, err := readDeclared(files, read)
	if err != nil {
		return nil, err
	}
	if d.entry == "" || d.entry != target {
		return nil, nil
	}
	intent, err := d.Intent()
	if err != nil {
		return nil, err
	}
	decl := &entryDecl{Intent: intent, Flatten: d.flatten}
	for _, c := range d.companions {
		if c == target || !slices.Contains(files, c) {
			continue
		}
		if id, _ := artifactGroup(files, read, c); id != "" {
			decl.Companions = append(decl.Companions, declaredCompanion{ID: id, Path: c})
		}
	}
	return decl, nil
}

// artifactGroup finds the design among a folder's files whose entry or companion is target, and
// returns the id and files PlanArtifacts gives that artifact. It returns "" and nil when target is
// neither.
func artifactGroup(files []string, read func(string) ([]byte, error), target string) (string, []PlannedFile) {
	// A design.yaml makes ProposeFromFiles propose the folder as one declared design, with no entry
	// or companions to match target against. So it proposes without the descriptors, as
	// PlanArtifacts does (agni issue 1851).
	var kept []string
	for _, f := range files {
		if b := path.Base(f); b != DesignDescriptorName && b != ProjectDescriptorName {
			kept = append(kept, f)
		}
	}
	resp := ProposeFromFiles("", kept, read)
	for _, d := range resp.GetDesigns() {
		named := d.GetDesign().GetEntryUri() == target || slices.Contains(d.GetDesign().GetCompanionUris(), target)
		if !named {
			continue
		}
		plan, err := PlanArtifacts(d.GetFiles(), read)
		if err != nil {
			return "", nil
		}
		for _, id := range plan.IDs {
			if slices.ContainsFunc(plan.Files[id], func(pf PlannedFile) bool { return pf.Folder == target }) {
				return id, plan.Files[id]
			}
		}
	}
	return "", nil
}

// manifestHash hashes files' paths and contents' hashes in path order, so two reads of the same
// files give one hash.
func manifestHash(files []*webapi.SnapshotFile) string {
	sorted := slices.Clone(files)
	slices.SortFunc(sorted, func(a, b *webapi.SnapshotFile) int { return strings.Compare(a.GetPath(), b.GetPath()) })
	h := sha256.New()
	for _, f := range sorted {
		fmt.Fprintf(h, "%s\x00%s\n", f.GetPath(), f.GetSha256())
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// cleanSourcePath checks a link's path: relative, with forward slashes, and never climbing out.
func cleanSourcePath(p string) (string, error) {
	if p == "" || p == "." || p == "/" {
		return "", nil
	}
	if strings.Contains(p, "\\") || strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("%w: path %q must be relative to the mount, with forward slashes", ErrInvalidArgument, p)
	}
	c := path.Clean(p)
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", fmt.Errorf("%w: path %q leaves the mount", ErrInvalidArgument, p)
	}
	return c, nil
}

// clock is the time a link or sync is stamped with.
func (s *DesignService) clock() time.Time {
	if s.now != nil {
		return s.now().UTC().Truncate(time.Second)
	}
	return time.Now().UTC().Truncate(time.Second)
}
