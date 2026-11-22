package authgate

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/panyam/agni/internal/records/recordstest"
	"github.com/panyam/oneauth/core"
)

func TestCheckAuthFlags(t *testing.T) {
	for _, c := range []struct {
		auth, store, cert, key, addr string
		proxies                      []string
		want                         string
	}{
		{"", "", "", "", ":8080", nil, ""},
		{Tokens, "/s", "", "", "localhost:8080", nil, ""},
		{Tokens, "/s", "", "", "127.0.0.1:0", nil, ""},
		{Tokens, "/s", "c", "k", ":8080", nil, ""},
		{Tokens, "", "", "", "localhost:8080", nil, "needs --store"},
		{Tokens, "/s", "", "", ":8080", nil, "in the clear"},
		{Tokens, "/s", "", "", "10.0.0.5:8080", nil, "in the clear"},
		{Tokens, "/s", "", "", "10.0.0.5:8080", nil, "name the proxy that ends TLS in front of the server with --trusted-proxy"},
		{"", "", "c", "", ":8080", nil, "go together"},
		{"oauth", "/s", "", "", "localhost:8080", nil, "the one kind"},
		// A proxy ends TLS in front of the server, so it may serve plain HTTP anywhere (agni issue 1810).
		{Tokens, "/s", "", "", ":8080", []string{"10.0.0.0/8"}, ""},
		{Tokens, "/s", "", "", "10.0.0.5:8080", []string{"192.0.2.10"}, ""},
		{Tokens, "", "", "", ":8080", []string{"10.0.0.0/8"}, "needs --store"},
	} {
		proxies, err := ParseProxies(c.proxies)
		if err != nil {
			t.Fatal(err)
		}
		err = CheckFlags(c.auth, c.store, c.cert, c.key, c.addr, proxies)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("CheckFlags(%q, %q, %q, %q, %q, %v) = %v, want nil", c.auth, c.store, c.cert, c.key, c.addr, c.proxies, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("CheckFlags(%q, %q, %q, %q, %q, %v) = %v, want %q", c.auth, c.store, c.cert, c.key, c.addr, c.proxies, err, c.want)
		}
	}
}

func TestParseProxiesNamesTheFlagAndTheBadEntry(t *testing.T) {
	if _, err := ParseProxies([]string{"10.0.0.0/8", "10.0.0.0/33"}); err == nil || !strings.Contains(err.Error(), "--trusted-proxy") || !strings.Contains(err.Error(), "10.0.0.0/33") {
		t.Errorf("ParseProxies with a bad range = %v; want an error naming --trusted-proxy and the range", err)
	}
	if got, err := ParseProxies([]string{"10.0.0.0/8", "192.0.2.10"}); err != nil || len(got) != 2 {
		t.Errorf("ParseProxies(a range, an address) = %v, %v; want both", got, err)
	}
}

