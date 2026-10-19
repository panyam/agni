package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/core/timing"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/formats"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// A snapshot is a design loaded once and asked many questions (agni issue 1061). Ingest resolves its
// tiers and composes its config, and every analysis rpc names it rather than files, so no question
// resolves a companion, a project or a convention again.
//
// Snapshots are held on the ProjectResolver the services share, so a host that composes its services
// over one resolver has one set of snapshots. Each holds a recorder of every file its questions read,
// and a question asked after one of them changed is refused as stale rather than answered from files
// the snapshot never read.

var (
	// ErrStale is a snapshot whose files changed since they were read. UpdateSnapshot reads it again.
	ErrStale = errors.New("stale")
	// ErrAlreadyExists is an Ingest naming a snapshot id already in use.
	ErrAlreadyExists = errors.New("already exists")
)

// SnapshotError is why a question's snapshot cannot answer it: the host holds no snapshot by that
// name, or its files changed. It wraps ErrNotFound or ErrStale, and a transport carries it as a
// webapi.SnapshotUnavailable detail so a client can tell it from a design file that is missing.
type SnapshotError struct {
	Name    string
	Stale   bool
	Changed []string
}

func (e *SnapshotError) Error() string {
	if e.Stale {
		return fmt.Sprintf("snapshot %s is stale: %s changed since it was read; update it", e.Name, strings.Join(e.Changed, ", "))
	}
	return fmt.Sprintf("snapshot %s not found", e.Name)
}

func (e *SnapshotError) Unwrap() error {
	if e.Stale {
		return ErrStale
	}
	return ErrNotFound
}

// Detail is the error as the wire carries it.
func (e *SnapshotError) Detail() *webapi.SnapshotUnavailable {
	d := &webapi.SnapshotUnavailable{Name: e.Name, Reason: webapi.SnapshotUnavailable_REASON_MISSING}
	if e.Stale {
		d.Reason, d.Changed = webapi.SnapshotUnavailable_REASON_STALE, e.Changed
	}
	return d
}

// Snap is a snapshot as a service reads it: what its definition resolved to.
type Snap struct {
	Name string
	// Design is the design as the definition names it, which a service still needs for what sits
	// beside it (an expectation sidecar, a project's checklists and query sets).
	Design                   artifact.URI
	Netlist, Board, Geometry artifact.URI
	AsNamed                  bool
	Overlay                  Overlay
	// Config is the overlay config the definition carried, before it was composed with the project's.
	Config   *webapi.OverlayConfig
	Revision string
	// Loader and Projects are what a snapshot of files the client sent reads through, and nil for one
	// of files the host serves, which reads through the service's own (agni issue 1061). They are
	// apart from the host's because a cache keys a read on its URI, and an upload names its files
	// with the client's own mount, which may be one the host also serves.
	Loader   *CachingLoader
	Projects *ProjectResolver
}

// LoaderOr is the loader to read s through: its upload's, or the service's l.
func LoaderOr[L any](s Snap, l L) L {
	if s.Loader != nil {
		if v, ok := any(s.Loader).(L); ok {
			return v
		}
	}
	return l
}

// ProjectsOr is the resolver for s's project: its upload's, or the service's r.
func (s Snap) ProjectsOr(r *ProjectResolver) *ProjectResolver {
	if s.Projects != nil {
		return s.Projects
	}
	return r
}

// BoardRead is the artifact the board is read from: the board tier, or the netlist's own artifact for
// a format that carries its copper itself.
func (s Snap) BoardRead() artifact.URI { return boardReadURI(s.Netlist, s.Board) }

type snapshotEntry struct {
	snap Snap
	// def is the definition and its output fields, handed out as a copy.
	mu  sync.Mutex
	def *webapi.Snapshot
	// touched is every file a question about this snapshot read, as it stamped when first read.
	touched *formats.Touched
	// upload is the host of the files a client sent, nil for a snapshot of the host's own files. A
	// snapshot holds its upload, so it stays readable however many uploads come after it.
	upload *uploadHost
	// changedTiers names the tiers whose entry file no longer hashes as it did, for a stale error. It
	// names tiers rather than the files touched recorded, which on a server are host paths (C28).
	changedTiers func(ctx context.Context) []string
	used         uint64
}

// snapshotRegistry holds a host's snapshots, the least recently asked evicted past max.
type snapshotRegistry struct {
	mu   sync.Mutex
	m    map[string]*snapshotEntry
	tick uint64
	max  int
}

