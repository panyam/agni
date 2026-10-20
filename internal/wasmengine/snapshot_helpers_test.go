package wasmengine

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
)

// ingestAt ingests def on the server at base, as a client of a served host does before it asks.
func ingestAt(t testing.TB, c *http.Client, base string, def *webapi.Snapshot) string {
	t.Helper()
	s, err := webapiconnect.NewSnapshotServiceClient(c, base).Ingest(context.Background(), connect.NewRequest(&webapi.IngestRequest{Snapshot: def}))
	if err != nil {
		t.Fatalf("ingest %s: %v", def.GetUri(), err)
	}
	return s.Msg.GetName()
}
