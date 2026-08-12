package service

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/artifact"
)

// projectTable is a ProjectStore that answers Project(name) from a map, which is all an extends walk
// asks of it.
type projectTable map[string]*webapi.Project

func (t projectTable) Project(_ context.Context, name string) (*webapi.Project, error) {
	return t[name], nil
}
func (t projectTable) Projects(context.Context) ([]*webapi.Project, error) { return nil, nil }
func (t projectTable) Design(context.Context, string) (*webapi.Design, error) {
	return nil, nil
}
func (t projectTable) Designs(context.Context, string) ([]*webapi.Design, error) {
	return nil, nil
}
func (t projectTable) ResolveDesign(context.Context, artifact.URI) (*webapi.Design, *webapi.Project, error) {
	return nil, nil, nil
}

func proj(name, extends string, cfg *webapi.AnalysisConfig) *webapi.Project {
	if cfg == nil {
		cfg = &webapi.AnalysisConfig{}
	}
	cfg.Extends = extends
	return &webapi.Project{Name: name, Config: cfg}
}

// TestExtendsLayersRootFirst: a project overrides what it inherits, which is the direction every
// other layer here runs (request over project, project over deployment default).
func TestExtendsLayersRootFirst(t *testing.T) {
	store := projectTable{
		"projects/base": proj("projects/base", "", &webapi.AnalysisConfig{
			ProfileUris: []string{"mount://m/base/profiles"},
			ParamUris:   []string{"mount://m/base/params"},
			Conventions: conventionNaming("base", "^BASE_"),
		}),
	}
	leaf := proj("projects/leaf", "projects/base", &webapi.AnalysisConfig{
		ProfileUris: []string{"mount://m/leaf/profiles"},
		Conventions: conventionNaming("leaf", "^LEAF_"),
	})
	got, err := resolveExtends(context.Background(), store, leaf)
	if err != nil {
		t.Fatalf("resolveExtends: %v", err)
	}
	// Ref tiers ACCUMULATE: inheriting a profile set and adding your own means running both.
	if n := len(got.GetProfileUris()); n != 2 {
		t.Errorf("inherited and own profiles should both survive, got %v", got.GetProfileUris())
	}
	if got.GetProfileUris()[0] != "mount://m/base/profiles" {
		t.Errorf("the inherited set should come first, got %v", got.GetProfileUris())
	}
	// A tier the leaf never mentioned is inherited whole.
	if n := len(got.GetParamUris()); n != 1 {
		t.Errorf("a tier only the base declares should be inherited, got %v", got.GetParamUris())
	}
	// The convention REPLACES rather than accumulating: two naming vocabularies cannot both be in
	// effect, so the nearest declaration wins.
	if got.GetConventions().GetName() != "leaf" {
		t.Errorf("the leaf's convention should win, got %q", got.GetConventions().GetName())
	}
	// extends is cleared, so nothing re-walks a chain that has already been followed.
	if got.GetExtends() != "" {
		t.Errorf("extends should not survive onto the composed config, got %q", got.GetExtends())
	}
}

// TestExtendsInheritsWhatTheLeafOmits: a project that declares only `extends` gets the whole parent
// config, which is the case shared config exists for.
func TestExtendsInheritsWhatTheLeafOmits(t *testing.T) {
	store := projectTable{
		"projects/house": proj("projects/house", "", &webapi.AnalysisConfig{
			ProfileUris: []string{"mount://m/house/profiles"},
			Conventions: conventionNaming("house", "^H_"),
		}),
	}
	got, err := resolveExtends(context.Background(), store, proj("projects/board", "projects/house", nil))
	if err != nil {
		t.Fatalf("resolveExtends: %v", err)
	}
	if len(got.GetProfileUris()) != 1 || got.GetConventions().GetName() != "house" {
		t.Errorf("a bare extends should inherit everything, got %+v", got)
	}
}

// TestExtendsRefusesACycle. A config that silently stopped resolving partway would compose a subset
// of what the operator declared, and the run would look clean for a reason nobody could see.
func TestExtendsRefusesACycle(t *testing.T) {
	store := projectTable{
		"projects/a": proj("projects/a", "projects/b", nil),
		"projects/b": proj("projects/b", "projects/a", nil),
	}
	_, err := resolveExtends(context.Background(), store, store["projects/a"])
	if err == nil {
		t.Fatal("a cycle must be an error")
	}
	if !strings.Contains(err.Error(), "cycle") || !strings.Contains(err.Error(), "projects/b") {
		t.Errorf("the error should name the loop, got %v", err)
	}
}

// TestExtendsRefusesSelfReference is the one-node cycle, which a naive visited-set that only recorded
// the PARENT would miss.
func TestExtendsRefusesSelfReference(t *testing.T) {
	me := proj("projects/me", "projects/me", nil)
	_, err := resolveExtends(context.Background(), projectTable{"projects/me": me}, me)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Errorf("a project extending itself must be a cycle, got %v", err)
	}
}

// TestExtendsBoundsDepth: a legitimately deep chain is also a smell. Config a reader has to open six
// files to understand is config nobody will reason about correctly.
func TestExtendsBoundsDepth(t *testing.T) {
	store := projectTable{}
	for i := range 8 {
		name := "projects/p" + string(rune('0'+i))
		next := "projects/p" + string(rune('0'+i+1))
		store[name] = proj(name, next, nil)
	}
	_, err := resolveExtends(context.Background(), store, store["projects/p0"])
	if err == nil || !strings.Contains(err.Error(), "levels deep") {
		t.Errorf("a chain past the bound must be an error, got %v", err)
	}
}

// TestExtendsRefusesAnAbsentParent and a store that cannot answer at all. Both resolve to "the config
// the operator declared is not what would run", which has to be loud.
func TestExtendsRefusesWhatItCannotReach(t *testing.T) {
	leaf := proj("projects/leaf", "projects/missing", nil)
	if _, err := resolveExtends(context.Background(), projectTable{}, leaf); err == nil ||
		!strings.Contains(err.Error(), "does not exist") {
		t.Errorf("an absent parent must be an error, got %v", err)
	}
	if _, err := resolveExtends(context.Background(), nil, leaf); err == nil ||
		!strings.Contains(err.Error(), "no project store") {
		t.Errorf("a host with no store must refuse a declared extends, got %v", err)
	}
	// A project that extends NOTHING is unaffected by either, which is why a nil store is not fatal
	// on its own.
	if _, err := resolveExtends(context.Background(), nil, proj("projects/solo", "", nil)); err != nil {
		t.Errorf("a project that inherits nothing should not need a store: %v", err)
	}
}
