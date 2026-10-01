package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/internal/paramclient"
)

// The whole read path `agni serve --params-url` takes: agnids serves a published corpus, the engine's
// Connect client fetches through param.Remote, and a part promoted while both run reaches the next
// prefetch once the generation is re-checked (agni issue 749).
func TestParamsURLReadsThePublishedCorpus(t *testing.T) {
	dir, draft := promoteCorpus(t)
	if _, err := runAgnids(t, "index", dir); err != nil {
		t.Fatal(err)
	}
	store, err := openCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(newWorkbenchMux(nil, "", nil, "", store))
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

	if _, err := runAgnids(t, "promote", draft, "--to", dir); err != nil {
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
	srv := httptest.NewServer(newWorkbenchMux(nil, "", nil, "", nil))
	defer srv.Close()
	_, err := paramclient.New(srv.URL).Generation(context.Background())
	if err == nil || !strings.Contains(err.Error(), "failed_precondition") || !strings.Contains(err.Error(), "--corpus") {
		t.Fatalf("Generation against a server with no corpus = %v", err)
	}
}