// defaultSnapshots is how many snapshots a resolver holds when SnapshotLimit is unset. A snapshot is
// small, since what it read is held by the DesignCache beneath it, so this bounds the registry rather
// than memory.
const defaultSnapshots = 256

// orEmptyResolver is r, or a resolver that resolves no projects when r is nil, so a service built
// without one still holds the snapshots its questions name.
func orEmptyResolver(r *ProjectResolver) *ProjectResolver {
	if r == nil {
		return &ProjectResolver{}
	}
	return r
}

func (r *ProjectResolver) registry() *snapshotRegistry {
	r.snapMu.Lock()
	defer r.snapMu.Unlock()
	if r.snaps == nil {
		max := r.SnapshotLimit
		if max <= 0 {
			max = defaultSnapshots
		}
		r.snaps = &snapshotRegistry{m: map[string]*snapshotEntry{}, max: max}
	}
	return r.snaps
}

func (g *snapshotRegistry) get(name string) (*snapshotEntry, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.m[name]
	if ok {
		g.tick++
		e.used = g.tick
	}
	return e, ok
}

func (g *snapshotRegistry) put(name string, e *snapshotEntry) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.tick++
	e.used = g.tick
	g.m[name] = e
	for len(g.m) > g.max {
		var oldest string
		var at uint64
		for k, v := range g.m {
			if oldest == "" || v.used < at {
				oldest, at = k, v.used
			}
		}
		delete(g.m, oldest)
	}
}

// Snapshot returns the snapshot a question names, and the context to answer it under, which records
// every file the answer reads onto the snapshot. A name the host does not hold, or a snapshot whose
// files changed since they were read, is a *SnapshotError.
func (r *ProjectResolver) Snapshot(ctx context.Context, name string) (context.Context, Snap, error) {
	if name == "" {
		return ctx, Snap{}, fmt.Errorf("%w: no snapshot named; ingest the design first", ErrInvalidArgument)
	}
	if r == nil {
		return ctx, Snap{}, &SnapshotError{Name: name}
	}
	e, ok := r.registry().get(name)
	if !ok {
		return ctx, Snap{}, &SnapshotError{Name: name}
	}
	if e.touched.Len() > 0 && !e.touched.Unchanged() {
		return ctx, Snap{}, &SnapshotError{Name: name, Stale: true, Changed: e.changedTiers(ctx)}
	}
	return withTouched(ctx, e.touched), e.snap, nil
}

// OptionalSnapshot is Snapshot for a request where naming none is allowed, returning ok false then.
func (r *ProjectResolver) OptionalSnapshot(ctx context.Context, name string) (context.Context, Snap, bool, error) {
	if name == "" {
		return ctx, Snap{}, false, nil
	}
	ctx, s, err := r.Snapshot(ctx, name)
	return ctx, s, err == nil, err
}

// SnapshotService creates, reads, updates and deletes snapshots (webapi.SnapshotService). It holds what
// a CheckService does, because ingesting with read_board builds the model a check would.
type SnapshotService struct {
	loader         SnapshotLoader
	projects       *ProjectResolver
	specs          param.ParamProvider
	baseConvention string
}

// SnapshotLoader is what ingesting reads through: the netlist always, and with read_board the model and
// its board, which needs a ModelLoader. Every service's loader satisfies it, so a test can ingest
// through whichever loader the service under test holds.
type SnapshotLoader interface {
	Design(ctx context.Context, uri artifact.URI, opts ...ReadOption) (*ir.Design, error)
}

// NewSnapshotService ingests designs into projects' snapshots. baseConvention is the catalog source
// name of the server's --conventions default, as on NewCheckService.
func NewSnapshotService(loader SnapshotLoader, projects *ProjectResolver, specs param.ParamProvider, baseConvention string) *SnapshotService {
	return &SnapshotService{loader: loader, projects: projects, specs: specs, baseConvention: baseConvention}
}

