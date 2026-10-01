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

	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/datasheet/corpus"
	"github.com/panyam/agni/internal/paramclient"
)

// The whole read path `agni serve --params-url` takes: agnids serves a published corpus, the engine's
// Connect client fetches through param.Remote, and a part published while both run reaches the next
// prefetch once the generation is re-checked (agni issue 749).
func TestParamsURLReadsThePublishedCorpus(t *testing.T) {
	dir := draftCorpus(t)
	if _, err := runAgnids(t, "index", dir); err != nil {
		t.Fatal(err)
	}
	store, err := openCorpus(dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(newWorkbenchMux(nil, "", nil, "", dir, store))
	defer srv.Close()

	remote := param.NewRemote(paramclient.New(srv.URL), 0) // re-check the generation on every prefetch
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
	srv := httptest.NewServer(newWorkbenchMux(nil, "", nil, "", "", nil))
	defer srv.Close()
	_, err := paramclient.New(srv.URL).Generation(context.Background())
	if err == nil || !strings.Contains(err.Error(), "failed_precondition") || !strings.Contains(err.Error(), "--corpus") {
		t.Fatalf("Generation against a server with no corpus = %v", err)
	}
}

// agnids with no workbench assets serves the API alone, so a deployment that only publishes a
// corpus needs no web build: the page is absent, and PartSpecService answers.
func TestServeWithNoWebDirIsTheAPIAlone(t *testing.T) {
	dir := draftCorpus(t)
	if _, err := runAgnids(t, "index", dir); err != nil {
		t.Fatal(err)
	}
	store, err := openCorpus(dir, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(newWorkbenchMux(nil, "", nil, "", dir, store))
	defer srv.Close()
	if resp, err := http.Get(srv.URL + "/datasheets/"); err != nil || resp.StatusCode != http.StatusNotFound {
		t.Errorf("the workbench page with no web dir = %v, %v; want 404", resp, err)
	}
	if g, err := paramclient.New(srv.URL).Generation(context.Background()); err != nil || g != 1 {
		t.Errorf("PartSpecService with no web dir = %d, %v", g, err)
	}
}

// A fresh corpus is indexed at start, so a new volume or directory needs no separate step, and a
// corpus that does not validate fails start by name rather than serving around a bad file.
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
