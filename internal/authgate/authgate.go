// Package authgate asks every caller of an agni server for a token (agni issues 1627 and 1765). agni
// serve and agnids both use it, over the tokens `agni token create` keeps in the shared --store.
package authgate

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	skmw "github.com/panyam/servicekit/middleware"

	"github.com/panyam/oneauth/apiauth"
	"github.com/panyam/oneauth/core"
	"github.com/panyam/oneauth/grpc/connectauth"
	oafs "github.com/panyam/oneauth/stores/fs"

	"github.com/panyam/agni/service"
)

// Tokens is --auth's one value today: callers present a token `agni token create` made (agni
// issue 1627). An identity provider joins it with agni issue 1176.
const Tokens = "tokens"

// StoreDir holds the server's tokens under --store, each as a bcrypt hash and never the
// token itself.
const StoreDir = "tokens"

// SessionCookie carries a browser's token once it signs in at /signin, since a page can't send an
// Authorization header.
const SessionCookie = "agni_session"

// CacheTTL is how long the server trusts a token it has checked before checking it again. A
// revoked token keeps working for up to this long, since `agni token revoke` runs in another
// process and can't clear the server's cache.
var CacheTTL = time.Minute

// Store is the store of the server's tokens, inside the --store folder.
func Store(storeDir string) Keys {
	return oafs.NewFSAPIKeyStore(filepath.Join(storeDir, StoreDir))
}

// Gate refuses every request without a valid token, apart from the health check, the viewer's
// static assets, the sign-in pages, GitHub's push webhook and Graph's notifications. It puts the
// token's subject in the request's context as a service.Caller.
type Gate struct {
	store core.APIKeyStore
	mw    *apiauth.APIMiddleware
	// proxies are the proxies in front of the server that end TLS for it (agni issue 1810). A request
	// they pass on as HTTPS gets Secure cookies, as one the server took over TLS itself does.
	proxies skmw.TrustedProxies
	// github signs people in through a GitHub App, and is set by WithGitHub (agni issue 1772).
	github *githubSignIns
	// oidc signs people in through an OpenID Connect provider, and is set by WithOIDC (agni issue 1811).
	oidc *oidcSignIns
	// local signs people in with a local username and password, and is set by WithLocalAccounts (agni
	// issue 1815).
	local *localSignIns
}

// New checks tokens against store. proxies are the proxies that end TLS in front of the server, as
// ParseProxies reads them from --trusted-proxy. Empty trusts no forwarded header, so a cookie is
// Secure only on a request the server took over TLS itself.
func New(store core.APIKeyStore, proxies skmw.TrustedProxies) *Gate {
	g := &Gate{store: store, proxies: proxies}
	g.mw = &apiauth.APIMiddleware{APIKeyStore: store, APIKeyCache: &apiauth.APIKeyCacheConfig{TTL: CacheTTL}}
	rpc := connectauth.OnAuthError(g.mw)
	g.mw.OnAuthError = func(w http.ResponseWriter, r *http.Request, err error) {
		// A person opening a page goes to sign in. A program gets a refusal in its own protocol.
		if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html") {
			http.Redirect(w, r, "/signin?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		rpc(w, r, err)
	}
	return g
}

// Wrap guards next.
func (g *Gate) Wrap(next http.Handler) http.Handler {
	checked := g.mw.ValidateToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := service.Caller{Subject: apiauth.GetSubjectFromAPIContext(r.Context())}
		next.ServeHTTP(w, r.WithContext(service.WithCaller(r.Context(), caller)))
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == AnyDevicePath || r.URL.Path == AnyDeviceTokenPath {
			g.anyDevice(w, r)
			return
		}
		if g.github != nil && g.github.serve(g, w, r) {
			return
		}
		if g.oidc != nil && g.oidc.serve(g, w, r) {
			return
		}
		if g.local != nil && r.URL.Path == LocalSignInPath {
			g.local.serve(g, w, r)
			return
		}
		// A provider's routes on a server that doesn't offer it answer 404, so agni login --with names
		// the missing provider rather than meeting the token check.
		for _, p := range []string{SignInPath, OIDCSignInPath, LocalSignInPath} {
			if r.URL.Path == p || strings.HasPrefix(r.URL.Path, p+"/") {
				http.Error(w, "this server doesn't sign people in that way", http.StatusNotFound)
				return
			}
		}
		switch {
		case r.URL.Path == "/signin":
			g.signIn(w, r)
			return
		case r.URL.Path == "/signout":
			g.signOut(w, r)
			return
		case r.URL.Path == "/healthz", r.URL.Path == "/readyz", strings.HasPrefix(r.URL.Path, "/static/"), r.URL.Path == "/hooks/github", r.URL.Path == "/hooks/msgraph":
			// A load balancer checks health without a token, and the viewer's scripts and styles
			// hold nothing a token protects. GitHub sends a webhook with no token, and its handler
			// checks the delivery's signature instead (agni issue 1726).
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Authorization") == "" {
			if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" {
				r = r.Clone(r.Context())
				r.Header.Set("Authorization", "Bearer "+c.Value)
			}
		}
		checked.ServeHTTP(w, r)
	})
}

