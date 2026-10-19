package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/core/review"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/proto"
)

// TestToConnectErr pins the one sentinel-to-code table every adapter shares, the mapping the
// per-RPC connect.NewError calls used to encode before WS9-026 moved translation here.
func TestToConnectErr(t *testing.T) {
	cases := []struct {
		err  error
		want connect.Code
		// detail is the SnapshotUnavailable a snapshot error carries, and nil for every other error.
		detail *webapi.SnapshotUnavailable
	}{
		{fmt.Errorf("no such mount: %w", service.ErrNotFound), connect.CodeNotFound, nil},
		{fmt.Errorf("%w: escaped", service.ErrInvalidPath), connect.CodeInvalidArgument, nil},
		{fmt.Errorf("%w: no netlist", service.ErrInvalidArgument), connect.CodeInvalidArgument, nil},
		{service.ErrNativeNoTool, connect.CodeUnimplemented, nil},
		{service.ErrNativeNotEnabled, connect.CodeFailedPrecondition, nil},
		{service.ErrNativeNotFound, connect.CodeFailedPrecondition, nil},
		{fmt.Errorf("%w: exec blew up", service.ErrInternal), connect.CodeInternal, nil},
		// A request that stopped rather than failed (agni issues 792, 795).
		{fmt.Errorf("query: evaluation stopped: %w", context.Canceled), connect.CodeCanceled, nil},
		{fmt.Errorf("query: evaluation stopped: %w", context.DeadlineExceeded), connect.CodeDeadlineExceeded, nil},
		{fmt.Errorf("%w: work passed its budget of 5", service.ErrResourceExhausted), connect.CodeResourceExhausted, nil},
		{errors.New("unclassified"), connect.CodeInvalidArgument, nil},
		// A snapshot the host does not hold, or whose files changed (agni issue 1061).
		{&service.SnapshotError{Name: "snapshots/gone"}, connect.CodeNotFound,
			&webapi.SnapshotUnavailable{Name: "snapshots/gone", Reason: webapi.SnapshotUnavailable_REASON_MISSING}},
		{fmt.Errorf("check: %w", &service.SnapshotError{Name: "snapshots/old", Stale: true, Changed: []string{"mount://m/d.edn", "mount://m/d.kicad_pcb"}}), connect.CodeFailedPrecondition,
			&webapi.SnapshotUnavailable{Name: "snapshots/old", Reason: webapi.SnapshotUnavailable_REASON_STALE, Changed: []string{"mount://m/d.edn", "mount://m/d.kicad_pcb"}}},
		{fmt.Errorf("%w: snapshots/taken", service.ErrAlreadyExists), connect.CodeAlreadyExists, nil},
	}
	for _, c := range cases {
		ce := ConnectError(c.err)
		if got := ce.Code(); got != c.want {
			t.Errorf("toConnectErr(%v) = %v, want %v", c.err, got, c.want)
		}
		var got *webapi.SnapshotUnavailable
		for _, d := range ce.Details() {
			if v, err := d.Value(); err == nil {
				if su, ok := v.(*webapi.SnapshotUnavailable); ok {
					got = su
				}
			}
		}
		if (got == nil) != (c.detail == nil) || (got != nil && !proto.Equal(got, c.detail)) {
			t.Errorf("toConnectErr(%v) detail = %v, want %v", c.err, got, c.detail)
		}
	}
	// The not-enabled gate carries the actionable hint.
	if msg := toConnectErr(service.ErrNativeNotEnabled).Error(); !strings.Contains(msg, "--enable-native") {
		t.Errorf("not-enabled error %q should hint at --enable-native", msg)
	}
}

// A query that would not parse reaches a client with where it stopped, as an error detail, so the
// viewer and a --remote CLI can point at the place. Any other invalid argument carries none.
func TestAParseErrorCarriesItsPosition(t *testing.T) {
	_, perr := query.Parse(`component.net(?r ?n)`)
	ce := toConnectErr(fmt.Errorf("%w: %w", service.ErrInvalidArgument, perr)).(*connect.Error)
	var at *webapi.TextPosition
	for _, d := range ce.Details() {
		if v, err := d.Value(); err == nil {
			at, _ = v.(*webapi.TextPosition)
		}
	}
	if at.GetLine() != 1 || at.GetColumn() != 15 {
		t.Errorf("detail %v, want line 1 column 15", at)
	}
	if n := len(toConnectErr(fmt.Errorf("%w: no netlist", service.ErrInvalidArgument)).(*connect.Error).Details()); n != 0 {
		t.Errorf("an error with no position carried %d details", n)
	}
}

// memWS is a minimal Workspace port for driving one adapter end to end.
type memWS struct{ err error }

func (m memWS) Mounts() []service.MountInfo { return []service.MountInfo{{Name: "m", Root: "/x"}} }
func (m memWS) ListDir(context.Context, artifact.URI) ([]service.DirEntry, error) {
	return nil, m.err
}

