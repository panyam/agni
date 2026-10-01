package dsservice

import (
	"context"
	"errors"
	"github.com/panyam/agni/artifact"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/core/param"
	docpb "github.com/panyam/agni/datasheet/gen/go/agni/v1/doc"
	dsapi "github.com/panyam/agni/datasheet/gen/go/agni/v1/dsapi"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// fakeWorkspace is a fixed two-mount listing, so the folder-tree rpcs can be checked against the
// engine's WorkspaceService over the same port.
type fakeWorkspace struct{}

func (fakeWorkspace) Mounts() []service.MountInfo {
	return []service.MountInfo{{Name: "ds", Root: "/ds"}, {Name: "boards", Root: "/boards"}}
}

func (fakeWorkspace) ListDir(context.Context, artifact.URI) ([]service.DirEntry, error) {
	return []service.DirEntry{{Name: "LM1117.pdf"}, {Name: "ti", IsDir: true}}, nil
}

// fakeDocLoader is a DocLoader whose result is fixed per test, so GetDocument's classification and
// extracted/not-extracted mapping are exercised without the OS adapter.
type fakeDocLoader struct {
	doc *docpb.Document
	err error
}

func (f *fakeDocLoader) Document(context.Context, artifact.URI) (*docpb.Document, error) {
	return f.doc, f.err
}

// fakeDraftStore stands in for the OS store; saveErr drives the conflict path and publishErr the
// publish outcomes.
type fakeDraftStore struct {
	draft      *dsapi.Draft
	found      bool
	saveErr    error
	saved      *dsapi.Draft
	publishErr error
}

func (f *fakeDraftStore) Get(context.Context, string) (*dsapi.Draft, bool, error) {
	return f.draft, f.found, nil
}

func (f *fakeDraftStore) ListByDocument(context.Context, string) ([]*dsapi.Draft, error) {
	if f.draft == nil {
		return nil, nil
	}
	return []*dsapi.Draft{f.draft}, nil
}

func (f *fakeDraftStore) Save(_ context.Context, d *dsapi.Draft, _ string) (string, error) {
	if f.saveErr != nil {
		return "", f.saveErr
	}
	f.saved = d
	return "v2", nil
}

func (f *fakeDraftStore) Publish(context.Context, string) (*Published, error) {
	if f.publishErr != nil {
		return nil, f.publishErr
	}
	return &Published{Generation: 7}, nil
}

// fakeDocExtractor stands in for the OS extractor; `available` drives the gate, `doc`/`err` the run.
type fakeDocExtractor struct {
	doc       *docpb.Document
	available bool
	err       error
}

func (f *fakeDocExtractor) Available() bool { return f.available }
func (f *fakeDocExtractor) Extract(context.Context, artifact.URI) (*docpb.Document, error) {
	return f.doc, f.err
}

// fakeAnnotationStore stands in for the OS annotation store; `sets` is what Get returns (the union),
// and `saved`/`author` capture the last SaveAnnotations for assertions.
type fakeAnnotationStore struct {
	sets   []*dsapi.AnnotationSet
	saved  *dsapi.AnnotationSet
	author string
}

func (f *fakeAnnotationStore) Get(context.Context, artifact.URI) ([]*dsapi.AnnotationSet, error) {
	return f.sets, nil
}

func (f *fakeAnnotationStore) Save(_ context.Context, _ artifact.URI, author string, set *dsapi.AnnotationSet) error {
	f.author = author
	f.saved = set
	return nil
}

// newDS builds a DatasheetService with throwaway store/extractor for the GetDocument-focused tests.
func newDS(l DocLoader) *DatasheetService {
	return NewDatasheetService(l, &fakeDraftStore{}, &fakeDocExtractor{}, &fakeAnnotationStore{}, fakeWorkspace{})
}

func TestGetDocumentExtracted(t *testing.T) {
	doc := &docpb.Document{ContentHash: "sha256:abc", Producer: "hand", PageCount: 1}
	svc := newDS(&fakeDocLoader{doc: doc})
	resp, err := svc.GetDocument(context.Background(), &dsapi.GetDocumentRequest{Uri: "mount://m/d.pdf"})
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if !resp.Extracted {
		t.Errorf("extracted = false, want true when a doc-IR exists")
	}
	if resp.GetDocument().GetContentHash() != "sha256:abc" {
		t.Errorf("document not returned: %+v", resp.GetDocument())
	}
}

func TestGetDocumentNotExtracted(t *testing.T) {
	svc := newDS(&fakeDocLoader{doc: nil})
	resp, err := svc.GetDocument(context.Background(), &dsapi.GetDocumentRequest{Uri: "mount://m/d.pdf"})
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if resp.Extracted {
		t.Errorf("extracted = true, want false for a datasheet with no doc-IR")
	}
	if resp.Document != nil {
		t.Errorf("document should be unset when not extracted, got %+v", resp.Document)
	}
}

func TestGetDocumentClassifiesErrors(t *testing.T) {
	// An unclassified loader error (a parse failure) maps to service.ErrInvalidArgument for the transport.
	parseErr := newDS(&fakeDocLoader{err: errors.New("bad textproto")})
	if _, err := parseErr.GetDocument(context.Background(), &dsapi.GetDocumentRequest{Uri: "mount://m/d.pdf"}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("parse error => %v, want service.ErrInvalidArgument", err)
	}
	// An already-classified error (unknown mount) keeps its classification.
	notFound := newDS(&fakeDocLoader{err: service.ErrNotFound})
	if _, err := notFound.GetDocument(context.Background(), &dsapi.GetDocumentRequest{Uri: "mount://m/d.pdf"}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("not-found => %v, want service.ErrNotFound", err)
	}
}

func TestGetDraftFound(t *testing.T) {
	store := &fakeDraftStore{draft: &dsapi.Draft{Mpn: "LM1117", Spec: &parampb.PartSpec{Mpn: "LM1117"}, Version: "v1"}, found: true}
	svc := NewDatasheetService(&fakeDocLoader{}, store, &fakeDocExtractor{}, &fakeAnnotationStore{}, fakeWorkspace{})
	resp, err := svc.GetDraft(context.Background(), &dsapi.GetDraftRequest{Mpn: "LM1117"})
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}
	if !resp.Found || resp.GetDraft().GetMpn() != "LM1117" || resp.GetDraft().GetVersion() != "v1" {
		t.Errorf("got %v", resp)
	}
}

