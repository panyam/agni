package fshost

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// The tests here are agni issue 1061's step 4: a snapshot as a resource, over a copy of the tutorial
// project on the real filesystem, so an edited file stamps as it does on disk.

// readCountingHost is a host that counts the netlist reads that reach it, which are the ones no
// cache answered.
type readCountingHost struct {
	*Host
	reads atomic.Int64
}

func (h *readCountingHost) Design(ctx context.Context, uri artifact.URI, opts ...service.ReadOption) (*ir.Design, error) {
	h.reads.Add(1)
	return h.Host.Design(ctx, uri, opts...)
}

type snapFixture struct {
	dir   string
	host  *readCountingHost
	res   *service.ProjectResolver
	snaps *service.SnapshotService
	check *service.CheckService
}

func newSnapFixture(t *testing.T, limit int) *snapFixture {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "examples", "tutorial-project"))); err != nil {
		t.Fatal(err)
	}
	ms := []Mount{{Name: "m", FS: os.DirFS(dir)}}
	res := &service.ProjectResolver{Store: projects.NewFSStore(projects.Tree{Mount: "m", FS: ms[0].FS}), Config: &ConfigResolver{Mounts: ms}, SnapshotLimit: limit}
	host := &readCountingHost{Host: New(ms...)}
	loader := service.NewCachingLoader(host, service.NewDesignCache(32))
	chk := service.NewCheckService(loader, check.DefaultCatalog(), nil, "", nil, res)
	return &snapFixture{dir: dir, host: host, res: res, snaps: chk.Snapshots(), check: chk}
}

const gateway = "mount://m/designs/gateway"

func (f *snapFixture) ingest(t *testing.T, req *webapi.IngestRequest) *webapi.Snapshot {
	t.Helper()
	s, err := f.snaps.Ingest(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (f *snapFixture) checkDesign(snap string) error {
	_, err := f.check.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Snapshot: snap, Rules: []string{"floating-input"}})
	return err
}

func TestTwoIngestsOfOneDesignAreTwoSnapshotsOverOneRead(t *testing.T) {
	f := newSnapFixture(t, 0)
	a := f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway}})
	if err := f.checkDesign(a.GetName()); err != nil {
		t.Fatalf("checking %s: %v", a.GetName(), err)
	}
	reads := f.host.reads.Load()
	if reads == 0 {
		t.Fatal("the first check read nothing through the host, so a second reading nothing proves nothing")
	}
	b := f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway}})
	if a.GetName() == b.GetName() {
		t.Errorf("two ingests were named %s alike; each ingest is a snapshot of its own", a.GetName())
	}
	if a.GetRevisionId() != b.GetRevisionId() {
		t.Errorf("one definition over the same files has revisions %s and %s", a.GetRevisionId(), b.GetRevisionId())
	}
	if err := f.checkDesign(b.GetName()); err != nil {
		t.Errorf("checking %s: %v", b.GetName(), err)
	}
	if n := f.host.reads.Load(); n != reads {
		t.Errorf("checking the second snapshot read the netlist again (%d reads after %d); what the first read is shared beneath both", n, reads)
	}
}

func TestAStaleSnapshotNamesTheFileAndAnUpdateRefreshesIt(t *testing.T) {
	f := newSnapFixture(t, 0)
	s := f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway}})
	if err := f.checkDesign(s.GetName()); err != nil {
		t.Fatalf("control: the fresh snapshot did not answer: %v", err)
	}
	netlist := filepath.Join(f.dir, "designs", "gateway", "gateway.edn")
	b, err := os.ReadFile(netlist)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(netlist, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(netlist, later, later)

	err = f.checkDesign(s.GetName())
	var se *service.SnapshotError
	if !errors.As(err, &se) || !se.Stale || !errors.Is(err, service.ErrStale) {
		t.Fatalf("a check after the netlist changed answered %v; want a stale snapshot", err)
	}
	// The netlist tier alone moved, and it is named as an artifact, never as the host's path.
	if !slices.Equal(se.Changed, []string{"mount://m/designs/gateway/gateway.edn"}) {
		t.Errorf("the stale snapshot names %v, want the netlist tier that changed", se.Changed)
	}
	u, err := f.snaps.UpdateSnapshot(context.Background(), &webapi.UpdateSnapshotRequest{Snapshot: &webapi.Snapshot{Name: s.GetName(), Uri: gateway}})
	if err != nil {
		t.Fatal(err)
	}
	if u.GetName() != s.GetName() || u.GetRevisionId() == s.GetRevisionId() {
		t.Errorf("the update answered %s at revision %s; want the same name at a new revision than %s", u.GetName(), u.GetRevisionId(), s.GetRevisionId())
	}
	if err := f.checkDesign(s.GetName()); err != nil {
		t.Errorf("the updated snapshot did not answer: %v", err)
	}
}

