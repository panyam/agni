// Package viewerrooms lets an agent drive a viewer tab the reader has open (agni issue 1154). It
// serves a massrelay relay and mcpkit's room bridge in agni's own process: each viewer page joins
// the room of the design it shows (web/src/viewerroom.ts) and offers ViewerPageService's methods
// there as tools, and the bridge serves those tools to agents over agni's /mcp, each call naming the
// design it is about. A tab takes calls only after its reader turns on "let my agents drive this
// tab".
//
// It is single-user local mode. The server has no identity provider, so every relay token it mints
// and every MCP session it serves act for one local subject, and the tab's switch is the only
// agreement there is. Serving several people (OAuth on /mcp, a relay token per user, pairing codes)
// is a later step; mcpkit's bridge already supports it.
//
// The bridge declares its page tools up front (rooms.WithPageTools), built from ViewerPageService's
// descriptor by ToolDecls, so tools/list is the same for every caller and the bridge serves the
// stateless MCP wire as well as the session one. A tab's tools are checked against the declaration
// byte for byte, which is why web/src/toolschema.ts builds the same schemas and both are held to
// testdata/viewer_tools.json.
//
// What a reader does on a tab reaches an agent three ways, all under the same agreement: polling
// next_request, reading rooms://events/{room}, and the SEP-3415 events mcpkit's roomsevents serves
// (room.ask, page.selection_changed, page.event). An event is pushed to a session attached to the
// room, so it needs the session wire, and rooms-channel turns each one into a message in a Claude
// Code session (agni issue 1173).
package viewerrooms

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/panyam/massrelay/services"
	relaymw "github.com/panyam/massrelay/web/middleware"
	relayserver "github.com/panyam/massrelay/web/server"
	"github.com/panyam/mcpkit/experimental/ext/events"
	"github.com/panyam/mcpkit/experimental/ext/rooms"
	"github.com/panyam/mcpkit/experimental/ext/rooms/massrelay"
	"github.com/panyam/mcpkit/experimental/integrations/roomsevents"
	mcpkit "github.com/panyam/mcpkit/server"
	"github.com/panyam/oneauth/keys"
	mw "github.com/panyam/servicekit/middleware"
)

const (
	// LocalSubject is who every relay token and every MCP session acts for: the one reader of a
	// local server. A tab's agents switch agrees to agents acting for it.
	LocalSubject = "local"
	// clientID names the app the relay tokens are minted for, which the relay looks the signing key
	// up by.
	clientID = "agni-viewer"
	// issuer is the relay tokens' "iss", which the relay requires.
	issuer = "agni"
	// bridgeSubject is the bridge's own subject on the relay.
	bridgeSubject = "agni-bridge"
	// DefaultTokenTTL is how long a minted token admits a join. A page already in the room stays.
	DefaultTokenTTL = 10 * time.Minute
)

// Options configure New.
type Options struct {
	// RoomOf is the room a design names, mount/path to "design:<mount>/<entry>"
	// (service.ViewerRoomService.Room), so an agent's call and a tab's join resolve one way. Required.
	RoomOf func(ctx context.Context, design string) (string, error)
	// Origins are the hosts whose pages may open the relay's WebSocket ("localhost", "127.0.0.1",
	// a LAN address), as the viewer is served at. Empty allows any origin.
	Origins []string
	// TokenTTL is how long a minted token admits a join; zero is DefaultTokenTTL.
	TokenTTL time.Duration
	// CallTimeout is how long the bridge waits for a tab to answer; zero is the bridge's default.
	CallTimeout time.Duration
}

// Rooms is the relay and the bridge, ready to mount on a server.
type Rooms struct {
	relay  *relayserver.RelayApp
	bridge *rooms.Bridge
	events *roomsevents.Events
	key    []byte
	ttl    time.Duration
}

