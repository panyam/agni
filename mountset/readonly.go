package mountset

import (
	"context"
	"errors"

	"github.com/panyam/goapplib/storage/connections"
	"github.com/panyam/oneauth/outbound"
	"golang.org/x/oauth2"
)

// A reader such as agnids opens agni serve's connections and people's sealed tokens to answer
// CanRead for its callers, and never writes either (agni issue 1778). goapplib's Service stamps a
// connection's LastUsed and status as it hands out tokens, so readOnlyConnections drops those
// writes. A refresh would be worse. GitHub rotates a person's refresh token on each use, so a
// refresh here would leave agni serve holding a dead one. noRefresh points the OAuth client at an
// address that never reaches GitHub, so a token that has expired fails here, and the caller's
// answer is no until agni serve refreshes it.

// readOnlyConnections is a connections.Store whose writes do nothing.
type readOnlyConnections struct{ connections.Store }

func (readOnlyConnections) PutConnection(context.Context, connections.Connection) error { return nil }

func (readOnlyConnections) DeleteConnection(context.Context, string, string) error { return nil }

// errReadOnlyTokens is why a reader refuses to write a person's token.
var errReadOnlyTokens = errors.New("this server reads people's tokens and never writes them")

// readOnlyTokens is an outbound.UserTokenStore that reads alone.
type readOnlyTokens struct{ outbound.UserTokenStore }

func (readOnlyTokens) PutUserToken(context.Context, *outbound.PutUserTokenRequest) (*outbound.PutUserTokenResponse, error) {
	return nil, errReadOnlyTokens
}

func (readOnlyTokens) DeleteUserToken(context.Context, *outbound.DeleteUserTokenRequest) (*outbound.DeleteUserTokenResponse, error) {
	return nil, errReadOnlyTokens
}

// noRefresh is the OAuth client a reader hands goapplib. Its token endpoint is an address no request
// reaches, so a person's refresh token is never sent anywhere.
var noRefresh = &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: "http://refresh.invalid/agni-reads-only"}} // #nosec G101 -- an address no request reaches, not a credential