func TestABoardOnlyUpdateKeepsTheNetlistItRead(t *testing.T) {
	f := newSnapFixture(t, 0)
	s := f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway}, ReadBoard: true})
	if s.GetTiers().GetBoardUri() != "mount://m/designs/gateway/gateway.kicad_pcb" || !s.GetTiers().GetBoardRead() {
		t.Fatalf("the ingest's board is %q, read %v; want the declared board, read", s.GetTiers().GetBoardUri(), s.GetTiers().GetBoardRead())
	}
	reads := f.host.reads.Load()
	other := "mount://m/designs/gateway/gateway-rev-b.kicad_pcb"
	u, err := f.snaps.UpdateSnapshot(context.Background(), &webapi.UpdateSnapshotRequest{
		Snapshot:   &webapi.Snapshot{Name: s.GetName(), BoardUri: other},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"board_uri"}},
		ReadBoard:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if u.GetTiers().GetBoardUri() != other || u.GetUri() != gateway {
		t.Errorf("the update reads board %q of %q; want %q of the same design", u.GetTiers().GetBoardUri(), u.GetUri(), gateway)
	}
	if u.GetRevisionId() == s.GetRevisionId() {
		t.Error("a new board left the revision where it was")
	}
	if n := f.host.reads.Load(); n != reads {
		t.Errorf("replacing the board read the netlist again (%d reads after %d)", n, reads)
	}
	// control: a mask naming a field the definition does not have is refused, and the snapshot stands.
	if _, err := f.snaps.UpdateSnapshot(context.Background(), &webapi.UpdateSnapshotRequest{
		Snapshot:   &webapi.Snapshot{Name: s.GetName()},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"tiers"}},
	}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("a mask naming an output field answered %v; want invalid_argument", err)
	}
	got, err := f.snaps.GetSnapshot(context.Background(), &webapi.GetSnapshotRequest{Name: s.GetName()})
	if err != nil || got.GetRevisionId() != u.GetRevisionId() {
		t.Errorf("after a refused update the snapshot is at %s (%v); want %s", got.GetRevisionId(), err, u.GetRevisionId())
	}
}

func TestIngestUnderAnIdInUse(t *testing.T) {
	f := newSnapFixture(t, 0)
	s := f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway}, SnapshotId: "page1"})
	if s.GetName() != "snapshots/page1" {
		t.Fatalf("named %s, want the id the client chose", s.GetName())
	}
	again := f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway}, SnapshotId: "page1", ReadBoard: true})
	if again.GetName() != s.GetName() || !again.GetTiers().GetBoardRead() {
		t.Errorf("the same definition under its id answered %s with board read %v; want the snapshot, its board now read", again.GetName(), again.GetTiers().GetBoardRead())
	}
	_, err := f.snaps.Ingest(context.Background(), &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway, AsNamed: true}, SnapshotId: "page1"})
	if !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("another definition under an id in use answered %v; want already_exists", err)
	}
	if _, err := f.snaps.Ingest(context.Background(), &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway}, SnapshotId: "a/b"}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("an id with a slash answered %v; want invalid_argument", err)
	}
}

