package mountset_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/goapplib/storage/connections"
	"github.com/panyam/goapplib/storage/mounts/github/githubtest"

	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
)

// The table answers whether a caller can read a mount's private repo by asking GitHub with the
// caller's own connection, never the server's (agni issue 1728).
func TestATableAsksGitHubWithTheCallersOwnConnection(t *testing.T) {
	gh := githubtest.New(t)
	board := gh.AddRepo("acme", "board", true, map[string]string{"hw/bb.kicad_sch": "(kicad_sch)"})
	gh.AddRepo("acme", "open", false, map[string]string{"hw/bb.kicad_sch": "(kicad_sch)"})
	gh.AddToken("ghp_bob")
	// Bob can't see the private repo, so GitHub answers his token 404 for it.
	board.Grant("ghp_server", "ghp_alice")
	api := gh.URL
	t.Setenv("AGNI_TEST_SERVER_TOKEN", "ghp_server")
	t.Setenv("AGNI_TEST_CONNECTIONS_KEY", strings.Repeat("ab", 32))
	dir := t.TempDir()
	table, err := mountset.Open(mountset.Config{
		Uploads: filepath.Join(dir, "uploads"), Records: filepath.Join(dir, "store", "mounts"), Connections: filepath.Join(dir, "store", "connections"),
		UserTokens: filepath.Join(dir, "store", mountset.UserTokensDir), ConnectionsKey: "env:AGNI_TEST_CONNECTIONS_KEY",
		GitHubAPI: api, GitHubRaw: gh.RawURL(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	conns := table.Connections()
	if err := conns.PutConnection(ctx, connections.Connection{ID: "server", Kind: connections.KindGitHubToken, Scope: connections.ScopeServer, TokenRef: "env:AGNI_TEST_SERVER_TOKEN", BaseURL: api, Status: connections.StatusOK}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct{ who, token string }{{"alice", "ghp_alice"}, {"bob", "ghp_bob"}} {
		id := p.who + "-gh"
		if err := conns.PutConnection(ctx, connections.Connection{ID: id, Kind: connections.KindGitHubUser, Scope: connections.ScopePerson, Owner: p.who, BaseURL: api, Status: connections.StatusOK}); err != nil {
			t.Fatal(err)
		}
		if err := table.SaveUserToken(ctx, id, p.who, p.token); err != nil {
			t.Fatalf("seal %s's token: %v", p.who, err)
		}
		// A person pastes their token, so a later disconnect mustn't ask GitHub to revoke it.
		if c, err := conns.GetConnection(ctx, id); err != nil || !c.Pasted {
			t.Errorf("%s's connection after sealing a pasted token = %+v, %v; want it marked pasted", p.who, c, err)
		}
	}
	server := service.WithCaller(ctx, service.Caller{Subject: "ops"})
	if _, err := table.CreateMount(server, "board", "github://acme/board", "main", "server"); err != nil {
		t.Fatalf("mount the private repo through the server's connection: %v", err)
	}
	if _, err := table.CreateMount(server, "open", "github://acme/open", "main", ""); err != nil {
		t.Fatalf("mount the public repo: %v", err)
	}

	if got := table.Guarding("board"); got != "github://acme/board" {
		t.Errorf("Guarding(board) = %q; want the repo, since a connection reads it", got)
	}
	if got := table.Guarding("open"); got != "" {
		t.Errorf("Guarding(open) = %q; want none, since it reads with no connection", got)
	}
	for _, c := range []struct {
		who  string
		want error
	}{
		{"alice", nil},
		{"bob", service.ErrPermissionDenied},
		{"carol", service.ErrPermissionDenied},
		{"ops", service.ErrPermissionDenied},
		{"", service.ErrPermissionDenied},
	} {
		err := table.CanRead(service.WithCaller(ctx, service.Caller{Subject: c.who}), "github://acme/board")
		if c.want == nil && err != nil || c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%q can read github://acme/board: %v; want %v", c.who, err, c.want)
		}
	}
	for _, r := range gh.Requests() {
		if r.Path == "/repos/acme/board" && r.Token == "ghp_server" {
			t.Error("the server's own token answered whether a person can read the repo")
		}
	}
	sealed := func() int {
		t.Helper()
		files, err := filepath.Glob(filepath.Join(dir, "store", mountset.UserTokensDir, "user_tokens", "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		return len(files)
	}
	if n := sealed(); n != 2 {
		t.Fatalf("the server holds %d sealed tokens; want alice's and bob's", n)
	}
	if err := table.Disconnect(ctx, "alice-gh"); err != nil {
		t.Fatal(err)
	}
	if _, err := conns.GetConnection(ctx, "alice-gh"); !errors.Is(err, connections.ErrNotFound) {
		t.Errorf("alice's connection after Disconnect: %v; want it gone", err)
	}
	if n := sealed(); n != 1 {
		t.Errorf("after alice disconnects, the server holds %d sealed tokens; want bob's alone", n)
	}

	// agnids reads the same records and can't check anyone's connection, so under --auth it hides
	// the private mount and serves the public one.
	reader, err := mountset.Open(mountset.Config{Uploads: filepath.Join(dir, "uploads"), Records: filepath.Join(dir, "store", "mounts"), ReadOnly: true, HidePrivate: true})
	if err != nil {
		t.Fatal(err)
	}
	var listed []string
	for _, m := range reader.List() {
		listed = append(listed, m.Name)
	}
	if strings.Join(listed, ",") != "open" {
		t.Errorf("a reader hiding private mounts lists %q; want open alone", listed)
	}
	if _, err := reader.Resolve(uri(t, "mount://board/hw/bb.kicad_sch")); !errors.Is(err, service.ErrPermissionDenied) {
		t.Errorf("a reader hiding private mounts resolves board: %v; want permission denied", err)
	}
	if _, err := reader.Resolve(uri(t, "mount://open/hw/bb.kicad_sch")); err != nil {
		t.Errorf("a reader hiding private mounts resolves open: %v", err)
	}
}
