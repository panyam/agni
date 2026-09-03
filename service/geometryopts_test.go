package service

import (
	"context"
	"sync"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/expect"
)

// recordingLoader answers every Loader method with a fixed fixture and records the ReadOptions each
// call received, keyed by method. It is the only way to observe config reaching a read: the options
// are consumed inside the adapter, so a test that looked at the RESULT would be asserting on the
// reader's symbol resolution rather than on whether the service passed the config at all.
type recordingLoader struct {
	mu   sync.Mutex
	seen map[string][]ReadOptions
}

func newRecordingLoader() *recordingLoader {
	return &recordingLoader{seen: map[string][]ReadOptions{}}
}

func (l *recordingLoader) record(method string, opts []ReadOption) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen[method] = append(l.seen[method], ReadOpts(opts...))
}

// symbolPathsFor returns the symbol paths every recorded call to method saw, one entry per call.
func (l *recordingLoader) symbolPathsFor(method string) [][]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out [][]string
	for _, o := range l.seen[method] {
		out = append(out, o.SymbolPaths)
	}
	return out
}

func (l *recordingLoader) Design(_ context.Context, _ artifact.URI, opts ...ReadOption) (*ir.Design, error) {
	l.record("Design", opts)
	return &ir.Design{Name: "D", Nets: []*ir.Net{{Name: "N1"}}}, nil
}

func (l *recordingLoader) Geometry(_ context.Context, _ artifact.URI, _ string, _ bool, opts ...ReadOption) (*geom.SchematicGeometry, error) {
	l.record("Geometry", opts)
	return twoSheetGeom(), nil
}

func (l *recordingLoader) Report(_ context.Context, _ artifact.URI, _ bool, opts ...ReadOption) (*graph.ConversionReport, error) {
	l.record("Report", opts)
	return &graph.ConversionReport{}, nil
}

func (l *recordingLoader) Expectations(context.Context, artifact.URI) (*expect.Expectations, error) {
	return nil, nil
}

func (l *recordingLoader) Board(context.Context, artifact.URI) (*geom.BoardGeometry, error) {
	return nil, nil
}

func (l *recordingLoader) DesignHash(context.Context, artifact.URI) (string, error) {
	return "", nil
}

// stubProjectStore resolves every artifact to one project declaring one symbol library, standing in
// for the descriptor tree the FS store walks.
type stubProjectStore struct{ project *webapi.Project }

func (s stubProjectStore) Project(context.Context, string) (*webapi.Project, error) {
	return s.project, nil
}
func (s stubProjectStore) Projects(context.Context) ([]*webapi.Project, error) {
	return []*webapi.Project{s.project}, nil
}
func (s stubProjectStore) Design(context.Context, string) (*webapi.Design, error) {
	return &webapi.Design{}, nil
}
func (s stubProjectStore) Designs(context.Context, string) ([]*webapi.Design, error) {
	return []*webapi.Design{{}}, nil
}
func (s stubProjectStore) ResolveDesign(context.Context, artifact.URI) (*webapi.Design, *webapi.Project, error) {
	return &webapi.Design{}, s.project, nil
}

// symbolProjectResolver is a resolver whose every design belongs to a project declaring hostDir as
// its symbol library.
func symbolProjectResolver(hostDir string) *ProjectResolver {
	return &ProjectResolver{
		Store: stubProjectStore{project: &webapi.Project{
			Name:   "projects/p",
			Config: &webapi.AnalysisConfig{SymbolPathUris: []string{"mount://m/p/symbols"}},
		}},
		Config: symbolResolver{dirs: []string{hostDir}},
	}
}

// TestSymbolPathsReachTheRenderRead is the twin of TestSymbolPathsReachTheRead, for the render tier.
//
// The reason is the same sentence: an unresolved symbol changes what the design CONTAINS rather than
// what is checked about it. On the render side the consequence is sharper, because it is invisible. A
// placement whose symbol did not resolve contributes no shapes, so it is dropped from the document
// along with the entity keys that make it pickable, while the annotation pass still draws its
// reference designator. The sheet then looks complete and every component and pin on it is silently
// unclickable (agni issue 347).
//
// This assertion could not be written before: Loader.Geometry took no options, so there was no
// channel for a call site to pass config through and nothing for a test to observe.
func TestSymbolPathsReachTheRenderRead(t *testing.T) {
	uri := "mount://m/p/board.kicad_sch"
	cases := []struct {
		name   string
		method string
		call   func(*DesignService) error
	}{
		{"GetDesign", "Geometry", func(s *DesignService) error {
			_, err := s.GetDesign(context.Background(), &webapi.GetDesignRequest{Uri: uri})
			return err
		}},
		{"GetSheet", "Geometry", func(s *DesignService) error {
			_, err := s.GetSheet(context.Background(), &webapi.GetSheetRequest{Uri: uri})
			return err
		}},
		{"HighlightSheet", "Geometry", func(s *DesignService) error {
			_, err := s.HighlightSheet(context.Background(), &webapi.HighlightSheetRequest{Uri: uri})
			return err
		}},
		{"GetLayoutReport", "Report", func(s *DesignService) error {
			_, err := s.GetLayoutReport(context.Background(), &webapi.GetLayoutReportRequest{Uri: uri})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ld := newRecordingLoader()
			svc := NewDesignService(ld, noNative{}, render.Style{}, symbolProjectResolver("/host/p/symbols"))
			if err := tc.call(svc); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			got := ld.symbolPathsFor(tc.method)
			if len(got) == 0 {
				t.Fatalf("%s made no %s call to observe", tc.name, tc.method)
			}
			for i, paths := range got {
				if len(paths) != 1 || paths[0] != "/host/p/symbols" {
					t.Errorf("%s: %s call %d saw symbol paths %v, want [/host/p/symbols]", tc.name, tc.method, i, paths)
				}
			}
		})
	}
}

