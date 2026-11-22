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
	// baseURL is the API a new connection reads through when it names none, set by WithBaseURL.
	baseURL string
}

// UserTokenKeeper seals and forgets the token of a person's own connection (agni issue 1728).
// mountset.Table is the server's.
type UserTokenKeeper interface {
	// SaveUserToken seals token as owner's token for their connection id. A server that can't seal
	// one returns ErrFailedPrecondition.
	SaveUserToken(ctx context.Context, owner, id, token string) error
	// Disconnect removes owner's connection id and its sealed token. A token the person pasted is
	// deleted, and one the server got by signing the person in is revoked at GitHub first. A
	// connection that isn't there is not an error. With owner "" it removes a sealed server
	// connection, its secret first.
	Disconnect(ctx context.Context, owner, id string) error
	// SaveServerSecret seals secret as server connection id's token or App key, replacing any before
	// (agni issue 1802). The connection's record must already say it is sealed.
	SaveServerSecret(ctx context.Context, id, secret string) error
}

// WithUserTokens lets people keep their own connections, sealing each token through k, and returns
// s. Without it, a person's connection is ErrFailedPrecondition.
func (s *ConnectionService) WithUserTokens(k UserTokenKeeper) *ConnectionService {
	s.users = k
	return s
}

// visible returns the connection id names for the caller: their own when they keep one by that id,
// else the server's (agni issue 1778). Each person's ids are their own, so another person's
// connection never answers, and its id says nothing about whose it is.
func (s *ConnectionService) visible(ctx context.Context, id string) (connections.Connection, error) {
	if me := CallerFrom(ctx).Subject; me != "" {
		c, err := s.store.GetConnection(ctx, me, id)
		if err == nil {
			return c, nil
		}
		if !errors.Is(err, connections.ErrNotFound) {
			return c, ConnectionErr(err)
		}
	}
	c, err := s.store.GetConnection(ctx, "", id)
	return c, ConnectionErr(err)
}

// WithBaseURL makes url the API a new connection reads through when the request names none, such
// as the server's --github-api for a GitHub Enterprise host (agni issue 1772). Empty leaves it to
// goapplib, which reads github.com. It returns s.
func (s *ConnectionService) WithBaseURL(url string) *ConnectionService {
	s.baseURL = url
	return s
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
	if c.BaseURL == "" {
		c.BaseURL = s.baseURL
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
		return nil, fmt.Errorf("%w: user_token is for a person's connection; a server connection names where its secret is, or sends it in server_secret", ErrInvalidArgument)
	case in.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_GITHUB_APP:
		c.Kind = connections.KindGitHubApp
	case in.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN:
		c.Kind = connections.KindGitHubToken
	default:
		return nil, fmt.Errorf("%w: a connection's kind is CONNECTION_KIND_GITHUB_APP, CONNECTION_KIND_GITHUB_TOKEN or CONNECTION_KIND_GITHUB_USER", ErrInvalidArgument)
	}
	sealed := req.GetServerSecret() != ""
	if sealed {
		// A sealed secret replaces the reference, so naming both says two things about one secret.
		if person {
			return nil, fmt.Errorf("%w: server_secret is for a server connection; a person's connection sends user_token", ErrInvalidArgument)
		}
		if c.TokenRef != "" || c.KeyRef != "" {
			return nil, fmt.Errorf("%w: a connection with server_secret keeps it sealed, so it names no token_source or key_source", ErrInvalidArgument)
		}
		if s.users == nil {
			return nil, fmt.Errorf("%w: this server seals no secret; start it with --connections-key env:NAME", ErrFailedPrecondition)
		}
		c.Sealed = true
	}
	if err := c.Validate(); err != nil {
		return nil, ConnectionErr(err)
	}
	// A person's ids are their own, so only their own connections and the server's can clash. The
	// server's are listed to every caller, so refusing one of those ids tells a person nothing new.
	if err := s.taken(ctx, c.Owner, c.ID); err != nil {
		return nil, err
	}
	if person {
		if err := s.taken(ctx, "", c.ID); err != nil {
			return nil, fmt.Errorf("%w; a person's connection can't share a server connection's id", err)
		}
	}
	if err := s.store.PutConnection(ctx, c); err != nil {
		return nil, ConnectionErr(err)
	}
	if person {
		if err := s.users.SaveUserToken(ctx, c.Owner, c.ID, req.GetUserToken()); err != nil {
			_ = s.store.DeleteConnection(ctx, c.Owner, c.ID)
			return nil, err
		}
	}
	if sealed {
		if err := s.users.SaveServerSecret(ctx, c.ID, req.GetServerSecret()); err != nil {
			_ = s.store.DeleteConnection(ctx, "", c.ID)
			return nil, ConnectionErr(err)
		}
	}
	return connectionProto(c), nil
}