// Ingest creates a snapshot from its definition and reads its netlist, and its board when read_board
// asks. Naming an id already in use with the same definition is not a second snapshot: it reads what
// read_board asks and returns that one, so a client may send its ingest again, to a host that may or
// may not hold it, without first asking. The same id with a different definition is ErrAlreadyExists.
func (s *SnapshotService) Ingest(ctx context.Context, req *webapi.IngestRequest) (*webapi.Snapshot, error) {
	def := definitionOf(req.GetSnapshot())
	name := ""
	if id := req.GetSnapshotId(); id != "" {
		if strings.ContainsAny(id, "/ ") {
			return nil, fmt.Errorf("%w: snapshot id %q may not contain a slash or a space", ErrInvalidArgument, id)
		}
		name = "snapshots/" + id
		if e, ok := s.projects.registry().get(name); ok {
			e.mu.Lock()
			same := proto.Equal(definitionOf(e.def), def)
			e.mu.Unlock()
			if !same {
				return nil, fmt.Errorf("%w: %s holds a different definition; update it instead", ErrAlreadyExists, name)
			}
			if !req.GetReadBoard() {
				return e.out(), nil
			}
		}
	} else {
		name = "snapshots/" + newSnapshotID()
	}
	e, err := s.load(ctx, name, def, req.GetReadBoard())
	if err != nil {
		return nil, err
	}
	s.projects.registry().put(name, e)
	return e.out(), nil
}

// GetSnapshot returns a snapshot's definition and what it resolved to.
func (s *SnapshotService) GetSnapshot(ctx context.Context, req *webapi.GetSnapshotRequest) (*webapi.Snapshot, error) {
	e, ok := s.projects.registry().get(req.GetName())
	if !ok {
		return nil, &SnapshotError{Name: req.GetName()}
	}
	return e.out(), nil
}

// ListSnapshots lists the snapshots this host holds, by name. The one filter is `uri="..."`.
func (s *SnapshotService) ListSnapshots(ctx context.Context, req *webapi.ListSnapshotsRequest) (*webapi.ListSnapshotsResponse, error) {
	uri, err := parseSnapshotFilter(req.GetFilter())
	if err != nil {
		return nil, err
	}
	g := s.projects.registry()
	g.mu.Lock()
	var all []*snapshotEntry
	for _, e := range g.m {
		all = append(all, e)
	}
	g.mu.Unlock()
	var outs []*webapi.Snapshot
	for _, e := range all {
		o := e.out()
		if uri != "" && o.GetUri() != uri {
			continue
		}
		outs = append(outs, o)
	}
	sort.Slice(outs, func(i, j int) bool { return outs[i].GetName() < outs[j].GetName() })
	start := 0
	if t := req.GetPageToken(); t != "" {
		start = sort.Search(len(outs), func(i int) bool { return outs[i].GetName() > t })
	}
	size := int(req.GetPageSize())
	if size <= 0 || size > 100 {
		size = 100
	}
	resp := &webapi.ListSnapshotsResponse{}
	end := min(start+size, len(outs))
	resp.Snapshots = outs[start:end]
	if end < len(outs) {
		resp.NextPageToken = outs[end-1].GetName()
	}
	return resp, nil
}

// UpdateSnapshot replaces a snapshot's definition, whole or by update_mask, and reads it again under
// the same name. An update that cannot be read leaves the snapshot as it was.
func (s *SnapshotService) UpdateSnapshot(ctx context.Context, req *webapi.UpdateSnapshotRequest) (*webapi.Snapshot, error) {
	name := req.GetSnapshot().GetName()
	old, ok := s.projects.registry().get(name)
	if !ok {
		return nil, &SnapshotError{Name: name}
	}
	old.mu.Lock()
	def := definitionOf(old.def)
	old.mu.Unlock()
	next := definitionOf(req.GetSnapshot())
	if paths := req.GetUpdateMask().GetPaths(); len(paths) == 0 {
		def = next
	} else {
		for _, p := range paths {
			switch p {
			case "uri":
				def.Uri = next.GetUri()
			case "board_uri":
				def.BoardUri = next.GetBoardUri()
			case "geometry_uri":
				def.GeometryUri = next.GetGeometryUri()
			case "as_named":
				def.AsNamed = next.GetAsNamed()
			case "overlay":
				def.Overlay = next.GetOverlay()
			case "files":
				def.Files = next.GetFiles()
			default:
				return nil, fmt.Errorf("%w: %q is not a field UpdateSnapshot can change; it can change uri, board_uri, geometry_uri, as_named, overlay and files", ErrInvalidArgument, p)
			}
		}
	}
	e, err := s.load(ctx, name, def, req.GetReadBoard())
	if err != nil {
		return nil, err
	}
	s.projects.registry().put(name, e)
	return e.out(), nil
}

