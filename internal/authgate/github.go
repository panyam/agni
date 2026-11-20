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
	oagithub "github.com/panyam/oneauth/oauth2"
	"golang.org/x/oauth2"
	ghendpoint "golang.org/x/oauth2/github"
)

// GitHub signs people in through a GitHub App (agni issue 1772). A browser signs in at
// /signin/github, through GitHub's web flow, and `agni login` through its device flow. Either way
// the server checks the person against Allow, mints them an agni token for the subject
// "github:<login>", and hands their GitHub token to Save, which keeps it as their own connection.
type GitHub struct {
	// ClientID and ClientSecret are the GitHub App's OAuth client.
	ClientID, ClientSecret string
	// API is the GitHub REST API the server reads through, the server's --github-api. Empty is
	// api.github.com, and sign-in goes through github.com.
	API string
	// PublicURL is the server's URL as a browser reaches it, such as https://agni.example.com.
	// GitHub sends the browser back to PublicURL + /signin/github/callback/.
	PublicURL string
	// Allow names who may sign in, each "org:<org>" for the members of an org or "user:<login>".
	// It can't be empty, since anyone with a GitHub account could sign in otherwise.
	Allow []string
	// Save keeps the GitHub token of the person signed in as login, as subject's own connection.
	Save func(ctx context.Context, subject, login string, tok *oauth2.Token) error
	// TTL is how long a minted agni token works. Zero is 30 days.
	TTL time.Duration
	// HTTPClient makes the calls to GitHub. Nil is http.DefaultClient.
	HTTPClient *http.Client
}

// SignInPath is where a browser signs in with GitHub, on a gate that has one (agni issue 1772). A
// page links here with ?next= naming where to land after.
const SignInPath = "/signin/github"

// Paths of the other sign-in routes the gate serves when it has a GitHub.
const (
	githubDevice      = "/signin/github/device"
	githubDeviceToken = "/signin/github/device/token" // #nosec G101 -- a URL path, not a credential
	nextCookie        = "agni_signin_next"
)

// DevicePath and DeviceTokenPath are where `agni login` starts and finishes GitHub's device flow.
const (
	DevicePath      = githubDevice
	DeviceTokenPath = githubDeviceToken
)

// DeviceStart is the server's answer to a POST to DevicePath. The person opens VerificationURI and
// types UserCode. The CLI then posts Handle to DeviceTokenPath until the server answers with a
// token.
type DeviceStart struct {
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	Interval        int    `json:"interval"`
	ExpiresIn       int    `json:"expires_in"`
	Handle          string `json:"handle"`
}

// DeviceToken is the server's answer to a POST to DeviceTokenPath. Pending says GitHub is still
// waiting for the person. Otherwise Token is the agni token minted for Subject.
type DeviceToken struct {
	Pending bool   `json:"pending,omitempty"`
	Token   string `json:"token,omitempty"`
	Subject string `json:"subject,omitempty"`
}

// githubSignIns is a GitHub the gate serves.
type githubSignIns struct {
	cfg  GitHub
	keys core.APIKeyStore
	web  *oagithub.GithubOAuth2
	app  oauth2.Config
	api  string
	// devices holds each device flow the server started, by its handle, until it ends.
	mu      sync.Mutex
	devices map[string]*oauth2.DeviceAuthResponse
}

// WithGitHub lets people sign in through cfg's GitHub App, and returns g. It refuses a cfg with no
// client, no PublicURL or an empty Allow.
func (g *Gate) WithGitHub(cfg GitHub) (*Gate, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("signing in with GitHub needs the GitHub App's client id and secret")
	}
	if len(cfg.Allow) == 0 {
		return nil, errors.New("signing in with GitHub needs --github-allow, such as org:acme, or anyone with a GitHub account could sign in")
	}
	for _, a := range cfg.Allow {
		kind, name, ok := strings.Cut(a, ":")
		if !ok || name == "" || kind != "org" && kind != "user" {
			return nil, fmt.Errorf("--github-allow %q: want org:<org> or user:<login>", a)
		}
	}
	pub, err := url.Parse(cfg.PublicURL)
	if err != nil || pub.Host == "" || pub.Scheme != "https" && pub.Scheme != "http" {
		return nil, fmt.Errorf("signing in with GitHub needs the server's public URL, such as https://agni.example.com; got %q", cfg.PublicURL)
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 30 * 24 * time.Hour
	}
	s := &githubSignIns{cfg: cfg, keys: g.store, api: strings.TrimSuffix(cfg.API, "/"), devices: map[string]*oauth2.DeviceAuthResponse{}}
	if s.api == "" {
		s.api = "https://api.github.com"
	}
	callback := strings.TrimSuffix(cfg.PublicURL, "/") + SignInPath + "/callback/"
	s.web = oagithub.NewGithubOAuth2(cfg.ClientID, cfg.ClientSecret, callback, s.signedIn)
	s.web.AuthFailureUrl = "/signin?github=failed"
	// A GitHub App's user token gets its access from the App's permissions, not from scopes.
	s.web.SetScopes()
	s.app = oauth2.Config{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Endpoint: GitHubEndpoint(cfg.API)}
	if host := webHost(cfg.API); host != "" {
		if err := s.web.SetEnterpriseURL(host); err != nil {
			return nil, err
		}
		s.web.UserInfoURL = s.api + "/user"
	}
	if cfg.HTTPClient != nil {
		s.web.SetHTTPClient(cfg.HTTPClient)
	}
	g.github = s
	return g, nil
}

