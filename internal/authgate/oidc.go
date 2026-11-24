package authgate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/panyam/oneauth/core"
	"github.com/panyam/oneauth/oauth2/oidc"
	"golang.org/x/oauth2"
)

// OIDC signs people in through any OpenID Connect provider, such as Entra ID, Okta, Google Workspace
// or Keycloak (agni issue 1811). A browser signs in at /signin/oidc and `agni login --with oidc`
// through the provider's device flow. Either way the server checks the verified ID token's claims
// against Allow and mints an agni token for the subject "oidc:<sub>". It keeps no connection, so a
// person who also needs private repos adds their own with `agni connections add --mine`.
type OIDC struct {
	// Issuer is the provider's issuer URL. The server reads <Issuer>/.well-known/openid-configuration
	// at start, so a wrong issuer fails the start, not the first sign-in.
	Issuer string
	// ClientID and ClientSecret are the client registered with the provider. ClientSecret may be empty
	// for a public client.
	ClientID, ClientSecret string
	// PublicURL is the server's URL as a browser reaches it. The provider sends the browser back to
	// PublicURL + /signin/oidc/callback/, which the client registration must list.
	PublicURL string
	// Allow names who may sign in: "domain:<domain>" for a verified email in a domain, "group:<id>" for
	// a value in the groups claim, "sub:<sub>" for one person, "hd:<domain>" for a Google Workspace
	// domain. It can't be empty, since anyone the provider knows could sign in otherwise.
	Allow []string
	// Label names the provider on the sign-in page's button. Empty is "single sign-on".
	Label string
	// TTL is how long a minted agni token works. Zero is 30 days.
	TTL time.Duration
	// HTTPClient makes the discovery, key, token and device requests. Nil uses oneauth's default.
	HTTPClient *http.Client
}

// OIDCSignInPath is where a browser signs in through the OpenID Connect provider (agni issue 1811).
const OIDCSignInPath = "/signin/oidc"

// Paths of the OIDC device routes. DeviceLogin reaches them through the provider's prefix.
const (
	oidcDevice      = OIDCSignInPath + "/device"
	oidcDeviceToken = OIDCSignInPath + "/device/token" // #nosec G101 -- a URL path, not a credential
)

// oidcSignIns is an OIDC provider the gate serves.
type oidcSignIns struct {
	cfg   OIDC
	keys  core.APIKeyStore
	allow oidc.AllowList
	p     *oidc.Provider
	mu    sync.Mutex
	// devices holds each device sign-in the server started, by its handle, until it ends.
	devices map[string]*oauth2.DeviceAuthResponse
}

// ParseOIDCAllow reads --oidc-allow values into an allow list. Each is domain:, group:, sub: or hd:
// followed by a value, and an empty list is an error.
func ParseOIDCAllow(allow []string) (oidc.AllowList, error) {
	var a oidc.AllowList
	if len(allow) == 0 {
		return a, errors.New("signing in through OIDC needs --oidc-allow, such as domain:example.com or group:<id>, or anyone the provider knows could sign in")
	}
	for _, v := range allow {
		kind, name, ok := strings.Cut(strings.TrimSpace(v), ":")
		if !ok || name == "" {
			return a, fmt.Errorf("--oidc-allow %q: want domain:<domain>, group:<id>, sub:<sub> or hd:<domain>", v)
		}
		switch kind {
		case "domain":
			a.EmailDomains = append(a.EmailDomains, name)
		case "group":
			a.Groups = append(a.Groups, name)
		case "sub":
			a.Subjects = append(a.Subjects, name)
		case "hd":
			a.HostedDomains = append(a.HostedDomains, name)
		default:
			return a, fmt.Errorf("--oidc-allow %q: want domain:<domain>, group:<id>, sub:<sub> or hd:<domain>", v)
		}
	}
	return a, nil
}

