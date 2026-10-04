package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
)

// TestProposeReadsAZipAsTheFolderItWasMadeFrom zips the tutorial gateway's files without its
// design.yaml and asks the CLI for a proposal, which must name the entry the tutorial declares, and
// whose json must be the rpc's message.
func TestProposeReadsAZipAsTheFolderItWasMadeFrom(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	src := filepath.Join("..", "..", "examples", "tutorial-project", "designs", "gateway")
	zipPath := filepath.Join(t.TempDir(), "gateway.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	err = filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == "design.yaml" {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		zf, err := w.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		_, err = zf.Write(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	f.Close()

	out := runRoot(t, "propose", zipPath)
	for _, want := range []string{"entry: gateway.edn", "  - gateway.kicad_sch", "  - entry: gateway-rev-b.edn"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	var resp webapi.ProposeDesignsResponse
	if err := protojson.Unmarshal([]byte(runRoot(t, "propose", "--format", "json", zipPath)), &resp); err != nil || len(resp.GetDesigns()) != 1 {
		t.Errorf("json is not one ProposeDesignsResponse design: %v %v", err, resp.GetDesigns())
	}
}
