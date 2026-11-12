package fshost

import (
	"context"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// ingestWith ingests def through ss and returns the snapshot. ss is the snapshot service of the
// service under test. A request then names the snapshot as a client's does after ingesting (agni
// issue 1061).
func ingestWith(t testing.TB, ss *service.SnapshotService, def *webapi.Snapshot) string {
	t.Helper()
	s, err := ss.Ingest(context.Background(), &webapi.IngestRequest{Snapshot: def})
	if err != nil {
		t.Fatalf("ingest %s: %v", def.GetUri(), err)
	}
	return s.GetName()
}
