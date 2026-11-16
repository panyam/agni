package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panyam/agni/mounts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// wireFixturesFile holds the calls every client in mission #1625 replays (agni issue 1633). Each is
// recorded from a server on the Jetson cut, so a client in any language proves it sends and reads
// the wire the server speaks. `make wire-fixtures` records them again after an intended change.
const wireFixturesFile = "../../clients/wirefixtures/fixtures.json"

type wireFixtures struct {
	// About says what the file is, for a reader who opens it cold.
	About    string        `json:"about"`
	Fixtures []wireFixture `json:"fixtures"`
}

// wireFixture is one call. A call that fails has error rather than response. ignore names response
// fields, top-level or dotted, that differ by build or configuration and are not compared.
type wireFixture struct {
	Name     string          `json:"name"`
	Service  string          `json:"service"`
	Method   string          `json:"method"`
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response,omitempty"`
	Error    *fixtureError   `json:"error,omitempty"`
	Ignore   []string        `json:"ignore,omitempty"`
}

// fixtureError is a failed call: its code as Connect names it, and each detail's type and value. The
// message is prose, and free to change, so it is not compared.
type fixtureError struct {
	Code    string       `json:"code"`
	Details []wireDetail `json:"details,omitempty"`
}

type wireDetail struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// wireCalls are the calls, in order. Ingest names its snapshot, so the calls after it can.
var wireCalls = []wireFixture{
	{Name: "server-info", Service: "agni.v1.webapi.ServerService", Method: "GetServerInfo", Request: json.RawMessage(`{}`), Ignore: []string{"version", "features"}},
	{Name: "ingest", Service: "agni.v1.webapi.SnapshotService", Method: "Ingest", Request: json.RawMessage(`{"snapshot":{"uri":"mount://probe/designs/jetson-cut"},"snapshotId":"wire"}`)},
	{Name: "get-design", Service: "agni.v1.webapi.DesignService", Method: "GetDesign", Request: json.RawMessage(`{"snapshot":"snapshots/wire"}`)},
	{Name: "check-design", Service: "agni.v1.webapi.CheckService", Method: "CheckDesign", Request: json.RawMessage(`{"snapshot":"snapshots/wire","rules":["bulk-cap"]}`)},
	{Name: "run-query", Service: "agni.v1.webapi.QueryService", Method: "RunQuery", Request: json.RawMessage(`{"snapshot":"snapshots/wire","query":"component.class(?c, \"resistor\") => ?c"}`)},
	// The board's second page at a one-byte budget, one placement, after a first page of the header
	// alone. The token is the one that first page answers.
	{Name: "board-geometry-page", Service: "agni.v1.webapi.DesignService", Method: "GetBoardGeometry", Request: json.RawMessage(`{"snapshot":"snapshots/wire","maxBytes":"1","pageToken":"YjF8ZTBjNTQyMDEwNzE5NDRlMnwwfDA"}`)},
	// C133 sits on the cut's back, so a board view follows it to the back with B.Cu and B.SilkS on
	// (agni issue 1635).
	{Name: "locate-on-board", Service: "agni.v1.webapi.DesignService", Method: "LocateSubjects", Request: json.RawMessage(`{"snapshot":"snapshots/wire","subjects":[{"kind":"component","ref":"C133"}],"board":true}`)},
	{Name: "query-that-does-not-parse", Service: "agni.v1.webapi.QueryService", Method: "RunQuery", Request: json.RawMessage(`{"snapshot":"snapshots/wire","query":"component.class(?c"}`)},
	{Name: "snapshot-not-found", Service: "agni.v1.webapi.DesignService", Method: "GetDesign", Request: json.RawMessage(`{"snapshot":"snapshots/nonesuch"}`)},
}

// wireServer serves the API on the cross-probe project as the mount "probe", over HTTP/1.1 and
// HTTP/2, as `agni serve` does.
func wireServer(t *testing.T) string {
	t.Helper()
	freshWorkspace(t)
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "cross-probe-project"))
	if err != nil {
		t.Fatal(err)
	}
	ms := []mounts.Mount{{Name: "probe", Root: root}}
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
	return ln.Addr().String()
}

