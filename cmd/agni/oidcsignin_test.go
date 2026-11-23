package main

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/panyam/goapplib/storage/mounts/github/githubtest"
	"github.com/panyam/oneauth/oauth2/oidc/oidctest"

	"github.com/panyam/agni/internal/authgate"
)

const (
	oidcClientID = "agni-test"
	oidcSecret   = "oidc-secret"
)

// oidcServer serves the viewer behind --auth, signing people in through a fake OpenID Connect
// provider that lets example.com's verified emails in, labelled Okta. With gh it signs people in
// with GitHub too. It returns the server, the fake, and the store, so a test can list its tokens.
func oidcServer(t *testing.T, gh *githubtest.Server) (string, *oidctest.Server, string) {
	t.Helper()
	old := authgate.CacheTTL
	authgate.CacheTTL = time.Millisecond
	t.Cleanup(func() { authgate.CacheTTL = old })
	t.Setenv("AGNI_CONFIG_DIR", t.TempDir())
	t.Setenv(envToken, "")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + ln.Addr().String()
	iss := oidctest.New(t, oidctest.WithClient(oidcClientID, oidcSecret, base+authgate.OIDCSignInPath+"/callback/"), oidctest.WithDeviceInterval(10*time.Millisecond))
	iss.AddUser("u-ada", map[string]any{"email": "ada@example.com", "email_verified": true})
	iss.AddUser("u-eve", map[string]any{"email": "eve@other.example", "email_verified": true})
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	o := viewerOpts{listener: ln, store: testStore(t, store), uploadsDir: filepath.Join(dir, "uploads"), auth: authgate.Tokens,
		oidcSignIn: &oidcSignIn{issuer: iss.URL, clientID: oidcClientID, clientSecret: oidcSecret, allow: []string{"domain:example.com"}, label: "Okta"}}
	if gh != nil {
		gh.AddApp(appClientID, appSecret)
		t.Setenv("AGNI_TEST_CONNECTIONS_KEY", strings.Repeat("5e", 32))
		o.connectionsKey = "env:AGNI_TEST_CONNECTIONS_KEY"
		o.githubAPI, o.githubRaw = gh.URL, gh.RawURL()
		o.githubSignIn = &githubSignIn{clientID: appClientID, clientSecret: appSecret, allow: []string{"org:acme"}}
	}
	return serveViewer(t, o), iss, store
}

// oidcBrowserSignIn signs a browser in as sub, from /signin/oidc?next=next, and returns where it
// ended, the status, the page's text, and the session the server set.
func oidcBrowserSignIn(t *testing.T, iss *oidctest.Server, server, sub, next string) (string, int, string, string) {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	browser := &http.Client{Jar: jar}
	iss.SignInAs(sub)
	resp, err := browser.Get(server + authgate.OIDCSignInPath + "?next=" + url.QueryEscape(next))
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

// A person with a verified example.com email signs in through the OpenID Connect provider from the
// browser, lands where they were going, and holds a token for oidc:<sub>. Someone outside the domain
// is refused with no session (agni issue 1811).
func TestSignInThroughOIDC(t *testing.T) {
	server, iss, store := oidcServer(t, nil)
	resp, err := http.Get(server + "/signin")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), `href="/signin/oidc?next=%2f">Sign in with Okta`) || strings.Contains(string(page), "/signin/github") {
		t.Errorf("/signin offers no OIDC sign-in labelled Okta, or offers GitHub:\n%s", page)
	}

	path, code, body, session := oidcBrowserSignIn(t, iss, server, "u-ada", "/designs")
	if path != "/designs" || code == http.StatusForbidden || session == "" {
		t.Fatalf("ada's sign-in ended at %s with %d and session %q:\n%s\nwant /designs, signed in", path, code, session, body)
	}
	if ls, err := runCobra(t, tokenCmd(), "ls", "--store", testStoreURL(t, store)); err != nil || !strings.Contains(ls, "\toidc:u-ada\t") {
		t.Errorf("token ls after ada signed in = %q, %v; want a token for oidc:u-ada", ls, err)
	}

	path, code, body, session = oidcBrowserSignIn(t, iss, server, "u-eve", "/designs")
	if code != http.StatusForbidden || session != "" || !strings.Contains(body, "eve@other.example isn") {
		t.Errorf("eve's sign-in ended at %s with %d and session %q:\n%s\nwant 403, no session, saying eve may not sign in", path, code, session, body)
	}
}

