package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/panyam/goapplib/storage/mounts/github/githubtest"
)

// rawDownloads lists the paths gh's raw host served since the last call, sorted.
func rawDownloads(gh *githubtest.Server, seen *int) []string {
	var out []string
	reqs := gh.Requests()
	for _, r := range reqs[*seen:] {
		if strings.HasPrefix(r.Path, "/raw/") {
			out = append(out, strings.SplitN(r.Path, "/", 6)[5])
		}
	}
	*seen = len(reqs)
	slices.Sort(out)
	return out
}

func TestTheDesignsCommandLinksSyncsAndUnlinksAGitHubArtifact(t *testing.T) {
	gh := githubtest.New(t)
	repo := gh.AddRepo("antmicro", "baseboard", false, map[string]string{
		"README.md":          "# baseboard\n",
		"hw/bb.kicad_pro":    "{}",
		"hw/bb.kicad_sch":    `(kicad_sch (sheet (property "Sheetfile" "power.kicad_sch")))`,
		"hw/power.kicad_sch": "(kicad_sch)",
		"hw/bb.kicad_pcb":    "(kicad_pcb)",
	})
	v1 := repo.Head("main")
	dir := t.TempDir()
	opts := viewerOpts{store: testStore(t, filepath.Join(dir, "store")), uploadsDir: filepath.Join(dir, "uploads"), githubAPI: gh.URL, githubRaw: gh.RawURL()}
	url := serveViewer(t, opts)
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	must(runMounts(t, "add", "bb", "github://antmicro/baseboard", "--remote", url))
	must(runDesigns(t, url, "create", "bb"))
	out := must(runDesigns(t, url, "link", "bb", "schematic", "bb/hw/bb.kicad_sch@main"))
	if !strings.Contains(out, "3 files") || !strings.Contains(out, "from bb/hw/bb.kicad_sch@main at "+v1+", followed") {
		t.Errorf("designs link = %q; want three files from main at its head, followed", out)
	}
	out = must(runDesigns(t, url, "link", "bb", "board", "bb/hw/bb.kicad_pcb@"+v1))
	if !strings.Contains(out, "from bb/hw/bb.kicad_pcb@"+v1+", pinned") {
		t.Errorf("designs link of the board = %q; want it pinned at its commit", out)
	}
	seen := len(gh.Requests())

	v2 := repo.Commit("main", map[string]string{"hw/power.kicad_sch": "(kicad_sch (power))"}, nil)
	out = must(runDesigns(t, url, "sync", "bb"))
	if want := "designs/bb/artifacts/board\tunchanged\t" + v1[:12] + " -> " + v1[:12] + "\tfetched 0 files, 0 bytes\n" +
		"designs/bb/artifacts/schematic\tchanged\t" + v1[:12] + " -> " + v2[:12] + "\tfetched 1 files, 19 bytes\n"; out != want {
		t.Errorf("designs sync =\n%s\nwant\n%s", out, want)
	}
	if got := rawDownloads(gh, &seen); !slices.Equal(got, []string{"hw/power.kicad_sch"}) {
		t.Errorf("the sync downloaded %v, want the changed sheet alone", got)
	}
	if out, err := runMounts(t, "rm", "bb", "--remote", url); err == nil || !strings.Contains(err.Error(), "designs/bb/artifacts/board, designs/bb/artifacts/schematic are linked to the mount") {
		t.Errorf("mounts rm with artifacts linked = %q, %v; want a refusal naming both", out, err)
	}

	// A restart over the same folders keeps each link where the sync left it.
	url = serveViewer(t, opts)
	out = must(runDesigns(t, url, "artifacts", "bb"))
	if !strings.Contains(out, "from bb/hw/bb.kicad_sch@main at "+v2) {
		t.Errorf("after a restart, designs artifacts = %q; want the schematic at the new head", out)
	}
	if out, err := runDesigns(t, url, "put", "bb", "board", filepath.Join(dir, "uploads")); err == nil || !strings.Contains(err.Error(), "UnlinkArtifact") {
		t.Errorf("designs put over a linked board = %q, %v; want a refusal saying to unlink", out, err)
	}
	must(runDesigns(t, url, "unlink", "bb", "board"))
	must(runDesigns(t, url, "unlink", "bb", "schematic"))
	if out, err := runMounts(t, "rm", "bb", "--remote", url); err != nil || out != "removed bb\n" {
		t.Errorf("mounts rm after unlinking = %q, %v; want it removed", out, err)
	}
}