func TestSaveDraftConflictAndShape(t *testing.T) {
	ctx := context.Background()
	draft := func(key, specMPN string) *dsapi.SaveDraftRequest {
		return &dsapi.SaveDraftRequest{Draft: &dsapi.Draft{Mpn: key, Spec: &parampb.PartSpec{Mpn: specMPN}, DocumentUris: []string{"mount://m/d.pdf"}}}
	}
	// A store conflict propagates as ErrConflict (the transport maps it to Aborted, "refetch").
	conflict := NewDatasheetService(&fakeDocLoader{}, &fakeDraftStore{saveErr: ErrConflict}, &fakeDocExtractor{}, &fakeAnnotationStore{}, fakeWorkspace{})
	if _, err := conflict.SaveDraft(ctx, draft("X", "X")); !errors.Is(err, ErrConflict) {
		t.Errorf("store conflict => %v, want ErrConflict", err)
	}
	svc := NewDatasheetService(&fakeDocLoader{}, &fakeDraftStore{}, &fakeDocExtractor{}, &fakeAnnotationStore{}, fakeWorkspace{})
	for name, req := range map[string]*dsapi.SaveDraftRequest{
		"no draft":                     {},
		"no mpn":                       draft("", ""),
		"a spec naming another MPN":    draft("LM1117", "LM317"),
		"a document that is not a URI": {Draft: &dsapi.Draft{Mpn: "X", Spec: &parampb.PartSpec{Mpn: "X"}, DocumentUris: []string{"not a uri"}}},
	} {
		if _, err := svc.SaveDraft(ctx, req); err == nil {
			t.Errorf("%s: saved, want a refusal", name)
		}
	}
	// The key matches the spec case-insensitively, as every MPN comparison does.
	if _, err := svc.SaveDraft(ctx, draft("lm1117", "LM1117")); err != nil {
		t.Errorf("a case difference between key and spec was refused: %v", err)
	}
}