// TestSymbolPathsReachTheNetlistReadsBesideTheRender covers the reads that sit NEXT to the render on
// the same service and were equally unconfigured: GetDesign's component/net counts, and the netlist
// HighlightSheet reads to resolve net ids to names for a name-join.
//
// They are here rather than in a separate test because they share the defect's cause. The service had
// no resolver at all, so every read it made was unconfigured, and fixing only the ones the render
// draws from would leave the counts beside the drawing describing a different read.
func TestSymbolPathsReachTheNetlistReadsBesideTheRender(t *testing.T) {
	ld := newRecordingLoader()
	svc := NewDesignService(ld, noNative{}, render.Style{}, symbolProjectResolver("/host/p/symbols"))
	// A netlist extension, so GetDesign takes the auto-layout branch that reads the IR for its counts.
	uri := "mount://m/p/board.edn"
	if _, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Uri: uri}); err != nil {
		t.Fatal(err)
	}
	got := ld.symbolPathsFor("Design")
	if len(got) == 0 {
		t.Fatal("GetDesign made no netlist read to observe")
	}
	for i, paths := range got {
		if len(paths) != 1 || paths[0] != "/host/p/symbols" {
			t.Errorf("GetDesign netlist read %d saw symbol paths %v, want [/host/p/symbols]", i, paths)
		}
	}
}

// TestDiffReadsBothSidesConfigured is the folded-in half of agni issue 347.
//
// A diff rests entirely on its two netlist reads, and both were made with no options. A project whose
// symbol library did not resolve was therefore compared from two reads that each lose every
// connection through the affected parts, so a revision that CHANGED such a connection showed no
// change at all. `agni diff` on the CLI reads through readDesign and is configured, so the two
// surfaces answered the same question differently — the service-side survivor of agni issue 228.
func TestDiffReadsBothSidesConfigured(t *testing.T) {
	ld := newRecordingLoader()
	svc := NewDiffService(ld, symbolProjectResolver("/host/p/symbols"))
	_, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{
		AUri: "mount://m/p/a.edn",
		BUri: "mount://m/p/b.edn",
	})
	if err != nil {
		t.Fatal(err)
	}
	reads := ld.symbolPathsFor("Design")
	if len(reads) != 2 {
		t.Fatalf("a diff reads exactly two netlists, observed %d", len(reads))
	}
	for i, paths := range reads {
		if len(paths) != 1 || paths[0] != "/host/p/symbols" {
			t.Errorf("diff side %d read with symbol paths %v, want [/host/p/symbols]", i, paths)
		}
	}
	// The geometry each side is annotated against has to come from the SAME config as its netlist,
	// or a change would be badged onto a sheet drawn from a shorter read than the one that found it.
	for i, paths := range ld.symbolPathsFor("Geometry") {
		if len(paths) != 1 || paths[0] != "/host/p/symbols" {
			t.Errorf("diff geometry read %d saw symbol paths %v, want [/host/p/symbols]", i, paths)
		}
	}
}

// TestNoProjectResolverReadsUnderDefaults is the positive control for the three tests above. A design
// belonging to no project genuinely has no config, and that has to stay a clean read rather than
// becoming an error or inheriting another project's library.
//
// Without it the assertions above would also pass if the service had started passing some FIXED set
// of paths, which is the failure mode a "config arrived" test cannot otherwise distinguish.
func TestNoProjectResolverReadsUnderDefaults(t *testing.T) {
	ld := newRecordingLoader()
	svc := NewDesignService(ld, noNative{}, render.Style{}, nil)
	if _, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{Uri: "mount://m/loose.kicad_sch"}); err != nil {
		t.Fatal(err)
	}
	for i, paths := range ld.symbolPathsFor("Geometry") {
		if len(paths) != 0 {
			t.Errorf("a design with no project must read under the defaults; call %d saw %v", i, paths)
		}
	}
}
