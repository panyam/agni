// Command go is the Go reference for `make exercise-clients` (agni issue 1628). It does what each
// language's example does, through that language's client, against a shared `agni serve`: it checks
// the server, ingests a design, asks it three questions, and prints one line of JSON summing up the
// answers. The exercise compares every language's line with this one and with the CLI's answers.
// docsite/content/build/client-contract.md names the line's fields.
//
//	go run ./clients/examples/go -server http://127.0.0.1:8080 -design mount://probe/designs/jetson-cut
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"slices"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
)

// Summary is the line every language's example prints. Its JSON names are the contract.
type Summary struct {
	Version    string   `json:"version"`
	API        string   `json:"api"`
	Features   []string `json:"features"`
	Components int32    `json:"components"`
	Nets       int32    `json:"nets"`
	Sheets     int      `json:"sheets"`
	Findings   int      `json:"findings"`
	QueryRows  int      `json:"query_rows"`
}

// The query every example asks, so their answers compare. The check runs every rule.
const resistorsOf = `component.class(?c, "resistor") => ?c`

func main() {
	server := flag.String("server", "http://127.0.0.1:8080", "the agni serve to call")
	design := flag.String("design", "mount://probe/designs/jetson-cut", "the design to ingest")
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
	hc := http.DefaultClient
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

	d, err := webapiconnect.NewDesignServiceClient(hc, server).GetDesign(ctx, connect.NewRequest(&webapi.GetDesignRequest{Snapshot: name}))
	if err != nil {
		return nil, fmt.Errorf("calling GetDesign: %w", err)
	}
	c, err := webapiconnect.NewCheckServiceClient(hc, server).CheckDesign(ctx, connect.NewRequest(&webapi.CheckDesignRequest{Snapshot: name}))
	if err != nil {
		return nil, fmt.Errorf("calling CheckDesign: %w", err)
	}
	q, err := webapiconnect.NewQueryServiceClient(hc, server).RunQuery(ctx, connect.NewRequest(&webapi.RunQueryRequest{Snapshot: name, Query: resistorsOf}))
	if err != nil {
		return nil, fmt.Errorf("calling RunQuery: %w", err)
	}
	return &Summary{
		Version:    info.Msg.GetVersion(),
		API:        info.Msg.GetApi(),
		Features:   slices.Sorted(slices.Values(info.Msg.GetFeatures())),
		Components: d.Msg.GetComponentCount(),
		Nets:       d.Msg.GetNetCount(),
		Sheets:     len(d.Msg.GetSheets()),
		Findings:   len(c.Msg.GetFindings()),
		QueryRows:  len(q.Msg.GetRows()),
	}, nil
}
