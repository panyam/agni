package main

import (
	"context"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// ingestWith is a test's snapshot of def through ss, the snapshot service of the service under test,
// so a request names it as a client's does after ingesting (agni issue 1061).
func ingestWith(t testing.TB, ss *service.SnapshotService, def *webapi.Snapshot) string {
	t.Helper()
	s, err := ss.Ingest(context.Background(), &webapi.IngestRequest{Snapshot: def})
	if err != nil {
		t.Fatalf("ingest %s: %v", def.GetUri(), err)
	}
	return s.GetName()
}

// snapOf is ingestWith for a design named by uri alone.
func snapOf(t testing.TB, ss *service.SnapshotService, uri string) string {
	t.Helper()
	return ingestWith(t, ss, &webapi.Snapshot{Uri: uri})
}
