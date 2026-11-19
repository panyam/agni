package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appfake "github.com/panyam/oneauth/outbound/githubtest"
	"github.com/spf13/cobra"

	"github.com/panyam/goapplib/storage/mounts/github/githubtest"
)

// privateRepo is goapplib's fake GitHub holding a private board repo, readable with the token
// secretToken alone.
func privateRepo(t *testing.T) (*githubtest.Server, string) {
	t.Helper()
	gh := githubtest.New(t)
	repo := gh.AddRepo("acme", "board", true, map[string]string{
		"hw/bb.kicad_sch":    `(kicad_sch (sheet (property "Sheetfile" "power.kicad_sch")))`,
		"hw/power.kicad_sch": "(kicad_sch)",
		"hw/bb.kicad_pcb":    "(kicad_pcb)",
	})
	gh.AddToken(secretToken)
	return gh, repo.Head("main")
}

const secretToken = "ghp_not_in_any_record"

// A private repo mounts, browses, links and syncs through a token connection, the connection is
// refused removal while a mount reads through it, and no secret reaches the store or an answer
// (agni issue 1724).
func TestAPrivateRepoReadsThroughATokenConnection(t *testing.T) {
	gh, head := privateRepo(t)
	t.Setenv("AGNI_TEST_GITHUB_TOKEN", secretToken)
	dir := t.TempDir()
	opts := viewerOpts{storeDir: filepath.Join(dir, "store"), uploadsDir: filepath.Join(dir, "uploads"), githubAPI: gh.URL, githubRaw: gh.RawURL()}
	url := serveViewer(t, opts)
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	conns := func(args ...string) (string, error) {
		cmd := connectionsCmd()
		return runCobra(t, cmd, append(args, "--remote", url)...)
	}

	if out, err := runMounts(t, "add", "board", "github://acme/board", "--remote", url); err == nil || !strings.Contains(err.Error(), "not_found") {
		t.Fatalf("mounts add of a private repo with no connection = %q, %v; want not_found", out, err)
	}
	if out, err := conns("add", "bare", "--github-token", secretToken); err == nil || !strings.Contains(err.Error(), "invalid_argument") {
		t.Errorf("connections add with a bare token = %q, %v; want invalid_argument, since a record names where a secret is", out, err)
	}
	out := must(conns("add", "acme", "--github-token", "env:AGNI_TEST_GITHUB_TOKEN"))
	if out != "acme\tgithub-token\tenv:AGNI_TEST_GITHUB_TOKEN\tok\n" {
		t.Errorf("connections add = %q", out)
	}
	out = must(runMounts(t, "add", "board", "github://acme/board", "--connection", "acme", "--remote", url))
	if out != "board\tgithub\tgithub://acme/board@"+head+"\truntime\tvia acme\n" {
		t.Errorf("mounts add --connection = %q", out)
	}
	must(runDesigns(t, url, "create", "bb"))
	out = must(runDesigns(t, url, "link", "bb", "schematic", "board/hw/bb.kicad_sch@main"))
	if !strings.Contains(out, "2 files") {
		t.Errorf("designs link through the connection = %q; want the root sheet and its child", out)
	}
	must(runDesigns(t, url, "sync", "bb"))
	if out, err := conns("rm", "acme"); err == nil || !strings.Contains(err.Error(), "board") {
		t.Errorf("connections rm while a mount reads through it = %q, %v; want a refusal naming the mount", out, err)
	}

	// A restart keeps the connection and the mount that reads through it.
	url = serveViewer(t, opts)
	if out := must(runMounts(t, "ls", "--remote", url)); !strings.Contains(out, "board\tgithub\tgithub://acme/board@"+head+"\truntime\tvia acme\n") {
		t.Errorf("after a restart, mounts ls = %q", out)
	}
	if out := must(conns("ls")); out != "acme\tgithub-token\tenv:AGNI_TEST_GITHUB_TOKEN\tok\n" {
		t.Errorf("after a restart, connections ls = %q", out)
	}
	if err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err == nil && strings.Contains(string(b), secretToken) {
			t.Errorf("%s holds the token", p)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for _, r := range gh.Requests() {
		if strings.Contains(r.Path, "acme/board") && r.Token != "" && r.Token != secretToken {
			t.Errorf("%s %s carried token %q, want the connection's", r.Method, r.Path, r.Token)
		}
	}
	must(runDesigns(t, url, "unlink", "bb", "schematic"))
	must(runMounts(t, "rm", "board", "--remote", url))
	if out := must(conns("rm", "acme")); out != "removed acme\n" {
		t.Errorf("connections rm after its mount went = %q", out)
	}
}

// A private repo mounts through a GitHub App installation: the server signs a JWT with the App's
// key, trades it for an installation token, and reads the repo with that token (agni issue 1724).
func TestAPrivateRepoReadsThroughAnAppInstallation(t *testing.T) {
	gh, head := privateRepo(t)
	app := appfake.New(t)
	gh.AcceptToken(app.ValidInstallationToken)
	// The key goes through a file ref. On Windows its path starts with a drive letter, which oneauth
	// reads since v0.3.8.
	key := filepath.Join(t.TempDir(), "app.pem")
	if err := os.WriteFile(key, app.AppKeyPEM(), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	url := serveViewer(t, viewerOpts{storeDir: filepath.Join(dir, "store"), uploadsDir: filepath.Join(dir, "uploads"), githubAPI: gh.URL, githubRaw: gh.RawURL()})
	cmd := connectionsCmd()
	if out, err := runCobra(t, cmd, "add", "acme-app", "--github-app", "12345", "--installation", "1", "--key", "file:"+key, "--base-url", app.URL, "--remote", url); err != nil {
		t.Fatalf("connections add --github-app = %q, %v", out, err)
	}
	out, err := runMounts(t, "add", "board", "github://acme/board", "--connection", "acme-app", "--remote", url)
	if err != nil || !strings.Contains(out, "@"+head+"\truntime\tvia acme-app") {
		t.Fatalf("mounts add through the App = %q, %v", out, err)
	}
	if app.InstallationCalls() == 0 {
		t.Error("the server read the private repo without trading the App's JWT for an installation token")
	}
}

// runCobra runs cmd with args and returns what it printed.
func runCobra(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}