// WithOIDC lets people sign in through cfg's provider, and returns g. It reads the provider's
// discovery document now, and refuses a cfg with no client, no PublicURL or an empty Allow.
func (g *Gate) WithOIDC(ctx context.Context, cfg OIDC) (*Gate, error) {
	if cfg.Issuer == "" || cfg.ClientID == "" {
		return nil, errors.New("signing in through OIDC needs --oidc-issuer and --oidc-client-id")
	}
	allow, err := ParseOIDCAllow(cfg.Allow)
	if err != nil {
		return nil, err
	}
	pub, err := url.Parse(cfg.PublicURL)
	if err != nil || pub.Host == "" || pub.Scheme != "https" && pub.Scheme != "http" {
		return nil, fmt.Errorf("signing in through OIDC needs the server's public URL, such as https://agni.example.com; got %q", cfg.PublicURL)
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 30 * 24 * time.Hour
	}
	if cfg.Label == "" {
		cfg.Label = "single sign-on"
	}
	s := &oidcSignIns{cfg: cfg, keys: g.store, allow: allow, devices: map[string]*oauth2.DeviceAuthResponse{}}
	s.p, err = oidc.New(ctx, oidc.Config{
		Issuer: cfg.Issuer, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret,
		CallbackURL:    strings.TrimSuffix(cfg.PublicURL, "/") + OIDCSignInPath + "/callback/",
		ProviderName:   "oidc",
		HandleUser:     s.signedIn,
		AuthFailureURL: "/signin?oidc=failed",
		HTTPClient:     cfg.HTTPClient,
	})
	if err != nil {
		return nil, fmt.Errorf("--oidc-issuer %s: %w", cfg.Issuer, err)
	}
	g.oidc = s
	return g, nil
}

// serve answers the OIDC sign-in routes. It reports false for a path that isn't one.
func (s *oidcSignIns) serve(g *Gate, w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.URL.Path == oidcDevice:
		s.deviceStart(w, r)
	case r.URL.Path == oidcDeviceToken:
		s.deviceToken(w, r)
	case r.URL.Path == OIDCSignInPath:
		http.SetCookie(w, &http.Cookie{Name: nextCookie, Value: safeNext(r.URL.Query().Get("next")), Path: OIDCSignInPath, MaxAge: 600, HttpOnly: true, Secure: g.https(r), SameSite: http.SameSiteLaxMode}) // #nosec G124 -- Secure follows HTTPS, as in signIn
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		s.p.Handler().ServeHTTP(w, r2)
	case strings.HasPrefix(r.URL.Path, OIDCSignInPath+"/callback/"):
		http.StripPrefix(OIDCSignInPath, s.p.Handler()).ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), gateKey{}, g)))
	default:
		return false
	}
	return true
}

// signedIn is oneauth's HandleUser. It runs once the provider's ID token has verified, with every
// claim of that token in info.
func (s *oidcSignIns) signedIn(_, _ string, _ *oauth2.Token, info map[string]any, w http.ResponseWriter, r *http.Request) {
	g, _ := r.Context().Value(gateKey{}).(*Gate)
	full, _, err := s.admit(r.Context(), info)
	if err != nil || g == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusForbidden)
		msg := "Signing in didn't work."
		if err != nil {
			msg = err.Error()
		}
		_ = signInPage.Execute(w, signInView{Next: "/", Error: msg, OIDC: s.cfg.Label})
		return
	}
	next := "/"
	if c, err := r.Cookie(nextCookie); err == nil {
		next = safeNext(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: nextCookie, Value: "", Path: OIDCSignInPath, MaxAge: -1, HttpOnly: true, Secure: g.https(r), SameSite: http.SameSiteLaxMode}) // #nosec G124 -- as above
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: full, Path: "/", HttpOnly: true, Secure: g.https(r), SameSite: http.SameSiteLaxMode})                   // #nosec G124 -- as above
	http.Redirect(w, r, next, http.StatusSeeOther)                                                                                                                     // #nosec G710 -- next came through safeNext
}

