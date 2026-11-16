package service

import (
	"context"
	"slices"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

type persistentStore struct{ *MemSnapshotStore }

func (persistentStore) Persistent() bool { return true }

// ServerInfo names the host's own features and the snapshot ones its resolver gives, sorted, and the
// snapshot_store feature only when definitions outlive the process (agni issue 1629).
func TestServerInfoNamesItsFeatures(t *testing.T) {
	ctx := context.Background()
	mem := &ServerInfoService{Version: "v0.7.0", Features: []string{"grpc"}, Projects: &ProjectResolver{}}
	got, err := mem.GetServerInfo(ctx, &webapi.GetServerInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetVersion() != "v0.7.0" || got.GetApi() != "agni.v1" || !slices.Equal(got.GetFeatures(), []string{"grpc", "snapshot_recovery"}) {
		t.Errorf("an in-memory host answered %v", got)
	}
	disk := &ServerInfoService{Version: "v0.7.0", Features: []string{"grpc"}, Projects: &ProjectResolver{Snapshots: persistentStore{NewMemSnapshotStore()}}}
	got, _ = disk.GetServerInfo(ctx, &webapi.GetServerInfoRequest{})
	if !slices.Equal(got.GetFeatures(), []string{"grpc", "snapshot_recovery", "snapshot_store"}) {
		t.Errorf("a host with a persistent store answered features %v", got.GetFeatures())
	}
}