// DeleteSnapshot forgets a snapshot. What it read stays in the DesignCache for another to use.
func (s *SnapshotService) DeleteSnapshot(ctx context.Context, req *webapi.DeleteSnapshotRequest) (*emptypb.Empty, error) {
	g := s.projects.registry()
	g.mu.Lock()
	_, ok := g.m[req.GetName()]
	delete(g.m, req.GetName())
	g.mu.Unlock()
	if !ok {
		return nil, &SnapshotError{Name: req.GetName()}
	}
	return &emptypb.Empty{}, nil
}

// load resolves a definition and reads it, into an entry not yet in the registry.
func (s *SnapshotService) load(ctx context.Context, name string, def *webapi.Snapshot, readBoard bool) (*snapshotEntry, error) {
	defer timing.Begin(ctx, "ingest")()
	u, err := ParseArtifactURI(def.GetUri())
	if err != nil {
		return nil, err
	}
	board, err := optionalArtifactURI(def.GetBoardUri())
	if err != nil {
		return nil, err
	}
	geometry, err := optionalArtifactURI(def.GetGeometryUri())
	if err != nil {
		return nil, err
	}
	// A design sent as files is read from them alone, through a host of its own (agni issue 1061).
	res, ld := s.projects, s.loader
	var up *uploadHost
	if len(def.GetFiles()) > 0 {
		if up, err = s.projects.uploaded(u.Mount, def.GetFiles()); err != nil {
			return nil, err
		}
		res, ld = up.resolver, up.loader
	}
	ov, err := res.Overlay(ctx, u, def.GetOverlay(), s.baseConvention)
	if err != nil {
		return nil, err
	}
	nu, bu, gu, err := res.TierURIs(ctx, u, board, def.GetAsNamed())
	if err != nil {
		return nil, err
	}
	if !geometry.IsZero() {
		gu = geometry
	}
	e := &snapshotEntry{
		snap:    Snap{Name: name, Design: u, Netlist: nu, Board: bu, Geometry: gu, AsNamed: def.GetAsNamed(), Overlay: ov, Config: def.GetOverlay()},
		upload:  up,
		touched: formats.NewTouched(),
	}
	rctx := withTouched(ctx, e.touched)
	out := proto.Clone(def).(*webapi.Snapshot)
	out.Name = name
	out.Tiers = &webapi.SnapshotTiers{NetlistUri: nu.String(), GeometryUri: gu.String()}
	if !bu.IsZero() {
		out.Tiers.BoardUri = bu.String()
	}
	if up != nil {
		e.snap.Loader, e.snap.Projects = up.loader, up.resolver
	}
	if res.Store != nil {
		if _, p, err := res.Store.ResolveDesign(ctx, u); err == nil && p != nil {
			out.Project = p.GetName()
		}
	}
	// Without read_board nothing is read: opening a KiCad design draws from its schematic's geometry
	// and parses its netlist only for the first check or query, so reading the netlist here added a
	// second of parsing to Jetson's open in the browser. A design that cannot be read fails at its
	// first question, as it did before snapshots. With read_board, the model a check would build is
	// built and kept, board included (agni issue 1061).
	if !readBoard {
	} else if ml, ok := ld.(ModelLoader); !ok {
		return nil, fmt.Errorf("%w: this host reads no boards, so read_board cannot be honoured", ErrInvalidArgument)
	} else if b, err := buildModelEntry(rctx, ml, nu, bu, ov, s.specs, ReadBoard); err != nil {
		return nil, err
	} else if b.model.HasBoard() {
		out.Tiers.BoardRead = true
		if p, ok := ld.(interface{ BoardPersisted(artifact.URI) bool }); ok {
			out.Tiers.BoardPersisted = p.BoardPersisted(boardReadURI(nu, bu))
		}
		if bg, err := ml.Board(rctx, boardReadURI(nu, bu)); err == nil && bg != nil {
			out.Consistency = consistency(b.model.Components(), bg)
		}
	}
	// Only the netlist's entry is hashed, which an open hashes anyway for its verdict links. Hashing
	// every tier hashed Jetson's 81 MB board on every ingest, about a second in wasm on each worker.
	tiers := []artifact.URI{nu, bu, gu}
	hash := tierHash(rctx, ld, nu)
	e.snap.Revision = revisionOf(def, tiers, hash)
	e.changedTiers = func(ctx context.Context) []string {
		if tierHash(ctx, ld, nu) != hash {
			return []string{nu.String()}
		}
		// The file that moved was another tier's, or one a tier read beneath its entry (a sub-sheet,
		// a symbol library), so every tier is named, since any of them may have read it.
		var all []string
		for _, t := range tiers {
			if !t.IsZero() && !slices.Contains(all, t.String()) {
				all = append(all, t.String())
			}
		}
		return all
	}
	out.RevisionId = e.snap.Revision
	e.def = out
	return e, nil
}

