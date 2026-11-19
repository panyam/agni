package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/panyam/goapplib/storage/connections"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// connectionsNamePrefix starts a connection's resource name, "connections/{connection}".
const connectionsNamePrefix = "connections/"

// ConnectionName builds a connection's resource name, "connections/{connection}".
func ConnectionName(id string) string { return connectionsNamePrefix + id }

// ConnectionID reads the id out of "connections/{connection}". A bare id is accepted too, since a
// mount record and the CLI name a connection by its id.
func ConnectionID(name string) string { return strings.TrimPrefix(name, connectionsNamePrefix) }

// ConnectionService serves the connections a server reads private sources through (agni issue
// 1724), over goapplib storage's connection store. It keeps server-scope connections only: a GitHub
// App installation or a token, each with its secret named by reference. A connection a person owns
// waits on the server knowing who is asking (agni issue 1728).
type ConnectionService struct {
	store connections.Store
	// usedBy names the mounts that read through a connection, so one in use isn't deleted.
	usedBy func(ctx context.Context, id string) ([]string, error)
}

// NewConnectionService serves the connections in store. usedBy names the mounts reading through a
// connection. A nil store serves none and refuses a write with ErrFailedPrecondition.
func NewConnectionService(store connections.Store, usedBy func(ctx context.Context, id string) ([]string, error)) *ConnectionService {
	return &ConnectionService{store: store, usedBy: usedBy}
}

// noConnections is why a server with no store refuses a connection.
const noConnections = "this server keeps no connections; start it with --store and --uploads"

// CreateConnection keeps a new server-scope connection.
func (s *ConnectionService) CreateConnection(ctx context.Context, req *webapi.CreateConnectionRequest) (*webapi.Connection, error) {
	if s.store == nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedPrecondition, noConnections)
	}
	in := req.GetConnection()
	c := connections.Connection{
		ID: req.GetConnectionId(), Scope: connections.ScopeServer, Name: in.GetDisplayName(), BaseURL: in.GetBaseUrl(),
		AppID: in.GetAppId(), InstallationID: in.GetInstallationId(), KeyRef: in.GetKeySource(), TokenRef: in.GetTokenSource(),
		Status: connections.StatusOK, Created: time.Now().UTC(),
	}
	switch in.GetKind() {
	case webapi.ConnectionKind_CONNECTION_KIND_GITHUB_APP:
		c.Kind = connections.KindGitHubApp
	case webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN:
		c.Kind = connections.KindGitHubToken
	default:
		return nil, fmt.Errorf("%w: a connection's kind is CONNECTION_KIND_GITHUB_APP or CONNECTION_KIND_GITHUB_TOKEN", ErrInvalidArgument)
	}
	if err := c.Validate(); err != nil {
		return nil, ConnectionErr(err)
	}
	if _, err := s.store.GetConnection(ctx, c.ID); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrAlreadyExists, ConnectionName(c.ID))
	} else if !errors.Is(err, connections.ErrNotFound) {
		return nil, ConnectionErr(err)
	}
	if err := s.store.PutConnection(ctx, c); err != nil {
		return nil, ConnectionErr(err)
	}
	return connectionProto(c), nil
}

// GetConnection returns one connection by resource name.
func (s *ConnectionService) GetConnection(ctx context.Context, req *webapi.GetConnectionRequest) (*webapi.Connection, error) {
	if s.store == nil {
		return nil, fmt.Errorf("%w: no connection %s", ErrNotFound, req.GetName())
	}
	c, err := s.store.GetConnection(ctx, ConnectionID(req.GetName()))
	if err != nil {
		return nil, ConnectionErr(err)
	}
	return connectionProto(c), nil
}

// ListConnections returns the server's connections, ordered by name.
func (s *ConnectionService) ListConnections(ctx context.Context, _ *webapi.ListConnectionsRequest) (*webapi.ListConnectionsResponse, error) {
	resp := &webapi.ListConnectionsResponse{}
	if s.store == nil {
		return resp, nil
	}
	all, err := s.store.ListConnections(ctx, "")
	if err != nil {
		return nil, ConnectionErr(err)
	}
	for _, c := range all {
		resp.Connections = append(resp.Connections, connectionProto(c))
	}
	return resp, nil
}

// DeleteConnection forgets a connection no mount reads through.
func (s *ConnectionService) DeleteConnection(ctx context.Context, req *webapi.DeleteConnectionRequest) error {
	if s.store == nil {
		return fmt.Errorf("%w: %s", ErrFailedPrecondition, noConnections)
	}
	id := ConnectionID(req.GetName())
	if _, err := s.store.GetConnection(ctx, id); err != nil {
		return ConnectionErr(err)
	}
	if s.usedBy != nil {
		mounts, err := s.usedBy(ctx, id)
		if err != nil {
			return err
		}
		if len(mounts) > 0 {
			return fmt.Errorf("%w: the mount%s %s read%s through %s; delete %s first", ErrFailedPrecondition, plural(len(mounts), "", "s"), listSome(mounts, 3), plural(len(mounts), "s", ""), ConnectionName(id), plural(len(mounts), "it", "them"))
		}
	}
	return ConnectionErr(s.store.DeleteConnection(ctx, id))
}

// ConnectionErr maps an error from goapplib's connections package onto this package's errors, and
// passes any other error through.
func ConnectionErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, connections.ErrNotFound):
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	case errors.Is(err, connections.ErrInvalid):
		return fmt.Errorf("%w: %w", ErrInvalidArgument, err)
	case errors.Is(err, connections.ErrNotConfigured), errors.Is(err, connections.ErrReconnect), errors.Is(err, connections.ErrNotOwner):
		return fmt.Errorf("%w: %w", ErrFailedPrecondition, err)
	}
	return err
}

// connectionProto is c as the wire carries it. Its secret references are names of places, not
// secrets, so they go back as they are.
func connectionProto(c connections.Connection) *webapi.Connection {
	out := &webapi.Connection{
		Name: ConnectionName(c.ID), DisplayName: c.Name, BaseUrl: c.BaseURL, AppId: c.AppID, InstallationId: c.InstallationID,
		KeySource: c.KeyRef, TokenSource: c.TokenRef, LastError: c.LastError, Status: webapi.ConnectionStatus_CONNECTION_STATUS_OK,
	}
	switch c.Kind {
	case connections.KindGitHubApp:
		out.Kind = webapi.ConnectionKind_CONNECTION_KIND_GITHUB_APP
	case connections.KindGitHubToken:
		out.Kind = webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN
	}
	if c.Status == connections.StatusFailed || c.Status == connections.StatusReconnect {
		out.Status = webapi.ConnectionStatus_CONNECTION_STATUS_FAILED
	}
	if !c.Created.IsZero() {
		out.CreateTime = timestamppb.New(c.Created)
	}
	if !c.LastUsed.IsZero() {
		out.LastUseTime = timestamppb.New(c.LastUsed)
	}
	return out
}
