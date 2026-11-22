package main

// The --seed mode serves a GitHub Enterprise Server for make exercise-enterprise (agni issue 1800).
// goapplib's fake answers the REST API under /api/v3 as a GHES host does, and sign-in and raw files
// at its root. So agni, given --github-api <host>/api/v3, finds sign-in at <host>, as on a real GHES.
//
// The seed file is JSON:
//
//	{
//	  "app":    {"client_id": "Iv1.exercise", "client_secret": "..."},
//	  "users":  [{"login": "alice", "orgs": ["acme"]}],
//	  "tokens": ["server-token"],
//	  "repos":  [{"repo": "hw/baseboard", "private": true, "readers": ["alice"],
//	              "tokens": ["server-token"], "commits": [{"tag": "rev-b", "dir": "rev-b"}]}]
//	}
//
// Each commit replaces the files on main with dir's, and tags the commit when tag is set. A
// relative dir is read beside the seed file. readers are the logins a repo is granted to, and
// tokens are fixed tokens it is granted to, such as a server connection's.
//
// The control address answers three requests, so a script can play GitHub's other side:
//
//	GET  /         {"web": ..., "api": ".../api/v3", "raw": ".../raw", "commits": {"hw/baseboard": {"rev-b": "<sha>"}}}
//	POST /approve  login=alice&code=ABCD-1234: alice types the device code and approves the App
//	POST /push     repo=...&branch=main&dir=...&hook=<url>&secret=<secret>: commit dir's files to
//	               branch, then deliver GitHub's signed push event to hook
//
// /push answers {"sha": "<new commit>", "status": <the hook's HTTP status>}.

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/panyam/goapplib/storage/mounts/github/githubtest"
)