func TestConsistencyComparesTheBoardWithTheNetlist(t *testing.T) {
	f := newSnapFixture(t, 0)
	own := f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway}, ReadBoard: true})
	c := own.GetConsistency()
	if c == nil {
		t.Fatal("a snapshot whose board was read reports no consistency")
	}
	if c.GetBoardOnlyCount() != 0 || c.GetNetlistOnlyCount() != 0 {
		t.Errorf("the gateway's own board and netlist differ by %v and %v", c.GetBoardOnly(), c.GetNetlistOnly())
	}
	// The second revision's netlist against the first revision's board, the shape a board updated
	// on its own takes.
	mixed := f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://m/designs/gateway/gateway-rev-c.edn", BoardUri: "mount://m/designs/gateway/gateway.kicad_pcb", AsNamed: true}, ReadBoard: true})
	m := mixed.GetConsistency()
	if m.GetBoardOnlyCount()+m.GetNetlistOnlyCount() == 0 {
		t.Errorf("a later revision's netlist against the first board reports no difference")
	}
	if int(m.GetBoardOnlyCount()) < len(m.GetBoardOnly()) || int(m.GetNetlistOnlyCount()) < len(m.GetNetlistOnly()) {
		t.Errorf("the counts %d and %d are below the names listed, %v and %v", m.GetBoardOnlyCount(), m.GetNetlistOnlyCount(), m.GetBoardOnly(), m.GetNetlistOnly())
	}
	// control: without the board read there is nothing to compare.
	plain := f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway}})
	if plain.GetConsistency() != nil {
		t.Error("a snapshot whose board was not read reports a consistency")
	}
}

func TestAnEvictedSnapshotIsNotFound(t *testing.T) {
	f := newSnapFixture(t, 1)
	a := f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway}})
	f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://m/designs/gateway/gateway-rev-b.edn"}})
	err := f.checkDesign(a.GetName())
	var se *service.SnapshotError
	if !errors.As(err, &se) || se.Stale || !errors.Is(err, service.ErrNotFound) || se.Name != a.GetName() {
		t.Fatalf("a check of an evicted snapshot answered %v; want it missing, by name", err)
	}
	// Ingesting it again under its id is how a client recovers.
	id := a.GetName()[len("snapshots/"):]
	f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway}, SnapshotId: id})
	if err := f.checkDesign(a.GetName()); err != nil {
		t.Errorf("the snapshot ingested again under its id did not answer: %v", err)
	}
}

func TestListSnapshotsFiltersByURI(t *testing.T) {
	f := newSnapFixture(t, 0)
	f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway}})
	f.ingest(t, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://m/designs/gateway/gateway-rev-b.edn"}})
	all, err := f.snaps.ListSnapshots(context.Background(), &webapi.ListSnapshotsRequest{})
	if err != nil || len(all.GetSnapshots()) != 2 {
		t.Fatalf("listed %d snapshots (%v), want both", len(all.GetSnapshots()), err)
	}
	one, err := f.snaps.ListSnapshots(context.Background(), &webapi.ListSnapshotsRequest{Filter: `uri="` + gateway + `"`})
	if err != nil || len(one.GetSnapshots()) != 1 || one.GetSnapshots()[0].GetUri() != gateway {
		t.Errorf("the uri filter listed %v (%v); want the gateway's alone", one.GetSnapshots(), err)
	}
	if _, err := f.snaps.ListSnapshots(context.Background(), &webapi.ListSnapshotsRequest{Filter: `name="x"`}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("an unsupported filter answered %v; want invalid_argument", err)
	}
	if _, err := f.snaps.DeleteSnapshot(context.Background(), &webapi.DeleteSnapshotRequest{Name: one.GetSnapshots()[0].GetName()}); err != nil {
		t.Fatal(err)
	}
	if left, _ := f.snaps.ListSnapshots(context.Background(), &webapi.ListSnapshotsRequest{}); len(left.GetSnapshots()) != 1 {
		t.Errorf("after a delete %d snapshots are listed, want 1", len(left.GetSnapshots()))
	}
}
