package mcpserver

import (
	"net/http"
	"slices"

	"github.com/panyam/agni/service"
	"github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/experimental/ext/rooms"
)

// callerAuth hands mcpkit the caller that agni's gate already checked (agni issue 1902). The gate
// wraps /mcp, refuses a call without a valid token, and puts the token's subject on the request as
// a service.Caller. mcpkit's tools then see that subject as the call's claims, and the room bridge
// acts for it. So a tab's switch agrees to its own reader's agents alone.
type callerAuth struct{}

// Validate refuses a request that reached /mcp with no caller, which happens only if the gate was
// left off.
func (callerAuth) Validate(r *http.Request) error {
	if service.CallerFrom(r.Context()).Subject == "" {
		return &core.AuthError{Code: http.StatusUnauthorized, Message: "this server signs people in, and the call came with no signed-in caller"}
	}
	return nil
}

// Claims is the caller as mcpkit's claims: its subject, and as scopes the grant claims a sign-in
// recorded plus the room bridge's three scopes. agni grants no scopes of its own, so every
// signed-in person may use the bridge's tools, and each tab's agreement decides which tabs they
// reach.
func (callerAuth) Claims(r *http.Request) *core.Claims {
	c := service.CallerFrom(r.Context())
	scopes := append(slices.Clone(c.Claims), rooms.ScopeRoomsAttach, rooms.ScopePagesRead, rooms.ScopePagesAct)
	return &core.Claims{Subject: c.Subject, Scopes: scopes}
}