// GitHubEndpoint is the OAuth endpoint that signs people in for the GitHub REST API at api, the
// server's --github-api. goapplib refreshes a signed-in person's token through it too.
func GitHubEndpoint(api string) oauth2.Endpoint {
	host := webHost(api)
	if host == "" {
		return ghendpoint.Endpoint
	}
	return oauth2.Endpoint{AuthURL: host + "/login/oauth/authorize", TokenURL: host + "/login/oauth/access_token", DeviceAuthURL: host + "/login/device/code"}
}

// webHost is the GitHub host that signs people in for the REST API at api, or "" for github.com.
// GitHub Enterprise Server serves its API under /api/v3, and a GHE.com host at api.<host>. Any other
// API, such as a test's fake, serves sign-in on its own host.
func webHost(api string) string {
	api = strings.TrimSuffix(api, "/")
	if api == "" || api == "https://api.github.com" {
		return ""
	}
	if host, ok := strings.CutSuffix(api, "/api/v3"); ok {
		return host
	}
	if u, err := url.Parse(api); err == nil && strings.HasPrefix(u.Host, "api.") && strings.HasSuffix(u.Host, ".ghe.com") {
		return u.Scheme + "://" + strings.TrimPrefix(u.Host, "api.")
	}
	return api
}

// serve answers the sign-in routes. It reports false for a path that isn't one.
func (s *githubSignIns) serve(g *Gate, w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.URL.Path == githubDevice:
		s.deviceStart(w, r)
	case r.URL.Path == githubDeviceToken:
		s.deviceToken(w, r)
	case r.URL.Path == SignInPath:
		// Where to go after GitHub sends the browser back, kept across the round trip.
		http.SetCookie(w, &http.Cookie{Name: nextCookie, Value: safeNext(r.URL.Query().Get("next")), Path: SignInPath, MaxAge: 600, HttpOnly: true, Secure: g.secure, SameSite: http.SameSiteLaxMode}) // #nosec G124 -- Secure follows TLS, as in signIn
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		s.web.Handler().ServeHTTP(w, r2)
	case strings.HasPrefix(r.URL.Path, SignInPath+"/callback/"):
		http.StripPrefix(SignInPath, s.web.Handler()).ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), gateKey{}, g)))
	default:
		return false
	}
	return true
}

type gateKey struct{}

// signedIn is oneauth's HandleUser. It runs once GitHub has signed the browser in and the server
// has read the person's profile.
func (s *githubSignIns) signedIn(_, _ string, tok *oauth2.Token, info map[string]any, w http.ResponseWriter, r *http.Request) {
	g, _ := r.Context().Value(gateKey{}).(*Gate)
	login, _ := info["login"].(string)
	full, _, err := s.admit(r.Context(), login, tok)
	if err != nil || g == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusForbidden)
		msg := "Signing in with GitHub didn't work."
		if err != nil {
			msg = err.Error()
		}
		_ = signInPage.Execute(w, signInView{Next: "/", Error: msg, GitHub: true})
		return
	}
	next := "/"
	if c, err := r.Cookie(nextCookie); err == nil {
		next = safeNext(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: nextCookie, Value: "", Path: SignInPath, MaxAge: -1, HttpOnly: true, Secure: g.secure, SameSite: http.SameSiteLaxMode}) // #nosec G124 -- as above
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: full, Path: "/", HttpOnly: true, Secure: g.secure, SameSite: http.SameSiteLaxMode})                 // #nosec G124 -- as above
	http.Redirect(w, r, next, http.StatusSeeOther)                                                                                                                 // #nosec G710 -- next came through safeNext
}

