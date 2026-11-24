package main

import (
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panyam/goapplib/storage/mounts/github/githubtest"
	"github.com/panyam/oneauth/oauth2/oidc/oidctest"

	"github.com/panyam/agni/internal/authgate"
	"github.com/panyam/agni/internal/grants"
)

// grantStore is a store holding two private repos' mounts and a design linked from each, and a
// signed-in person of every kind (agni issue 1889). The mounts read through the server's connection
// acme, and nobody but ops holds a connection or a grant yet.
type grantStore struct {
	gh             *githubtest.Server
	store, uploads string
	// open serves the store without --auth, as the setup did.
	open string
	// tokens maps each person to the token their calls send: carol signed in with a local account,
	// ada through OIDC, dave holds a token from agni token create, and so does github:erin, a GitHub
	// subject with no connection.
	tokens map[string]string
}

// people are grantStore's signed-in callers, in the order the tests ask for them.
var people = []string{"carol", "ada", "dave", "github:erin"}

// newGrantStore serves the store once without --auth to make its mounts and designs, since nobody
// holds a connection of their own to link them through. Each design records the repo it read, so
// --auth guards it later (agni issue 1778). It then serves the store with --auth and no grant, for
// carol and ada to sign in.
func newGrantStore(t *testing.T) *grantStore {
	t.Helper()
	old := authgate.CacheTTL
	authgate.CacheTTL = time.Millisecond
	t.Cleanup(func() { authgate.CacheTTL = old })
	t.Setenv("AGNI_CONFIG_DIR", t.TempDir())
	t.Setenv("AGNI_TEST_GITHUB_TOKEN", secretToken)
	t.Setenv("AGNI_TEST_CONNECTIONS_KEY", strings.Repeat("5e", 32))
	gh, board := privateGitHubRepo(t)
	other := gh.AddRepo("acme", "other", true, map[string]string{"hw/bb.kicad_sch": "(kicad_sch)"})
	board.Grant(secretToken)
	other.Grant(secretToken)
	dir := t.TempDir()
	g := &grantStore{gh: gh, store: filepath.Join(dir, "store"), uploads: filepath.Join(dir, "uploads"), tokens: map[string]string{}}
	storeURL := testStoreURL(t, g.store)
	for _, who := range []string{"dave", "github:erin"} {
		out, err := runCobra(t, tokenCmd(), "create", who, "--store", storeURL)
		if err != nil {
			t.Fatalf("agni token create %s: %v", who, err)
		}
		g.tokens[who], _, _ = strings.Cut(strings.TrimSpace(out), "\n")
	}
	if out, err := runUsers(t, "a password for carol\n", "add", "carol", "--store", storeURL, "--password-stdin"); err != nil {
		t.Fatalf("users add carol = %q, %v", out, err)
	}

	open := g.opts()
	open.auth, open.store = "", testStore(t, g.store)
	setup := serveViewer(t, open)
	g.open = setup
	t.Setenv(envToken, "")
	mustRun(t)(runCobra(t, connectionsCmd(), "add", "acme", "--github-token", "env:AGNI_TEST_GITHUB_TOKEN", "--remote", setup))
	for _, repo := range []string{"board", "other"} {
		mustRun(t)(runMounts(t, "add", repo, "github://acme/"+repo, "--connection", "acme", "--remote", setup))
	}
	for _, d := range []struct{ name, mount string }{{"bb", "board"}, {"oo", "other"}} {
		mustRun(t)(runDesigns(t, setup, "create", d.name))
		mustRun(t)(runDesigns(t, setup, "link", d.name, "schematic", d.mount+"/hw/bb.kicad_sch@main"))
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + ln.Addr().String()
	iss := oidctest.New(t, oidctest.WithClient(oidcClientID, oidcSecret, base+authgate.OIDCSignInPath+"/callback/"))
	iss.AddUser("u-ada", map[string]any{"email": "ada@example.com", "email_verified": true})
	o := g.opts()
	o.listener, o.localAccounts, o.store = ln, true, testStore(t, g.store)
	o.oidcSignIn = &oidcSignIn{issuer: iss.URL, clientID: oidcClientID, clientSecret: oidcSecret, allow: []string{"domain:example.com"}}
	server := serveViewer(t, o)

	status, carol := localSignIn(t, server, "carol", "a password for carol")
	if status != http.StatusSeeOther || carol == "" {
		t.Fatalf("carol's sign-in = %d with session %q; want 303 and a session", status, carol)
	}
	_, _, body, ada := oidcBrowserSignIn(t, iss, server, "u-ada", "/")
	if ada == "" {
		t.Fatalf("ada's OIDC sign-in set no session:\n%s", body)
	}
	g.tokens["carol"], g.tokens["ada"] = carol, ada
	return g
}

// opts serves the store, once the caller opens it, under --auth tokens with vs as --grant values.
func (g *grantStore) opts(vs ...string) viewerOpts {
	parsed, err := grants.ParseAll(vs)
	if err != nil {
		panic(err)
	}
	return viewerOpts{uploadsDir: g.uploads, auth: authgate.Tokens, connectionsKey: "env:AGNI_TEST_CONNECTIONS_KEY",
		githubAPI: g.gh.URL, githubRaw: g.gh.RawURL(), grants: parsed}
}

// serve serves the store again with grants, and returns its URL.
func (g *grantStore) serve(t *testing.T, grants ...string) string {
	t.Helper()
	o := g.opts(grants...)
	o.store = testStore(t, g.store)
	return serveViewer(t, o)
}

func (g *grantStore) as(t *testing.T, who string) { t.Setenv(envToken, g.tokens[who]) }

// reads reports whether who can open design on server, and fails the test when a refusal is
// anything but permission_denied naming source.
func (g *grantStore) reads(t *testing.T, server, who, design, source string) bool {
	t.Helper()
	g.as(t, who)
	out, err := runDesigns(t, server, "get", design)
	if err == nil {
		return true
	}
	if !strings.Contains(err.Error(), "permission_denied") || !strings.Contains(err.Error(), source) {
		t.Errorf("%s's designs get %s = %q, %v; want it read or refused with permission_denied naming %s", who, design, out, err, source)
	}
	return false
}

func mustRun(t *testing.T) func(string, error) string {
	return func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
}

// A grant of * on a private source opens its designs and its mount to a person signed in with a
// local account, one signed in through OIDC, and one holding a token, none of whom has a GitHub
// connection. The same store served without the grant refuses all three (agni issue 1889).
func TestAGrantOfStarOpensASourceToEveryoneSignedIn(t *testing.T) {
	g := newGrantStore(t)
	granted := g.serve(t, "github://acme/board=*")
	plain := g.serve(t)
	asked := 0
	for _, who := range []string{"carol", "ada", "dave"} {
		asked++
		if !g.reads(t, granted, who, "bb", "github://acme/board") {
			t.Errorf("%s can't open bb under a grant of * on github://acme/board", who)
		}
		if out := mustRun(t)(runDesigns(t, granted, "list")); !strings.Contains(out, "designs/bb") {
			t.Errorf("%s's designs list under the grant = %q; want bb", who, out)
		}
		if out := mustRun(t)(runMounts(t, "ls", "--remote", granted)); !strings.Contains(out, "board\tgithub\tgithub://acme/board@") {
			t.Errorf("%s's mounts ls under the grant = %q; want board's repo", who, out)
		}
		if g.reads(t, plain, who, "bb", "github://acme/board") {
			t.Errorf("%s opens bb on the server with no grant", who)
		}
		if out := mustRun(t)(runDesigns(t, plain, "list")); strings.Contains(out, "designs/bb") {
			t.Errorf("%s's designs list with no grant = %q; want bb left out", who, out)
		}
		if out := mustRun(t)(runMounts(t, "ls", "--remote", plain)); strings.Contains(out, "acme/board") || !strings.Contains(out, "board\tgithub\ta private source you can't read") {
			t.Errorf("%s's mounts ls with no grant = %q; want board listed without its repo", who, out)
		}
	}
	if asked != 3 {
		t.Fatalf("asked %d people; want 3", asked)
	}
}

// A grant of local:* admits a person signed in with a local account, and nobody else: not a GitHub
// subject with no connection, an OIDC person, or a token's subject.
func TestALocalGrantAdmitsLocalAccountsAlone(t *testing.T) {
	g := newGrantStore(t)
	server := g.serve(t, "github://acme/board=local:*")
	want := map[string]bool{"carol": true, "ada": false, "dave": false, "github:erin": false}
	for _, who := range people {
		if got := g.reads(t, server, who, "bb", "github://acme/board"); got != want[who] {
			t.Errorf("%s opens bb under a grant of local:* = %v; want %v", who, got, want[who])
		}
	}
	if len(want) != len(people) {
		t.Fatalf("checked %d people; want %d", len(people), len(want))
	}
}

// A grant on one source opens that source alone. A design linked from another private source stays
// closed, and its mount stays hidden.
func TestAGrantOnOneSourceLeavesAnotherClosed(t *testing.T) {
	g := newGrantStore(t)
	server := g.serve(t, "github://acme/board=*")
	for _, who := range people {
		if !g.reads(t, server, who, "bb", "github://acme/board") {
			t.Errorf("%s can't open bb under a grant of *", who)
		}
		if g.reads(t, server, who, "oo", "github://acme/other") {
			t.Errorf("%s opens oo, linked from github://acme/other, under a grant on github://acme/board alone", who)
		}
		if out := mustRun(t)(runMounts(t, "ls", "--remote", server)); strings.Contains(out, "acme/other") || !strings.Contains(out, "other\tgithub\ta private source you can't read") {
			t.Errorf("%s's mounts ls = %q; want other listed without its repo", who, out)
		}
	}
}

// A design unlinked from a granted source, with the mount gone, keeps its files guarded by the
// source they recorded, so the grant still opens it and a server without the grant still refuses.
func TestAnUnlinkedDesignKeepsItsGrant(t *testing.T) {
	g := newGrantStore(t)
	granted := g.serve(t, "github://acme/board=*")
	plain := g.serve(t)
	t.Setenv(envToken, "")
	mustRun(t)(runDesigns(t, g.open, "unlink", "bb", "schematic"))
	mustRun(t)(runMounts(t, "rm", "board", "--remote", g.open))
	if a := artifactJSON(t, g.open, "designs/bb/artifacts/schematic"); a.Source.AccessSource != "github://acme/board" {
		t.Fatalf("the unlinked schematic records access source %q; want github://acme/board", a.Source.AccessSource)
	}
	for _, who := range []string{"carol", "ada", "dave"} {
		if !g.reads(t, granted, who, "bb", "github://acme/board") {
			t.Errorf("%s can't open the unlinked bb under a grant of *", who)
		}
		if g.reads(t, plain, who, "bb", "github://acme/board") {
			t.Errorf("%s opens the unlinked bb with no grant", who)
		}
	}
}

// --grant and a serve.yaml's access: list each refuse a malformed grant by name before the server
// listens, and access: sets --grant unless --grant was typed.
func TestServeRefusesAMalformedGrant(t *testing.T) {
	for _, c := range []struct{ yml, want string }{
		{"access:\n  - {source: github://acme/board, allow: [\"lo*cal\"]}\n", `access[0] at line 2: connections: invalid source grant: viewer pattern "lo*cal"`},
		{"access:\n  - {source: github://acme/board, allow: [\"*\"]}\n  - {source: acme/board, allow: [local:*]}\n", `access[1] at line 3: connections: invalid source grant: source "acme/board"`},
		{"access:\n  - {source: github://acme/board, allow: []}\n", `access[0] at line 2: connections: invalid source grant: source "github://acme/board" has no viewers`},
		{"access:\n  - {source: github://acme/board, alow: [\"*\"]}\n", "access[0] at line 2: want {source, allow}"},
		{"access:\n  - {source: github://acme/board, allow: [\"a,b\"]}\n", `pattern "a,b" has a comma`},
		{"access: github://acme/board\n", "want a list of {source, allow}"},
		{"grant: [\"github://acme/board=*\"]\naccess:\n  - {source: github://acme/board, allow: [\"*\"]}\n", "grant and access both set --grant"},
	} {
		if _, _, err := configured(t, c.yml); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: err = %v; want one saying %q", c.yml, err, c.want)
		}
	}
	cmd, _, err := configured(t, "access:\n  - {source: github://acme/board, allow: [local:*, oidc:*]}\n  - {source: github://acme/other, allow: [\"*\"]}\n")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := cmd.Flags().GetStringArray("grant"); strings.Join(got, " ") != "github://acme/board=local:*,oidc:* github://acme/other=*" {
		t.Errorf("access: set --grant to %q; want both entries", got)
	}
	if cmd, _, err = configured(t, "access:\n  - {source: github://acme/board, allow: [\"*\"]}\n", "--grant", "github://acme/board=local:alice"); err != nil || flagString(t, cmd, "grant") != "[github://acme/board=local:alice]" {
		t.Errorf("a typed --grant beside access: = %q, %v; want the typed grant alone", flagString(t, cmd, "grant"), err)
	}

	t.Setenv("AGNI_CONFIG_DIR", t.TempDir())
	grantsFile, badFile := filepath.Join(t.TempDir(), "access.yaml"), filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(grantsFile, []byte("access:\n  - {source: github://acme/board, allow: [\"*\"]}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badFile, []byte("grant: [\"github://acme/board=*\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--grant", "github://acme/board"}, `--grant "github://acme/board": want source=pattern,pattern`},
		{[]string{"--grant", "github://acme/board=local:*,"}, `--grant "github://acme/board=local:*,": connections: invalid source grant: viewer pattern ""`},
		{[]string{"--grant", "github://acme/board=*"}, "--grant opens a private source to people who sign in, so it needs --auth tokens"},
		{[]string{"--grants-file", grantsFile}, "--grant opens a private source to people who sign in, so it needs --auth tokens"},
		{[]string{"--grants-file", grantsFile, "--grant", "github://acme/board=*"}, "--grants-file and --grant, or a serve.yaml's access:, both set grants"},
		{[]string{"--grants-file", badFile}, `--grants-file ` + badFile + `: want only access:`},
	} {
		cmd := rootCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(append([]string{"serve", "--addr", "192.0.2.1:0"}, c.args...))
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("serve %q = %v; want one saying %q", c.args, err, c.want)
		}
	}
}
