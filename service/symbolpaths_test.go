package service

import (
	"context"
	"testing"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// symbolResolver resolves symbol_path_uris to fixed host paths, standing in for the OS adapter.
type symbolResolver struct{ dirs []string }

func (r symbolResolver) ResolveConfig(_ context.Context, cfg *webapi.AnalysisConfig, _ string) (ResolvedConfig, error) {
	var out ResolvedConfig
	for range cfg.GetSymbolPathUris() {
		out.SymbolPaths = append(out.SymbolPaths, r.dirs...)
	}
	return out, nil
}

// TestSymbolPathsReachTheRead shows why symbol libraries go in analysis config. They have
// to arrive BEFORE the design is parsed, because an unresolved symbol changes what the design
// contains rather than what is checked about it.
//
// Imagine a schematic naming a library that nothing resolves. The read comes back SHORT, because the
// components it could not resolve are simply absent. Every rule then evaluates cleanly over the
// shortened read, and the run reports fewer findings with no error to explain them. This tier exists
// to prevent that.
func TestSymbolPathsReachTheRead(t *testing.T) {
	project := &webapi.Project{
		Name:   "projects/p",
		Config: &webapi.AnalysisConfig{SymbolPathUris: []string{"mount://m/p/symbols"}},
	}
	ov, err := OverlayFor(context.Background(), symbolResolver{dirs: []string{"/host/p/symbols"}}, nil, project, &webapi.Design{}, nil, "")
	if err != nil {
		t.Fatalf("OverlayFor: %v", err)
	}
	got := ReadOpts(ov.ReadOptions()...)
	if len(got.SymbolPaths) != 1 || got.SymbolPaths[0] != "/host/p/symbols" {
		t.Errorf("a project's declared symbol library must reach the read, got %v", got.SymbolPaths)
	}
}

// TestSymbolPathsAccumulate checks that a request naming a library adds a place to look. It must not
// replace the places the project already looks. A design that resolved half its symbols would read
// short with no error, and this tier exists to prevent that.
func TestSymbolPathsAccumulate(t *testing.T) {
	project := &webapi.Project{
		Name:   "projects/p",
		Config: &webapi.AnalysisConfig{SymbolPathUris: []string{"mount://m/p/symbols"}},
	}
	req := &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{SymbolPathUris: []string{"mount://m/extra"}}}
	ov, err := OverlayFor(context.Background(), symbolResolver{dirs: []string{"/host/one"}}, nil, project, &webapi.Design{}, req, "")
	if err != nil {
		t.Fatalf("OverlayFor: %v", err)
	}
	if n := len(ReadOpts(ov.ReadOptions()...).SymbolPaths); n != 2 {
		t.Errorf("project and request symbol paths should both reach the read, got %d", n)
	}
}

// TestSymbolPathsNeedAResolver covers a request naming a directory. A directory is a ref, so the
// request gets the same refusal as every other ref tier. The run must never read on silently without
// the library.
func TestSymbolPathsNeedAResolver(t *testing.T) {
	req := &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{SymbolPathUris: []string{"mount://m/symbols"}}}
	if _, err := OverlayFor(context.Background(), nil, nil, nil, nil, req, ""); err == nil {
		t.Error("a host that cannot resolve a symbol directory must refuse rather than read short")
	}
}

// TestADesignInNoProjectKeepsItsOwnConfig covers agni issue 887. A design.yaml with no project.yaml
// above it still declares its symbol library and its intent, and both must reach the run. Before, a
// projectless design composed nothing of its own, so it drew with its externally-symboled parts
// missing and its intent rules never ran, with no error to say so.
func TestADesignInNoProjectKeepsItsOwnConfig(t *testing.T) {
	design := &webapi.Design{
		Name: "designs/g",
		Config: &webapi.AnalysisConfig{
			SymbolPathUris: []string{"mount://m/g/symbols"},
			Intent:         &configpb.DesignIntent{Modules: []*configpb.IntentModule{{Name: "MCU", Class: "ic"}}},
		},
	}
	ov, err := OverlayFor(context.Background(), symbolResolver{dirs: []string{"/host/g/symbols"}}, nil, nil, design, nil, "")
	if err != nil {
		t.Fatalf("OverlayFor: %v", err)
	}
	if got := ReadOpts(ov.ReadOptions()...); len(got.SymbolPaths) != 1 || got.SymbolPaths[0] != "/host/g/symbols" {
		t.Errorf("the design's own symbol library must reach the read, got %v", got.SymbolPaths)
	}
	if !ov.Intent || ov.DesignIntent == nil {
		t.Errorf("the design's own intent must compose, got Intent=%v", ov.Intent)
	}
}