// Every draft rpc on a server with no corpus says so, rather than reading as no drafts.
func TestDraftRPCsNeedACorpus(t *testing.T) {
	ctx := context.Background()
	svc := NewDatasheetService(&fakeDocLoader{}, nil, &fakeDocExtractor{}, &fakeAnnotationStore{}, fakeWorkspace{})
	_, e1 := svc.GetDraft(ctx, &dsapi.GetDraftRequest{Mpn: "X"})
	_, e2 := svc.ListDrafts(ctx, &dsapi.ListDraftsRequest{DocumentUri: "mount://m/d.pdf"})
	_, e3 := svc.SaveDraft(ctx, &dsapi.SaveDraftRequest{})
	_, e4 := svc.PublishDraft(ctx, &dsapi.PublishDraftRequest{Mpn: "X"})
	for i, err := range []error{e1, e2, e3, e4} {
		if !errors.Is(err, ErrNoCorpus) {
			t.Errorf("rpc %d without a corpus = %v, want ErrNoCorpus", i+1, err)
		}
	}
}

// A refused publish is an answer for the author, carried in the response with its problems, not a
// transport error; a store failure is still an error.
func TestPublishDraftReportsARefusal(t *testing.T) {
	ctx := context.Background()
	refused := &PublishRefused{Reason: "not ready", Problems: []param.Problem{{Kind: param.ProblemCompleteness, Message: "no provenance"}}}
	svc := NewDatasheetService(&fakeDocLoader{}, &fakeDraftStore{publishErr: refused}, &fakeDocExtractor{}, &fakeAnnotationStore{}, fakeWorkspace{})
	resp, err := svc.PublishDraft(ctx, &dsapi.PublishDraftRequest{Mpn: "X"})
	if err != nil || resp.GetPublished() || resp.GetReason() != "not ready" || len(resp.GetProblems()) != 1 {
		t.Errorf("refusal = %v, %v", resp, err)
	}
	ok := NewDatasheetService(&fakeDocLoader{}, &fakeDraftStore{}, &fakeDocExtractor{}, &fakeAnnotationStore{}, fakeWorkspace{})
	if resp, err := ok.PublishDraft(ctx, &dsapi.PublishDraftRequest{Mpn: "X"}); err != nil || !resp.GetPublished() || resp.GetGeneration() != 7 {
		t.Errorf("publish = %v, %v", resp, err)
	}
	failing := NewDatasheetService(&fakeDocLoader{}, &fakeDraftStore{publishErr: errors.New("disk full")}, &fakeDocExtractor{}, &fakeAnnotationStore{}, fakeWorkspace{})
	if _, err := failing.PublishDraft(ctx, &dsapi.PublishDraftRequest{Mpn: "X"}); err == nil {
		t.Error("a store failure was answered as a refusal")
	}
}

func TestExtractDocIRGated(t *testing.T) {
	// No producer configured -> ErrExtractNotEnabled (transport maps it to FailedPrecondition).
	off := NewDatasheetService(&fakeDocLoader{}, &fakeDraftStore{}, &fakeDocExtractor{available: false}, &fakeAnnotationStore{}, fakeWorkspace{})
	if _, err := off.ExtractDocIR(context.Background(), &dsapi.ExtractDocIRRequest{Uri: "mount://m/d.pdf"}); !errors.Is(err, ErrExtractNotEnabled) {
		t.Errorf("disabled => %v, want ErrExtractNotEnabled", err)
	}
	// Configured -> returns the produced doc-IR.
	produced := &docpb.Document{ContentHash: "sha256:x", Producer: "docling"}
	on := NewDatasheetService(&fakeDocLoader{}, &fakeDraftStore{}, &fakeDocExtractor{available: true, doc: produced}, &fakeAnnotationStore{}, fakeWorkspace{})
	resp, err := on.ExtractDocIR(context.Background(), &dsapi.ExtractDocIRRequest{Uri: "mount://m/d.pdf"})
	if err != nil || resp.GetDocument().GetContentHash() != "sha256:x" {
		t.Fatalf("extract: resp=%v err=%v", resp, err)
	}
}

func TestGetDocumentReportsExtractAvailable(t *testing.T) {
	on := NewDatasheetService(&fakeDocLoader{doc: nil}, &fakeDraftStore{}, &fakeDocExtractor{available: true}, &fakeAnnotationStore{}, fakeWorkspace{})
	resp, _ := on.GetDocument(context.Background(), &dsapi.GetDocumentRequest{Uri: "mount://m/d.pdf"})
	if !resp.ExtractAvailable {
		t.Error("extract_available should be true when a producer is configured")
	}
	off := newDS(&fakeDocLoader{doc: nil}) // newDS uses a disabled extractor
	resp2, _ := off.GetDocument(context.Background(), &dsapi.GetDocumentRequest{Uri: "mount://m/d.pdf"})
	if resp2.ExtractAvailable {
		t.Error("extract_available should be false with no producer")
	}
}

