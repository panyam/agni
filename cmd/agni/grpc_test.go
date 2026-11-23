package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/goapplib/storage/mounts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// A gRPC client reaches `agni serve` on the port the viewer uses, and gets the message a Connect
// client gets (agni issue 1626). The client is grpc-go, the reference gRPC implementation that tonic,
// grpc-java and gRPC's C++ library interoperate with, and it calls each rpc by its full name with no
// generated gRPC stubs. A failed rpc's details, such as where a query stopped parsing, arrive as gRPC
// status details.
func TestAGRPCClientCallsTheServer(t *testing.T) {
	freshWorkspace(t)
	root, err := filepath.Abs(parityMounts[0].dir)
	if err != nil {
		t.Fatal(err)
	}
	ms := []mounts.Mount{mountset.Folder(parityMounts[0].name, root)}
	api := servedAPIFor(t, ms, ms)
	mux := http.NewServeMux()
	api.Register(mux)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := newHTTPServer("", mux)
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	snap := &webapi.Snapshot{}
	ingest := &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://" + parityMounts[0].name + "/boards/gateway"}}
	if err := conn.Invoke(ctx, "/agni.v1.webapi.SnapshotService/Ingest", ingest, snap); err != nil {
		t.Fatalf("Ingest over gRPC: %v", err)
	}
	if snap.GetName() == "" {
		t.Fatal("Ingest over gRPC answered no snapshot name")
	}

	query := &webapi.RunQueryRequest{Snapshot: snap.GetName(), Query: `component.class(?c, "resistor") => ?c`}
	viaGRPC := &webapi.RunQueryResponse{}
	if err := conn.Invoke(ctx, "/agni.v1.webapi.QueryService/RunQuery", query, viaGRPC); err != nil {
		t.Fatalf("RunQuery over gRPC: %v", err)
	}
	if len(viaGRPC.GetRows()) == 0 {
		t.Fatal("RunQuery over gRPC found no resistor on the tutorial gateway")
	}
	viaConnect, err := webapiconnect.NewQueryServiceClient(http.DefaultClient, "http://"+ln.Addr().String()).RunQuery(ctx, connect.NewRequest(query))
	if err != nil {
		t.Fatalf("RunQuery over Connect: %v", err)
	}
	if !proto.Equal(viaGRPC, viaConnect.Msg) {
		t.Errorf("gRPC answered %d rows and Connect %d, or the messages differ otherwise", len(viaGRPC.GetRows()), len(viaConnect.Msg.GetRows()))
	}

	// A client checks the server first (agni issue 1629).
	info := &webapi.ServerInfo{}
	if err := conn.Invoke(ctx, "/agni.v1.webapi.ServerService/GetServerInfo", &webapi.GetServerInfoRequest{}, info); err != nil {
		t.Fatalf("GetServerInfo over gRPC: %v", err)
	}
	if info.GetApi() != "agni.v1" || info.GetVersion() == "" || !slices.Contains(info.GetFeatures(), "snapshot_recovery") {
		t.Errorf("GetServerInfo over gRPC answered %v", info)
	}

	bad := &webapi.RunQueryRequest{Snapshot: snap.GetName(), Query: `component.class(?c`}
	err = conn.Invoke(ctx, "/agni.v1.webapi.QueryService/RunQuery", bad, &webapi.RunQueryResponse{})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("an unparsable query over gRPC answered %v, want InvalidArgument", err)
	}
	var at *webapi.TextPosition
	for _, d := range st.Details() {
		if p, ok := d.(*webapi.TextPosition); ok {
			at = p
		}
	}
	if at == nil || at.GetLine() != 1 || at.GetColumn() == 0 {
		t.Errorf("the gRPC status carries no TextPosition saying where the query stopped: %v", st.Details())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("the call timed out")
	}
}
