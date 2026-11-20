package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/goapplib/storage/mounts/github/githubtest"

	"github.com/panyam/agni/mountset"
)

// A push to the branch a linked design follows reaches the design with no one running sync, once
// GitHub's signature checks out. A design set to mark notes the commit instead, and a report from
// before the push says it ran on earlier files (agni issue 1726).
func TestAPushWebhookMovesTheDesign(t *testing.T) {
	gh := githubtest.New(t)
	repo := gh.AddRepo("acme", "board", false, map[string]string{"hw/bb.kicad_sch": v1Sheet})
	dir := t.TempDir()
	secret := []byte("hook-secret")
	var f *follower
	url := serveViewer(t, viewerOpts{
		storeDir: filepath.Join(dir, "store"), uploadsDir: filepath.Join(dir, "uploads"), githubAPI: gh.URL, githubRaw: gh.RawURL(),
		webhookSecret: secret, follow: func(fl *follower) { f = fl },
	})
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	must(runMounts(t, "add", "board", repo.Source(), "--remote", url))
	must(runDesigns(t, url, "create", "bb"))
	must(runDesigns(t, url, "link", "bb", "schematic", "board/hw/bb.kicad_sch@main"))
	must(runReports(t, url, "create", "bb"))
	c2 := repo.Commit("main", map[string]string{"hw/bb.kicad_sch": v2Sheet}, nil)
	send := func(key []byte) int {
		t.Helper()
		resp, err := http.DefaultClient.Do(repo.PushRequest(url+githubHookPath, key, "main"))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		f.wg.Wait()
		return resp.StatusCode
	}

	if code := send([]byte("not-the-secret")); code != http.StatusUnauthorized {
		t.Errorf("a push signed with another secret = %d; want 401", code)
	}
	if arts := must(runDesigns(t, url, "artifacts", "bb")); strings.Contains(arts, c2) {
		t.Errorf("after a badly signed push, designs artifacts bb =\n%s\nwant it still at the first commit", arts)
	}
	if code := send(secret); code != http.StatusAccepted {
		t.Errorf("a signed push = %d; want 202", code)
	}
	if arts := must(runDesigns(t, url, "artifacts", "bb")); !strings.Contains(arts, "at "+c2) {
		t.Errorf("after a push of %s to main, designs artifacts bb =\n%s\nwant the schematic at it", c2, arts)
	}
	if reps := must(runReports(t, url, "list", "bb")); !strings.Contains(reps, "run on earlier design files") {
		t.Errorf("reports list bb after the push =\n%s\nwant the report to say it ran on earlier files", reps)
	}

	if out := must(runDesigns(t, url, "update", "bb", "--on-source-change", "mark")); !strings.Contains(out, "marks a source change") {
		t.Errorf("designs update bb --on-source-change mark = %q; want the design to say it marks", out)
	}
	c3 := repo.Commit("main", map[string]string{"hw/bb.kicad_sch": v1Sheet}, nil)
	send(secret)
	if arts := must(runDesigns(t, url, "artifacts", "bb")); !strings.Contains(arts, "at "+c2) || !strings.Contains(arts, "newer at the source: "+c3) {
		t.Errorf("after a push of %s to a design that marks, designs artifacts bb =\n%s\nwant it at %s with %s newer", c3, arts, c2, c3)
	}

	// A server not given the secret takes no deliveries.
	plain := serveViewer(t, viewerOpts{storeDir: filepath.Join(dir, "store2"), uploadsDir: filepath.Join(dir, "uploads2"), githubAPI: gh.URL, githubRaw: gh.RawURL()})
	resp, err := http.DefaultClient.Do(repo.PushRequest(plain+githubHookPath, secret, "main"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("a push to a server with no --github-webhook-secret = %d; want 404", resp.StatusCode)
	}
}

// A server polling its sources syncs a design linked from a folder once a file in it changes
// (agni issue 1726).
func TestAPollSyncsADesignWhoseFolderChanged(t *testing.T) {
	lab := t.TempDir()
	sheet := filepath.Join(lab, "bb.kicad_sch")
	if err := os.WriteFile(sheet, []byte(v1Sheet), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	url := serveViewer(t, viewerOpts{
		storeDir: filepath.Join(dir, "store"), uploadsDir: filepath.Join(dir, "uploads"),
		extraMounts: []mounts.Mount{mountset.Folder("lab", lab)}, poll: 20 * time.Millisecond,
	})
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	// content is the digest of the schematic's files, which a read of new files changes.
	content := func() string {
		out := must(runDesigns(t, url, "artifacts", "bb"))
		_, rest, _ := strings.Cut(out, "\tcontent ")
		digest, _, _ := strings.Cut(rest, "\t")
		return digest
	}
	must(runDesigns(t, url, "create", "bb"))
	must(runDesigns(t, url, "link", "bb", "schematic", "lab/bb.kicad_sch"))
	before := content()
	if before == "" {
		t.Fatal("designs artifacts bb names no content digest")
	}
	if err := os.WriteFile(sheet, []byte(v2Sheet), 0o600); err != nil {
		t.Fatal(err)
	}
	// A folder's version is its files' times, which a filesystem may round to the second.
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(sheet, later, later); err != nil {
		t.Fatal(err)
	}
	after := before
	for range 200 {
		if after = content(); after != before {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if after == before {
		t.Errorf("after the folder's sheet changed, the schematic's content is still %s; want a poll to have read the new sheet", after)
	}
}
