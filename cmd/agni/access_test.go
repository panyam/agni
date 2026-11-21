package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/panyam/agni/internal/authgate"
)

// A design linked from a private repo opens for a teammate whose own GitHub connection reads the
// repo, and is refused to one whose doesn't, by name, by check, in a listing and by its files' path
// (agni issue 1728).
func TestALinkedDesignOpensOnlyForWhoCanReadItsRepo(t *testing.T) {
	old := authgate.CacheTTL
	authgate.CacheTTL = time.Millisecond
	t.Cleanup(func() { authgate.CacheTTL = old })

	gh, _ := privateRepo(t)
	const aliceGitHub = "ghp_alice_reads_acme"
	gh.AddToken(aliceGitHub)
	t.Setenv("AGNI_TEST_GITHUB_TOKEN", secretToken)
	t.Setenv("AGNI_TEST_ALICE_GITHUB", aliceGitHub)
	t.Setenv("AGNI_TEST_CONNECTIONS_KEY", strings.Repeat("5e", 32))
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	tokens := map[string]string{}
	for _, who := range []string{"ops", "alice", "bob"} {
		out, err := runCobra(t, tokenCmd(), "create", who, "--store", "file:///"+strings.TrimPrefix(filepath.ToSlash(store), "/"))
		if err != nil {
			t.Fatalf("agni token create %s: %v", who, err)
		}
		tokens[who], _, _ = strings.Cut(strings.TrimSpace(out), "\n")
	}
	url := serveViewer(t, viewerOpts{storeDir: store, uploadsDir: filepath.Join(dir, "uploads"), auth: authgate.Tokens,
		connectionsKey: "env:AGNI_TEST_CONNECTIONS_KEY", githubAPI: gh.URL, githubRaw: gh.RawURL()})
	as := func(who string) { t.Setenv(envToken, tokens[who]) }
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	denied := func(what, out string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "permission_denied") || !strings.Contains(err.Error(), "github://acme/board") {
			t.Errorf("bob %s = %q, %v; want permission_denied naming github://acme/board", what, out, err)
		}
	}

	as("ops")
	must(runCobra(t, connectionsCmd(), "add", "acme", "--github-token", "env:AGNI_TEST_GITHUB_TOKEN", "--remote", url))
	must(runMounts(t, "add", "board", "github://acme/board", "--connection", "acme", "--remote", url))

	as("alice")
	if out := must(runCobra(t, connectionsCmd(), "add", "alice-gh", "--mine", "--github-token", "env:AGNI_TEST_ALICE_GITHUB", "--remote", url)); out != "alice-gh\tgithub-user\tsealed, for alice\tok\n" {
		t.Errorf("connections add --mine = %q", out)
	}
	must(runDesigns(t, url, "create", "bb"))
	must(runDesigns(t, url, "create", "notes"))
	must(runDesigns(t, url, "link", "bb", "schematic", "board/hw/bb.kicad_sch@main"))
	got := must(runDesigns(t, url, "get", "bb"))
	if !strings.Contains(got, "; opens for who can read github://acme/board") {
		t.Errorf("designs get bb = %q; want it to say who it opens for", got)
	}
	read := regexp.MustCompile(`reads mount://uploads/([^\s;]+)`).FindStringSubmatch(got)
	if read == nil {
		t.Fatalf("designs get bb = %q; want the uploads path it reads", got)
	}
	must(runAgni(t, "check", "designs/bb", "--remote", url))

	as("bob")
	out, err := runDesigns(t, url, "get", "bb")
	denied("designs get bb", out, err)
	out, err = runAgni(t, "check", "designs/bb", "--remote", url)
	denied("check designs/bb", out, err)
	if out := must(runDesigns(t, url, "list")); strings.Contains(out, "designs/bb") || !strings.Contains(out, "designs/notes") {
		t.Errorf("bob's designs list = %q; want notes and not bb", out)
	}
	req, _ := http.NewRequest(http.MethodGet, url+"/raw/uploads/"+read[1], nil)
	req.Header.Set("Authorization", "Bearer "+tokens["bob"])
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("bob GET /raw/uploads/%s = %d; want 403", read[1], resp.StatusCode)
	}
	if out := must(runCobra(t, connectionsCmd(), "ls", "--remote", url)); strings.Contains(out, "alice-gh") {
		t.Errorf("bob's connections ls = %q; want alice's left out", out)
	}
	if out := must(runMounts(t, "ls", "--remote", url)); strings.Contains(out, "acme/board") || !strings.Contains(out, "board\tgithub\ta private source you can't read") {
		t.Errorf("bob's mounts ls = %q; want board listed without its repo", out)
	}

	as("alice")
	if out := must(runMounts(t, "ls", "--remote", url)); !strings.Contains(out, "board\tgithub\tgithub://acme/board@") {
		t.Errorf("alice's mounts ls = %q; want board's repo, since she can read it", out)
	}
	req.Header.Set("Authorization", "Bearer "+tokens["alice"])
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != http.StatusOK {
		t.Errorf("alice GET /raw/uploads/%s = %v, %v; want 200", read[1], resp, err)
	} else {
		resp.Body.Close()
	}
	if err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err == nil && strings.Contains(string(b), aliceGitHub) {
			t.Errorf("%s holds alice's GitHub token in the clear", p)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

// A design linked from a private repo on a server without --auth records the repo it read, so it
// opens only for the repo's readers once the same store is served with --auth (agni issue 1778).
func TestADesignLinkedWithoutAuthIsGuardedOnceAuthIsOn(t *testing.T) {
	old := authgate.CacheTTL
	authgate.CacheTTL = time.Millisecond
	t.Cleanup(func() { authgate.CacheTTL = old })

	gh, _ := privateRepo(t)
	t.Setenv("AGNI_TEST_GITHUB_TOKEN", secretToken)
	t.Setenv("AGNI_TEST_CONNECTIONS_KEY", strings.Repeat("5e", 32))
	t.Setenv(envToken, "")
	dir := t.TempDir()
	store, uploads := filepath.Join(dir, "store"), filepath.Join(dir, "uploads")
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}

	open := serveViewer(t, viewerOpts{storeDir: store, uploadsDir: uploads,
		connectionsKey: "env:AGNI_TEST_CONNECTIONS_KEY", githubAPI: gh.URL, githubRaw: gh.RawURL()})
	must(runCobra(t, connectionsCmd(), "add", "acme", "--github-token", "env:AGNI_TEST_GITHUB_TOKEN", "--remote", open))
	must(runMounts(t, "add", "board", "github://acme/board", "--connection", "acme", "--remote", open))
	must(runDesigns(t, open, "create", "bb"))
	must(runDesigns(t, open, "link", "bb", "schematic", "board/hw/bb.kicad_sch@main"))
	// The server reads every design for every caller without --auth, but keeps the repo on the files.
	if a := artifactJSON(t, open, "designs/bb/artifacts/schematic"); a.Source.AccessSource != "github://acme/board" {
		t.Errorf("the schematic linked with no --auth records access source %q; want github://acme/board", a.Source.AccessSource)
	}
	// Unlinked, and its mount gone, the files are guarded by the repo they recorded and nothing else.
	must(runDesigns(t, open, "unlink", "bb", "schematic"))
	must(runMounts(t, "rm", "board", "--remote", open))

	out, err := runCobra(t, tokenCmd(), "create", "bob", "--store", "file:///"+strings.TrimPrefix(filepath.ToSlash(store), "/"))
	if err != nil {
		t.Fatalf("agni token create bob: %v", err)
	}
	bob, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	guarded := serveViewer(t, viewerOpts{storeDir: store, uploadsDir: uploads, auth: authgate.Tokens,
		connectionsKey: "env:AGNI_TEST_CONNECTIONS_KEY", githubAPI: gh.URL, githubRaw: gh.RawURL()})
	t.Setenv(envToken, bob)
	if out, err := runDesigns(t, guarded, "get", "bb"); err == nil || !strings.Contains(err.Error(), "permission_denied") || !strings.Contains(err.Error(), "github://acme/board") {
		t.Errorf("bob's designs get bb with --auth = %q, %v; want permission_denied naming github://acme/board", out, err)
	}
}

// artifactJSON reads an artifact over Connect's JSON codec, with no token.
func artifactJSON(t *testing.T, server, name string) (a struct {
	Source struct {
		AccessSource string `json:"accessSource"`
	} `json:"source"`
}) {
	t.Helper()
	resp, err := http.Post(server+"/agni.v1.webapi.DesignService/GetArtifact", "application/json", strings.NewReader(`{"name":"`+name+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		t.Fatal(err)
	}
	return a
}
