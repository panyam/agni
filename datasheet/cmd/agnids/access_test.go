package main

import (
	"context"
	"crypto/sha256"
	"hash"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panyam/goapplib/storage/connections"
	"github.com/panyam/goapplib/storage/mounts/github/githubtest"
	"github.com/panyam/oneauth/core"

	"github.com/panyam/agni/internal/authgate"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
)

// storeSum hashes every file, with its path, under the store folders agnids reads for access:
// connections, people's sealed tokens and mount records. A test can then tell whether anything wrote
// there.
func storeSum(t *testing.T, store string) [32]byte {
	t.Helper()
	h := sha256.New()
	for _, dir := range []string{mountset.ConnectionsDir, mountset.UserTokensDir, mountset.RecordsDir} {
		storeSumDir(t, h, filepath.Join(store, dir))
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func storeSumDir(t *testing.T, h hash.Hash, dir string) {
	t.Helper()
	if err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p) // #nosec G304 -- a test's own temp folder
		if err != nil {
			return err
		}
		h.Write([]byte(p))
		h.Write(b)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// agnids with --auth and --connections-key serves a private mount to a caller whose own connection
// reads its repo, and refuses one whose connection can't. It reads agni serve's records and sealed
// tokens and writes neither (agni issue 1778).
func TestAgnidsWithAuthServesAPrivateMountToItsReaders(t *testing.T) {
	const serverGitHub, aliceGitHub, bobGitHub = "ghp_server", "ghp_alice", "ghp_bob"
	gh := githubtest.New(t)
	repo := gh.AddRepo("acme", "sheets", true, map[string]string{"ti/TPS62.pdf": "%PDF-1.4"})
	repo.Grant(serverGitHub, aliceGitHub)
	gh.AddToken(bobGitHub)
	t.Setenv("AGNI_TEST_SERVER_GITHUB", serverGitHub)
	t.Setenv("AGNI_TEST_CONNECTIONS_KEY", strings.Repeat("c4", 32))
	store, uploads := t.TempDir(), t.TempDir()
	ctx := context.Background()

	// agni serve's side: a server connection, a mount through it, and alice's and bob's own
	// connections with their tokens sealed.
	serve, err := mountset.Open(mountset.Config{
		Uploads: uploads, Records: filepath.Join(store, mountset.RecordsDir), Connections: filepath.Join(store, mountset.ConnectionsDir),
		UserTokens: filepath.Join(store, mountset.UserTokensDir), ConnectionsKey: "env:AGNI_TEST_CONNECTIONS_KEY", GitHubAPI: gh.URL, GitHubRaw: gh.RawURL(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := serve.Connections().PutConnection(ctx, connections.Connection{ID: "acme", Kind: connections.KindGitHubToken, Scope: connections.ScopeServer,
		TokenRef: "env:AGNI_TEST_SERVER_GITHUB", BaseURL: gh.URL, Status: connections.StatusOK}); err != nil {
		t.Fatal(err)
	}
	if _, err := serve.CreateMount(service.WithCaller(ctx, service.Caller{Subject: "ops"}), "sheets", "github://acme/sheets", "", "acme"); err != nil {
		t.Fatal(err)
	}
	for who, tok := range map[string]string{"alice": aliceGitHub, "bob": bobGitHub} {
		if err := serve.Connections().PutConnection(ctx, connections.Connection{ID: "gh", Kind: connections.KindGitHubUser, Scope: connections.ScopePerson,
			Owner: who, BaseURL: gh.URL, Status: connections.StatusOK}); err != nil {
			t.Fatal(err)
		}
		if err := serve.SaveUserToken(ctx, who, "gh", tok); err != nil {
			t.Fatal(err)
		}
	}
	tokens := map[string]string{}
	for _, who := range []string{"alice", "bob"} {
		made, err := authgate.Store(store).CreateAPIKey(ctx, &core.CreateAPIKeyRequest{Subject: who, Name: who})
		if err != nil {
			t.Fatal(err)
		}
		tokens[who] = made.FullKey
	}

	table, err := serveTable(nil, "", "file://"+filepath.ToSlash(store), "file://"+filepath.ToSlash(uploads),
		tableAccess{auth: true, connectionsKey: "env:AGNI_TEST_CONNECTIONS_KEY", githubAPI: gh.URL})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 40; i++ {
		if _, ok := table.Find("sheets"); ok {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	srv := httptest.NewServer(guard(newWorkbenchMux(table, workbenchAccess{guard: service.NewAccessGuard(table, ""), hideRoots: true}, "", nil, "", "", nil), authgate.Tokens, store, false))
	defer srv.Close()
	before := storeSum(t, store)

	do := func(who, method, path, body string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tokens[who])
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var b strings.Builder
		_, _ = io.Copy(&b, resp.Body)
		return resp.StatusCode, b.String()
	}
	const rpc = "/agni.v1.webapi.WorkspaceService/"

	if _, got := do("alice", "POST", rpc+"ListMounts", `{}`); !strings.Contains(got, `"sheets"`) || !strings.Contains(got, "github://acme/sheets") || strings.Contains(got, uploads) {
		t.Errorf("alice's ListMounts = %s; want sheets with its repo, and no folder on the server", got)
	}
	if _, got := do("alice", "POST", rpc+"ListDir", `{"uri":"mount://sheets/ti"}`); !strings.Contains(got, "TPS62.pdf") {
		t.Errorf("alice's ListDir mount://sheets/ti = %s; want the PDF", got)
	}
	if code, got := do("alice", "GET", "/datasheets/raw/sheets/ti/TPS62.pdf", ""); code != http.StatusOK || got != "%PDF-1.4" {
		t.Errorf("alice's raw TPS62.pdf = %d %q; want 200 and the PDF", code, got)
	}

	const ds = "/agni.v1.dsapi.DatasheetService/GetDocument"
	if _, got := do("alice", "POST", ds, `{"uri":"mount://sheets/ti/TPS62.pdf"}`); strings.Contains(got, "permission_denied") {
		t.Errorf("alice's GetDocument = %s; want it answered", got)
	}
	if _, got := do("bob", "POST", ds, `{"uri":"mount://sheets/ti/TPS62.pdf"}`); !strings.Contains(got, "permission_denied") {
		t.Errorf("bob's GetDocument = %s; want permission_denied", got)
	}

	if _, got := do("bob", "POST", rpc+"ListMounts", `{}`); strings.Contains(got, "acme/sheets") {
		t.Errorf("bob's ListMounts = %s; want sheets without its repo", got)
	}
	if _, got := do("bob", "POST", rpc+"ListDir", `{"uri":"mount://sheets/ti"}`); !strings.Contains(got, "permission_denied") {
		t.Errorf("bob's ListDir mount://sheets/ti = %s; want permission_denied", got)
	}
	if code, _ := do("bob", "GET", "/datasheets/raw/sheets/ti/TPS62.pdf", ""); code != http.StatusForbidden {
		t.Errorf("bob's raw TPS62.pdf = %d; want 403", code)
	}

	// The token check stamps each API key's last use under tokens/, as it did before this change, so
	// only the folders this change reads are compared.
	if storeSum(t, store) != before {
		t.Error("agnids wrote to agni serve's connections, sealed tokens or mount records while checking its callers")
	}
}
