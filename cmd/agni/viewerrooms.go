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

// relayPath is where --viewer-rooms serves the relay the viewer's tabs join.
const relayPath = "/relay/"

// serveViewerRooms mounts what --viewer-rooms adds (agni issue 1154): the relay at relayPath,
// ViewerRoomService for a tab to learn its room and get a token, and the room bridge's tools on the
// MCP server. urls are the addresses the server answers at, whose hosts are the only origins the
// relay takes a tab from. The caller closes what it returns when the server stops.
func serveViewerRooms(mux *http.ServeMux, mcpSrv *mcpkit.Server, projects *service.ProjectResolver, urls []string, opts ...connect.HandlerOption) (*viewerrooms.Rooms, error) {
	// The relay and the WebSocket library under it log every connection to the default loggers, which
	// nothing in agni writes to. Warnings and errors still print.
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

// originHosts are the hosts of the addresses the server answers at, with localhost, which also
// admits 127.0.0.1 and ::1.
func originHosts(urls []string) []string {
	hosts := []string{"localhost"}
	for _, u := range urls {
		if p, err := url.Parse(u); err == nil && p.Hostname() != "" {
			hosts = append(hosts, p.Hostname())
		}
	}
	return hosts
}
