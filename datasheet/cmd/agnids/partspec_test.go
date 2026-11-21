package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/datasheet/corpus"
	dsapi "github.com/panyam/agni/datasheet/gen/go/agni/v1/dsapi"
	"github.com/panyam/agni/internal/authgate"
	"github.com/panyam/agni/internal/paramclient"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/oneauth/core"
)

// This test covers the whole read path of `agni serve --params-url`. agnids serves a published
// corpus, and the engine's Connect client fetches through param.Remote. A part published while both
// run reaches the next prefetch once the generation is re-checked (agni issue 749).
func TestParamsURLReadsThePublishedCorpus(t *testing.T) {
	dir := draftCorpus(t)
	if _, err := runAgnids(t, "index", dir); err != nil {
		t.Fatal(err)
	}
	store, err := openCorpus(dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(newWorkbenchMux(nil, workbenchAccess{}, "", nil, "", dir, store))
	defer srv.Close()

	remote := param.NewRemote(paramclient.New(srv.URL, ""), 0) // re-check the generation on every prefetch
	ctx := context.Background()
	if err := remote.Prefetch(ctx, []string{"acme-buck-3v3", "ACME-LDO-1V8", "UNSEEDED"}); err != nil {
		t.Fatal(err)
	}
	if remote.Lookup("ACME-BUCK-3V3") == nil {
		t.Error("the published spec did not reach the engine")
	}
	if remote.Lookup("ACME-LDO-1V8") != nil {
		t.Error("a part that is only a draft was served")
	}

	if _, err := runAgnids(t, "publish", "ACME-LDO-1V8", "--corpus", dir); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond) // the index's modification time must move for the reader to notice
	if err := remote.Prefetch(ctx, []string{"ACME-LDO-1V8"}); err != nil {
		t.Fatal(err)
	}
	if remote.Lookup("ACME-LDO-1V8") == nil {
		t.Error("a part promoted while the server ran did not reach the next request")
	}
}

// A server started without --corpus answers PartSpecService with a precondition that says what to
// pass, not a 404 or an empty corpus that would read as every part unseeded.
func TestPartSpecServiceWithoutACorpusSaysSo(t *testing.T) {
	srv := httptest.NewServer(newWorkbenchMux(nil, workbenchAccess{}, "", nil, "", "", nil))
	defer srv.Close()
	_, err := paramclient.New(srv.URL, "").Generation(context.Background())
	if err == nil || !strings.Contains(err.Error(), "failed_precondition") || !strings.Contains(err.Error(), "--corpus") {
		t.Fatalf("Generation against a server with no corpus = %v", err)
	}
}

// agnids with no workbench assets serves the API alone, so a deployment that only publishes a
// corpus needs no web build. The page is absent, and PartSpecService answers.
func TestServeWithNoWebDirIsTheAPIAlone(t *testing.T) {
	dir := draftCorpus(t)
	if _, err := runAgnids(t, "index", dir); err != nil {
		t.Fatal(err)
	}
	store, err := openCorpus(dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(newWorkbenchMux(nil, workbenchAccess{}, "", nil, "", dir, store))
	defer srv.Close()
	if resp, err := http.Get(srv.URL + "/datasheets/"); err != nil || resp.StatusCode != http.StatusNotFound {
		t.Errorf("the workbench page with no web dir = %v, %v; want 404", resp, err)
	}
	if g, err := paramclient.New(srv.URL, "").Generation(context.Background()); err != nil || g != 1 {
		t.Errorf("PartSpecService with no web dir = %d, %v", g, err)
	}
}

// A fresh corpus is indexed at start, so a new volume or directory needs no separate step, and a
// corpus that does not validate fails start by name rather than serving around a bad file.
// The corpus store also holds each PDF's derived doc-IR and annotations (agni issue 1734). param's
// corpus walk reads every *.textproto as a PartSpec, so a derived file must never look like one, or
// one Extract would stop the corpus loading.
func TestADerivedDocIRLeavesTheCorpusLoadable(t *testing.T) {
	dir := draftCorpus(t)
	mount := t.TempDir()
	writePDF(t, filepath.Join(mount, "LM1117.pdf"), "pdf-lm1117")
	stub := filepath.Join(t.TempDir(), "stub.sh")
	if err := os.WriteFile(stub, []byte(stubProducer), 0o755); err != nil {
		t.Fatal(err)
	}
	d := derived{mounts: tableOf([]mounts.Mount{mountset.Folder("m", mount)}), corpus: dir}
	if _, err := (&osDocExtractor{derived: d, cmd: []string{stub}}).Extract(context.Background(), mustURI("m", "LM1117.pdf")); err != nil {
		t.Fatal(err)
	}
	if err := (&osAnnotationStore{derived: d}).Save(context.Background(), mustURI("m", "LM1117.pdf"), "alice", &dsapi.AnnotationSet{Author: "alice"}); err != nil {
		t.Fatal(err)
	}
	var notes strings.Builder
	if _, err := openCorpus(dir, &notes); err != nil {
		t.Fatalf("the corpus stopped loading once it held a derived doc-IR: %v", err)
	}
	if !strings.Contains(notes.String(), "1 specs") {
		t.Errorf("start note = %q, want the one published spec and nothing derived", notes.String())
	}
}

func TestServeIndexesTheCorpusAtStart(t *testing.T) {
	dir := draftCorpus(t)
	var notes strings.Builder
	if _, err := openCorpus(dir, &notes); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(notes.String(), "indexed the corpus at start: 1 specs, generation 1") {
		t.Errorf("start note = %q", notes.String())
	}
	if _, err := os.Stat(filepath.Join(dir, corpus.IndexFile)); err != nil {
		t.Fatalf("no index written at start: %v", err)
	}
	notes.Reset()
	if _, err := openCorpus(dir, &notes); err != nil || notes.Len() != 0 {
		t.Errorf("an unchanged corpus was re-indexed: %q, %v", notes.String(), err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.textproto"), []byte("mpn: "), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := openCorpus(dir, io.Discard); err == nil || !strings.Contains(err.Error(), "broken.textproto") {
		t.Errorf("a corpus that does not validate started: %v", err)
	}
}

// agnids --auth tokens refuses a PartSpec call with no token or a bad one as unauthenticated, and
// answers one carrying a token `agni token create` made in the shared store, the way agni serve's
// --params-url sends it (agni issue 1765).
func TestAnAgnidsWithAuthAsksForAToken(t *testing.T) {
	dir := draftCorpus(t)
	if _, err := runAgnids(t, "index", dir); err != nil {
		t.Fatal(err)
	}
	corpus, err := openCorpus(dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	storeDir := t.TempDir()
	made, err := authgate.Store(storeDir).CreateAPIKey(context.Background(), &core.CreateAPIKeyRequest{Subject: "agni-serve", Name: "agni-serve"})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(guard(newWorkbenchMux(nil, workbenchAccess{}, "", nil, "", dir, corpus), authgate.Tokens, storeDir, false))
	defer srv.Close()
	for _, bad := range []string{"", "oa_0_wrong"} {
		if _, err := paramclient.New(srv.URL, bad).Generation(context.Background()); connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("PartSpecService with token %q = %v, want unauthenticated", bad, err)
		}
	}
	if g, err := paramclient.New(srv.URL, made.FullKey).Generation(context.Background()); err != nil || g != 1 {
		t.Errorf("PartSpecService with a good token = %d, %v", g, err)
	}
	if resp, err := http.Get(srv.URL + "/healthz"); err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("/healthz with no token = %v, %v; want 200, so a load balancer can check it", resp, err)
	}
}
