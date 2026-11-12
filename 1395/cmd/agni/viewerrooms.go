package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/server"
	"github.com/panyam/agni/internal/viewerrooms"
	"github.com/panyam/agni/service"
	mcpkit "github.com/panyam/mcpkit/server"
)

// relayPath is where --viewer-rooms serves the relay. The viewer's tabs join the relay there.
const relayPath = "/relay/"

// serveViewerRooms mounts three things for --viewer-rooms (agni issue 1154). It mounts the relay at
// relayPath. It mounts ViewerRoomService, where a tab learns its room and gets a token. It registers
// the room bridge's tools on the MCP server. urls lists the addresses the server answers at, and the
// relay takes a tab only from an origin whose host is one of theirs. The caller closes the returned
// Rooms when the server stops.
func serveViewerRooms(mux *http.ServeMux, mcpSrv *mcpkit.Server, projects *service.ProjectResolver, urls []string, opts ...connect.HandlerOption) (*viewerrooms.Rooms, error) {
	// The relay and the WebSocket library under it log every connection to the default loggers.
	// Nothing in agni writes to those loggers, so this keeps warnings and errors only.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	var rooms *service.ViewerRoomService
	vr, err := viewerrooms.New(viewerrooms.Options{
		RoomOf:  func(ctx context.Context, design string) (string, error) { return rooms.Room(ctx, design) },
		Origins: originHosts(urls),
	})
	if err != nil {
		return nil, err
	}
	rooms = service.NewViewerRoomService(projects, vr, relayPath)
	mux.Handle(webapiconnect.NewViewerRoomServiceHandler(server.NewViewerRoom(rooms), opts...))
	mux.Handle(relayPath, http.StripPrefix(relayPath[:len(relayPath)-1], vr.Relay()))
	vr.Register(mcpSrv)
	return vr, nil
}

// originHosts lists the host of each address the server answers at, plus localhost. The relay treats
// localhost as admitting 127.0.0.1 and ::1 too.
func originHosts(urls []string) []string {
	hosts := []string{"localhost"}
	for _, u := range urls {
		if p, err := url.Parse(u); err == nil && p.Hostname() != "" {
			hosts = append(hosts, p.Hostname())
		}
	}
	return hosts
}