type seedFile struct {
	App struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"app"`
	Users []struct {
		Login string   `json:"login"`
		Orgs  []string `json:"orgs"`
	} `json:"users"`
	Tokens []string   `json:"tokens"`
	Repos  []seedRepo `json:"repos"`
}

type seedRepo struct {
	Repo    string   `json:"repo"`
	Private bool     `json:"private"`
	Readers []string `json:"readers"`
	Tokens  []string `json:"tokens"`
	Commits []struct {
		Tag string `json:"tag"`
		Dir string `json:"dir"`
	} `json:"commits"`
}

// enterprise is a seeded fake behind its GHES front.
type enterprise struct {
	gh      *githubtest.Server
	web     string // the fake's root, as a GHES host's
	mu      sync.Mutex
	repos   map[string]*githubtest.Repo
	files   map[string]map[string]string // each repo's files on main, so a push can delete what's gone
	commits map[string]map[string]string // repo -> tag -> sha
	users   map[string]*githubtest.User
	hooks   *http.Client
}

// seedEnterprise reads the seed file at path and builds the fake it describes.
func seedEnterprise(path string) (*enterprise, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- a test tool reads the seed file its caller names
	if err != nil {
		return nil, err
	}
	var s seedFile
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	e := &enterprise{gh: githubtest.New(process{}), repos: map[string]*githubtest.Repo{},
		files: map[string]map[string]string{}, commits: map[string]map[string]string{}, users: map[string]*githubtest.User{}, hooks: http.DefaultClient}
	if s.App.ClientID != "" {
		e.gh.AddApp(s.App.ClientID, s.App.ClientSecret)
	}
	for _, u := range s.Users {
		e.users[strings.ToLower(u.Login)] = e.gh.AddUser(u.Login, u.Orgs...)
	}
	for _, tok := range s.Tokens {
		e.gh.AddToken(tok)
	}
	// A device-flow client polls at the fake's interval, so a short one keeps a sign-in quick.
	e.gh.SetDeviceInterval(200 * time.Millisecond)
	base := filepath.Dir(path)
	for _, sr := range s.Repos {
		owner, name, ok := strings.Cut(sr.Repo, "/")
		if !ok || len(sr.Commits) == 0 {
			return nil, fmt.Errorf("%s: repo %q wants owner/name and at least one commit", path, sr.Repo)
		}
		e.commits[sr.Repo] = map[string]string{}
		for _, c := range sr.Commits {
			dir := c.Dir
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(base, dir)
			}
			files, err := read(dir)
			if err != nil {
				return nil, err
			}
			sha := e.commit(sr, owner, name, files)
			if c.Tag != "" {
				e.repos[sr.Repo].Tag(c.Tag, sha, false)
				e.commits[sr.Repo][c.Tag] = sha
			}
		}
	}
	return e, nil
}

// commit replaces main's files in repo with files, making the repo on its first commit, and
// returns the new commit.
func (e *enterprise) commit(sr seedRepo, owner, name string, files map[string]string) string {
	r := e.repos[sr.Repo]
	if r == nil {
		r = e.gh.AddRepo(owner, name, sr.Private, files)
		r.GrantUser(sr.Readers...)
		r.Grant(sr.Tokens...)
		e.repos[sr.Repo] = r
	} else {
		var gone []string
		for p := range e.files[sr.Repo] {
			if _, kept := files[p]; !kept {
				gone = append(gone, p)
			}
		}
		r.Commit("main", files, gone)
	}
	e.files[sr.Repo] = files
	return r.Head("main")
}

// control answers the control address's three requests.
func (e *enterprise) control() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		e.mu.Lock()
		defer e.mu.Unlock()
		writeJSON(w, map[string]any{"web": e.web, "api": e.web + "/api/v3", "raw": e.web + "/raw", "commits": e.commits})
	})
	mux.HandleFunc("POST /approve", func(w http.ResponseWriter, r *http.Request) {
		login, code := r.FormValue("login"), r.FormValue("code")
		u := e.users[strings.ToLower(login)]
		if u == nil || !u.Approve(code) {
			http.Error(w, fmt.Sprintf("no pending device code %q for %q", code, login), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /push", func(w http.ResponseWriter, r *http.Request) {
		repo, branch, dir, hook := r.FormValue("repo"), r.FormValue("branch"), r.FormValue("dir"), r.FormValue("hook")
		e.mu.Lock()
		gr := e.repos[repo]
		e.mu.Unlock()
		if gr == nil || branch != "main" {
			http.Error(w, fmt.Sprintf("no repo %q, or branch %q isn't main", repo, branch), http.StatusNotFound)
			return
		}
		files, err := read(dir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		owner, name, _ := strings.Cut(repo, "/")
		e.mu.Lock()
		sha := e.commit(seedRepo{Repo: repo}, owner, name, files)
		e.mu.Unlock()
		status := 0
		if hook != "" {
			req := gr.PushRequest(hook, []byte(r.FormValue("secret")), branch)
			resp, err := e.hooks.Do(req) // #nosec G704 -- a test tool delivers to the hook its caller names
			if err != nil {
				http.Error(w, "delivering the push: "+err.Error(), http.StatusBadGateway)
				return
			}
			_ = resp.Body.Close()
			status = resp.StatusCode
		}
		writeJSON(w, map[string]any{"sha": sha, "status": status})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// serveEnterprise seeds the fake from seed, serves the control requests on addr, and returns only
// when serving fails. The fake itself listens where goapplib's githubtest puts it, and GET / on the
// control address names it.
func serveEnterprise(seed, addr, hookCA string) error {
	e, err := seedEnterprise(seed)
	if err != nil {
		return err
	}
	if hookCA != "" {
		pem, err := os.ReadFile(hookCA) // #nosec G304 -- the CA file the caller names
		if err != nil {
			return err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return fmt.Errorf("--hook-ca %s holds no certificate", hookCA)
		}
		e.hooks = &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}
	}
	e.web = e.gh.URL
	cl, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "fakegithub: an Enterprise Server at %s, answering at http://%s\n", e.web, cl.Addr())
	return (&http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: e.control()}).Serve(cl)
}
