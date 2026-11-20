package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
// 1724), over goapplib storage's connection store. A server connection is a GitHub App installation
// or a token, each with its secret named by reference. A person's connection holds the caller's own
// GitHub token, sealed, and decides which linked designs they may open (agni issue 1728). Only its
// owner sees it.
type ConnectionService struct {
	store connections.Store
	// usedBy names the mounts that read through a connection, so one in use isn't deleted.
	usedBy func(ctx context.Context, id string) ([]string, error)
	// users seals and forgets people's tokens, and is set by WithUserTokens.
	users UserTokenKeeper
}

// UserTokenKeeper seals and forgets the token of a person's own connection (agni issue 1728).
// mountset.Table is the server's.
type UserTokenKeeper interface {
	// SaveUserToken seals token as owner's token for connection id. A server that can't seal one
	// returns ErrFailedPrecondition.
	SaveUserToken(ctx context.Context, id, owner, token string) error
	// Disconnect removes connection id and its owner's sealed token. A token the person pasted is
	// deleted, and one the server got by signing the person in is revoked at GitHub first. A
	// connection that isn't there is not an error.
	Disconnect(ctx context.Context, id string) error
}

// WithUserTokens lets people keep their own connections, sealing each token through k, and returns
// s. Without it, a person's connection is ErrFailedPrecondition.
func (s *ConnectionService) WithUserTokens(k UserTokenKeeper) *ConnectionService {
	s.users = k
	return s
}

// visible returns connection id when the caller may see it: a server connection, or one of the
// caller's own. Another person's is ErrNotFound, so its id says nothing about whose it is.
func (s *ConnectionService) visible(ctx context.Context, id string) (connections.Connection, error) {
	c, err := s.store.GetConnection(ctx, id)
	if err != nil {
		return c, ConnectionErr(err)
	}
	if c.Scope == connections.ScopePerson && c.Owner != CallerFrom(ctx).Subject {
		return connections.Connection{}, fmt.Errorf("%w: no connection %s", ErrNotFound, ConnectionName(c.ID))
	}
	return c, nil
}

// NewConnectionService serves the connections in store. usedBy names the mounts reading through a
// connection. A nil store serves none and refuses a write with ErrFailedPrecondition.
func NewConnectionService(store connections.Store, usedBy func(ctx context.Context, id string) ([]string, error)) *ConnectionService {
	return &ConnectionService{store: store, usedBy: usedBy}
}

// noConnections is why a server with no store refuses a connection.
const noConnections = "this server keeps no connections; start it with --store and --uploads"

// CreateConnection keeps a new connection: a server's, or the caller's own.
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
	person := in.GetScope() == webapi.ConnectionScope_CONNECTION_SCOPE_PERSON || in.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_GITHUB_USER
	switch {
	case person:
		// A person's connection takes their token itself, never a reference. A reference would let a
		// caller name one of the server's own secrets and read with it as themselves.
		if in.GetKind() != webapi.ConnectionKind_CONNECTION_KIND_GITHUB_USER && in.GetKind() != webapi.ConnectionKind_CONNECTION_KIND_UNSPECIFIED {
			return nil, fmt.Errorf("%w: a person's connection is CONNECTION_KIND_GITHUB_USER, with the token in user_token", ErrInvalidArgument)
		}
		if req.GetUserToken() == "" {
			return nil, fmt.Errorf("%w: a person's connection needs their GitHub token in user_token", ErrInvalidArgument)
		}
		c.Owner = CallerFrom(ctx).Subject
		if c.Owner == "" {
			return nil, fmt.Errorf("%w: a person's connection belongs to the caller, and this server doesn't know who is asking; start it with --auth tokens", ErrFailedPrecondition)
		}
		if s.users == nil {
			return nil, fmt.Errorf("%w: this server keeps no person's token; start it with --connections-key env:NAME", ErrFailedPrecondition)
		}
		c.Kind, c.Scope = connections.KindGitHubUser, connections.ScopePerson
	case req.GetUserToken() != "":
		return nil, fmt.Errorf("%w: user_token is for a person's connection; a server connection names where its secret is", ErrInvalidArgument)
	case in.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_GITHUB_APP:
		c.Kind = connections.KindGitHubApp
	case in.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN:
		c.Kind = connections.KindGitHubToken
	default:
		return nil, fmt.Errorf("%w: a connection's kind is CONNECTION_KIND_GITHUB_APP, CONNECTION_KIND_GITHUB_TOKEN or CONNECTION_KIND_GITHUB_USER", ErrInvalidArgument)
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
	if person {
		if err := s.users.SaveUserToken(ctx, c.ID, c.Owner, req.GetUserToken()); err != nil {
			_ = s.store.DeleteConnection(ctx, c.ID)
			return nil, err
		}
	}
	return connectionProto(c), nil
}

// GetConnection returns one connection by resource name.
func (s *ConnectionService) GetConnection(ctx context.Context, req *webapi.GetConnectionRequest) (*webapi.Connection, error) {
	if s.store == nil {
		return nil, fmt.Errorf("%w: no connection %s", ErrNotFound, req.GetName())
	}
	c, err := s.visible(ctx, ConnectionID(req.GetName()))
	if err != nil {
		return nil, err
	}
	return connectionProto(c), nil
}

// ListConnections returns the server's connections and the caller's own, ordered by name.
func (s *ConnectionService) ListConnections(ctx context.Context, _ *webapi.ListConnectionsRequest) (*webapi.ListConnectionsResponse, error) {
	resp := &webapi.ListConnectionsResponse{}
	if s.store == nil {
		return resp, nil
	}
	all, err := s.store.ListConnections(ctx, "")
	if err != nil {
		return nil, ConnectionErr(err)
	}
	if me := CallerFrom(ctx).Subject; me != "" {
		mine, err := s.store.ListConnections(ctx, me)
		if err != nil {
			return nil, ConnectionErr(err)
		}
		all = append(all, mine...)
		slices.SortFunc(all, func(a, b connections.Connection) int { return strings.Compare(a.ID, b.ID) })
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
	c, err := s.visible(ctx, id)
	if err != nil {
		return err
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
	if c.Kind == connections.KindGitHubUser && s.users != nil {
		return ConnectionErr(s.users.Disconnect(ctx, id))
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
	case connections.KindGitHubUser:
		out.Kind = webapi.ConnectionKind_CONNECTION_KIND_GITHUB_USER
	}
	out.Scope, out.Owner = webapi.ConnectionScope_CONNECTION_SCOPE_SERVER, c.Owner
	if c.Scope == connections.ScopePerson {
		out.Scope = webapi.ConnectionScope_CONNECTION_SCOPE_PERSON
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
