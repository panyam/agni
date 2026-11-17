package server

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/panyam/agni/service"
)

// The view rpcs answer under DesignViewService, and DesignService no longer serves them, so the name
// is free for the stored designs of agni issue 1694 (agni issue 1705).
func TestTheViewRpcsServeAsDesignViewService(t *testing.T) {
	mux := http.NewServeMux()
	paths := API{DesignView: &service.DesignViewService{}}.Register(mux)
	if !slices.Contains(paths, "/agni.v1.webapi.DesignViewService/") {
		t.Fatalf("registered paths = %v, want /agni.v1.webapi.DesignViewService/", paths)
	}
	for _, p := range paths {
		if strings.HasPrefix(p, "/agni.v1.webapi.DesignService/") {
			t.Errorf("registered %s; DesignService is kept for stored designs", p)
		}
	}
	for path, want := range map[string]bool{
		"/agni.v1.webapi.DesignViewService/GetSummary": true,
		"/agni.v1.webapi.DesignService/GetDesign":      false,
	} {
		_, pattern := mux.Handler(httptest.NewRequest(http.MethodPost, path, nil))
		if routed := pattern != ""; routed != want {
			t.Errorf("POST %s routes to %q, want routed: %v", path, pattern, want)
		}
	}
}
