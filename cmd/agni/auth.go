package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/panyam/oneauth/apiauth"
	"github.com/panyam/oneauth/core"
	"github.com/panyam/oneauth/grpc/connectauth"
	oafs "github.com/panyam/oneauth/stores/fs"

	"github.com/panyam/agni/service"
)

// authTokens is --auth's one value today: callers present a token `agni token create` made (agni
// issue 1627). An identity provider joins it with agni issue 1176.
const authTokens = "tokens"

// storeTokensDir holds the server's tokens under --store, each as a bcrypt hash and never the
// token itself.
const storeTokensDir = "tokens"

// sessionCookie carries a browser's token once it signs in at /signin, since a page can't send an
// Authorization header.
const sessionCookie = "agni_session"

// authCacheTTL is how long the server trusts a token it has checked before checking it again. A
// revoked token keeps working for up to this long, since `agni token revoke` runs in another
// process and can't clear the server's cache.
var authCacheTTL = time.Minute

// tokenStore is the store of the server's tokens, inside the --store folder.
func tokenStore(storeDir string) *oafs.FSAPIKeyStore {
	return oafs.NewFSAPIKeyStore(filepath.Join(storeDir, storeTokensDir))
}

// authGate refuses every request without a valid token, apart from the health check, the viewer's
// static assets and the sign-in pages. It puts the token's subject in the request's context as a
// service.Caller.
type authGate struct {
	store  core.APIKeyStore
	mw     *apiauth.APIMiddleware
	secure bool // the server speaks TLS, so the session cookie is marked Secure
}

// newAuthGate checks tokens against store. secure says the server speaks TLS.
func newAuthGate(store core.APIKeyStore, secure bool) *authGate {
	g := &authGate{store: store, secure: secure}
	g.mw = &apiauth.APIMiddleware{APIKeyStore: store, APIKeyCache: &apiauth.APIKeyCacheConfig{TTL: authCacheTTL}}
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

// wrap guards next.
func (g *authGate) wrap(next http.Handler) http.Handler {
	checked := g.mw.ValidateToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := service.Caller{Subject: apiauth.GetSubjectFromAPIContext(r.Context())}
		next.ServeHTTP(w, r.WithContext(service.WithCaller(r.Context(), caller)))
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/signin":
			g.signIn(w, r)
			return
		case r.URL.Path == "/signout":
			g.signOut(w, r)
			return
		case r.URL.Path == "/healthz", strings.HasPrefix(r.URL.Path, "/static/"):
			// A load balancer checks health without a token, and the viewer's scripts and styles
			// hold nothing a token protects.
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Authorization") == "" {
			if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
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
</style></head>
<body>
<h1>Sign in to agni</h1>
<p>This server asks who you are. Paste the token its operator gave you, made with <code>agni token create</code>.</p>
{{if .Error}}<p class="error" role="alert">{{.Error}}</p>{{end}}
<form method="post" action="/signin">
<input type="hidden" name="next" value="{{.Next}}">
<label for="token">Token</label>
<input id="token" name="token" type="password" autocomplete="off" required autofocus>
<button type="submit">Sign in</button>
</form>
</body></html>
`))

// signIn shows the sign-in form, and on a post checks the token and keeps it in a session cookie.
func (g *authGate) signIn(w http.ResponseWriter, r *http.Request) {
	next := safeNext(r.FormValue("next"))
	page := struct{ Next, Error string }{Next: next}
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		token := strings.TrimSpace(r.PostFormValue("token"))
		if _, err := g.store.ValidateAPIKey(r.Context(), &core.ValidateAPIKeyRequest{FullKey: token}); err == nil {
			http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: g.secure, SameSite: http.SameSiteStrictMode})
			http.Redirect(w, r, next, http.StatusSeeOther)
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
func (g *authGate) signOut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: g.secure, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/signin", http.StatusSeeOther)
}

// safeNext is where a sign-in goes on to: a path on this server, or "/". A full URL or a
// scheme-relative "//host" would send a person off the server.
func safeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/"
	}
	return next
}

// checkAuthFlags refuses a combination of --auth and the TLS flags that would serve tokens badly.
func checkAuthFlags(auth, storeDir, tlsCert, tlsKey, addr string) error {
	if (tlsCert == "") != (tlsKey == "") {
		return fmt.Errorf("--tls-cert and --tls-key go together; pass both or neither")
	}
	switch auth {
	case "":
		return nil
	case authTokens:
	default:
		return fmt.Errorf("--auth %q: the one kind this build checks is %q", auth, authTokens)
	}
	if storeDir == "" {
		return fmt.Errorf("--auth %s needs --store, where the tokens agni token create makes are kept", authTokens)
	}
	if tlsCert == "" && !loopbackAddr(addr) {
		return fmt.Errorf("--auth %s on %s would send tokens over the network in the clear; pass --tls-cert and --tls-key, or listen on localhost only with --addr localhost:8080", authTokens, addr)
	}
	return nil
}

// loopbackAddr says whether addr listens on this machine only.
func loopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// listTokens returns every token in the store, newest first, without their hashes.
func listTokens(ctx context.Context, store *oafs.FSAPIKeyStore) ([]*core.APIKey, error) {
	entries, err := os.ReadDir(filepath.Join(store.StoragePath, "api_keys"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*core.APIKey
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok {
			continue
		}
		resp, err := store.GetAPIKeyByID(ctx, &core.GetAPIKeyByIDRequest{KeyID: id})
		if err != nil {
			return nil, err
		}
		k := *resp.APIKey
		k.KeyHash = ""
		out = append(out, &k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
