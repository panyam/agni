package server

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/panyam/agni/service"
)

// The view rpcs answer under DesignViewService (agni issue 1705). DesignService serves the stored
// designs of agni issue 1694 and none of the view rpcs.
func TestTheViewRpcsServeAsDesignViewService(t *testing.T) {
	mux := http.NewServeMux()
	paths := API{DesignView: &service.DesignViewService{}}.Register(mux)
	for _, want := range []string{"/agni.v1.webapi.DesignViewService/", "/agni.v1.webapi.DesignService/"} {
		if !slices.Contains(paths, want) {
			t.Fatalf("registered paths = %v, want %s", paths, want)
		}
	}
	for path, wantFound := range map[string]bool{
		"/agni.v1.webapi.DesignService/GetDesign":  true,
		"/agni.v1.webapi.DesignService/GetSheet":   false,
		"/agni.v1.webapi.DesignService/GetSummary": false,
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if found := rec.Code != http.StatusNotFound; found != wantFound {
			t.Errorf("POST %s = %d, want answered: %v", path, rec.Code, wantFound)
		}
	}
}
