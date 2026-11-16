package service

import (
	"context"
	"slices"
	"sync"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/proto"
)

// SnapshotStore keeps snapshot definitions, so a host that evicted a snapshot, or restarted, can
// ingest it again when a question names it (agni issue 1639). Like ReviewStore, it is a port: the
// service does no I/O, and an os-backed store in cmd/agni keeps definitions in a writable state
// directory. A definition is what a client sent to Ingest or UpdateSnapshot, a few hundred bytes. The
// parsed design is not kept here, since the DesignCache beneath the snapshots keeps it by content.
type SnapshotStore interface {
	// Put records name's definition, replacing any it had.
	Put(ctx context.Context, name string, def *webapi.Snapshot) error
	// Get returns name's definition. A name the store does not hold is ErrNotFound.
	Get(ctx context.Context, name string) (*webapi.Snapshot, error)
	// Delete forgets name. Deleting a name the store does not hold is not an error, since a snapshot
	// of uploaded files was never put.
	Delete(ctx context.Context, name string) error
	// Persistent reports whether the store outlives the process, which ServerInfo reports as the
	// snapshot_store feature.
	Persistent() bool
}

// memSnapshotStoreLimit bounds the in-memory store. A definition is small, so it holds far more than
// the registry's 256 snapshots, and an evicted snapshot stays recoverable long after.
const memSnapshotStoreLimit = 16384

// MemSnapshotStore keeps definitions in memory, the default when a host names no store. It recovers
// a snapshot the registry evicted, and nothing across a restart. Past its limit it forgets the
// oldest definition put.
type MemSnapshotStore struct {
	mu    sync.Mutex
	defs  map[string]*webapi.Snapshot
	order []string
}

// NewMemSnapshotStore returns an empty in-memory store.
func NewMemSnapshotStore() *MemSnapshotStore {
	return &MemSnapshotStore{defs: map[string]*webapi.Snapshot{}}
}

func (m *MemSnapshotStore) Put(_ context.Context, name string, def *webapi.Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.defs[name]; !ok {
		m.order = append(m.order, name)
	}
	m.defs[name] = proto.Clone(def).(*webapi.Snapshot)
	for len(m.defs) > memSnapshotStoreLimit && len(m.order) > 0 {
		delete(m.defs, m.order[0])
		m.order = m.order[1:]
	}
	return nil
}

func (m *MemSnapshotStore) Get(_ context.Context, name string) (*webapi.Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.defs[name]
	if !ok {
		return nil, ErrNotFound
	}
	return proto.Clone(d).(*webapi.Snapshot), nil
}

func (m *MemSnapshotStore) Delete(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.defs[name]; ok {
		delete(m.defs, name)
		m.order = slices.DeleteFunc(m.order, func(n string) bool { return n == name })
	}
	return nil
}

// Persistent is false: the store ends with the process.
func (m *MemSnapshotStore) Persistent() bool { return false }