var signInPage = template.Must(template.New("signin").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sign in to agni</title>
<style>
:root { color-scheme: light dark; }
body { font: 15px/1.5 system-ui, sans-serif; max-width: 26rem; margin: 4rem auto; padding: 0 16px; }
input { width: 100%; box-sizing: border-box; padding: .5rem; font: inherit; }
button { margin-top: .75rem; padding: .5rem 1rem; font: inherit; }
.error { color: #c0392b; }
a.github { display: inline-block; padding: .5rem 1rem; border: 1px solid currentColor; border-radius: 6px; text-decoration: none; }
</style></head>
<body>
<h1>Sign in to agni</h1>
<p>This server asks who you are.{{if or .GitHub .OIDC .Local}} Sign in below, or paste{{else}} Paste{{end}} the token its operator gave you, made with <code>agni token create</code>.</p>
{{if .Error}}<p class="error" role="alert">{{.Error}}</p>{{end}}
{{if .GitHub}}<p><a class="github" href="/signin/github?next={{.Next}}">Sign in with GitHub</a></p>{{end}}
{{if .OIDC}}<p><a class="github" href="/signin/oidc?next={{.Next}}">Sign in with {{.OIDC}}</a></p>{{end}}
{{if .Local}}<form method="post" action="/signin/local">
<input type="hidden" name="next" value="{{.Next}}">
<label for="username">Username</label>
<input id="username" name="username" autocomplete="username" required autofocus>
<label for="password">Password</label>
<input id="password" name="password" type="password" autocomplete="current-password" required>
<button type="submit">Sign in</button>
</form>
<p>Or paste a token.</p>{{end}}
<form method="post" action="/signin">
<input type="hidden" name="next" value="{{.Next}}">
<label for="token">Token</label>
<input id="token" name="token" type="password" autocomplete="off" required{{if not .Local}} autofocus{{end}}>
<button type="submit">Sign in</button>
</form>
</body></html>
`))

// signInView is what the sign-in page shows. GitHub offers the GitHub button, a non-empty OIDC
// offers the OIDC provider's button under that label, and Local offers the username and password
// form.
type signInView struct {
	Next, Error string
	GitHub      bool
	OIDC        string
	Local       bool
}

// signInView is the sign-in page for this gate's providers, going on to next.
func (g *Gate) signInView(next string) signInView {
	page := signInView{Next: next, GitHub: g.github != nil, Local: g.local != nil}
	if g.oidc != nil {
		page.OIDC = g.oidc.cfg.Label
	}
	return page
}

// signIn shows the sign-in form, and on a post checks the token and keeps it in a session cookie.
func (g *Gate) signIn(w http.ResponseWriter, r *http.Request) {
	next := safeNext(r.FormValue("next"))
	page := g.signInView(next)
	if r.URL.Query().Get("github") == "failed" {
		page.Error = "Signing in with GitHub didn't work. Try again, or paste a token."
	}
	if r.URL.Query().Get("oidc") == "failed" {
		page.Error = "Signing in with " + page.OIDC + " didn't work. Try again, or paste a token."
	}
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		token := strings.TrimSpace(r.PostFormValue("token"))
		if _, err := g.store.ValidateAPIKey(r.Context(), &core.ValidateAPIKeyRequest{FullKey: token}); err == nil {
			// Secure is set whenever the browser reached the server over HTTPS, directly or through a
			// trusted proxy. Otherwise the server listens on localhost only (CheckFlags), where a Secure
			// cookie would never come back.
			http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: g.https(r), SameSite: http.SameSiteStrictMode}) // #nosec G124 -- Secure follows HTTPS, as above
			http.Redirect(w, r, next, http.StatusSeeOther)                                                                                                       // #nosec G710 -- next came through safeNext, so it is a path on this server
			return
		}
		page.Error = "That token isn't one this server knows, or it was revoked."
		w.WriteHeader(http.StatusUnauthorized)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = signInPage.Execute(w, page)
}

// signOut forgets the browser's session and goes back to the sign-in form.
func (g *Gate) signOut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: g.https(r), SameSite: http.SameSiteStrictMode}) // #nosec G124 -- Secure follows HTTPS, as in signIn
	http.Redirect(w, r, "/signin", http.StatusSeeOther)
}

// https says the browser reached the server over HTTPS: the server took the request over TLS, or a
// trusted proxy took it over TLS and says so in X-Forwarded-Proto or Forwarded. A forwarded header
// from any other peer is ignored, so a client can't claim HTTPS for itself.
func (g *Gate) https(r *http.Request) bool { return g.proxies.IsHTTPS(r) }

// safeNext is where a sign-in goes on to: a path on this server, or "/". A full URL or a
// scheme-relative "//host" would send a person off the server.
func safeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/"
	}
	return next
}

// ParseProxies reads --trusted-proxy, each an address or a CIDR range such as 10.0.0.0/8.
// servicekit's middleware owns the type, so every service on servicekit reads forwarding headers the
// same way (panyam/servicekit#67).
func ParseProxies(entries []string) (skmw.TrustedProxies, error) {
	out, err := skmw.ParseTrustedProxies(entries)
	if err != nil {
		return nil, fmt.Errorf("--trusted-proxy: %w", err)
	}
	return out, nil
}

// CheckFlags refuses a combination of --auth and the TLS flags that would serve tokens badly. A
// server with trusted proxies may serve plain HTTP on any address, since the proxies end TLS in front
// of it (agni issue 1810).
func CheckFlags(auth, store, tlsCert, tlsKey, addr string, proxies skmw.TrustedProxies) error {
	if (tlsCert == "") != (tlsKey == "") {
		return fmt.Errorf("--tls-cert and --tls-key go together; pass both or neither")
	}
	switch auth {
	case "":
		return nil
	case Tokens:
	default:
		return fmt.Errorf("--auth %q: the one kind this build checks is %q", auth, Tokens)
	}
	if store == "" {
		return fmt.Errorf("--auth %s needs --store, where the tokens agni token create makes are kept", Tokens)
	}
	// servicekit's startup check decides, and the refusal names agni's flags.
	switch err := skmw.CheckPlaintext(addr, tlsCert != "", proxies); {
	case errors.Is(err, skmw.ErrPlaintextCredentials):
		return fmt.Errorf("--auth %s on %s would send tokens over the network in the clear; pass --tls-cert and --tls-key, name the proxy that ends TLS in front of the server with --trusted-proxy, or listen on localhost only with --addr localhost:8080", Tokens, addr)
	case err != nil:
		return fmt.Errorf("--addr %q: %w", addr, err)
	}
	return nil
}

// Prune deletes the tokens whose expiry is at or before now, and returns how many it deleted (agni
// issue 1778). Each GitHub sign-in mints a token that expires, so without this the store keeps one
// record per sign-in for good. A token with no expiry, as `agni token create` makes, stays, and so
// does a revoked one, which `agni token ls` still lists.
func Prune(ctx context.Context, store Keys, now time.Time) (int, error) {
	resp, err := store.CleanupExpiredAPIKeys(ctx, &core.CleanupExpiredAPIKeysRequest{Now: now})
	if err != nil {
		return 0, err
	}
	return resp.Removed, nil
}

// List returns every token in the store, newest first, without their hashes.
func List(ctx context.Context, store Keys) ([]*core.APIKey, error) {
	resp, err := store.ListAPIKeys(ctx, &core.ListAPIKeysRequest{})
	if err != nil {
		return nil, err
	}
	out := resp.APIKeys
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