// Behind a proxy that ends TLS, a browser's session cookie is Secure when the request came from the
// trusted proxy and the proxy says it took it over HTTPS, or when the server took it over TLS itself.
// A forwarded header from any other peer counts for nothing (agni issue 1810).
func TestASessionCookieIsSecureOnlyWhenTheRequestCameOverHTTPS(t *testing.T) {
	store := Store(t.TempDir())
	made, err := store.CreateAPIKey(context.Background(), &core.CreateAPIKeyRequest{Subject: "bob", Name: "bob"})
	if err != nil {
		t.Fatal(err)
	}
	proxies, err := ParseProxies([]string{"10.1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	h := New(store, proxies).Wrap(http.NotFoundHandler())
	for _, c := range []struct {
		name, peer, proto string
		tls, want         bool
	}{
		{"the proxy says https", "10.1.2.3:5000", "https", false, true},
		{"the proxy says http", "10.1.2.3:5000", "http", false, false},
		{"the proxy says nothing", "10.1.2.3:5000", "", false, false},
		{"another peer says https", "203.0.113.9:5000", "https", false, false},
		{"the server took TLS itself", "203.0.113.9:5000", "", true, true},
	} {
		for _, path := range []string{"/signin", "/signout"} {
			r := httptest.NewRequest(http.MethodPost, path, strings.NewReader("token="+made.FullKey))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.RemoteAddr = c.peer
			if c.proto != "" {
				r.Header.Set("X-Forwarded-Proto", c.proto)
			}
			if c.tls {
				r.TLS = &tls.ConnectionState{}
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			var session *http.Cookie
			for _, k := range w.Result().Cookies() {
				if k.Name == SessionCookie {
					session = k
				}
			}
			if session == nil {
				t.Errorf("%s: POST %s = %d with no %s cookie", c.name, path, w.Code, SessionCookie)
				continue
			}
			if session.Secure != c.want {
				t.Errorf("%s: POST %s set a cookie with Secure %v; want %v", c.name, path, session.Secure, c.want)
			}
		}
	}
}

func TestSafeNext(t *testing.T) {
	for in, want := range map[string]string{
		"/designs/x?y=1":       "/designs/x?y=1",
		"":                     "/",
		"//evil.example/":      "/",
		"/\\evil.example":      "/",
		"https://evil.example": "/",
		"designs":              "/",
	} {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}

// GitHub sends a push webhook with no token, so the gate lets that one path through to the handler,
// which checks the delivery's signature, and still refuses its neighbors (agni issue 1726).
func TestTheGateLetsAWebhookThroughWithNoToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
	h := New(testKeys(t), nil).Wrap(next)
	for path, want := range map[string]int{"/hooks/github": http.StatusAccepted, "/hooks/gitlab": http.StatusUnauthorized, "/hooks/github/x": http.StatusUnauthorized} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}")))
		if w.Code != want {
			t.Errorf("POST %s with no token = %d; want %d", path, w.Code, want)
		}
	}
}

// Prune deletes the tokens that have expired, and keeps one with no expiry, one still valid and a
// revoked one (agni issue 1778).
func TestPruneDeletesOnlyExpiredTokens(t *testing.T) {
	ctx := context.Background()
	store := testKeys(t)
	now := time.Now()
	mint := func(subject string, expires *time.Time) string {
		t.Helper()
		resp, err := store.CreateAPIKey(ctx, &core.CreateAPIKeyRequest{Subject: subject, Name: subject, ExpiresAt: expires})
		if err != nil {
			t.Fatal(err)
		}
		return resp.APIKey.KeyID
	}
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	mint("github:old", &past)
	mint("github:fresh", &future)
	mint("ops", nil)
	revoked := mint("gone", nil)
	if _, err := store.RevokeAPIKey(ctx, &core.RevokeAPIKeyRequest{KeyID: revoked}); err != nil {
		t.Fatal(err)
	}

	n, err := Prune(ctx, store, now)
	if err != nil || n != 1 {
		t.Fatalf("Prune = %d, %v; want the one expired token deleted", n, err)
	}
	keys, err := List(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for _, k := range keys {
		left = append(left, k.Subject)
	}
	sort.Strings(left)
	if got := strings.Join(left, ","); got != "github:fresh,gone,ops" {
		t.Errorf("tokens left = %s; want github:fresh, the revoked gone, and ops", got)
	}
	if n, err := Prune(ctx, store, now); err != nil || n != 0 {
		t.Errorf("a second Prune = %d, %v; want nothing left to delete", n, err)
	}
}

// A key deleted between List reading the folder and reading the key is left out, rather than failing
// the listing. Expired tokens are deleted while the server runs, so `agni token list` can race that
// sweep (agni issue 1796). A dangling link stands in for the deleted key, since the folder lists it
// and reading it finds nothing.
func TestListSkipsAKeyDeletedWhileListing(t *testing.T) {
	dir := t.TempDir()
	keys := Store(dir)
	if _, err := keys.CreateAPIKey(context.Background(), &core.CreateAPIKeyRequest{Subject: "ops", Name: "ops"}); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(dir, StoreDir, "api_keys", "gone.json")
	if err := os.Symlink(filepath.Join(dir, "nowhere.json"), gone); err != nil {
		t.Fatal(err)
	}
	left, err := List(context.Background(), keys)
	if err != nil {
		t.Fatalf("List failed on a key deleted while listing: %v", err)
	}
	if len(left) != 1 || left[0].Subject != "ops" {
		t.Fatalf("List = %v, want only ops", left)
	}
}

// testKeys is an empty token store in the backend AGNI_TEST_RECORDS names.
func testKeys(t *testing.T) Keys {
	t.Helper()
	dir := t.TempDir()
	switch recordstest.Backend() {
	case "sqlite", "postgres":
		k, err := SQLStore(recordstest.DB(t, dir))
		if err != nil {
			t.Fatal(err)
		}
		return k
	case "datastore":
		return GAEStore(recordstest.Datastore(t, dir))
	}
	return Store(dir)
}