func TestSaveAnnotationsValidation(t *testing.T) {
	svc := NewDatasheetService(&fakeDocLoader{}, &fakeDraftStore{}, &fakeDocExtractor{}, &fakeAnnotationStore{}, fakeWorkspace{})
	// A nil set is rejected before the store.
	if _, err := svc.SaveAnnotations(context.Background(), &dsapi.SaveAnnotationsRequest{Uri: "mount://m/d"}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("nil set => %v, want service.ErrInvalidArgument", err)
	}
	// An empty author is rejected, because the author names the file and cannot be inferred.
	req := &dsapi.SaveAnnotationsRequest{Set: &dsapi.AnnotationSet{DocId: "LM1117"}}
	if _, err := svc.SaveAnnotations(context.Background(), req); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("empty author => %v, want service.ErrInvalidArgument", err)
	}
}

func TestSaveAndGetAnnotations(t *testing.T) {
	store := &fakeAnnotationStore{}
	svc := NewDatasheetService(&fakeDocLoader{}, &fakeDraftStore{}, &fakeDocExtractor{}, store, fakeWorkspace{})
	set := &dsapi.AnnotationSet{DocId: "LM1117", Author: "alice", Annotations: []*dsapi.RegionAnnotation{{RegionId: "p4.t1", Type: "table"}}}
	if _, err := svc.SaveAnnotations(context.Background(), &dsapi.SaveAnnotationsRequest{Uri: "mount://m/d.pdf", Set: set}); err != nil {
		t.Fatalf("SaveAnnotations: %v", err)
	}
	if store.author != "alice" || store.saved.GetAnnotations()[0].GetRegionId() != "p4.t1" {
		t.Errorf("store got author=%q saved=%v", store.author, store.saved)
	}
	// GetAnnotations returns the union the store provides (one set per author).
	store.sets = []*dsapi.AnnotationSet{{Author: "alice"}, {Author: "bob"}}
	resp, err := svc.GetAnnotations(context.Background(), &dsapi.GetAnnotationsRequest{Uri: "mount://m/d.pdf"})
	if err != nil {
		t.Fatalf("GetAnnotations: %v", err)
	}
	if len(resp.GetSets()) != 2 {
		t.Errorf("union = %d sets, want 2", len(resp.GetSets()))
	}
}

// Saving records what the author has. It is NOT a judgment about whether the spec is any good, so
// neither incompleteness nor structural incoherence may block a write. A rejected save costs work,
// and every mutation path would otherwise have to preserve an invariant or strand the document.
//
// Nothing downstream needs the gate. The sibling is <stem>.partspec.json and param.LoadSet reads
// *.textproto, so a draft cannot reach the corpus by sitting on disk; promotion is a separate step
// and that is where param.Validate belongs.
func TestSaveDraftRecordsWhateverTheAuthorHas(t *testing.T) {
	svc := NewDatasheetService(&fakeDocLoader{}, &fakeDraftStore{}, &fakeDocExtractor{}, &fakeAnnotationStore{}, fakeWorkspace{})
	save := func(spec *parampb.PartSpec) error {
		spec.Mpn = "D1" // the key; everything else is whatever the author has so far
		_, err := svc.SaveDraft(context.Background(), &dsapi.SaveDraftRequest{Draft: &dsapi.Draft{Mpn: "D1", Spec: spec}})
		return err
	}

	// No parameters: the ordinary state of a datasheet someone has started transcribing.
	if err := save(&parampb.PartSpec{
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "d"}},
		Pins: []*parampb.Pin{{Id: "vcc", Name: "VCC"}},
	}); err != nil {
		t.Errorf("an incomplete spec must save; got %v", err)
	}

	// Structurally incoherent too. param.Validate would reject both of these, and that is the right
	// answer for loading a corpus and the wrong one for persisting a draft.
	if err := save(&parampb.PartSpec{
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "d"}},
		Pins: []*parampb.Pin{{Id: "vcc", Name: "VCC"}, {Id: "vcc", Name: "VCC2"}},
	}); err != nil {
		t.Errorf("a duplicate pin id is a problem to SHOW, not one to refuse a save over; got %v", err)
	}
	if err := save(&parampb.PartSpec{
		Docs:       []*parampb.SourceDoc{{Id: "ds", Title: "d"}},
		Pins:       []*parampb.Pin{{Id: "vcc", Name: "VCC"}},
		Parameters: []*parampb.Parameter{{Symbol: "VCC", PinRefs: []string{"ghost"}}},
	}); err != nil {
		t.Errorf("a dangling binding must not strand the document; got %v", err)
	}
}