// New builds the relay, with a signing key of its own for this process, and the bridge joined to it
// in-process.
func New(o Options) (*Rooms, error) {
	if o.RoomOf == nil {
		return nil, errors.New("viewerrooms: Options.RoomOf is required")
	}
	r := &Rooms{key: make([]byte, 32), ttl: o.TokenTTL, events: roomsevents.New()}
	if r.ttl <= 0 {
		r.ttl = DefaultTokenTTL
	}
	if _, err := rand.Read(r.key); err != nil {
		return nil, err
	}
	ks := keys.NewInMemoryKeyStore()
	if _, err := ks.PutKey(context.Background(), &keys.PutKeyRequest{Record: &keys.KeyRecord{
		ClientID: clientID, Key: r.key, Algorithm: "HS256",
	}}); err != nil {
		return nil, err
	}
	auth := relaymw.NewRelayAuthenticator(relaymw.RelayAuthConfig{
		KeyStore: ks, Required: true, Issuer: issuer, TokenQueryParam: "token",
	})
	// NewRelayApp reads RELAY_* from the environment for a standalone relay. The parts that decide
	// who gets in are replaced here, so none of those variables can open this one up.
	r.relay = relayserver.NewRelayApp()
	origins := mw.NewOriginChecker(o.Origins)
	r.relay.OriginChecker = origins
	r.relay.AdminToken = ""
	r.relay.PRM = nil
	r.relay.Guard = relaymw.NewGuard(origins, nil, auth, nil, nil)
	if err := r.relay.Init(); err != nil {
		return nil, err
	}
	opts := []rooms.Option{
		rooms.WithLocalSubject(LocalSubject),
		rooms.WithPageTools(ToolDecls()),
		// The bridge's own tools (list_pages, attach, pair, next_request) take design too, so an agent
		// names a room the same way for every tool (agni issue 1173, mcpkit#1633).
		rooms.WithRoomOf(func(args map[string]any) (string, error) {
			design, _ := args["design"].(string)
			if design == "" {
				return "", errors.New("name the design the tab shows, as mount/path (the designs tool lists them)")
			}
			return o.RoomOf(context.Background(), design)
		}, rooms.RoomArgs{"design": {"type": "string", "description": "The design the tab shows, as mount/path (tut/designs/gateway)."}}),
		// A reader's question from a tab's Ask box, and what the reader selected, reach an agent through
		// next_request or the room's events resource, from the tabs that agreed to it.
		rooms.WithEventFallbacks(0),
		rooms.WithPageEvents(r.events.Publish),
	}
	if o.CallTimeout > 0 {
		opts = append(opts, rooms.WithCallTimeout(o.CallTimeout))
	}
	r.bridge = rooms.New(massrelay.Embedded(r.relay.Service, &services.Principal{Subject: bridgeSubject, ClientID: clientID}), opts...)
	return r, nil
}

// Relay is the relay's HTTP handler, which expects its own paths, so mount it under a prefix with
// http.StripPrefix: mux.Handle("/relay/", http.StripPrefix("/relay", r.Relay())).
func (r *Rooms) Relay() http.Handler { return r.relay }

// Register adds the bridge's tools (attach, list_pages, pair and the declared viewer_* tools) to an
// MCP server, and the room events an attached session subscribes to. The events act for
// LocalSubject on a call with no authenticated caller, as the bridge's tools do, since events/stream
// refuses one otherwise.
func (r *Rooms) Register(srv *mcpkit.Server) {
	r.bridge.Register(srv)
	r.events.Register(srv, r.bridge, events.Config{UnsafeAnonymousPrincipal: LocalSubject})
}

// Mint is a relay token for the local subject admitting its holder to room alone, which
// service.ViewerRoomService hands a page.
func (r *Rooms) Mint(room string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(r.ttl)
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": LocalSubject, "client_id": clientID, "type": "access", "iss": issuer,
		"rooms": []string{room}, "iat": now.Unix(), "exp": exp.Unix(),
	}).SignedString(r.key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("viewerrooms: minting a relay token: %w", err)
	}
	return tok, exp, nil
}

// Bridges are the relay subjects the bridge joins as. A page that answers only these
// (trustBridges) cannot be called by another page holding a token for its room, since every token
// Mint signs is for LocalSubject.
func (r *Rooms) Bridges() []string { return []string{bridgeSubject} }

// Close leaves every room the bridge joined.
func (r *Rooms) Close() { r.bridge.Close() }