// TestWireFixtures replays every fixture against a server, over Connect's JSON form, the plainest
// client any language can write, and over gRPC (agni issue 1633). With WIRE_FIXTURES_RECORD=1 it
// records the responses instead.
func TestWireFixtures(t *testing.T) {
	addr := wireServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if os.Getenv("WIRE_FIXTURES_RECORD") != "" {
		recordWireFixtures(t, ctx, addr)
		return
	}
	raw, err := os.ReadFile(wireFixturesFile)
	if err != nil {
		t.Fatalf("%v (make wire-fixtures records them)", err)
	}
	var file wireFixtures
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Fixtures) != len(wireCalls) {
		t.Fatalf("%s holds %d fixtures and the test makes %d calls; make wire-fixtures records them again", wireFixturesFile, len(file.Fixtures), len(wireCalls))
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, f := range file.Fixtures {
		viaConnect := callConnectJSON(t, ctx, addr, f)
		viaGRPC := callGRPC(t, ctx, conn, f)
		for transport, got := range map[string]wireFixture{"connect": viaConnect, "grpc": viaGRPC} {
			if msg := wireDiff(t, f, got); msg != "" {
				t.Errorf("%s over %s: %s", f.Name, transport, msg)
			}
		}
	}
}

func recordWireFixtures(t *testing.T, ctx context.Context, addr string) {
	file := wireFixtures{About: "The calls every agni client replays to prove it speaks the server's wire (agni issue 1633). Recorded by make wire-fixtures from a server on examples/cross-probe-project mounted as probe. docsite/content/build/client-contract.md says how a client replays them."}
	for _, c := range wireCalls {
		got := callConnectJSON(t, ctx, addr, c)
		got.Request, got.Ignore = compactJSON(t, c.Request), c.Ignore
		file.Fixtures = append(file.Fixtures, got)
	}
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(wireFixturesFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wireFixturesFile, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("recorded %d fixtures in %s", len(file.Fixtures), wireFixturesFile)
}

// wireMethod finds the rpc a fixture names.
func wireMethod(t *testing.T, f wireFixture) protoreflect.MethodDescriptor {
	t.Helper()
	d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(f.Service))
	if err != nil {
		t.Fatalf("%s: %v", f.Name, err)
	}
	m := d.(protoreflect.ServiceDescriptor).Methods().ByName(protoreflect.Name(f.Method))
	if m == nil {
		t.Fatalf("%s: %s has no rpc %s", f.Name, f.Service, f.Method)
	}
	return m
}

func newWireMessage(t *testing.T, name protoreflect.FullName) proto.Message {
	t.Helper()
	mt, err := protoregistry.GlobalTypes.FindMessageByName(name)
	if err != nil {
		t.Fatal(err)
	}
	return mt.New().Interface()
}

// callConnectJSON posts a fixture's request as Connect's JSON form does, with no Connect library, and
// returns the call as a fixture would record it.
func callConnectJSON(t *testing.T, ctx context.Context, addr string, f wireFixture) wireFixture {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://%s/%s/%s", addr, f.Service, f.Method), bytes.NewReader(f.Request))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s: %v", f.Name, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	got := wireFixture{Name: f.Name, Service: f.Service, Method: f.Method}
	if resp.StatusCode == http.StatusOK {
		msg := newWireMessage(t, wireMethod(t, f).Output().FullName())
		if err := protojson.Unmarshal(body, msg); err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		got.Response = fixtureJSON(t, msg)
		return got
	}
	var wire struct {
		Code    string `json:"code"`
		Details []struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"details"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatalf("%s: HTTP %d: %s", f.Name, resp.StatusCode, body)
	}
	got.Error = &fixtureError{Code: wire.Code}
	for _, d := range wire.Details {
		b, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(d.Value, "="))
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		m := newWireMessage(t, protoreflect.FullName(d.Type))
		if err := proto.Unmarshal(b, m); err != nil {
			t.Fatal(err)
		}
		got.Error.Details = append(got.Error.Details, wireDetail{Type: d.Type, Value: fixtureJSON(t, m)})
	}
	return got
}

