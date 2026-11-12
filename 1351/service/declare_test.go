package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
)

// declare asks DeclareCompanion over an in-memory mount "m" and checks that the answer loads.
func declare(t *testing.T, files map[string]string, design, companion string) (*webapi.DeclareCompanionResponse, error) {
	t.Helper()
	mem := map[string][]byte{}
	for p, b := range files {
		mem[p] = []byte(b)
	}
	host := fshost.New(fshost.Mount{Name: "m", FS: fshost.MemFS(mem)})
	svc := service.NewWorkspaceService(host.Workspace()).WithDesignFiles(nil, host.Workspace())
	r, err := svc.DeclareCompanion(context.Background(), &webapi.DeclareCompanionRequest{DesignUri: "mount://m/" + design, CompanionUri: "mount://m/" + companion})
	if err != nil {
		return nil, err
	}
	_, d, _, perr := projects.ParseDesign(strings.NewReader(r.GetDesignYaml()))
	if perr != nil {
		t.Fatalf("the declared design.yaml does not load: %v\n%s", perr, r.GetDesignYaml())
	}
	found := false
	for _, c := range d.GetCompanionUris() {
		if c == strings.TrimPrefix(companion, folderOf(design)) {
			found = true
		}
	}
	if !found {
		t.Errorf("the declared design.yaml loads with companions %v, which lack %s\n%s", d.GetCompanionUris(), companion, r.GetDesignYaml())
	}
	return r, nil
}

func folderOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i+1]
	}
	return ""
}

func TestDeclareCompanionWritesANewDescriptor(t *testing.T) {
	r, err := declare(t, map[string]string{"odb/gateway.edn": "", "odb/layout/matrix/matrix": ""}, "odb/gateway.edn", "odb/layout/matrix/matrix")
	if err != nil {
		t.Fatal(err)
	}
	want := "name: gateway\nentry: gateway.edn\ncompanions:\n  - layout/matrix/matrix\n"
	if r.GetDesignYaml() != want || r.GetDescriptorUri() != "mount://m/odb/design.yaml" || r.GetExisted() || !r.GetChanged() {
		t.Errorf("got %+v, want the text %q", r, want)
	}
}

func TestDeclareCompanionExtendsTheProjectAbove(t *testing.T) {
	files := map[string]string{"project.yaml": "", "designs/gw/gateway.edn": "", "designs/gw/board.kicad_pcb": ""}
	r, err := declare(t, files, "designs/gw/gateway.edn", "designs/gw/board.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.GetDesignYaml(), "\nextends: ../../project.yaml\n") {
		t.Errorf("a design two folders under a project.yaml does not extend it:\n%s", r.GetDesignYaml())
	}
}

func TestDeclareCompanionKeepsAnExistingDescriptor(t *testing.T) {
	old := "# the gateway, read from its EDIF\nname: gateway\nentry: gateway.edn\ncompanions:\n  - gateway.eds # drawn sheets\nflatten_hierarchy: true\n"
	files := map[string]string{"gw/design.yaml": old, "gw/gateway.edn": "", "gw/gateway.eds": "", "gw/board.kicad_pcb": ""}
	r, err := declare(t, files, "gw/gateway.edn", "gw/board.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	got := r.GetDesignYaml()
	for _, keep := range []string{"# the gateway, read from its EDIF", "# drawn sheets", "flatten_hierarchy: true", "- gateway.eds", "- board.kicad_pcb"} {
		if !strings.Contains(got, keep) {
			t.Errorf("the edited descriptor lost or lacks %q:\n%s", keep, got)
		}
	}
	if !r.GetExisted() || !r.GetChanged() {
		t.Errorf("existed=%v changed=%v, want both", r.GetExisted(), r.GetChanged())
	}
}

func TestDeclareCompanionAddsTheListWhenAbsent(t *testing.T) {
	files := map[string]string{"gw/design.yaml": "name: gateway\nentry: gateway.edn\n", "gw/gateway.edn": "", "gw/board.kicad_pcb": ""}
	if _, err := declare(t, files, "gw/gateway.edn", "gw/board.kicad_pcb"); err != nil {
		t.Fatal(err)
	}
}

func TestDeclareCompanionAnswersAnAlreadyDeclaredCompanionUnchanged(t *testing.T) {
	old := "name: gateway\nentry: gateway.edn\ncompanions:\n  - ./board.kicad_pcb\n"
	files := map[string]string{"gw/design.yaml": old, "gw/gateway.edn": "", "gw/board.kicad_pcb": ""}
	r, err := declare(t, files, "gw/gateway.edn", "gw/board.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	if r.GetChanged() || r.GetDesignYaml() != old {
		t.Errorf("changed=%v, text %q; want the file as it was", r.GetChanged(), r.GetDesignYaml())
	}
}

func TestDeclareCompanionRefusesABoardOutsideTheFolder(t *testing.T) {
	files := map[string]string{"gw/gateway.edn": "", "boards/board.kicad_pcb": ""}
	_, err := declare(t, files, "gw/gateway.edn", "boards/board.kicad_pcb")
	if !errors.Is(err, service.ErrInvalidArgument) || !strings.Contains(err.Error(), "outside the design's folder") {
		t.Errorf("err = %v, want an invalid argument naming the folder", err)
	}
}
