package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panyam/goapplib/storage/mounts/github/githubtest"
)

// writeSeed writes a seed for one private repo with two tagged commits, granted to alice and to a
// server token, and returns its path.
func writeSeed(t *testing.T) string {
	t.Helper()
	return writeSeedWith(t, `{"tag": "rev-c", "dir": "rev-c"}`)
}

// writeSeedWith writes writeSeed's seed with revC as the second commit.
func writeSeedWith(t *testing.T, revC string) string {
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
	             "commits": [{"tag": "rev-b", "dir": "rev-b"}, ` + revC + `]}]}`
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
	e, err := seedEnterprise(writeSeed(t), "", nil)
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
	e, err := seedEnterprise(writeSeed(t), "", nil)
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

func body(t *testing.T, u, token string) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestASeedCommitKeepsItsLFSPathsAsPointers(t *testing.T) {
	e, err := seedEnterprise(writeSeedWith(t, `{"tag": "rev-c", "dir": "rev-c", "lfs": ["board.txt"]}`), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := func(tag string) string {
		return body(t, e.gh.RawURL()+"/hw/baseboard/"+e.commits["hw/baseboard"][tag]+"/board.txt", "server-token")
	}
	pointer, _ := githubtest.LFSPointer("c")
	if got := raw("rev-c"); got != pointer {
		t.Errorf("board.txt at rev-c = %q; want the LFS pointer to its contents, %q", got, pointer)
	}
	if got := raw("rev-b"); got != "b" {
		t.Errorf("board.txt at rev-b = %q; want the plain file", got)
	}
	if _, err := seedEnterprise(writeSeedWith(t, `{"tag": "rev-c", "dir": "rev-c", "lfs": ["nope.txt"]}`), "", nil); err == nil || !strings.Contains(err.Error(), "nope.txt") {
		t.Errorf("a seed listing a missing file under lfs = %v; want an error naming it", err)
	}
}

// With a host, the fake listens there and names it in every URL it hands out, so an app in a
// container follows an LFS download link to the address it reaches (agni issue 1802). "localhost"
// stands in for the docker bridge's gateway, since it differs from the default 127.0.0.1 in each URL.
func TestASeededFakeOnAHostNamesItInTheURLsItHandsOut(t *testing.T) {
	e, err := seedEnterprise(writeSeedWith(t, `{"tag": "rev-c", "dir": "rev-c", "lfs": ["board.txt"]}`), "localhost", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(e.gh.URL, "http://localhost:") {
		t.Fatalf("the fake's URL = %s; want it on localhost", e.gh.URL)
	}
	pointer, oid := githubtest.LFSPointer("c")
	if got := body(t, e.gh.RawURL()+"/hw/baseboard/"+e.commits["hw/baseboard"]["rev-c"]+"/board.txt", "server-token"); got != pointer {
		t.Errorf("board.txt at rev-c through localhost = %q; want its LFS pointer", got)
	}
	req, _ := http.NewRequest(http.MethodPost, e.gh.LFSURL()+"/hw/baseboard.git/info/lfs/objects/batch",
		strings.NewReader(`{"operation": "download", "objects": [{"oid": "`+oid+`", "size": 1}]}`))
	req.Header.Set("Authorization", "Bearer server-token")
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), `"href":"http://localhost:`) && !strings.Contains(string(b), `"href": "http://localhost:`) {
		t.Errorf("the LFS batch answer = %s; want a download href on localhost", b)
	}
}

// With a certificate, the fake on a host serves HTTPS and names https URLs, since oneauth refuses a
// plain-HTTP GitHub Enterprise host off loopback (agni issue 1802).
func TestASeededFakeWithACertificateServesHTTPS(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(der)
	e, err := seedEnterprise(writeSeed(t), "localhost", &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(e.gh.URL, "https://localhost:") {
		t.Fatalf("the fake's URL = %s; want https on localhost", e.gh.URL)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}
	req, _ := http.NewRequest(http.MethodGet, e.gh.RawURL()+"/hw/baseboard/"+e.commits["hw/baseboard"]["rev-b"]+"/board.txt", nil)
	req.Header.Set("Authorization", "Bearer server-token")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("a read from the fake over HTTPS: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if b, _ := io.ReadAll(resp.Body); string(b) != "b" {
		t.Errorf("board.txt at rev-b over HTTPS = %q; want b", b)
	}
	if _, err := seedEnterprise(writeSeed(t), "", &tls.Certificate{}); err == nil || !strings.Contains(err.Error(), "--host") {
		t.Errorf("a certificate with no host = %v; want an error asking for --host", err)
	}
}
