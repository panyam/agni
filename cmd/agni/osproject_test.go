package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/project"
)

// projectMount builds the layout a review project takes on disk and returns a store over it.
//
//	<mount>/project.yaml                     name: gateway
//	<mount>/designs/gateway/design.yaml      name: gateway, entry gateway.edn, board companion
//	<mount>/scratch/loose.edn                belongs to no design
func projectMount(t *testing.T) (*osProjects, string) {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, project.ProjectDescriptor), "name: gateway\ntitle: Gateway program\n")
	write(t, filepath.Join(root, "designs", "gateway", project.DesignDescriptor), `
name: gateway
title: Gateway ECU
entry: gateway.edn
companions:
  - gateway.kicad_pcb
`)
	write(t, filepath.Join(root, "designs", "gateway", "gateway.edn"), "x")
	write(t, filepath.Join(root, "designs", "gateway", "gateway.kicad_pcb"), "x")
	write(t, filepath.Join(root, "scratch", "loose.edn"), "x")
	return &osProjects{mounts: []mounts.Mount{{Name: "boards", Root: root}}}, root
}

func TestOSProjectsDiscovers(t *testing.T) {
	store, _ := projectMount(t)
	ctx := context.Background()

	ps, err := store.Projects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].ID != "gateway" || ps[0].Title != "Gateway program" || ps[0].Mount != "boards" {
		t.Fatalf("projects = %+v", ps)
	}
	if ps[0].DirRef != "" {
		t.Errorf("dir_ref = %q, want the mount root spelled as empty", ps[0].DirRef)
	}

	ds, err := store.Designs(ctx, "gateway")
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 {
		t.Fatalf("designs = %+v", ds)
	}
	d := ds[0]
	// The descriptor's design-relative names become mount-relative refs, because every consumer above
	// the port addresses files by (mount, ref) and none knows where the design folder sits.
	if d.EntryRef != "designs/gateway/gateway.edn" {
		t.Errorf("entry_ref = %q", d.EntryRef)
	}
	if len(d.CompanionRefs) != 1 || d.CompanionRefs[0] != "designs/gateway/gateway.kicad_pcb" {
		t.Errorf("companion_refs = %v", d.CompanionRefs)
	}
	if d.ProjectID != "gateway" {
		t.Errorf("project id = %q", d.ProjectID)
	}
}

// TestOSProjectsResolve walks UP from a ref, so the cost is a few stats no matter how many designs a
// mount holds. A file under no design resolves to nothing, which is the ordinary case.
func TestOSProjectsResolve(t *testing.T) {
	store, _ := projectMount(t)
	ctx := context.Background()

	for _, ref := range []string{
		"designs/gateway/gateway.edn",
		"designs/gateway/gateway.kicad_pcb",
		"designs/gateway", // the design folder itself
	} {
		d, p, ok, err := store.Resolve(ctx, "boards", ref)
		if err != nil || !ok {
			t.Fatalf("Resolve(%q) = ok %v, err %v", ref, ok, err)
		}
		if d.ID != "gateway" || p.ID != "gateway" {
			t.Errorf("Resolve(%q) = design %q, project %q", ref, d.ID, p.ID)
		}
	}

	_, _, ok, err := store.Resolve(ctx, "boards", "scratch/loose.edn")
	if err != nil {
		t.Fatalf("a ref under no design is ordinary, not an error: %v", err)
	}
	if ok {
		t.Error("scratch/loose.edn should resolve to no design")
	}

	// The mount is still the containment boundary: resolution cannot be steered out of it.
	if _, _, _, err := store.Resolve(ctx, "boards", "../elsewhere/design.yaml"); !errors.Is(err, service.ErrInvalidPath) {
		t.Errorf("escaping ref error = %v, want ErrInvalidPath", err)
	}
	if _, _, _, err := store.Resolve(ctx, "nope", "a.edn"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("unknown mount error = %v, want ErrNotFound", err)
	}
}

