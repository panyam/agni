package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/mcpserver"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/service"
	"github.com/panyam/mcpkit/client"
	"github.com/panyam/mcpkit/core"
)

// TestViewerRoomsServesTheViewerToolsBesideAgnis composes what `serve --mcp --viewer-rooms` adds over
// the tutorial project (agni issue 1154): the bridge's tools join agni's own on /mcp without
// displacing any, and a tab opened on a design's folder is told the room of its declared entry, which
// is the room an agent naming the design reaches.
func TestViewerRoomsServesTheViewerToolsBesideAgnis(t *testing.T) {
	freshWorkspace(t)
	root, err := filepath.Abs(parityMounts[0].dir)
	if err != nil {
		t.Fatal(err)
	}
	ms := []mounts.Mount{{Name: parityMounts[0].name, Root: root}}
	api := servedAPIFor(t, ms, ms)
	resolver := &service.ProjectResolver{Store: projects.NewFSStore(projectTrees(ms)...), Config: &osProjectConfig{mounts: ms}}
	mux := http.NewServeMux()
	api.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	m, err := mcpserver.New(api, mcpserver.Options{URLBase: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	vr, err := serveViewerRooms(mux, m, resolver, []string{srv.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vr.Close)
	mux.Handle("/mcp", mcpserver.HTTPHandler(m, "/mcp"))

	for _, wire := range []string{"session", "stateless"} {
		t.Run(wire, func(t *testing.T) {
			var opts []client.ClientOption
			if wire == "stateless" {
				opts = append(opts, client.WithClientMode(client.ClientModeStateless))
			}
			c := client.NewClient(srv.URL+"/mcp", core.Implementation{Name: "agni-test", Version: "0"}, opts...)
			if err := c.Connect(context.Background()); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = c.Close() })
			got := toolNames(t, c)
			want := slices.Concat(mcpTools, []string{"attach", "detach", "list_pages", "pair",
				"viewer_apply_location", "viewer_get_location", "viewer_locate_entity", "viewer_locate_verdict", "viewer_select_finding", "viewer_show_sheet"})
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("tools %v, want %v", got, want)
			}
		})
	}

	rooms := webapiconnect.NewViewerRoomServiceClient(http.DefaultClient, srv.URL)
	for _, design := range []string{mcpGateway, mcpGateway + "/gateway.edn"} {
		resp, err := rooms.JoinViewerRoom(context.Background(), connect.NewRequest(&webapi.JoinViewerRoomRequest{Design: design}))
		if err != nil {
			t.Fatal(err)
		}
		if got := resp.Msg.GetRoom(); got != "design:"+mcpGateway+"/gateway.edn" {
			t.Errorf("%s joins room %q", design, got)
		}
		if resp.Msg.GetToken() == "" || !strings.HasPrefix(resp.Msg.GetRelayUrl(), "/relay") {
			t.Errorf("%s: %v", design, resp.Msg)
		}
	}
}

func TestViewerRoomsNeedsMCP(t *testing.T) {
	cmd := rootCmd()
	cmd.SetArgs([]string{"serve", "--viewer-rooms", "--addr", "127.0.0.1:0"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--mcp") {
		t.Fatalf("serve --viewer-rooms without --mcp: %v", err)
	}
}
