package main

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/goapplib/storage/mounts/github/githubtest"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"

	"github.com/panyam/agni/internal/authgate"
	"github.com/panyam/agni/internal/credentials"
)

const (
	appClientID = "Iv1.agni-test"
	appSecret   = "app-secret"
)

// signInServer serves the viewer behind --auth, signing people in through a GitHub App on gh that
// lets the members of acme in. ops holds a token from agni token create.
func signInServer(t *testing.T, gh *githubtest.Server) (url, ops string) {
	t.Helper()
	old := authgate.CacheTTL
	authgate.CacheTTL = time.Millisecond
	t.Cleanup(func() { authgate.CacheTTL = old })
	gh.AddApp(appClientID, appSecret)
	t.Setenv("AGNI_TEST_CONNECTIONS_KEY", strings.Repeat("5e", 32))
	t.Setenv("AGNI_CONFIG_DIR", t.TempDir())
	t.Setenv(envToken, "")
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	out, err := runCobra(t, tokenCmd(), "create", "ops", "--store", "file:///"+strings.TrimPrefix(filepath.ToSlash(store), "/"))
	if err != nil {
		t.Fatalf("agni token create ops: %v", err)
	}
	ops, _, _ = strings.Cut(strings.TrimSpace(out), "\n")
	url = serveViewer(t, viewerOpts{storeDir: store, uploadsDir: filepath.Join(dir, "uploads"), auth: authgate.Tokens,
		connectionsKey: "env:AGNI_TEST_CONNECTIONS_KEY", githubAPI: gh.URL, githubRaw: gh.RawURL(),
		githubSignIn: &githubSignIn{clientID: appClientID, clientSecret: appSecret, allow: []string{"org:acme"}}})
	return url, ops
}

// browserSignIn signs a browser in as login, from /signin/github?next=next, and returns where it
// ended, the page's text, and the session the server set.
func browserSignIn(t *testing.T, gh *githubtest.Server, server, login, next string) (string, int, string, string) {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	browser := &http.Client{Jar: jar}
	gh.SignInAs(login)
	resp, err := browser.Get(server + "/signin/github?next=" + url.QueryEscape(next))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	u, _ := url.Parse(server)
	session := ""
	for _, c := range jar.Cookies(u) {
		if c.Name == authgate.SessionCookie {
			session = c.Value
		}
	}
	return resp.Request.URL.Path, resp.StatusCode, string(body), session
}