// TestOSProjectsResolveWithoutAProjectIsAMiss: a design is addressable only under a parent, so a
// design.yaml with no enclosing project.yaml has no resource name and resolves to nothing.
func TestOSProjectsResolveWithoutAProjectIsAMiss(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "board", project.DesignDescriptor), "name: board\nentry: board.edn\n")
	write(t, filepath.Join(root, "board", "board.edn"), "x")
	store := &osProjects{mounts: []mounts.Mount{{Name: "m", Root: root}}}

	_, _, ok, err := store.Resolve(context.Background(), "m", "board/board.edn")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("a design outside any project has no resource name, so it must not resolve")
	}
}

// TestOSProjectsRejectsDuplicateIDs: two projects claiming one name means one is unreachable through
// its own resource name, and serving the other would answer a question about A with B's designs.
func TestOSProjectsRejectsDuplicateIDs(t *testing.T) {
	rootA, rootB := t.TempDir(), t.TempDir()
	write(t, filepath.Join(rootA, project.ProjectDescriptor), "name: gateway\n")
	write(t, filepath.Join(rootB, project.ProjectDescriptor), "name: gateway\n")
	store := &osProjects{mounts: []mounts.Mount{{Name: "a", Root: rootA}, {Name: "b", Root: rootB}}}

	if _, err := store.Projects(context.Background()); err == nil || !strings.Contains(err.Error(), "duplicate project id") {
		t.Fatalf("error = %v, want a duplicate-id error", err)
	}
}

// TestOSProjectsMalformedDescriptorFailsLoudly: a skipped descriptor would leave an operator reading
// default behaviour as the engine agreeing with what they wrote.
func TestOSProjectsMalformedDescriptorFailsLoudly(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, project.ProjectDescriptor), "name: Gateway\n") // ids are lowercase
	store := &osProjects{mounts: []mounts.Mount{{Name: "m", Root: root}}}
	if _, err := store.Projects(context.Background()); err == nil {
		t.Fatal("an invalid project id should fail the listing, not be skipped")
	}
}

// TestOSProjectsSkipsDotDirsAndStopsAtDepth keeps the walk off a mount's `.git` and off however deep
// a bind-mounted home directory happens to be.
func TestOSProjectsSkipsDotDirsAndStopsAtDepth(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".git", "modules", project.ProjectDescriptor), "name: hidden\n")
	deep := filepath.Join(root, "a", "b", "c", "d", "e")
	write(t, filepath.Join(deep, project.ProjectDescriptor), "name: toodeep\n")
	write(t, filepath.Join(root, "ok", project.ProjectDescriptor), "name: shallow\n")
	store := &osProjects{mounts: []mounts.Mount{{Name: "m", Root: root}}}

	ps, err := store.Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].ID != "shallow" {
		t.Fatalf("projects = %+v, want only the shallow one", ps)
	}
}

// TestOSProjectsNestedProjectsDoNotCompound: a project inside a project is an ambiguity nobody meant,
// so the walk stops at the outer one rather than reporting both.
func TestOSProjectsNestedProjectsDoNotCompound(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "outer", project.ProjectDescriptor), "name: outer\n")
	write(t, filepath.Join(root, "outer", "inner", project.ProjectDescriptor), "name: inner\n")
	store := &osProjects{mounts: []mounts.Mount{{Name: "m", Root: root}}}

	ps, err := store.Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].ID != "outer" {
		t.Fatalf("projects = %+v, want only the outer one", ps)
	}
}

// TestOSProjectsAbsentIDs classify as NotFound so the service can hand the transport a code.
func TestOSProjectsAbsentIDs(t *testing.T) {
	store, _ := projectMount(t)
	ctx := context.Background()
	if _, err := store.Project(ctx, "nope"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("Project(absent) = %v, want ErrNotFound", err)
	}
	if _, err := store.Design(ctx, "gateway", "nope"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("Design(absent) = %v, want ErrNotFound", err)
	}
	if _, err := store.Designs(ctx, "nope"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("Designs(absent project) = %v, want ErrNotFound", err)
	}
}
