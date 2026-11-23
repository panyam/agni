package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/panyam/goapplib/storage/mounts"

	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/mountset"
)

// writeItemsChecklist writes, under a new folder, the same review three ways: inline.yaml, items.yaml
// naming items_dir checklist, and the checklist/ folder of item files (agni issue 1836).
func writeItemsChecklist(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"inline.yaml": `name: CAN review
areas:
  - name: can
    items:
      - {id: "198", title: bus signals present, profile: CAN}
      - {id: "202", title: termination strategy, rule: profile/can-termination-missing}
  - name: board
    items:
      - {id: "b1", title: track widths, rule: track-width}
`,
		"items.yaml":               "name: CAN review\nitems_dir: checklist\n",
		"checklist/01-can/198.md":  "---\nid: \"198\"\ntitle: bus signals present\nprofile: CAN\n---\n",
		"checklist/01-can/202.md":  "---\nid: \"202\"\ntitle: termination strategy\nrule: profile/can-termination-missing\n---\n",
		"checklist/02-board/b1.md": "---\nid: b1\ntitle: track widths\nrule: track-width\n---\n",
		"checklist/README.md":      "One file per item.\n",
	}
	for name, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// agni review --checklist takes a manifest file naming items_dir, or the folder of item files itself
// spelled as a path, and reviews as with the inline manifest (agni issue 1836).
func TestReviewTakesAChecklistFolder(t *testing.T) {
	design := isolatedDesign(t, "testdata/review/can-broken.edn")
	dir := writeItemsChecklist(t)
	want := reviewOutcomes(t, "review", design, "--checklist", filepath.Join(dir, "inline.yaml"))
	if len(want) != 3 || !slices.ContainsFunc(want, func(k string) bool { return strings.HasSuffix(k, "|fail") || strings.HasSuffix(k, "|pass") }) {
		t.Fatalf("the inline review decided nothing, so this test proves nothing: %v", want)
	}
	for _, c := range []string{filepath.Join(dir, "items.yaml"), filepath.Join(dir, "checklist") + string(filepath.Separator)} {
		if got := reviewOutcomes(t, "review", design, "--checklist", c); !slices.Equal(got, want) {
			t.Errorf("review --checklist %s = %v; want the inline manifest's %v", c, got, want)
		}
	}
}

// A served mount reads a manifest naming items_dir as the CLI does, so GetReviewManifest on it
// returns the folder's items (agni issue 1836).
func TestTheServerLoaderReadsAManifestNamingItemsDir(t *testing.T) {
	dir := writeItemsChecklist(t)
	l := &osLoader{mounts: tableOf([]mounts.Mount{mountset.Folder("t", dir)}), loader: newLoader()}
	read := func(name string) review.Manifest {
		t.Helper()
		u, err := mounts.NewURI("t", name)
		if err != nil {
			t.Fatal(err)
		}
		m, err := l.Manifest(context.Background(), u)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return m
	}
	want, got := read("inline.yaml"), read("items.yaml")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the served manifest naming items_dir reads as\n%+v\nwant\n%+v", got, want)
	}
}
