package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSeed writes a seed for one private repo with two tagged commits, granted to alice and to a
// server token, and returns its path.
func writeSeed(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for rev, body := range map[string]string{"rev-b": "b", "rev-c": "c"} {
		if err := os.MkdirAll(filepath.Join(dir, rev), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, rev, "board.txt"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	seed := `{"app": {"client_id": "Iv1.x", "client_secret": "s"},
	  "users": [{"login": "alice", "orgs": ["acme"]}, {"login": "mallory", "orgs": ["acme"]}],
	  "tokens": ["server-token"],
	  "repos": [{"repo": "hw/baseboard", "private": true, "readers": ["alice"], "tokens": ["server-token"],
	             "commits": [{"tag": "rev-b", "dir": "rev-b"}, {"tag": "rev-c", "dir": "rev-c"}]}]}`
	p := filepath.Join(dir, "seed.json")
	if err := os.WriteFile(p, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func get(t *testing.T, u, token string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// The seed's private repo answers, under /api/v3 as on a GHES host, only a token it granted.
func TestTheSeedGrantsAPrivateRepoToItsTokensAlone(t *testing.T) {
	e, err := seedEnterprise(writeSeed(t))
	if err != nil {
		t.Fatal(err)
	}
	front := e.gh
	if got := get(t, front.URL+"/api/v3/repos/hw/baseboard", "server-token"); got != http.StatusOK {
		t.Errorf("GET /api/v3/repos/hw/baseboard with the granted token = %d, want 200", got)
	}
	if got := get(t, front.URL+"/api/v3/repos/hw/baseboard", "someone-else"); got != http.StatusNotFound && got != http.StatusUnauthorized {
		t.Errorf("GET a private repo with an ungranted token = %d, want 404 or 401", got)
	}
	if e.commits["hw/baseboard"]["rev-b"] == "" || e.commits["hw/baseboard"]["rev-b"] == e.commits["hw/baseboard"]["rev-c"] {
		t.Errorf("the two tags name commits %v, want two different ones", e.commits["hw/baseboard"])
	}
}

// /push commits to main and delivers a push event signed with the secret it was given, and
// /approve refuses a code no device flow issued.
func TestTheControlAddressPushesAndApproves(t *testing.T) {
	e, err := seedEnterprise(writeSeed(t))
	if err != nil {
		t.Fatal(err)
	}
	var gotSig, gotBody string
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotSig = string(b), r.Header.Get("X-Hub-Signature-256")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer hook.Close()
	ctl := httptest.NewServer(e.control())
	defer ctl.Close()

	next := t.TempDir()
	if err := os.WriteFile(filepath.Join(next, "board.txt"), []byte("d"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := e.repos["hw/baseboard"].Head("main")
	resp, err := http.PostForm(ctl.URL+"/push", url.Values{"repo": {"hw/baseboard"}, "branch": {"main"}, "dir": {next}, "hook": {hook.URL}, "secret": {"shh"}})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Sha    string `json:"sha"`
		Status int    `json:"status"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	_ = resp.Body.Close()
	if out.Sha == "" || out.Sha == before || out.Status != http.StatusAccepted {
		t.Fatalf("push = %+v, want a new commit and the hook's 202", out)
	}
	mac := hmac.New(sha256.New, []byte("shh"))
	mac.Write([]byte(gotBody))
	if want := "sha256=" + hex.EncodeToString(mac.Sum(nil)); gotSig != want || !strings.Contains(gotBody, out.Sha) {
		t.Errorf("the delivery's signature %q or body doesn't match the push of %s", gotSig, out.Sha)
	}

	resp, err = http.PostForm(ctl.URL+"/approve", url.Values{"login": {"alice"}, "code": {"NOPE-0000"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("approving a code no device flow issued = %d, want 404", resp.StatusCode)
	}
}
