package main

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
)

// zipTree packs a folder into a zip, under its own name, as a person zips a project to send it.
func zipTree(t *testing.T, dir, into string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	base := filepath.Base(dir)
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		w, err := zw.Create(path.Join(base, filepath.ToSlash(rel)))
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(into, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

// isolated runs the CLI from an empty folder with no personal agni.yaml.
func isolated(t *testing.T) string {
	t.Helper()
	freshWorkspace(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

func statsOf(t *testing.T, design string) *webapi.GetSummaryResponse {
	t.Helper()
	var got webapi.GetSummaryResponse
	if err := protojson.Unmarshal([]byte(runRoot(t, "stats", design, "--format", "json")), &got); err != nil {
		t.Fatal(err)
	}
	return &got
}

// A zipped KiCad project opens as the folder it was zipped from (agni issue 1591). So an archive is
// a folder to every reader, not only to ODB++. The archive holds one design, so naming the archive
// names that design.
func TestAZippedKiCadProjectReadsAsItsFolder(t *testing.T) {
	src, err := filepath.Abs(filepath.Join("..", "..", "examples", "cross-probe-project"))
	if err != nil {
		t.Fatal(err)
	}
	want := statsOf(t, filepath.Join(src, "designs", "jetson-cut"))
	if want.GetComponentCount() != 19 || want.GetNetCount() == 0 {
		t.Fatalf("the cut reads %d components and %d nets from its folder", want.GetComponentCount(), want.GetNetCount())
	}
	dir := isolated(t)
	// The project with the cut alone, so the archive holds one design.
	tree := filepath.Join(dir, "cross-probe-project")
	for _, rel := range []string{"project.yaml", "designs/jetson-cut/design.yaml", "designs/jetson-cut/jetson-cut.kicad_sch",
		"designs/jetson-cut/jetson-cut.kicad_pcb", "designs/jetson-cut/jetson-cut.kicad_pro", "designs/jetson-cut/usb_debug_pd.kicad_sch", "designs/jetson-cut/power.kicad_sch"} {
		b, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		into := filepath.Join(tree, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(into), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(into, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	zipTree(t, tree, filepath.Join(dir, "probe.zip"))
	for _, named := range []string{"probe.zip", "probe.zip/cross-probe-project/designs/jetson-cut"} {
		got := statsOf(t, named)
		if got.GetComponentCount() != want.GetComponentCount() || got.GetNetCount() != want.GetNetCount() || got.GetSourceFormat() != want.GetSourceFormat() {
			t.Errorf("%s reads %d components and %d nets as %s, want the folder's %d and %d as %s", named,
				got.GetComponentCount(), got.GetNetCount(), got.GetSourceFormat(), want.GetComponentCount(), want.GetNetCount(), want.GetSourceFormat())
		}
		var sheets []string
		for _, s := range got.GetSheets() {
			sheets = append(sheets, s.GetId())
		}
		if !strings.Contains(strings.Join(sheets, ","), "board") {
			t.Errorf("%s reads no board sheet, among %v", named, sheets)
		}
	}
}

// An archive holding two designs undeclared names both rather than choosing one.
func TestAnArchiveOfTwoDesignsNamesBoth(t *testing.T) {
	src, err := filepath.Abs(filepath.Join("..", "..", "examples", "common", "designs"))
	if err != nil {
		t.Fatal(err)
	}
	dir := isolated(t)
	pair := filepath.Join(dir, "pair")
	for _, f := range []string{"mixer.edn", "rev-a.edn"} {
		b, err := os.ReadFile(filepath.Join(src, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(pair, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pair, f), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	zipTree(t, pair, filepath.Join(dir, "pair.zip"))
	cmd := rootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"stats", "pair.zip"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "holds 2 designs") || !strings.Contains(err.Error(), "mixer.edn") || !strings.Contains(err.Error(), "rev-a.edn") {
		t.Errorf("stats pair.zip = %v, want it to name both designs", err)
	}
}
