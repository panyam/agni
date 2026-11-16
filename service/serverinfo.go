package service

import (
	"context"
	"slices"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// APIPackage is the proto package every rpc of this API belongs to. A client compiled against
// another package cannot rely on any rpc's shape (agni issue 1629).
const APIPackage = "agni.v1"

// ServerInfoService answers what a host is, so a client can check it before relying on it (agni
// issue 1629): the release it was built from, the proto package it speaks, and the optional features
// it has on.
type ServerInfoService struct {
	// Version is the release the host was built from, internal/version's answer.
	Version string
	// Features are the host's own optional features, such as "grpc" for a server whose listener
	// speaks it. The snapshot features are added from Projects.
	Features []string
	// Projects is the resolver the host's snapshots live on. It adds snapshot_recovery, and
	// snapshot_store when its definitions outlive the process.
	Projects *ProjectResolver
}

// GetServerInfo answers the host's version, package and features. Features are sorted, so a client
// or a test can compare them as a list.
func (s *ServerInfoService) GetServerInfo(context.Context, *webapi.GetServerInfoRequest) (*webapi.ServerInfo, error) {
	features := append(slices.Clone(s.Features), "snapshot_recovery")
	if s.Projects.SnapshotsPersist() {
		features = append(features, "snapshot_store")
	}
	slices.Sort(features)
	return &webapi.ServerInfo{Version: s.Version, Api: APIPackage, Features: slices.Compact(features)}, nil
}
