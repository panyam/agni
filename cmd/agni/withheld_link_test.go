package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/mounts"
)

// The reason a run withheld its links has to reach the saved page, not just stderr (issue 626).
// core/report pins the templates; these pin the wiring, so a withholding path that stops setting
// LinksWithheld fails here rather than shipping a page that is silent about its plain-text subjects.
//
// Asserted fragments carry no quotes, because html/template escapes them and the reasons quote the
// mount name.

const (
	checkWithheldBanner  = "Subjects on this page are not linked to the viewer."
	reviewWithheldBanner = "Findings on this page are not linked to the viewer."
	mintedReason         = "was minted for this run rather than declared"
	otherRootReason      = "so every link would name a different design"
)

// mountServer answers ListMounts with one mount. Given another root it is the second withholding
// path, a server that disagrees about what the name means; given the declared root it is the control
// that proves the same run WOULD link, so an absent link below is the withholding and not a fixture
// that never had anything to link.
type mountServer struct {
	webapiconnect.UnimplementedWorkspaceServiceHandler
	mount *webapi.Mount
}

func (s mountServer) ListMounts(context.Context, *connect.Request[webapi.ListMountsRequest]) (*connect.Response[webapi.ListMountsResponse], error) {
	return connect.NewResponse(&webapi.ListMountsResponse{Mounts: []*webapi.Mount{s.mount}}), nil
}

func newMountServer(t *testing.T, name, root string) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(webapiconnect.NewWorkspaceServiceHandler(mountServer{mount: &webapi.Mount{Name: name, Root: root}}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func newOtherRootServer(t *testing.T, name string) string {
	return newMountServer(t, name, "/somewhere/else")
}

// newAgreeingServer serves the declared mount from the root this run declared it at.
func newAgreeingServer(t *testing.T, name string) string {
	t.Helper()
	ws, err := workspace()
	if err != nil {
		t.Fatal(err)
	}
	m, ok := mounts.Find(ws.Mounts(), name)
	if !ok {
		t.Fatalf("mount %q is not declared", name)
	}
	return newMountServer(t, name, m.Root)
}

func assertLinked(t *testing.T, page string) {
	t.Helper()
	if !strings.Contains(page, "<a href") {
		t.Errorf("control: a server that agrees about the mount should get links, so the fixture has nothing to link")
	}
	if strings.Contains(page, "not linked to the viewer") {
		t.Errorf("control: a page that linked its subjects must not explain a withholding")
	}
}

func assertWithheld(t *testing.T, page, banner, reason string) {
	t.Helper()
	if strings.Contains(page, "<a href") {
		t.Errorf("links were emitted, so there was nothing to explain")
	}
	if !strings.Contains(page, banner) {
		t.Errorf("the page does not say its subjects are unlinked")
	}
	if !strings.Contains(page, reason) {
		t.Errorf("the page does not carry the reason %q", reason)
	}
}

func TestCheckHTMLCarriesTheWithheldLinkReason(t *testing.T) {
	t.Run("minted mount", func(t *testing.T) {
		page := runCheck(t, "--format", "html", "--server", "http://127.0.0.1:1",
			"testdata/conformance/showcase.fires.kicad_sch")
		assertWithheld(t, page, checkWithheldBanner, mintedReason)
	})
	t.Run("server serves the name from another root", func(t *testing.T) {
		withMount(t)
		page := runCheck(t, "--format", "html", "--server", newOtherRootServer(t, "demo"),
			"mount://demo/showcase.fires.kicad_sch")
		assertWithheld(t, page, checkWithheldBanner, otherRootReason)
	})
	t.Run("control: server agrees", func(t *testing.T) {
		withMount(t)
		assertLinked(t, runCheck(t, "--format", "html", "--server", newAgreeingServer(t, "demo"),
			"mount://demo/showcase.fires.kicad_sch"))
	})
}

func TestReviewHTMLCarriesTheWithheldLinkReason(t *testing.T) {
	t.Run("minted mount", func(t *testing.T) {
		page := runReview(t, "--checklist", "testdata/review/mini.yaml", "--format", "html",
			"--server", "http://127.0.0.1:1", "testdata/review/can-broken.edn")
		assertWithheld(t, page, reviewWithheldBanner, mintedReason)
	})
	t.Run("server serves the name from another root", func(t *testing.T) {
		withDeclaredMount(t, "testdata/review")
		page := runReview(t, "--checklist", "testdata/review/mini.yaml", "--format", "html",
			"--server", newOtherRootServer(t, "demo"), "mount://demo/can-broken.edn")
		assertWithheld(t, page, reviewWithheldBanner, otherRootReason)
	})
	t.Run("control: server agrees", func(t *testing.T) {
		withDeclaredMount(t, "testdata/review")
		assertLinked(t, runReview(t, "--checklist", "testdata/review/mini.yaml", "--format", "html",
			"--server", newAgreeingServer(t, "demo"), "mount://demo/can-broken.edn"))
	})
}