// callGRPC calls a fixture's rpc through grpc-go by its full name, as tonic, grpc-java or gRPC's C++
// library would.
func callGRPC(t *testing.T, ctx context.Context, conn *grpc.ClientConn, f wireFixture) wireFixture {
	t.Helper()
	m := wireMethod(t, f)
	in, out := newWireMessage(t, m.Input().FullName()), newWireMessage(t, m.Output().FullName())
	if err := protojson.Unmarshal(f.Request, in); err != nil {
		t.Fatalf("%s: %v", f.Name, err)
	}
	got := wireFixture{Name: f.Name, Service: f.Service, Method: f.Method}
	err := conn.Invoke(ctx, "/"+f.Service+"/"+f.Method, in, out)
	if err == nil {
		got.Response = fixtureJSON(t, out)
		return got
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("%s over gRPC: %v", f.Name, err)
	}
	// Connect names a code in snake case, and gRPC in camel case.
	got.Error = &fixtureError{Code: snakeCase(st.Code().String())}
	for _, d := range st.Details() {
		if m, ok := d.(proto.Message); ok {
			got.Error.Details = append(got.Error.Details, wireDetail{Type: string(m.ProtoReflect().Descriptor().FullName()), Value: fixtureJSON(t, m)})
		}
	}
	return got
}

func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// wireDiff says how a call differs from its fixture, or "".
func wireDiff(t *testing.T, want, got wireFixture) string {
	t.Helper()
	switch {
	case want.Error != nil && got.Error == nil:
		return fmt.Sprintf("answered, want the error %s", want.Error.Code)
	case want.Error == nil && got.Error != nil:
		return fmt.Sprintf("failed with %s, want an answer", got.Error.Code)
	case want.Error != nil:
		if got.Error.Code != want.Error.Code {
			return fmt.Sprintf("failed with %s, want %s", got.Error.Code, want.Error.Code)
		}
		if len(got.Error.Details) != len(want.Error.Details) {
			return fmt.Sprintf("carries %d details, want %d", len(got.Error.Details), len(want.Error.Details))
		}
		for i, d := range want.Error.Details {
			g := got.Error.Details[i]
			if g.Type != d.Type || !sameMessage(t, protoreflect.FullName(d.Type), g.Value, d.Value, nil) {
				return fmt.Sprintf("detail %d is %s %s, want %s %s", i, g.Type, g.Value, d.Type, d.Value)
			}
		}
		return ""
	}
	if !sameMessage(t, wireMethod(t, want).Output().FullName(), got.Response, want.Response, want.Ignore) {
		return fmt.Sprintf("answered %s, want %s", got.Response, want.Response)
	}
	return ""
}

// sameMessage compares two messages of one type, leaving out the ignored fields.
func sameMessage(t *testing.T, typ protoreflect.FullName, a, b json.RawMessage, ignore []string) bool {
	t.Helper()
	x, y := newWireMessage(t, typ), newWireMessage(t, typ)
	if err := protojson.Unmarshal(a, x); err != nil {
		t.Fatal(err)
	}
	if err := protojson.Unmarshal(b, y); err != nil {
		t.Fatal(err)
	}
	for _, p := range ignore {
		clearPath(x.ProtoReflect(), p)
		clearPath(y.ProtoReflect(), p)
	}
	return proto.Equal(x, y)
}

// clearPath clears a field named by its proto name, dotted through singular messages.
func clearPath(m protoreflect.Message, path string) {
	head, rest, nested := strings.Cut(path, ".")
	fd := m.Descriptor().Fields().ByName(protoreflect.Name(head))
	if fd == nil {
		return
	}
	if !nested {
		m.Clear(fd)
		return
	}
	if fd.Message() != nil && !fd.IsList() && !fd.IsMap() && m.Has(fd) {
		clearPath(m.Mutable(fd).Message(), rest)
	}
}

// fixtureJSON is a message's protojson, compacted, since protojson varies its whitespace by build.
func fixtureJSON(t *testing.T, m proto.Message) json.RawMessage {
	t.Helper()
	b, err := protojson.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return compactJSON(t, b)
}

func compactJSON(t *testing.T, b []byte) json.RawMessage {
	t.Helper()
	var out bytes.Buffer
	if err := json.Compact(&out, b); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