// A person in the allowed org signs in with GitHub from the browser and lands where they were
// going, holding a session. The server keeps their GitHub token as their own connection, so a
// design linked from a private repo they can read opens for them. Someone outside the org is
// refused, with no session (agni issue 1772).
func TestSignInWithGitHubKeepsThePersonsConnection(t *testing.T) {
	gh := githubtest.New(t)
	board := gh.AddRepo("acme", "board", true, map[string]string{"hw/bb.kicad_sch": "(kicad_sch)"})
	gh.AddUser("alice", "acme")
	gh.AddUser("mallory")
	board.Grant(secretToken)
	board.GrantUser("alice")
	server, ops := signInServer(t, gh)

	resp, err := http.Get(server + "/signin")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), `href="/signin/github?next=%2f"`) {
		t.Errorf("/signin offers no GitHub sign-in:\n%s", page)
	}

	path, code, body, session := browserSignIn(t, gh, server, "alice", "/designs")
	// This server has no viewer, so /designs itself answers 404 once the gate lets alice through.
	if path != "/designs" || code == http.StatusForbidden || session == "" {
		t.Fatalf("alice's sign-in ended at %s with %d and session %q:\n%s\nwant /designs, signed in", path, code, session, body)
	}

	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	t.Setenv(envToken, ops)
	t.Setenv("AGNI_TEST_GITHUB_TOKEN", secretToken)
	must(runCobra(t, connectionsCmd(), "add", "acme", "--github-token", "env:AGNI_TEST_GITHUB_TOKEN", "--remote", server))
	must(runMounts(t, "add", "board", "github://acme/board", "--connection", "acme", "--remote", server))

	// The viewer's mount form links a person to sign in when they have no connection (agni issue 1727).
	ws := webapiconnect.NewWorkspaceServiceClient(http.DefaultClient, server, bearer(session))
	if listed, err := ws.ListMounts(context.Background(), connect.NewRequest(&webapi.ListMountsRequest{})); err != nil || listed.Msg.GetSignInUrl() != authgate.SignInPath {
		t.Errorf("ListMounts on a server with GitHub sign-in = %v, %v; want sign_in_url %s", listed, err, authgate.SignInPath)
	}

	t.Setenv(envToken, session)
	if out := must(runCobra(t, connectionsCmd(), "ls", "--remote", server)); !strings.Contains(out, "github-alice\tgithub-user\tsealed, for github:alice\tok") {
		t.Errorf("alice's connections ls after signing in = %q; want her GitHub sign-in kept as her connection", out)
	}
	must(runDesigns(t, server, "create", "bb"))
	must(runDesigns(t, server, "link", "bb", "schematic", "board/hw/bb.kicad_sch@main"))
	if out := must(runDesigns(t, server, "get", "bb")); !strings.Contains(out, "opens for who can read github://acme/board") {
		t.Errorf("alice's designs get bb = %q; want the design, guarded by the repo", out)
	}

	path, code, body, session = browserSignIn(t, gh, server, "mallory", "/designs")
	if code != http.StatusForbidden || session != "" || !strings.Contains(body, "mallory isn") {
		t.Errorf("mallory's sign-in ended at %s with %d and session %q:\n%s\nwant 403, no session, saying mallory may not sign in", path, code, session, body)
	}
}

// agni login signs a person in with GitHub's device flow through the server and keeps the token,
// so later --remote calls send it with no AGNI_TOKEN (agni issue 1772).
func TestAgniLoginSignsInFromATerminal(t *testing.T) {
	gh := githubtest.New(t)
	alice := gh.AddUser("alice", "acme")
	gh.SetDeviceInterval(10 * time.Millisecond)
	server, _ := signInServer(t, gh)

	pr, pw := io.Pipe()
	cmd := loginCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(pw)
	cmd.SetArgs([]string{"--remote", server})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- cmd.ExecuteContext(ctx); pw.Close() }()

	line, err := bufio.NewReader(pr).ReadString('\n')
	if err != nil {
		t.Fatalf("agni login printed %q, %v; want the code to enter", line, err)
	}
	m := regexp.MustCompile(`Open (\S+) and enter the code (\S+)`).FindStringSubmatch(line)
	if m == nil || !strings.HasPrefix(m[1], gh.URL) {
		t.Fatalf("agni login printed %q; want GitHub's address and a code", line)
	}
	go func() { _, _ = io.Copy(io.Discard, pr) }()
	if !alice.Approve(m[2]) {
		t.Fatalf("GitHub doesn't know the code %s", m[2])
	}
	if err := <-done; err != nil {
		t.Fatalf("agni login: %v", err)
	}
	if !strings.Contains(out.String(), "signed in to "+server+" as github:alice") {
		t.Errorf("agni login printed %q; want alice signed in", out.String())
	}

	p, _ := credentials.File()
	// Windows has no Unix modes, and Go reports any writable file there as 0666.
	if fi, err := os.Stat(p); err != nil || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600) {
		t.Errorf("the kept token at %s: %v, %v; want a file only its owner reads", p, fi, err)
	}
	ls, err := runCobra(t, connectionsCmd(), "ls", "--remote", server)
	if err != nil || !strings.Contains(ls, "github-alice\tgithub-user\tsealed, for github:alice") {
		t.Errorf("connections ls with no AGNI_TOKEN after agni login = %q, %v; want alice's kept token sent, and her connection listed", ls, err)
	}
}