// out is a copy of the entry's definition and outputs.
func (e *snapshotEntry) out() *webapi.Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return proto.Clone(e.def).(*webapi.Snapshot)
}

// definitionOf is a snapshot's definition fields alone.
func definitionOf(s *webapi.Snapshot) *webapi.Snapshot {
	return &webapi.Snapshot{
		Uri:         s.GetUri(),
		BoardUri:    s.GetBoardUri(),
		GeometryUri: s.GetGeometryUri(),
		AsNamed:     s.GetAsNamed(),
		Overlay:     s.GetOverlay(),
		Files:       s.GetFiles(),
	}
}

// tierHash is the content hash of a tier's entry file, for a loader that hashes, or "".
func tierHash(ctx context.Context, loader SnapshotLoader, t artifact.URI) string {
	hasher, _ := loader.(interface {
		DesignHash(ctx context.Context, uri artifact.URI) (string, error)
	})
	if hasher == nil || t.IsZero() {
		return ""
	}
	c, _ := hasher.DesignHash(ctx, t)
	return c
}

// revisionOf hashes a definition with the tiers it resolved to and its netlist entry's content, so
// two hosts that read one definition over the same netlist name one revision.
func revisionOf(def *webapi.Snapshot, tiers []artifact.URI, netlistHash string) string {
	h := sha256.New()
	b, _ := proto.MarshalOptions{Deterministic: true}.Marshal(def)
	h.Write(b)
	for _, t := range tiers {
		h.Write([]byte{0})
		h.Write([]byte(t.String()))
	}
	h.Write([]byte{0})
	h.Write([]byte(netlistHash))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// consistency compares a netlist's reference designators with a board's placements.
func consistency(netlist []*ir.Component, board *geom.BoardGeometry) *webapi.SnapshotConsistency {
	inNetlist := map[string]bool{}
	for _, c := range netlist {
		inNetlist[c.GetRefDes()] = true
	}
	onBoard := map[string]bool{}
	for _, p := range board.GetPlacements() {
		onBoard[p.GetRefDes()] = true
	}
	c := &webapi.SnapshotConsistency{}
	c.BoardOnly, c.BoardOnlyCount = onlyIn(onBoard, inNetlist)
	c.NetlistOnly, c.NetlistOnlyCount = onlyIn(inNetlist, onBoard)
	return c
}

// onlyIn is the names in a and not b, sorted, at most 20, and how many there are.
func onlyIn(a, b map[string]bool) ([]string, int32) {
	var out []string
	for n := range a {
		if n != "" && !b[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	n := int32(len(out))
	if len(out) > 20 {
		out = out[:20]
	}
	return out, n
}

// newSnapshotID is a random id, 16 hex digits.
func newSnapshotID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// parseSnapshotFilter reads the one supported AIP-160 filter, `uri="..."`. Anything else is an error,
// as with parseReviewFilter.
func parseSnapshotFilter(filter string) (string, error) {
	f := strings.TrimSpace(filter)
	if f == "" {
		return "", nil
	}
	v, ok := strings.CutPrefix(f, "uri=")
	if !ok || len(v) < 2 || v[0] != '"' || v[len(v)-1] != '"' {
		return "", fmt.Errorf("%w: filter %q is not supported; the only supported filter is uri=\"<uri>\"", ErrInvalidArgument, filter)
	}
	return v[1 : len(v)-1], nil
}

// Snapshots is the SnapshotService over this service's loader and resolver, which holds the snapshots
// its questions name (agni issue 1061).
func (s *DesignService) Snapshots() *SnapshotService {
	return NewSnapshotService(s.loader, s.projects, nil, "")
}

// Snapshots is the SnapshotService over this service's loader, resolver and datasheet corpus.
func (s *QueryService) Snapshots() *SnapshotService {
	return NewSnapshotService(s.loader, s.projects, s.specs, "")
}

// Snapshots is the SnapshotService over this service's loader, resolver, datasheet corpus and base
// convention, as CheckService.Snapshots is.
func (s *ReviewService) Snapshots() *SnapshotService {
	return NewSnapshotService(s.loader, s.projects, s.specs, s.baseConvention)
}

// Snapshots is the SnapshotService over this service's loader and resolver.
func (s *DiffService) Snapshots() *SnapshotService {
	return NewSnapshotService(s.loader, s.projects, nil, "")
}