// admit checks the person signed in as login against Allow, keeps their GitHub token, and mints
// them an agni token. It returns the token and its subject.
func (s *githubSignIns) admit(ctx context.Context, login string, tok *oauth2.Token) (string, string, error) {
	if login == "" || tok == nil {
		return "", "", errors.New("GitHub didn't say who signed in")
	}
	ok, err := s.allowed(ctx, login, tok)
	if err != nil {
		return "", "", fmt.Errorf("asking GitHub whether %s may sign in: %w", login, err)
	}
	if !ok {
		return "", "", fmt.Errorf("%s isn't someone this server lets sign in; ask its operator to add them to --github-allow", login)
	}
	subject := "github:" + login
	if err := s.cfg.Save(ctx, subject, login, tok); err != nil {
		return "", "", fmt.Errorf("keeping %s's GitHub token: %w", login, err)
	}
	expires := time.Now().Add(s.cfg.TTL)
	resp, err := s.keys.CreateAPIKey(ctx, &core.CreateAPIKeyRequest{Subject: subject, Name: "github sign-in", ExpiresAt: &expires})
	if err != nil {
		return "", "", fmt.Errorf("minting a token for %s: %w", login, err)
	}
	return resp.FullKey, subject, nil
}

// allowed says whether Allow lets login in. An org entry asks GitHub, with the person's own token,
// whether login is a member.
func (s *githubSignIns) allowed(ctx context.Context, login string, tok *oauth2.Token) (bool, error) {
	var orgs []string
	for _, a := range s.cfg.Allow {
		kind, name, _ := strings.Cut(a, ":")
		if kind == "user" && strings.EqualFold(name, login) {
			return true, nil
		}
		if kind == "org" {
			orgs = append(orgs, name)
		}
	}
	for _, org := range orgs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.api+"/orgs/"+url.PathEscape(org)+"/members/"+url.PathEscape(login), nil)
		if err != nil {
			return false, err
		}
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := s.client().Do(req)
		if err != nil {
			return false, err
		}
		resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusNoContent:
			return true, nil
		case http.StatusNotFound, http.StatusFound:
		default:
			return false, fmt.Errorf("GitHub answered %d about %s's membership of %s", resp.StatusCode, login, org)
		}
	}
	return false, nil
}

func (s *githubSignIns) client() *http.Client {
	if s.cfg.HTTPClient != nil {
		return s.cfg.HTTPClient
	}
	return http.DefaultClient
}

func (s *githubSignIns) oauthCtx(ctx context.Context) context.Context {
	if s.cfg.HTTPClient != nil {
		return context.WithValue(ctx, oauth2.HTTPClient, s.cfg.HTTPClient)
	}
	return ctx
}

// deviceStart asks GitHub for a device code and keeps it under a fresh handle. The device code
// stays on the server, so the CLI holds nothing it could trade for a GitHub token itself.
func (s *githubSignIns) deviceStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	da, err := s.app.DeviceAuth(s.oauthCtx(r.Context()))
	if err != nil {
		http.Error(w, "asking GitHub for a device code: "+err.Error(), http.StatusBadGateway)
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
	writeJSON(w, http.StatusOK, DeviceStart{UserCode: da.UserCode, VerificationURI: da.VerificationURI, Interval: interval, ExpiresIn: int(time.Until(da.Expiry).Seconds()), Handle: handle})
}

// devicePoll is how long one POST to DeviceTokenPath waits on GitHub before it answers pending.
var devicePoll = 25 * time.Second

// deviceToken waits up to devicePoll for the person to approve the device code under the posted
// handle. Once they do, it admits them as the web flow does and answers with their agni token.
func (s *githubSignIns) deviceToken(w http.ResponseWriter, r *http.Request) {
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
	tok, err := s.app.DeviceAccessToken(s.oauthCtx(ctx), da)
	if err != nil {
		if ctx.Err() != nil && r.Context().Err() == nil {
			writeJSON(w, http.StatusAccepted, DeviceToken{Pending: true})
			return
		}
		s.forget(req.Handle)
		http.Error(w, "GitHub refused the sign-in: "+err.Error(), http.StatusForbidden)
		return
	}
	s.forget(req.Handle)
	login, err := s.login(r.Context(), tok)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	full, subject, err := s.admit(r.Context(), login, tok)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	writeJSON(w, http.StatusOK, DeviceToken{Token: full, Subject: subject})
}

func (s *githubSignIns) forget(handle string) {
	s.mu.Lock()
	delete(s.devices, handle)
	s.mu.Unlock()
}

// login reads who tok belongs to from GitHub's /user.
func (s *githubSignIns) login(ctx context.Context, tok *oauth2.Token) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.api+"/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := s.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("asking GitHub who signed in: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("asking GitHub who signed in: %s", resp.Status)
	}
	var u struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil || u.Login == "" {
		return "", fmt.Errorf("GitHub didn't say who signed in")
	}
	return u.Login, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