// RotateConnection replaces a sealed server connection's secret.
func (s *ConnectionService) RotateConnection(ctx context.Context, req *webapi.RotateConnectionRequest) (*webapi.Connection, error) {
	if s.store == nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedPrecondition, noConnections)
	}
	if req.GetServerSecret() == "" {
		return nil, fmt.Errorf("%w: server_secret is empty", ErrInvalidArgument)
	}
	id := ConnectionID(req.GetName())
	c, err := s.visible(ctx, id)
	if err != nil {
		return nil, err
	}
	switch {
	case c.Owner != "":
		return nil, fmt.Errorf("%w: %s is your own connection; add it again to change its token", ErrInvalidArgument, ConnectionName(id))
	case !c.Sealed:
		return nil, fmt.Errorf("%w: %s names where its secret is; change the file or variable it names", ErrFailedPrecondition, ConnectionName(id))
	case s.users == nil:
		return nil, fmt.Errorf("%w: this server seals no secret; start it with --connections-key env:NAME", ErrFailedPrecondition)
	}
	if err := s.users.SaveServerSecret(ctx, id, req.GetServerSecret()); err != nil {
		return nil, ConnectionErr(err)
	}
	c, err = s.store.GetConnection(ctx, "", id)
	if err != nil {
		return nil, ConnectionErr(err)
	}
	return connectionProto(c), nil
}

// taken is ErrAlreadyExists when owner keeps a connection id, where "" is the server.
func (s *ConnectionService) taken(ctx context.Context, owner, id string) error {
	_, err := s.store.GetConnection(ctx, owner, id)
	switch {
	case err == nil:
		return fmt.Errorf("%w: %s", ErrAlreadyExists, ConnectionName(id))
	case errors.Is(err, connections.ErrNotFound):
		return nil
	}
	return ConnectionErr(err)
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
	// Mounts read only through server connections, so only a server connection can be in use.
	if s.usedBy != nil && c.Owner == "" {
		mounts, err := s.usedBy(ctx, id)
		if err != nil {
			return err
		}
		if len(mounts) > 0 {
			return fmt.Errorf("%w: the mount%s %s read%s through %s; delete %s first", ErrFailedPrecondition, plural(len(mounts), "", "s"), listSome(mounts, 3), plural(len(mounts), "s", ""), ConnectionName(id), plural(len(mounts), "it", "them"))
		}
	}
	// A sealed secret lives beside the record, so Disconnect deletes it before the record.
	if (c.Kind == connections.KindGitHubUser || c.Sealed) && s.users != nil {
		return ConnectionErr(s.users.Disconnect(ctx, c.Owner, id))
	}
	return ConnectionErr(s.store.DeleteConnection(ctx, c.Owner, id))
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
	case errors.Is(err, connections.ErrNotConfigured), errors.Is(err, connections.ErrReconnect), errors.Is(err, connections.ErrNotOwner),
		errors.Is(err, connections.ErrNoSecret):
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
		Sealed: c.Sealed,
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
