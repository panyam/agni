package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// readyz is /readyz's status and body.
func readyz(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// waitReadyz polls /readyz until it answers code, and returns its body.
func waitReadyz(t *testing.T, url string, code int) string {
	t.Helper()
	var got int
	var body string
	for range 400 {
		if got, body = readyz(t, url); got == code {
			return body
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("/readyz = %d %q; want %d", got, body, code)
	return ""
}

// A server started with --bootstrap makes the connection, mounts, pack, design and project its file
// names. It answers /readyz with 503 naming the entry that fails while the connection's token is
// missing, and 200 once a retry applies everything. A restart over the same store is ready again
// with nothing changed (agni issue 1808).
func TestBootstrapBuildsTheServerFromItsFileAndIsReadyOnceEveryEntryApplies(t *testing.T) {
	first, maxWait := bootstrapFirstWait, bootstrapMaxWait
	bootstrapFirstWait, bootstrapMaxWait = 20*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { bootstrapFirstWait, bootstrapMaxWait = first, maxWait })

	gh, board := privateGitHubRepo(t)
	rules := gh.AddRepo("acme", "rules", false, tutorialRules(t))
	dir := t.TempDir()
	file := filepath.Join(dir, "bootstrap.yaml")
	if err := os.WriteFile(file, []byte(`
connections:
  - id: acme
    kind: github-token
    token: env:AGNI_TEST_BOOT_TOKEN
mounts:
  - id: board
    source: github://acme/board
    ref: main
    connection: acme
  - id: rules
    source: `+rules.Source()+`
    ref: main
packs:
  - id: house
    link: rules@main
designs:
  - id: bb
    title: Baseboard
    packs: [house]
    link:
      schematic: board/hw/bb.kicad_sch@main
projects:
  - id: team
    designs: [bb]
    packs: [house]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := readBootstrap(file)
	if err != nil {
		t.Fatal(err)
	}
	opts := viewerOpts{store: testStore(t, filepath.Join(dir, "store")), uploadsDir: filepath.Join(dir, "uploads"), githubAPI: gh.URL, githubRaw: gh.RawURL(), bootstrap: plan}
	url := serveViewer(t, opts)

	body := waitReadyz(t, url, http.StatusServiceUnavailable)
	for body == "bootstrap: applying\n" {
		time.Sleep(10 * time.Millisecond)
		_, body = readyz(t, url)
	}
	if !strings.Contains(body, "mounts/board failed") || strings.Contains(body, "mounts/rules failed") {
		t.Errorf("/readyz with the token missing = %q; want mounts/board named as failing, and mounts/rules not", body)
	}
	t.Setenv("AGNI_TEST_BOOT_TOKEN", secretToken)
	if body := waitReadyz(t, url, http.StatusOK); body != "ok\n" {
		t.Errorf("/readyz once the token is there = %q", body)
	}

	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	check := func(url string) {
		t.Helper()
		if out := must(runMounts(t, "ls", "--remote", url)); !strings.Contains(out, "board\tgithub\tgithub://acme/board@"+board.Head("main")+"\truntime\tvia acme\n") {
			t.Errorf("mounts ls = %q; want board through acme", out)
		}
		if out := must(runDesigns(t, url, "get", "bb")); !strings.Contains(out, `"Baseboard"`) || !strings.Contains(out, "2 files") || !strings.Contains(out, "from board/hw/bb.kicad_sch@main") {
			t.Errorf("designs get bb =\n%s\nwant Baseboard with its root sheet and child, linked from main", out)
		}
		if out := must(runPacks(t, url, "get", "house")); !strings.Contains(out, "from rules@main at "+rules.Head("main")+", followed") {
			t.Errorf("packs get house =\n%s\nwant it linked from rules on main", out)
		}
		if out := must(runProjects(t, url, "get", "team")); !strings.Contains(out, "designs/bb") || !strings.Contains(out, "packs/house") {
			t.Errorf("projects get team =\n%s\nwant designs/bb under packs/house", out)
		}
	}
	check(url)

	reads := len(gh.Requests())
	fileReads := 0
	for _, r := range gh.Requests() {
		if isFileRead(r.Path) {
			fileReads++
		}
	}
	if fileReads == 0 {
		t.Fatalf("the first start read no file the filter names, so the restart's check below proves nothing: %v", gh.Requests())
	}
	url = serveViewer(t, opts)
	if body := waitReadyz(t, url, http.StatusOK); body != "ok\n" {
		t.Errorf("/readyz after a restart = %q", body)
	}
	check(url)
	for _, r := range gh.Requests()[reads:] {
		if isFileRead(r.Path) {
			t.Errorf("the restart read %s %s; want no file read again", r.Method, r.Path)
		}
	}
}

// isFileRead reports whether a request to the fake GitHub fetched one file's bytes, as a link does.
func isFileRead(path string) bool {
	return strings.HasPrefix(path, "/raw/")
}

// A server with no --bootstrap is ready as soon as it serves.
func TestReadyzWithNoBootstrapIsOK(t *testing.T) {
	url := serveViewer(t, viewerOpts{})
	if code, body := readyz(t, url); code != http.StatusOK || body != "ok\n" {
		t.Errorf("/readyz = %d %q; want 200", code, body)
	}
}