// TestAdapterRoundTrip drives one adapter method both ways. A success comes back wrapped in a
// connect.Response, and a classified service error comes back as a coded connect error. Every other
// adapter method is the same three lines, so one probe stands for the pattern.
func TestAdapterRoundTrip(t *testing.T) {
	a := NewWorkspace(service.NewWorkspaceService(memWS{}))
	resp, err := a.ListMounts(context.Background(), connect.NewRequest(&webapi.ListMountsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Msg.GetMounts(); len(got) != 1 || got[0].GetName() != "m" {
		t.Fatalf("mounts = %+v, want [m]", got)
	}

	failing := NewWorkspace(service.NewWorkspaceService(memWS{err: errors.New("boom")}))
	_, err = failing.ListDir(context.Background(), connect.NewRequest(&webapi.ListDirRequest{Uri: "mount://m"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("want NotFound (the service classifies unknown workspace errors), got %v", err)
	}
}

// memReviewLoader is a minimal service.ReviewLoader for driving the Review adapter end to end.
type memReviewLoader struct {
	design *ir.Design
	man    review.Manifest
	err    error
}

func (m memReviewLoader) Design(context.Context, artifact.URI, ...service.ReadOption) (*ir.Design, error) {
	return m.design, m.err
}
func (m memReviewLoader) Board(context.Context, artifact.URI) (*geom.BoardGeometry, error) {
	return nil, nil
}
func (m memReviewLoader) DesignHash(context.Context, artifact.URI) (string, error) {
	return "sha256:stub", nil
}
func (m memReviewLoader) Manifest(context.Context, artifact.URI) (review.Manifest, error) {
	return m.man, m.err
}

// TestReviewAdapterRoundTrip drives the Review adapter both ways. A served CreateReview returns a
// wrapped response (proving the mux-visible handler delegates to the service, the CodeUnimplemented
// gotcha), and a classified service error comes back as a coded connect error.
func TestReviewAdapterRoundTrip(t *testing.T) {
	man := review.Manifest{Name: "M", Areas: []review.Area{{
		Name:  "A",
		Items: []review.Item{{ID: "1", Title: "t1", Note: "manual"}},
	}}}
	svc := service.NewReviewService(memReviewLoader{design: &ir.Design{}, man: man}, service.NewMemReviewStore(), check.DefaultCatalog(), nil, nil, service.ReviewEnv{ProducerVersion: "test"}, "", nil)
	a := NewReview(svc)
	resp, err := a.CreateReview(context.Background(), connect.NewRequest(&webapi.CreateReviewRequest{
		Manifest: service.ManifestProto(man), Snapshot: snapOf(t, svc.Snapshots(), "mount://m/d.edn"),
	}))
	if err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	if resp.Msg.GetName() == "" || resp.Msg.GetResults().GetManifest() != "M" {
		t.Fatalf("created review = %+v", resp.Msg)
	}

	// The loader fails once the design is ingested, so the error comes from the review's own read.
	fl := &memReviewLoader{design: &ir.Design{}, man: man}
	fsvc := service.NewReviewService(fl, service.NewMemReviewStore(), check.DefaultCatalog(), nil, nil, service.ReviewEnv{ProducerVersion: "test"}, "", nil)
	snap := snapOf(t, fsvc.Snapshots(), "mount://m/d.edn")
	fl.design, fl.err = nil, fmt.Errorf("no netlist: %w", service.ErrInvalidArgument)
	failing := NewReview(fsvc)
	_, err = failing.CreateReview(context.Background(), connect.NewRequest(&webapi.CreateReviewRequest{Manifest: service.ManifestProto(man), Snapshot: snap}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

// TestGetReviewManifestAdapter drives the second Review RPC the same way. The mux-visible handler
// must delegate to the service rather than inheriting the generated CodeUnimplemented stub, and a
// classified service error must come back coded. This is the gotcha that makes a hand-written
// adapter worth testing at all, since an unimplemented method compiles fine and fails only at
// request time.
func TestGetReviewManifestAdapter(t *testing.T) {
	man := review.Manifest{Name: "M", Areas: []review.Area{{
		Name:  "A",
		Items: []review.Item{{ID: "1", Title: "t1", Note: "manual"}},
	}}}
	a := NewReview(service.NewReviewService(memReviewLoader{man: man}, service.NewMemReviewStore(), check.DefaultCatalog(), nil, nil, service.ReviewEnv{ProducerVersion: "test"}, "", nil))
	resp, err := a.GetReviewManifest(context.Background(), connect.NewRequest(&webapi.GetReviewManifestRequest{Uri: "mount://m/m.yaml"}))
	if err != nil {
		t.Fatalf("GetReviewManifest: %v", err)
	}
	if got := resp.Msg.GetManifest(); got.GetName() != "M" || len(got.GetAreas()) != 1 {
		t.Fatalf("manifest = %+v", got)
	}

	failing := NewReview(service.NewReviewService(memReviewLoader{err: fmt.Errorf("no such file: %w", service.ErrNotFound)}, service.NewMemReviewStore(), check.DefaultCatalog(), nil, nil, service.ReviewEnv{ProducerVersion: "test"}, "", nil))
	_, err = failing.GetReviewManifest(context.Background(), connect.NewRequest(&webapi.GetReviewManifestRequest{Uri: "mount://m/m.yaml"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}
