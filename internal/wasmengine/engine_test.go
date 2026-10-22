package wasmengine

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/goapplib/wasmhost"
)

func readTree(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		files[filepath.ToSlash(rel)] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// gatewayAlone is the tutorial gateway's folder on its own, as a design in no project: its descriptor
// is read without the `extends` naming the project it is copied out of, which the folder alone does not
// hold.
func gatewayAlone(t *testing.T) map[string][]byte {
	t.Helper()
	files := readTree(t, filepath.Join("..", "..", "examples", "tutorial-project", "designs", "gateway"))
	var kept []string
	for _, line := range strings.Split(string(files["design.yaml"]), "\n") {
		if !strings.HasPrefix(line, "extends:") {
			kept = append(kept, line)
		}
	}
	if len(kept) == strings.Count(string(files["design.yaml"]), "\n")+1 {
		t.Fatal("the gateway's design.yaml names no project, so this fixture no longer needs to drop one")
	}
	files["design.yaml"] = []byte(strings.Join(kept, "\n"))
	return files
}

func checkReport(t *testing.T, h *wasmhost.Host) string {
	t.Helper()
	// The host rebuilds its engine when files are added, so the design is ingested on each ask.
	ing := hostDo(t, h, "/agni.v1.webapi.SnapshotService/Ingest", `{"snapshot":{"uri":"mount://tut/designs/gateway"}}`)
	var snap struct{ Name string }
	if err := json.Unmarshal([]byte(ing), &snap); err != nil || snap.Name == "" {
		t.Fatalf("Ingest answered %s (%v)", ing, err)
	}
	return hostDo(t, h, "/agni.v1.webapi.CheckService/GetCheckReport", fmt.Sprintf(`{"snapshot":%q}`, snap.Name))
}

// hostDo posts a JSON body to a Connect method on h and returns the answer's body.
func hostDo(t *testing.T, h *wasmhost.Host, url, body string) string {
	t.Helper()
	res, err := h.Do(context.Background(), wasmhost.Request{
		Method: "POST",
		URL:    url,
		Header: http.Header{"Content-Type": {"application/json"}},
		Body:   []byte(body),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != http.StatusOK {
		t.Fatalf("%s: %d %s", url, res.Status, res.Body)
	}
	return string(res.Body)
}

// TestBuildServesThroughWasmhost runs the composition the browser runs, through goapplib's host
// natively: files go in by mount and add, the host rebuilds, and the check report carries the
// project's own profile rules. Adding a design's files to a mount must keep what the mount held,
// which is what the page relies on when it brings designs in one at a time (agni issue 853). Until
// the project's files arrive, the design names a project.yaml the mount does not hold, which is
// refused rather than read without the project (agni issue 1136).
func TestBuildServesThroughWasmhost(t *testing.T) {
	h := wasmhost.New(Namespace)
	if err := h.Rebuild(Build); err != nil {
		t.Fatal(err)
	}
	all := readTree(t, filepath.Join("..", "..", "examples", "tutorial-project"))
	design, project := map[string][]byte{}, map[string][]byte{}
	for p, b := range all {
		if strings.HasPrefix(p, "designs/") {
			design[p] = b
		} else {
			project[p] = b
		}
	}
	if err := h.Mount("tut", design); err != nil {
		t.Fatal(err)
	}
	res, err := h.Do(context.Background(), wasmhost.Request{
		Method: "POST", URL: "/agni.v1.webapi.SnapshotService/Ingest",
		Header: http.Header{"Content-Type": {"application/json"}},
		Body:   []byte(`{"snapshot":{"uri":"mount://tut/designs/gateway"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status == http.StatusOK || !strings.Contains(string(res.Body), `extends \"../../project.yaml\" names nothing`) {
		t.Fatalf("before the project's files arrive, the design must be refused naming the missing project, got %d %s", res.Status, res.Body)
	}
	if err := h.Add("tut", project); err != nil {
		t.Fatal(err)
	}
	if body := checkReport(t, h); !strings.Contains(body, "gateway-profiles/") || !strings.Contains(body, `"sourceFile":"designs/gateway/gateway.edn"`) {
		t.Errorf("after adding the project's files, the report lacks its profile rules or its locators:\n%.400s", body)
	}
}

// TestADesignInNoProjectDrawsWithItsOwnSymbols mounts the tutorial gateway's folder on its own, as a
// design in no project, its descriptor naming no parent. Its symbol library
// beside the descriptor must draw every part, and its declared intent must run (agni issue 887). The
// same folder inside its project drew 0 undrawn placements and on its own drew 19.
func TestADesignInNoProjectDrawsWithItsOwnSymbols(t *testing.T) {
	eng, err := New(fshost.Mount{Name: "m", FS: fshost.MemFS(gatewayAlone(t))})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(eng.Handler)
	defer srv.Close()
	ctx := context.Background()
	snap := snapAt(t, srv)
	d, err := webapiconnect.NewDesignServiceClient(srv.Client(), srv.URL).GetDesign(ctx, connect.NewRequest(&webapi.GetDesignRequest{Snapshot: snap}))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(d.Msg.GetUndrawn()); n != 0 {
		t.Errorf("%d placements undrawn, so the design's own symbols/ did not reach the read", n)
	}
	c, err := webapiconnect.NewCheckServiceClient(srv.Client(), srv.URL).GetCheckReport(ctx, connect.NewRequest(&webapi.GetCheckReportRequest{Snapshot: snap}))
	if err != nil {
		t.Fatal(err)
	}
	intent := 0
	for _, s := range c.Msg.GetReport().GetSections() {
		for _, g := range s.GetRules() {
			if strings.HasPrefix(g.GetRule(), "intent/") {
				intent++
			}
		}
	}
	if intent == 0 {
		t.Error("no finding from the design's declared intent, which deliberately misdeclares a rail and must fail")
	}
}