// loginThrough runs agni login with args against server, approves the code it prints as sub at
// iss, and returns what it printed and its error.
func loginThrough(t *testing.T, iss *oidctest.Server, server, sub string, args ...string) (string, error) {
	t.Helper()
	pr, pw := io.Pipe()
	cmd := loginCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(pw)
	cmd.SetArgs(append([]string{"--remote", server}, args...))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- cmd.ExecuteContext(ctx); pw.Close() }()
	lines := bufio.NewReader(pr)
	line, rerr := lines.ReadString('\n')
	go func() { _, _ = io.Copy(io.Discard, lines) }()
	if m := regexp.MustCompile(`Open (\S+) and enter the code (\S+)`).FindStringSubmatch(line); m != nil {
		if !strings.HasPrefix(m[1], iss.URL) {
			t.Errorf("agni login printed %q; want the provider's address", line)
		}
		if !iss.Approve(m[2], sub) {
			t.Fatalf("the provider doesn't know the code %s", m[2])
		}
	} else if rerr == nil {
		t.Logf("agni login printed %q", line)
	}
	err := <-done
	return out.String(), err
}

// agni login signs in through the provider's device flow, with --with oidc or, on a server with no
// other sign-in, without --with (agni issue 1811).
func TestAgniLoginThroughOIDC(t *testing.T) {
	server, iss, _ := oidcServer(t, nil)
	for _, args := range [][]string{{"--with", "oidc"}, nil} {
		out, err := loginThrough(t, iss, server, "u-ada", args...)
		if err != nil || !strings.Contains(out, "signed in to "+server+" as oidc:u-ada") {
			t.Errorf("agni login %v = %q, %v; want ada signed in as oidc:u-ada", args, out, err)
		}
	}
	if _, err := loginThrough(t, iss, server, "u-ada", "--with", "github"); err == nil || !strings.Contains(err.Error(), "doesn't sign people in with github") {
		t.Errorf("agni login --with github on a server without it = %v; want it refused, naming github", err)
	}
}

// A server offering GitHub and OIDC shows both on /signin, and agni login without --with asks which
// one (agni issue 1811).
func TestAServerWithTwoSignInsAsksWhichOne(t *testing.T) {
	gh := githubtest.New(t)
	server, iss, _ := oidcServer(t, gh)
	resp, err := http.Get(server + "/signin")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), `href="/signin/github?next=%2f"`) || !strings.Contains(string(page), `href="/signin/oidc?next=%2f"`) {
		t.Errorf("/signin on a server with both sign-ins:\n%s\nwant both buttons", page)
	}
	if _, err := loginThrough(t, iss, server, "u-ada"); err == nil || !strings.Contains(err.Error(), "--with github or --with oidc") {
		t.Errorf("agni login without --with = %v; want it to ask for --with github or --with oidc", err)
	}
	if out, err := loginThrough(t, iss, server, "u-ada", "--with", "oidc"); err != nil || !strings.Contains(out, "as oidc:u-ada") {
		t.Errorf("agni login --with oidc = %q, %v; want ada signed in", out, err)
	}
}

// serve refuses OIDC sign-in with no allow list, an allow rule of an unknown kind, or no --auth
// tokens, by name (agni issue 1811).
func TestServeRefusesABadOIDCSignIn(t *testing.T) {
	t.Setenv("AGNI_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()
	store := "file:///" + strings.TrimPrefix(filepath.ToSlash(filepath.Join(dir, "store")), "/")
	uploads := "file:///" + strings.TrimPrefix(filepath.ToSlash(filepath.Join(dir, "uploads")), "/")
	base := []string{"serve", "--store", store, "--uploads", uploads, "--addr", "127.0.0.1:0", "--oidc-issuer", "https://login.example.com", "--oidc-client-id", "agni"}
	for _, c := range []struct {
		args []string
		want string
	}{
		{append(append([]string{}, base...), "--auth", authgate.Tokens), "needs --oidc-allow"},
		{append(append([]string{}, base...), "--auth", authgate.Tokens, "--oidc-allow", "org:acme"), "want domain:<domain>, group:<id>, sub:<sub> or hd:<domain>"},
		{append(append([]string{}, base...), "--oidc-allow", "domain:example.com"), "needs --auth tokens"},
		{[]string{"serve", "--store", store, "--uploads", uploads, "--auth", authgate.Tokens, "--addr", "127.0.0.1:0", "--oidc-issuer", "https://login.example.com", "--oidc-allow", "domain:example.com"}, "needs both --oidc-issuer and --oidc-client-id"},
	} {
		cmd := rootCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(c.args)
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("agni %s = %v; want an error saying %q", strings.Join(c.args, " "), err, c.want)
		}
	}
}