// The save response is where the editor learns what is wrong, so the two kinds have to arrive
// distinguishable. Structural problems are worth interrupting for, and completeness ones are the
// ordinary state of unfinished work.
func TestSaveDraftReportsClassifiedProblems(t *testing.T) {
	svc := NewDatasheetService(&fakeDocLoader{}, &fakeDraftStore{}, &fakeDocExtractor{}, &fakeAnnotationStore{}, fakeWorkspace{})
	resp, err := svc.SaveDraft(context.Background(), &dsapi.SaveDraftRequest{Draft: &dsapi.Draft{
		Mpn: "D1",
		Spec: &parampb.PartSpec{ // a row with no provenance (incomplete) AND a duplicate pin id (incoherent)
			Mpn:        "D1",
			Docs:       []*parampb.SourceDoc{{Id: "ds", Title: "d"}},
			Pins:       []*parampb.Pin{{Id: "vcc", Name: "VCC"}, {Id: "vcc", Name: "VCC2"}},
			Parameters: []*parampb.Parameter{{Symbol: "VIN"}},
		},
	}})
	if err != nil {
		t.Fatalf("save must succeed regardless of problems: %v", err)
	}
	byKind := map[dsapi.ValidationProblem_Kind][]string{}
	for _, p := range resp.GetProblems() {
		byKind[p.GetKind()] = append(byKind[p.GetKind()], p.GetMessage())
	}
	if got := strings.Join(byKind[dsapi.ValidationProblem_KIND_STRUCTURAL], " "); !strings.Contains(got, "duplicate pin id") {
		t.Errorf("structural problems = %q, want the duplicate pin id", got)
	}
	if got := byKind[dsapi.ValidationProblem_KIND_COMPLETENESS]; len(got) == 0 {
		t.Errorf("no completeness problems for a row with no limit kind or provenance")
	}
	// A spec good enough to load reports nothing, so the editor shows an empty panel rather than
	// having to filter noise.
	clean, _ := svc.SaveDraft(context.Background(), &dsapi.SaveDraftRequest{Draft: &dsapi.Draft{Mpn: "ACME-1", Spec: cleanSpec()}})
	if n := len(clean.GetProblems()); n != 0 {
		t.Errorf("a corpus-ready spec reports %d problems, want 0: %v", n, clean.GetProblems())
	}
}

// cleanSpec is the minimum spec param.Validate accepts, so the no-problems case is asserted against
// something real rather than against an empty message.
func cleanSpec() *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	return &parampb.PartSpec{
		Mpn:  "ACME-1",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "d"}},
		Parameters: []*parampb.Parameter{{
			Symbol:    "VIN",
			LimitKind: parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
			Value:     &parampb.RangeValue{Max: f(20)},
			Unit:      "V",
			Prov:      &parampb.ParamProvenance{DocRef: "ds", Page: 1, Method: "hand", Confidence: 1},
		}},
	}
}

// The workbench's folder tree answers exactly as the viewer's, because both list through one port
// with the engine's WorkspaceService (agni issue 744).
func TestFolderTreeAnswersAsWorkspaceServiceDoes(t *testing.T) {
	ctx := context.Background()
	ds := NewDatasheetService(&fakeDocLoader{}, &fakeDraftStore{}, &fakeDocExtractor{}, &fakeAnnotationStore{}, fakeWorkspace{})
	ws := service.NewWorkspaceService(fakeWorkspace{})

	gotM, err := ds.ListMounts(ctx, &webapi.ListMountsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	wantM, _ := ws.ListMounts(ctx, &webapi.ListMountsRequest{})
	if len(gotM.GetMounts()) != 2 || !proto.Equal(gotM, wantM) {
		t.Errorf("ListMounts = %v, want %v", gotM, wantM)
	}
	req := &webapi.ListDirRequest{Uri: "mount://ds/"}
	gotD, err := ds.ListDir(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	wantD, _ := ws.ListDir(ctx, req)
	if len(gotD.GetEntries()) == 0 || !proto.Equal(gotD, wantD) {
		t.Errorf("ListDir = %v, want %v", gotD, wantD)
	}
}
