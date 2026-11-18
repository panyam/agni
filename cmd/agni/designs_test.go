package main

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
)

// runDesigns runs `agni designs <args> --remote url` and returns what it printed.
func runDesigns(t *testing.T, url string, args ...string) (string, error) {
	t.Helper()
	cmd := designsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append(args, "--remote", url))
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for p, body := range files {
		dst := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// A KiCad folder pushes as a schematic artifact holding the root and its sheets, and a board
// artifact. A design.yaml there sets the title and series, and is not sent (agni issue 1696).
func TestPushKeepsAKiCadFolderAsASchematicAndABoard(t *testing.T) {
	ctx := context.Background()
	srv := storeServer(t, t.TempDir())
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"design.yaml":     "name: bb-1.1.3\ntitle: Baseboard 1.1.3\nseries: bb\nentry: bb.kicad_sch\nintent:\n  components:\n    J1: {exposure: internal}\n",
		"bb.kicad_pro":    "{}",
		"bb.kicad_sch":    `(kicad_sch (sheet (property "Sheetname" "Power") (property "Sheetfile" "power.kicad_sch")))`,
		"power.kicad_sch": "(kicad_sch)",
		"bb.kicad_pcb":    "(kicad_pcb)",
		"README.md":       "notes",
		".git/config":     "hidden",
	})
	out, err := runDesigns(t, srv.URL, "push", dir)
	if err != nil {
		t.Fatalf("push: %v\n%s", err, out)
	}
	if !strings.Contains(out, "not kept, as no reader opens them: README.md") {
		t.Errorf("push output does not name README.md as not sent:\n%s", out)
	}
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	d, err := designs.GetDesign(ctx, connect.NewRequest(&webapi.GetDesignRequest{Name: "designs/bb-1.1.3"}))
	if err != nil {
		t.Fatal(err)
	}
	if d.Msg.GetTitle() != "Baseboard 1.1.3" || d.Msg.GetSeries() != "bb" ||
		d.Msg.GetEntryUri() != "mount://store/bb-1.1.3/schematic/bb.kicad_sch" ||
		!slices.Equal(d.Msg.GetCompanionUris(), []string{"mount://store/bb-1.1.3/board/bb.kicad_pcb"}) {
		t.Errorf("pushed design = %v; want the descriptor's title and series, the root sheet as entry and the board as companion", d.Msg)
	}
	list, err := designs.ListArtifacts(ctx, connect.NewRequest(&webapi.ListArtifactsRequest{Parent: "designs/bb-1.1.3"}))
	if err != nil || len(list.Msg.GetArtifacts()) != 2 {
		t.Fatalf("artifacts = %v, %v; want board and schematic", list, err)
	}
	var sch []string
	for _, f := range list.Msg.GetArtifacts()[1].GetFiles() {
		sch = append(sch, f.GetPath())
	}
	if !slices.Equal(sch, []string{"bb.kicad_pro", "bb.kicad_sch", "power.kicad_sch"}) {
		t.Errorf("schematic artifact holds %v; want the project file, the root and its sheet", sch)
	}

	// A second push replaces both artifacts in one write.
	writeFiles(t, dir, map[string]string{"bb.kicad_pcb": "(kicad_pcb rev2)"})
	before := list.Msg.GetArtifacts()[0].GetFiles()[0].GetSha256()
	if out, err := runDesigns(t, srv.URL, "push", dir); err != nil {
		t.Fatalf("second push: %v\n%s", err, out)
	}
	b, err := designs.GetArtifact(ctx, connect.NewRequest(&webapi.GetArtifactRequest{Name: "designs/bb-1.1.3/artifacts/board"}))
	if err != nil || b.Msg.GetFiles()[0].GetSha256() == before {
		t.Errorf("board after the second push = %v, %v; want its new contents", b, err)
	}
}

func TestPushRefusesAFolderHoldingTwoDesigns(t *testing.T) {
	srv := storeServer(t, t.TempDir())
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"alpha.edn": "(edif alpha)", "beta.edn": "(edif beta)"})
	out, err := runDesigns(t, srv.URL, "push", dir, "--id", "two")
	if err == nil || !strings.Contains(err.Error(), "hold 2 designs") {
		t.Errorf("push of two designs = %v\n%s; want a refusal naming both", err, out)
	}
	if out, err := runDesigns(t, srv.URL, "get", "two"); err == nil {
		t.Errorf("a refused push left designs/two behind:\n%s", out)
	}
}

// put sends one artifact, a folder by its relative paths, and replaces it whole.
func TestPutReplacesAnArtifactWhole(t *testing.T) {
	ctx := context.Background()
	srv := storeServer(t, t.TempDir())
	if out, err := runDesigns(t, srv.URL, "create", "mul", "--series", "m"); err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	first, second := t.TempDir(), t.TempDir()
	writeFiles(t, first, map[string]string{"mul.kicad_sch": `(kicad_sch (sheet (property "Sheetfile" "sub/a.kicad_sch")))`, "sub/a.kicad_sch": "(kicad_sch)"})
	writeFiles(t, second, map[string]string{"mul.kicad_sch": "(kicad_sch)"})
	for _, dir := range []string{first, second} {
		if out, err := runDesigns(t, srv.URL, "put", "mul", "schematic", dir); err != nil {
			t.Fatalf("put %s: %v\n%s", dir, err, out)
		}
	}
	a, err := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL).GetArtifact(ctx, connect.NewRequest(&webapi.GetArtifactRequest{Name: "designs/mul/artifacts/schematic"}))
	if err != nil || len(a.Msg.GetFiles()) != 1 || a.Msg.GetFiles()[0].GetPath() != "mul.kicad_sch" {
		t.Errorf("schematic after the second put = %v, %v; want only mul.kicad_sch, the first put's sheet gone", a, err)
	}
	out, err := runDesigns(t, srv.URL, "get", "mul")
	if err != nil || !strings.Contains(out, "series m") || !strings.Contains(out, "designs/mul/artifacts/schematic\tschematic\tentry mul.kicad_sch") {
		t.Errorf("get = %v\n%s; want the series and the schematic artifact", err, out)
	}
	if out, err := runDesigns(t, srv.URL, "rm", "mul", "schematic"); err != nil {
		t.Fatalf("rm artifact: %v\n%s", err, out)
	}
	if out, err := runDesigns(t, srv.URL, "artifacts", "mul"); err != nil || strings.TrimSpace(out) != "" {
		t.Errorf("artifacts after rm = %q, %v; want none", out, err)
	}
}
