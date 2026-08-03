package main

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/service"
)

func kicadInstalled() bool {
	_, err := exec.LookPath("kicad-cli")
	return err == nil
}

func kicadSvc(t *testing.T, enabled map[string]bool) *service.DesignService {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "readers", "kicad", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	return newDesignSvcNative([]mounts.Mount{{Name: "k", Root: root}}, enabled)
}

func TestGetSheetNativeGates(t *testing.T) {
	native := func(t *testing.T, svc *service.DesignService, mount, path string) error {
		_, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{
			Mount: mount, Path: path, Format: webapi.SheetFormat_SHEET_FORMAT_NATIVE,
		})
		return err
	}

	t.Run("disabled tool gates as not-enabled", func(t *testing.T) {
		err := native(t, kicadSvc(t, map[string]bool{}), "k", "geom.kicad_sch")
		if !errors.Is(err, service.ErrNativeNotEnabled) {
			t.Fatalf("want ErrNativeNotEnabled, got %v", err)
		}
	})

	t.Run("unsupported format gates as no-tool", func(t *testing.T) {
		root, _ := filepath.Abs(filepath.Join("..", "..", "readers", "edif", "testdata"))
		svc := newDesignSvcNative([]mounts.Mount{{Name: "e", Root: root}}, map[string]bool{"kicad-cli": true})
		if err := native(t, svc, "e", "sample.eds"); !errors.Is(err, service.ErrNativeNoTool) {
			t.Fatalf("want ErrNativeNoTool for .eds native, got %v", err)
		}
	})
}

func TestGetSheetNativeRender(t *testing.T) {
	if !kicadInstalled() {
		t.Skip("kicad-cli not installed")
	}
	svc := kicadSvc(t, map[string]bool{"kicad-cli": true})
	resp, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{
		Mount: "k", Path: "geom.kicad_sch", Format: webapi.SheetFormat_SHEET_FORMAT_NATIVE,
	})
	if err != nil {
		t.Fatal(err)
	}
	if svg := resp.GetSvg(); !strings.Contains(svg, "<svg") {
		t.Fatalf("want an <svg> from kicad-cli, got %.60q", svg)
	}
}

func TestGetSheetNativeRenderPcb(t *testing.T) {
	if !kicadInstalled() {
		t.Skip("kicad-cli not installed")
	}
	svc := kicadSvc(t, map[string]bool{"kicad-cli": true})
	resp, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{
		Mount: "k", Path: "pcb.kicad_pcb", Format: webapi.SheetFormat_SHEET_FORMAT_NATIVE,
	})
	if err != nil {
		t.Fatal(err)
	}
	if svg := resp.GetSvg(); !strings.Contains(svg, "<svg") {
		t.Fatalf("want an <svg> board render from kicad-cli, got %.60q", svg)
	}
}
