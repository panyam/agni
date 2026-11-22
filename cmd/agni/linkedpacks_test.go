package main

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/goapplib/storage/mounts/github/githubtest"
)

// tutorialRules is the tutorial project's rules as a repo holds them: project.yaml and the folders it
// names, at the repo's root.
func tutorialRules(t *testing.T) map[string]string {
	t.Helper()
	root := "../../examples/tutorial-project"
	out := map[string]string{}
	for _, top := range []string{"project.yaml", "profiles", "params", "lib", "queries"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := os.ReadFile(p) // #nosec G304 -- a fixture under examples/
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			out[filepath.ToSlash(rel)] = string(b)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// withStrictRule adds a second house naming rule to a project.yaml, one that also flags CLK nets.
func withStrictRule(t *testing.T, projectYAML string) string {
	t.Helper()
	const anchor = "  rules:\n"
	if !strings.Contains(projectYAML, anchor) {
		t.Fatalf("project.yaml has no conventions rules to add to")
	}
	return strings.Replace(projectYAML, anchor, anchor+"    - name: strict-net-naming\n      severity: warning\n      why: \"a stricter house style\"\n      allow: [\"^(PMIC|CAN[0-9]+|I2C|MCU|GND)\"]\n", 1)
}

// A pack linked from a repo on main moves when GitHub reports a push to main, the next check runs
// the rule the push added, and each report names the commit its pack was read at. A pack pinned at
// a commit stays, and the mount can't be deleted under the packs (agni issue 1801).
func TestAPushMovesALinkedPackAndTheReportNamesItsCommit(t *testing.T) {
	gh := githubtest.New(t)
	files := tutorialRules(t)
	repo := gh.AddRepo("acme", "rules", false, files)
	c1 := repo.Head("main")
	dir := t.TempDir()
	secret := []byte("hook-secret")
	var f *follower
	url := serveViewer(t, viewerOpts{
		storeDir: filepath.Join(dir, "store"), uploadsDir: filepath.Join(dir, "uploads"), githubAPI: gh.URL, githubRaw: gh.RawURL(),
		webhookSecret: secret, follow: func(fl *follower) { f = fl }, uploadMaxBytes: 1 << 26,
	})
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	project := tutorialProject(t)
	must(runDesigns(t, url, "push", filepath.Join(project, "designs", "gw"), "--id", "gw"))
	must(runMounts(t, "add", "rules", repo.Source(), "--remote", url))

	out := must(runPacks(t, url, "link", "house", "rules@main"))
	if !strings.Contains(out, "packs/house\t") || !strings.Contains(out, "from rules@main at "+c1+", followed") || !strings.Contains(out, "profiles: profiles") {
		t.Errorf("packs link house rules@main =\n%s\nwant the pack from main at its head, followed, with its profiles", out)
	}
	out = must(runPacks(t, url, "link", "pinned", "rules@"+c1))
	if !strings.Contains(out, "from rules@"+c1+", pinned") {
		t.Errorf("packs link at a commit =\n%s\nwant it pinned", out)
	}
	before := findingKeys(t, "check", "designs/gw", "--pack", "house", "--remote", url)
	if !hasRulePrefix(before, "gateway/signal-net-naming") || hasRulePrefix(before, "gateway/strict-net-naming") {
		t.Fatalf("check under the linked pack before the push = %v; want the house rule and not the new one", before)
	}
	if out := must(runReports(t, url, "create", "gw", "--pack", "house")); !strings.Contains(out, "packs/house at "+c1[:12]) {
		t.Errorf("reports create gw --pack house =\n%s\nwant it to name the pack's commit", out)
	}

	files["project.yaml"] = withStrictRule(t, files["project.yaml"])
	c2 := repo.Commit("main", map[string]string{"project.yaml": files["project.yaml"]}, nil)
	resp, err := http.DefaultClient.Do(repo.PushRequest(url+githubHookPath, secret, "main"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	f.wg.Wait()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("a signed push = %d; want 202", resp.StatusCode)
	}
	if out := must(runPacks(t, url, "get", "house")); !strings.Contains(out, "at "+c2+", followed") {
		t.Errorf("after a push of %s to main, packs get house =\n%s\nwant the pack at it", c2, out)
	}
	if out := must(runPacks(t, url, "get", "pinned")); !strings.Contains(out, "from rules@"+c1+", pinned") {
		t.Errorf("after the push, packs get pinned =\n%s\nwant it left at %s", out, c1)
	}
	after := findingKeys(t, "check", "designs/gw", "--pack", "house", "--remote", url)
	if !hasRulePrefix(after, "gateway/strict-net-naming") {
		t.Errorf("check under the linked pack after the push = %v; want the rule the push added", after)
	}
	if out := must(runReports(t, url, "create", "gw", "--pack", "house")); !strings.Contains(out, "packs/house at "+c2[:12]) {
		t.Errorf("reports create after the push =\n%s\nwant it to name the new commit", out)
	}
	if out := must(runReports(t, url, "list", "gw")); !strings.Contains(out, "packs/house changed since; packs/house at "+c1[:12]) {
		t.Errorf("reports list gw =\n%s\nwant the first report to say its pack changed, at %s", out, c1[:12])
	}
	if out := must(runPacks(t, url, "sync", "house")); !strings.Contains(out, "packs/house\tunchanged\t"+c2[:12]+" -> "+c2[:12]) {
		t.Errorf("packs sync house with main unmoved = %q; want unchanged", out)
	}

	if out, err := runPacks(t, url, "push", project, "--id", "house"); err == nil || !strings.Contains(err.Error(), "UnlinkPack") {
		t.Errorf("packs push over a linked pack = %q, %v; want a refusal saying to unlink", out, err)
	}
	if out, err := runMounts(t, "rm", "rules", "--remote", url); err == nil || !strings.Contains(err.Error(), "packs/house, packs/pinned") {
		t.Errorf("mounts rm with packs linked = %q, %v; want a refusal naming both", out, err)
	}
	if out := must(runPacks(t, url, "unlink", "house")); strings.Contains(out, "from rules") {
		t.Errorf("packs unlink house = %q; want no source", out)
	}
}
