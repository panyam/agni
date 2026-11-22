package main

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/goapplib/storage/mounts/github/githubtest"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/authgate"
)

// tlsProxy is a proxy that ends TLS in front of a server, as an ingress or a load balancer does. It
// sets X-Forwarded-Proto itself and keeps every Set-Cookie the server answers with.
type tlsProxy struct {
	*httptest.Server
	mu      sync.Mutex
	target  *url.URL
	cookies []*http.Cookie
}

func newTLSProxy(t *testing.T) *tlsProxy {
	t.Helper()
	p := &tlsProxy{}
	rp := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			p.mu.Lock()
			defer p.mu.Unlock()
			r.SetURL(p.target)
			r.SetXForwarded()
		},
		ModifyResponse: func(resp *http.Response) error {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.cookies = append(p.cookies, resp.Cookies()...)
			return nil
		},
	}
	p.Server = httptest.NewTLSServer(rp)
	t.Cleanup(p.Close)
	return p
}

func (p *tlsProxy) to(server string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.target, _ = url.Parse(server)
}

// set returns the cookies the server set through the proxy, by name.
func (p *tlsProxy) set() map[string]*http.Cookie {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[string]*http.Cookie{}
	for _, c := range p.cookies {
		out[c.Name] = c
	}
	return out
}

// A shared server behind a proxy that ends TLS serves plain HTTP to the proxy. A browser that signs in
// through the proxy, with a token or with GitHub, gets Secure cookies, and GitHub sends it back to
// --public-url. A call with a token works through the proxy. The same sign-in straight to the server
// over plain HTTP gets a cookie that isn't Secure (agni issue 1810).
func TestASharedServerBehindAProxyThatEndsTLS(t *testing.T) {
	old := authgate.CacheTTL
	authgate.CacheTTL = time.Millisecond
	t.Cleanup(func() { authgate.CacheTTL = old })
	gh := githubtest.New(t)
	gh.AddApp(appClientID, appSecret)
	gh.AddUser("alice", "acme")
	t.Setenv("AGNI_TEST_CONNECTIONS_KEY", strings.Repeat("5e", 32))
	t.Setenv("AGNI_CONFIG_DIR", t.TempDir())
	t.Setenv(envToken, "")
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	out, err := runCobra(t, tokenCmd(), "create", "ops", "--store", testStoreURL(t, store))
	if err != nil {
		t.Fatalf("agni token create ops: %v", err)
	}
	ops, _, _ := strings.Cut(strings.TrimSpace(out), "\n")

	proxy := newTLSProxy(t)
	proxies, err := authgate.ParseProxies([]string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	server := serveViewer(t, viewerOpts{store: testStore(t, store), uploadsDir: filepath.Join(dir, "uploads"), auth: authgate.Tokens,
		trustedProxies: proxies, publicURL: proxy.URL,
		connectionsKey: "env:AGNI_TEST_CONNECTIONS_KEY", githubAPI: gh.URL, githubRaw: gh.RawURL(),
		githubSignIn: &githubSignIn{clientID: appClientID, clientSecret: appSecret, allow: []string{"org:acme"}}})
	proxy.to(server)

	noFollow := *proxy.Client()
	noFollow.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := noFollow.PostForm(proxy.URL+"/signin", url.Values{"token": {ops}, "next": {"/"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if c := proxy.set()[authgate.SessionCookie]; resp.StatusCode != http.StatusSeeOther || c == nil || !c.Secure {
		t.Errorf("signing in with a token through the proxy = %d with %+v; want 303 and a Secure %s cookie", resp.StatusCode, c, authgate.SessionCookie)
	}

	jar, _ := cookiejar.New(nil)
	browser := *proxy.Client()
	browser.Jar = jar
	gh.SignInAs("alice")
	resp, err = browser.Get(proxy.URL + authgate.SignInPath + "?next=" + url.QueryEscape("/designs"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.Request.URL.Host != strings.TrimPrefix(proxy.URL, "https://") || resp.Request.URL.Path != "/designs" || resp.StatusCode == http.StatusForbidden {
		t.Fatalf("alice's GitHub sign-in through the proxy ended at %s with %d:\n%s\nwant /designs on the proxy", resp.Request.URL, resp.StatusCode, body)
	}
	set := proxy.set()
	for _, name := range []string{authgate.SessionCookie, "agni_signin_next"} {
		if c := set[name]; c == nil || !c.Secure {
			t.Errorf("GitHub sign-in through the proxy set %s = %+v; want it Secure", name, c)
		}
	}

	info, err := webapiconnect.NewServerServiceClient(proxy.Client(), proxy.URL, bearer(ops)).GetServerInfo(context.Background(), connect.NewRequest(&webapi.GetServerInfoRequest{}))
	if err != nil || info.Msg.GetVersion() == "" && len(info.Msg.GetFeatures()) == 0 {
		t.Errorf("GetServerInfo with a token through the proxy = %v, %v; want an answer", info, err)
	}

	direct := &http.Client{Timeout: 5 * time.Second, CheckRedirect: noFollow.CheckRedirect}
	resp, err = direct.PostForm(server+"/signin", url.Values{"token": {ops}, "next": {"/"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	var session *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == authgate.SessionCookie {
			session = c
		}
	}
	if session == nil || session.Secure {
		t.Errorf("signing in straight to the server over plain HTTP set %+v; want a %s cookie that isn't Secure", session, authgate.SessionCookie)
	}
}

// Behind --trusted-proxy, signing in with GitHub needs --public-url, since GitHub sends the browser
// back there and the address the server listens on is behind the proxy. A bad --trusted-proxy is
// refused by name (agni issue 1810).
func TestServeRefusesGitHubSignInBehindAProxyWithoutAPublicURL(t *testing.T) {
	t.Setenv("AGNI_TEST_SECRET", "s")
	t.Setenv("AGNI_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()
	store := "file:///" + strings.TrimPrefix(filepath.ToSlash(filepath.Join(dir, "store")), "/")
	uploads := "file:///" + strings.TrimPrefix(filepath.ToSlash(filepath.Join(dir, "uploads")), "/")
	base := []string{"serve", "--store", store, "--uploads", uploads, "--auth", authgate.Tokens, "--addr", "192.0.2.1:0"}
	signIn := []string{"--connections-key", "env:AGNI_TEST_CONNECTIONS_KEY", "--github-client-id", appClientID, "--github-client-secret", "env:AGNI_TEST_SECRET", "--github-allow", "org:acme"}
	t.Setenv("AGNI_TEST_CONNECTIONS_KEY", strings.Repeat("5e", 32))
	for _, c := range []struct {
		args []string
		want string
	}{
		{append(append(append([]string{}, base...), signIn...), "--trusted-proxy", "10.0.0.0/8"), "needs --public-url"},
		{append(append([]string{}, base...), "--trusted-proxy", "10.0.0.0/33"), "--trusted-proxy: trusted proxy \"10.0.0.0/33\""},
		{base, "in the clear"},
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
