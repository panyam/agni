package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/authgate"
	"github.com/panyam/agni/internal/credentials"
)

// runUsers runs agni users with args, giving stdin as its input.
func runUsers(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := usersCmd()
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

// localServer serves the viewer behind --auth tokens --local-accounts over a fresh --store, and
// returns the server and the store's URL.
func localServer(t *testing.T) (string, string) {
	t.Helper()
	old := authgate.CacheTTL
	authgate.CacheTTL = time.Millisecond
	t.Cleanup(func() { authgate.CacheTTL = old })
	t.Setenv("AGNI_CONFIG_DIR", t.TempDir())
	t.Setenv(envToken, "")
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	o := viewerOpts{store: testStore(t, store), uploadsDir: filepath.Join(dir, "uploads"), auth: authgate.Tokens, localAccounts: true}
	return serveViewer(t, o), testStoreURL(t, store)
}

// localSignIn posts the sign-in form to server and returns the status and the session it set, or ""
// for none. It doesn't follow the redirect.
func localSignIn(t *testing.T, server, user, password string) (int, string) {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.PostForm(server+authgate.LocalSignInPath, url.Values{"username": {user}, "password": {password}, "next": {"/"}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	for _, c := range resp.Cookies() {
		if c.Name == authgate.SessionCookie {
			return resp.StatusCode, c.Value
		}
	}
	return resp.StatusCode, ""
}

// An operator adds alice, she signs in at the form and her session reads the API, a new password
// replaces the old one at the next sign-in, and removing her stops her signing in (agni issue 1815).
func TestUsersRoundTripThroughASignIn(t *testing.T) {
	server, store := localServer(t)
	if out, err := runUsers(t, "first password\n", "add", "alice", "--store", store, "--password-stdin"); err != nil || !strings.Contains(out, "local:alice") {
		t.Fatalf("users add alice = %q, %v", out, err)
	}
	if out, err := runUsers(t, "first password\n", "add", "Alice", "--store", store, "--password-stdin"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("users add Alice again = %q, %v; want a refusal saying it exists", out, err)
	}
	if out, err := runUsers(t, "", "ls", "--store", store); err != nil || !strings.HasPrefix(out, "alice\tlocal:alice\t") || strings.Count(out, "\n") != 1 {
		t.Fatalf("users ls = %q, %v; want one line for alice", out, err)
	}

	status, session := localSignIn(t, server, "alice", "first password")
	if status != http.StatusSeeOther || session == "" {
		t.Fatalf("signing in as alice = %d with session %q; want 303 and a session", status, session)
	}
	if ls, err := runCobra(t, tokenCmd(), "ls", "--store", store); err != nil || !strings.Contains(ls, "\tlocal:alice\t") {
		t.Fatalf("token ls after the sign-in = %q, %v; want a token for local:alice", ls, err)
	}
	if _, err := webapiconnect.NewWorkspaceServiceClient(http.DefaultClient, server, bearer(session)).ListMounts(context.Background(), connect.NewRequest(&webapi.ListMountsRequest{})); err != nil {
		t.Fatalf("ListMounts with alice's session: %v", err)
	}

	if out, err := runUsers(t, "second password\n", "passwd", "alice", "--store", store, "--password-stdin"); err != nil {
		t.Fatalf("users passwd alice = %q, %v", out, err)
	}
	if status, session := localSignIn(t, server, "alice", "first password"); status != http.StatusUnauthorized || session != "" {
		t.Errorf("the old password after passwd = %d with session %q; want 401 and none", status, session)
	}
	if status, session := localSignIn(t, server, "alice", "second password"); status != http.StatusSeeOther || session == "" {
		t.Errorf("the new password after passwd = %d with session %q; want 303 and a session", status, session)
	}

	if out, err := runUsers(t, "", "rm", "alice", "--store", store); err != nil || !strings.Contains(out, "removed alice") {
		t.Fatalf("users rm alice = %q, %v", out, err)
	}
	if status, session := localSignIn(t, server, "alice", "second password"); status != http.StatusUnauthorized || session != "" {
		t.Errorf("signing in after rm = %d with session %q; want 401 and none", status, session)
	}
	if out, err := runUsers(t, "", "ls", "--store", store); err != nil || out != "" {
		t.Errorf("users ls after rm = %q, %v; want nothing", out, err)
	}
	if _, err := runUsers(t, "", "rm", "alice", "--store", store); err == nil || !strings.Contains(err.Error(), "no such user") {
		t.Errorf("users rm alice twice = %v; want no such user", err)
	}
}

// runLogin runs agni login with args, giving stdin as its input.
func runLogin(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := loginCmd()
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

// agni login --with local posts alice's username and password to a live server and keeps the
// token it answers, which then reads the API. A wrong password keeps no token (agni issue 1879).
func TestAgniLoginWithLocalKeepsTheToken(t *testing.T) {
	server, store := localServer(t)
	if out, err := runUsers(t, "first password\n", "add", "alice", "--store", store, "--password-stdin"); err != nil {
		t.Fatalf("users add alice = %q, %v", out, err)
	}
	out, err := runLogin(t, "wrong password\n", "--remote", server, "--with", "local", "--user", "alice", "--password-stdin")
	if err == nil || !strings.Contains(err.Error(), "don't match an account") || credentials.Token(server) != "" {
		t.Fatalf("agni login with a wrong password = %q, %v, kept %q; want refused, keeping nothing", out, err, credentials.Token(server))
	}
	out, err = runLogin(t, "first password\n", "--remote", server, "--with", "local", "--user", "Alice", "--password-stdin")
	if err != nil || !strings.Contains(out, "signed in to "+server+" as local:alice") {
		t.Fatalf("agni login --with local = %q, %v; want alice signed in as local:alice", out, err)
	}
	kept := credentials.Token(server)
	if kept == "" {
		t.Fatal("agni login --with local kept no token")
	}
	if _, err := webapiconnect.NewWorkspaceServiceClient(http.DefaultClient, server, bearer(kept)).ListMounts(context.Background(), connect.NewRequest(&webapi.ListMountsRequest{})); err != nil {
		t.Fatalf("ListMounts with the kept token: %v", err)
	}
	if ls, err := runCobra(t, tokenCmd(), "ls", "--store", store); err != nil || !strings.Contains(ls, "\tlocal:alice\t") {
		t.Errorf("token ls after agni login = %q, %v; want a token for local:alice", ls, err)
	}
	if _, err := runLogin(t, "", "--remote", server, "--with", "local", "--user", "alice"); err == nil || !strings.Contains(err.Error(), "--password-stdin") {
		t.Errorf("agni login --with local without --password-stdin = %v; want a refusal naming it", err)
	}
	if _, err := runLogin(t, "first password\n", "--remote", server, "--with", "local", "--password-stdin"); err == nil || !strings.Contains(err.Error(), "--user") {
		t.Errorf("agni login --with local without --user = %v; want a refusal naming it", err)
	}
}

// carol signs in with a local account, adds her own GitHub connection with --mine, lists it, and
// opens a design linked from a private repo through it, the same way an OIDC or token user does.
// dave, another local account, sees none of it (agni issue 1879).
func TestALocalUserKeepsAndUsesHerOwnConnection(t *testing.T) {
	old := authgate.CacheTTL
	authgate.CacheTTL = time.Millisecond
	t.Cleanup(func() { authgate.CacheTTL = old })
	t.Setenv("AGNI_CONFIG_DIR", t.TempDir())
	gh, repo := privateGitHubRepo(t)
	const carolGitHub = "ghp_carol_reads_acme"
	repo.Grant(secretToken, carolGitHub)
	t.Setenv("AGNI_TEST_GITHUB_TOKEN", secretToken)
	t.Setenv("AGNI_TEST_CAROL_GITHUB", carolGitHub)
	t.Setenv("AGNI_TEST_CONNECTIONS_KEY", strings.Repeat("5e", 32))
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	storeURL := testStoreURL(t, store)
	out, err := runCobra(t, tokenCmd(), "create", "ops", "--store", storeURL)
	if err != nil {
		t.Fatalf("agni token create ops: %v", err)
	}
	ops, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	for _, who := range []string{"carol", "dave"} {
		if out, err := runUsers(t, "a password for "+who+"\n", "add", who, "--store", storeURL, "--password-stdin"); err != nil {
			t.Fatalf("users add %s = %q, %v", who, out, err)
		}
	}
	server := serveViewer(t, viewerOpts{store: testStore(t, store), uploadsDir: filepath.Join(dir, "uploads"), auth: authgate.Tokens, localAccounts: true,
		connectionsKey: "env:AGNI_TEST_CONNECTIONS_KEY", githubAPI: gh.URL, githubRaw: gh.RawURL()})
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}

	t.Setenv(envToken, ops)
	must(runCobra(t, connectionsCmd(), "add", "acme", "--github-token", "env:AGNI_TEST_GITHUB_TOKEN", "--remote", server))
	must(runMounts(t, "add", "board", "github://acme/board", "--connection", "acme", "--remote", server))

	// carol's calls send the token agni login kept, since AGNI_TOKEN is empty.
	t.Setenv(envToken, "")
	if out := must(runLogin(t, "a password for carol\n", "--remote", server, "--with", "local", "--user", "carol", "--password-stdin")); !strings.Contains(out, "as local:carol") {
		t.Fatalf("carol's agni login = %q; want local:carol", out)
	}
	if out := must(runCobra(t, connectionsCmd(), "add", "carol-gh", "--mine", "--github-token", "env:AGNI_TEST_CAROL_GITHUB", "--remote", server)); out != "carol-gh\tgithub-user\tsealed, for local:carol\tok\n" {
		t.Errorf("carol's connections add --mine = %q; want her own, sealed, for local:carol", out)
	}
	if out := must(runCobra(t, connectionsCmd(), "ls", "--remote", server)); !strings.Contains(out, "carol-gh\tgithub-user\tsealed, for local:carol") {
		t.Errorf("carol's connections ls = %q; want carol-gh, for local:carol", out)
	}
	must(runDesigns(t, server, "create", "bb"))
	must(runDesigns(t, server, "link", "bb", "schematic", "board/hw/bb.kicad_sch@main"))
	if got := must(runDesigns(t, server, "get", "bb")); !strings.Contains(got, "; opens for who can read github://acme/board") {
		t.Errorf("carol's designs get bb = %q; want the linked design, opening for who can read the repo", got)
	}

	// dave signs in with a session of his own, and sees neither carol's connection nor her design.
	status, dave := localSignIn(t, server, "dave", "a password for dave")
	if status != http.StatusSeeOther || dave == "" {
		t.Fatalf("dave's sign-in = %d with session %q; want 303 and a session", status, dave)
	}
	t.Setenv(envToken, dave)
	if out := must(runCobra(t, connectionsCmd(), "ls", "--remote", server)); strings.Contains(out, "carol-gh") {
		t.Errorf("dave's connections ls = %q; want carol's left out", out)
	}
	if out, err := runDesigns(t, server, "get", "bb"); err == nil || !strings.Contains(err.Error(), "permission_denied") {
		t.Errorf("dave's designs get bb = %q, %v; want permission_denied, since he has no connection reading the repo", out, err)
	}
}

// agni login --with local names the missing sign-in on a server without local accounts.
func TestAgniLoginWithLocalOnAServerWithoutThem(t *testing.T) {
	t.Setenv("AGNI_CONFIG_DIR", t.TempDir())
	t.Setenv(envToken, "")
	dir := t.TempDir()
	server := serveViewer(t, viewerOpts{store: testStore(t, filepath.Join(dir, "store")), uploadsDir: filepath.Join(dir, "uploads"), auth: authgate.Tokens})
	if _, err := runLogin(t, "first password\n", "--remote", server, "--with", "local", "--user", "alice", "--password-stdin"); err == nil || !strings.Contains(err.Error(), "doesn't sign people in with local accounts") {
		t.Errorf("agni login --with local on a server without them = %v; want it refused, naming local accounts", err)
	}
}

// agni users takes a password only on stdin, so it never lands in the shell history.
func TestUsersNeedsPasswordStdin(t *testing.T) {
	store := testStoreURL(t, t.TempDir())
	for _, args := range [][]string{{"add", "alice"}, {"passwd", "alice"}} {
		if _, err := runUsers(t, "a password\n", append(args, "--store", store)...); err == nil || !strings.Contains(err.Error(), "--password-stdin") {
			t.Errorf("users %v without --password-stdin = %v; want a refusal naming it", args, err)
		}
	}
	if _, err := runUsers(t, "", "add", "alice", "--store", store, "--password-stdin"); err == nil || !strings.Contains(err.Error(), "empty password") {
		t.Errorf("users add with an empty stdin = %v; want a refusal", err)
	}
	if _, err := runUsers(t, "a password\n", "add", "alice", "--password-stdin"); err == nil || !strings.Contains(err.Error(), "--store") {
		t.Errorf("users add with no --store = %v; want a refusal naming --store", err)
	}
}

// --local-accounts mints tokens, so serve refuses it without --auth tokens, whether it comes as a
// flag or from a --config file (agni issue 1853).
func TestLocalAccountsNeedAuthTokens(t *testing.T) {
	t.Setenv("AGNI_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()
	run := func(args ...string) error {
		cmd := rootCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(append([]string{"serve", "--addr", "192.0.2.1:0"}, args...))
		return cmd.Execute()
	}
	if err := run("--local-accounts"); err == nil || !strings.Contains(err.Error(), "--local-accounts mints a token") {
		t.Errorf("serve --local-accounts without --auth = %v; want a refusal naming --auth tokens", err)
	}
	path := filepath.Join(dir, "serve.yaml")
	if err := os.WriteFile(path, []byte("local_accounts: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run("--config", path); err == nil || !strings.Contains(err.Error(), "--local-accounts mints a token") {
		t.Errorf("serve --config with local_accounts and no --auth = %v; want the same refusal", err)
	}
}
