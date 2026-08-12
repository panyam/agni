package service

import (
	"context"
	"testing"

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

// TestSymbolPathsReachTheRead is the point of putting symbol libraries in analysis config: they have
// to arrive BEFORE the design is parsed, because an unresolved symbol changes what the design
// contains rather than what is checked about it.
//
// A schematic naming a library nothing resolves reads SHORT — the components it could not resolve are
// simply absent, every rule then evaluates cleanly over the shortened read, and the run reports fewer
// findings with no error to explain them. That is why this tier is worth carrying at all.
func TestSymbolPathsReachTheRead(t *testing.T) {
	project := &webapi.Project{
		Name:   "projects/p",
		Config: &webapi.AnalysisConfig{SymbolPathUris: []string{"mount://m/p/symbols"}},
	}
	ov, err := OverlayFor(context.Background(), symbolResolver{dirs: []string{"/host/p/symbols"}}, nil, project, &webapi.Design{}, nil, Overlay{}, "")
	if err != nil {
		t.Fatalf("OverlayFor: %v", err)
	}
	got := ReadOpts(ov.ReadOptions()...)
	if len(got.SymbolPaths) != 1 || got.SymbolPaths[0] != "/host/p/symbols" {
		t.Errorf("a project's declared symbol library must reach the read, got %v", got.SymbolPaths)
	}
}

// TestSymbolPathsAccumulate: a request naming a library is adding somewhere to look, not replacing
// where the project already looks. A design that resolved half its symbols would read short in
// exactly the silent way this tier exists to prevent.
func TestSymbolPathsAccumulate(t *testing.T) {
	project := &webapi.Project{
		Name:   "projects/p",
		Config: &webapi.AnalysisConfig{SymbolPathUris: []string{"mount://m/p/symbols"}},
	}
	req := &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{SymbolPathUris: []string{"mount://m/extra"}}}
	ov, err := OverlayFor(context.Background(), symbolResolver{dirs: []string{"/host/one"}}, nil, project, &webapi.Design{}, req, Overlay{}, "")
	if err != nil {
		t.Fatalf("OverlayFor: %v", err)
	}
	if n := len(ReadOpts(ov.ReadOptions()...).SymbolPaths); n != 2 {
		t.Errorf("project and request symbol paths should both reach the read, got %d", n)
	}
}

// TestSymbolPathsNeedAResolver: naming a directory is a ref, so it falls under the same refusal every
// other ref tier does. Silently reading without the library is the failure this must not have.
func TestSymbolPathsNeedAResolver(t *testing.T) {
	req := &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{SymbolPathUris: []string{"mount://m/symbols"}}}
	if _, err := OverlayFor(context.Background(), nil, nil, nil, nil, req, Overlay{}, ""); err == nil {
		t.Error("a host that cannot resolve a symbol directory must refuse rather than read short")
	}
}