// admit checks a verified ID token's claims against Allow and mints an agni token for "oidc:<sub>",
// carrying each value of the groups claim as a claim. It returns the token and its subject.
func (s *oidcSignIns) admit(ctx context.Context, claims map[string]any) (string, string, error) {
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", "", errors.New("the identity provider didn't say who signed in")
	}
	who := sub
	if email, _ := claims["email"].(string); email != "" {
		who = email
	}
	d := s.allow.Decide(claims)
	if !d.Allowed {
		if d.GroupsOverage {
			return "", "", fmt.Errorf("%s is in too many groups for the identity provider to list them in the sign-in, so --oidc-allow group: can't be checked; ask the operator to allow them by domain or subject", who)
		}
		return "", "", fmt.Errorf("%s isn't someone this server lets sign in; ask its operator to add them to --oidc-allow", who)
	}
	subject := "oidc:" + sub
	expires := time.Now().Add(s.cfg.TTL)
	groupsClaim := s.allow.GroupsClaim
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	// The groups ride on the token, so a grant of oidc:group:<id> admits this person (agni issue
	// 1896). A provider that left the claim out for overage gives none.
	resp, err := s.keys.CreateAPIKey(ctx, &core.CreateAPIKeyRequest{Subject: subject, Name: "oidc sign-in " + who, Scopes: groupClaims(claims[groupsClaim]), ExpiresAt: &expires})
	if err != nil {
		return "", "", fmt.Errorf("minting a token for %s: %w", who, err)
	}
	return resp.FullKey, subject, nil
}

// deviceStart asks the provider for a device code and keeps it under a fresh handle, so the CLI
// holds nothing it could trade for a token at the provider itself.
func (s *oidcSignIns) deviceStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	da, err := s.p.DeviceAuth(r.Context())
	if errors.Is(err, oidc.ErrNoDeviceEndpoint) {
		http.Error(w, "the identity provider offers no device sign-in, so agni login can't use it; sign in at /signin in a browser instead", http.StatusNotImplemented)
		return
	}
	if err != nil {
		http.Error(w, "asking the identity provider for a device code: "+err.Error(), http.StatusBadGateway)
		return
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	handle := hex.EncodeToString(b)
	s.mu.Lock()
	for h, d := range s.devices {
		if time.Now().After(d.Expiry) {
			delete(s.devices, h)
		}
	}
	s.devices[handle] = da
	s.mu.Unlock()
	interval := int(da.Interval)
	if interval <= 0 {
		interval = 5
	}
	uri := da.VerificationURIComplete
	if uri == "" {
		uri = da.VerificationURI
	}
	writeJSON(w, http.StatusOK, DeviceStart{UserCode: da.UserCode, VerificationURI: uri, Interval: interval, ExpiresIn: int(time.Until(da.Expiry).Seconds()), Handle: handle})
}

// deviceToken waits up to devicePoll for the person to approve the device code under the posted
// handle. Once they do, it admits them as the browser flow does and answers with their agni token.
func (s *oidcSignIns) deviceToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Handle string `json:"handle"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		http.Error(w, "want {\"handle\": ...}", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	da := s.devices[req.Handle]
	s.mu.Unlock()
	if da == nil {
		http.Error(w, "no sign-in under that handle; it ended or expired, so run agni login again", http.StatusNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), devicePoll)
	defer cancel()
	_, claims, err := s.p.DeviceAccessToken(ctx, da)
	if err != nil {
		if ctx.Err() != nil && r.Context().Err() == nil {
			writeJSON(w, http.StatusAccepted, DeviceToken{Pending: true})
			return
		}
		s.forget(req.Handle)
		http.Error(w, "the identity provider refused the sign-in: "+err.Error(), http.StatusForbidden)
		return
	}
	s.forget(req.Handle)
	full, subject, err := s.admit(r.Context(), claims.Claims)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	writeJSON(w, http.StatusOK, DeviceToken{Token: full, Subject: subject})
}

func (s *oidcSignIns) forget(handle string) {
	s.mu.Lock()
	delete(s.devices, handle)
	s.mu.Unlock()
}
