// Command go is the Go reference for `make exercise-clients` (agni issue 1628). It does what each
// language's example does, through that language's client, against a shared `agni serve`: it checks
// the server, ingests a design, asks it three questions, and prints one line of JSON summing up the
// answers. The exercise compares every language's line with this one and with the CLI's answers.
// docsite/content/build/client-contract.md names the line's fields.
//
//	go run ./clients/examples/go -server http://127.0.0.1:8080 -design mount://probe/boards/jetson-cut
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"slices"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
)

// Summary is the line every language's example prints. Its JSON names are the contract.
type Summary struct {
	Version     string   `json:"version"`
	API         string   `json:"api"`
	Features    []string `json:"features"`
	Components  int32    `json:"components"`
	Nets        int32    `json:"nets"`
	Sheets      int      `json:"sheets"`
	Findings    int      `json:"findings"`
	QueryRows   int      `json:"query_rows"`
	BoardLayers int      `json:"board_layers"`
	BoardParts  int      `json:"board_parts"`
	BoardTracks int      `json:"board_tracks"`
}

// The query every example asks, so their answers compare. The check runs every rule.
const resistorsOf = `component.class(?c, "resistor") => ?c`

// boardPageBytes is small, so the cut's 51 KB board comes in several pages and every example pages.
const boardPageBytes = 16 << 10

func main() {
	server := flag.String("server", "http://127.0.0.1:8080", "the agni serve to call")
	design := flag.String("design", "mount://probe/boards/jetson-cut", "the design to ingest")
	flag.Parse()
	s, err := run(context.Background(), *server, *design)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b, _ := json.Marshal(s) // #nosec G104 -- a struct of plain fields always marshals
	fmt.Println(string(b))
}

func run(ctx context.Context, server, design string) (*Summary, error) {
	// A shared server asks every call for a token (agni issue 1627), and its certificate may come from
	// a team's own CA (agni issue 1765). The exercise passes them in AGNI_TOKEN and AGNI_CA.
	var transport http.RoundTripper = http.DefaultTransport
	if ca := os.Getenv("AGNI_CA"); ca != "" {
		pemBytes, err := os.ReadFile(ca) // #nosec G304 G703 -- a CA file the caller named in AGNI_CA
		if err != nil {
			return nil, fmt.Errorf("AGNI_CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			return nil, fmt.Errorf("AGNI_CA: %s holds no certificate", ca)
		}
		transport = &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}
	}
	if token := os.Getenv("AGNI_TOKEN"); token != "" {
		transport = bearerTransport{token: token, next: transport}
	}
	hc := &http.Client{Transport: transport}
	info, err := webapiconnect.NewServerServiceClient(hc, server).GetServerInfo(ctx, connect.NewRequest(&webapi.GetServerInfoRequest{}))
	if err != nil {
		return nil, fmt.Errorf("calling GetServerInfo: %w", err)
	}
	// The contract's first rule: refuse a server that speaks another package.
	if info.Msg.GetApi() != "agni.v1" {
		return nil, fmt.Errorf("the server speaks %q, and this client was built for agni.v1", info.Msg.GetApi())
	}
	snaps := webapiconnect.NewSnapshotServiceClient(hc, server)
	snap, err := snaps.Ingest(ctx, connect.NewRequest(&webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: design}}))
	if err != nil {
		return nil, fmt.Errorf("calling Ingest: %w", err)
	}
	name := snap.Msg.GetName()
	defer snaps.DeleteSnapshot(ctx, connect.NewRequest(&webapi.DeleteSnapshotRequest{Name: name})) //nolint:errcheck

	d, err := webapiconnect.NewDesignViewServiceClient(hc, server).GetSummary(ctx, connect.NewRequest(&webapi.GetSummaryRequest{Snapshot: name}))
	if err != nil {
		return nil, fmt.Errorf("calling GetSummary: %w", err)
	}
	c, err := webapiconnect.NewCheckServiceClient(hc, server).CheckDesign(ctx, connect.NewRequest(&webapi.CheckDesignRequest{Snapshot: name}))
	if err != nil {
		return nil, fmt.Errorf("calling CheckDesign: %w", err)
	}
	q, err := webapiconnect.NewQueryServiceClient(hc, server).RunQuery(ctx, connect.NewRequest(&webapi.RunQueryRequest{Snapshot: name, Query: resistorsOf}))
	if err != nil {
		return nil, fmt.Errorf("calling RunQuery: %w", err)
	}
	board, err := boardGeometry(ctx, webapiconnect.NewDesignViewServiceClient(hc, server), name)
	if err != nil {
		return nil, err
	}
	tracks := 0
	for _, n := range board.GetNets() {
		tracks += len(n.GetSegments())
	}
	return &Summary{
		Version:     info.Msg.GetVersion(),
		API:         info.Msg.GetApi(),
		Features:    slices.Sorted(slices.Values(info.Msg.GetFeatures())),
		Components:  d.Msg.GetComponentCount(),
		Nets:        d.Msg.GetNetCount(),
		Sheets:      len(d.Msg.GetSheets()),
		Findings:    len(c.Msg.GetFindings()),
		QueryRows:   len(q.Msg.GetRows()),
		BoardLayers: len(board.GetLayers()),
		BoardParts:  len(board.GetPlacements()),
		BoardTracks: tracks,
	}, nil
}

// boardGeometry reads every page of the snapshot's board and merges them, as
// docsite/content/build/board-geometry.md says a client does.
func boardGeometry(ctx context.Context, designs webapiconnect.DesignViewServiceClient, snapshot string) (*geom.BoardGeometry, error) {
	board, token := &geom.BoardGeometry{}, ""
	for {
		page, err := designs.GetBoardGeometry(ctx, connect.NewRequest(&webapi.GetBoardGeometryRequest{Snapshot: snapshot, MaxBytes: boardPageBytes, PageToken: token}))
		if err != nil {
			return nil, fmt.Errorf("calling GetBoardGeometry: %w", err)
		}
		proto.Merge(board, page.Msg.GetBoard())
		if token = page.Msg.GetNextPageToken(); token == "" {
			return board, nil
		}
	}
}

// bearerTransport sends a bearer token on every request.
type bearerTransport struct {
	token string
	next  http.RoundTripper
}

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.next.RoundTrip(r)
}
