package service_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// declaredFolder is kicadFolder with a design.yaml naming bb.kicad_sch its entry. intent is the
// descriptor's intent: section, left out when empty.
func declaredFolder(intent string) map[string]string {
	files := kicadFolder("(kicad_sch)")
	files["hw/design.yaml"] = "entry: bb.kicad_sch\n" + intent
	return files
}

// internalParts names the parts the design's intent declares internal, sorted.
func internalParts(t *testing.T, svc *service.DesignService) []string {
	t.Helper()
	d, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Name: "designs/bb"})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for ref, c := range d.GetConfig().GetIntent().GetComponents() {
		if c.GetExposure() == "internal" {
			out = append(out, ref)
		}
	}
	slices.Sort(out)
	return out
}

// A link of a declared design's entry keeps the intent its design.yaml declares, as a push of the
// folder does. A board linked from the same folder reads no intent (agni issue 1854).
func TestALinkedEntryKeepsTheIntentItsDesignYAMLDeclares(t *testing.T) {
	t.Run("the board alone", func(t *testing.T) {
		b := blobs{}
		repo := &fakeMount{b: b, refs: map[string]string{"v1": "c1"}, files: map[string]map[string]string{"c1": declaredFolder("intent:\n  components:\n    J1: {exposure: internal}\n")}}
		svc, _, _ := linkedService(t, b, map[string]*fakeMount{"repo": repo})
		link(t, svc, "board", "repo", "hw/bb.kicad_pcb", "v1")
		if got := internalParts(t, svc); len(got) != 0 {
			t.Errorf("after linking the board alone, the design declares %v internal; want no intent", got)
		}
	})
	t.Run("the entry", func(t *testing.T) {
		b := blobs{}
		repo := &fakeMount{b: b, refs: map[string]string{"v1": "c1"}, files: map[string]map[string]string{"c1": declaredFolder("intent:\n  components:\n    J1: {exposure: internal}\n")}}
		svc, _, _ := linkedService(t, b, map[string]*fakeMount{"repo": repo})
		link(t, svc, "schematic", "repo", "hw/bb.kicad_sch", "v1")
		if got := internalParts(t, svc); !slices.Equal(got, []string{"J1"}) {
			t.Errorf("after linking the entry, the design declares %v internal; want [J1] from its design.yaml", got)
		}
	})
}

// A sync that moves the entry stores the intent the descriptor declares at the new version. One
// whose descriptor declares no intent leaves the stored intent alone, as a push does. A dry run
// stores nothing (agni issue 1854).
func TestASyncStoresTheIntentTheEntrysDesignYAMLDeclaresNow(t *testing.T) {
	ctx := context.Background()
	b := blobs{}
	repo := &fakeMount{
		b:        b,
		refs:     map[string]string{"main": "c1"},
		branches: map[string]bool{"main": true},
		files: map[string]map[string]string{
			"c1": declaredFolder("intent:\n  components:\n    J1: {exposure: internal}\n"),
			"c2": declaredFolder("intent:\n  components:\n    J1: {exposure: internal}\n    J2: {exposure: internal}\n"),
			"c3": declaredFolder("# intent moved out\n"),
		},
	}
	svc, _, _ := linkedService(t, b, map[string]*fakeMount{"repo": repo})
	link(t, svc, "schematic", "repo", "hw/bb.kicad_sch", "main")

	repo.refs["main"] = "c2"
	if _, err := svc.SyncDesign(ctx, &webapi.SyncDesignRequest{Name: "designs/bb", ValidateOnly: true}); err != nil {
		t.Fatal(err)
	}
	if got := internalParts(t, svc); !slices.Equal(got, []string{"J1"}) {
		t.Errorf("after a dry run, the design declares %v internal; want [J1] unchanged", got)
	}
	if _, err := svc.SyncDesign(ctx, &webapi.SyncDesignRequest{Name: "designs/bb"}); err != nil {
		t.Fatal(err)
	}
	if got := internalParts(t, svc); !slices.Equal(got, []string{"J1", "J2"}) {
		t.Errorf("after a sync to c2, the design declares %v internal; want [J1 J2]", got)
	}

	repo.refs["main"] = "c3"
	if _, err := svc.SyncDesign(ctx, &webapi.SyncDesignRequest{Name: "designs/bb"}); err != nil {
		t.Fatal(err)
	}
	if got := internalParts(t, svc); !slices.Equal(got, []string{"J1", "J2"}) {
		t.Errorf("after a sync to a descriptor with no intent, the design declares %v internal; want [J1 J2] kept", got)
	}
}

// A design.yaml whose intent doesn't compile fails the link and names the descriptor, as it fails a
// push.
func TestALinkedEntryWithABadIntentFails(t *testing.T) {
	b := blobs{}
	repo := &fakeMount{b: b, refs: map[string]string{"v1": "c1"}, files: map[string]map[string]string{"c1": declaredFolder("intent:\n  components:\n    J1: {exposre: internal}\n")}}
	svc, _, _ := linkedService(t, b, map[string]*fakeMount{"repo": repo})
	_, err := svc.LinkArtifact(context.Background(), &webapi.LinkArtifactRequest{
		Name:         "designs/bb/artifacts/schematic",
		Source:       &webapi.ArtifactSource{Mount: "repo", Path: "hw/bb.kicad_sch", Ref: "v1"},
		AllowMissing: true,
	})
	if !errors.Is(err, service.ErrInvalidArgument) || !strings.Contains(err.Error(), "design.yaml") {
		t.Errorf("linking an entry whose design.yaml has a bad intent = %v; want ErrInvalidArgument naming design.yaml", err)
	}
}
