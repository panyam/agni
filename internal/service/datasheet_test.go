package service

import (
	"context"
	"errors"
	"github.com/panyam/agni/internal/artifact"
	"testing"

	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// fakeDocLoader is a DocLoader whose result is fixed per test, so GetDocument's classification and
// extracted/not-extracted mapping are exercised without the OS adapter.
type fakeDocLoader struct {
	doc *docpb.Document
	err error
}

func (f *fakeDocLoader) Document(context.Context, artifact.URI) (*docpb.Document, error) {
	return f.doc, f.err
}

// fakePartSpecStore stands in for the OS store; saveErr lets a test drive the conflict path.
type fakePartSpecStore struct {
	spec    *parampb.PartSpec
	found   bool
	saveErr error
	saved   *parampb.PartSpec
}

func (f *fakePartSpecStore) Get(context.Context, artifact.URI) (*parampb.PartSpec, string, bool, error) {
	return f.spec, "v1", f.found, nil
}

func (f *fakePartSpecStore) Save(_ context.Context, _ artifact.URI, spec *parampb.PartSpec, _ string) (string, error) {
	if f.saveErr != nil {
		return "", f.saveErr
	}
	f.saved = spec
	return "v2", nil
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
	sets   []*webapi.AnnotationSet
	saved  *webapi.AnnotationSet
	author string
}

func (f *fakeAnnotationStore) Get(context.Context, artifact.URI) ([]*webapi.AnnotationSet, error) {
	return f.sets, nil
}

func (f *fakeAnnotationStore) Save(_ context.Context, _ artifact.URI, author string, set *webapi.AnnotationSet) error {
	f.author = author
	f.saved = set
	return nil
}

// newDS builds a DatasheetService with throwaway store/extractor for the GetDocument-focused tests.
func newDS(l DocLoader) *DatasheetService {
	return NewDatasheetService(l, &fakePartSpecStore{}, &fakeDocExtractor{}, &fakeAnnotationStore{})
}

func TestGetDocumentExtracted(t *testing.T) {
	doc := &docpb.Document{ContentHash: "sha256:abc", Producer: "hand", PageCount: 1}
	svc := newDS(&fakeDocLoader{doc: doc})
	resp, err := svc.GetDocument(context.Background(), &webapi.GetDocumentRequest{Uri: "mount://m/d.pdf"})
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
	resp, err := svc.GetDocument(context.Background(), &webapi.GetDocumentRequest{Uri: "mount://m/d.pdf"})
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
	// An unclassified loader error (a parse failure) maps to ErrInvalidArgument for the transport.
	parseErr := newDS(&fakeDocLoader{err: errors.New("bad textproto")})
	if _, err := parseErr.GetDocument(context.Background(), &webapi.GetDocumentRequest{Uri: "mount://m/d.pdf"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("parse error => %v, want ErrInvalidArgument", err)
	}
	// An already-classified error (unknown mount) keeps its classification.
	notFound := newDS(&fakeDocLoader{err: ErrNotFound})
	if _, err := notFound.GetDocument(context.Background(), &webapi.GetDocumentRequest{Uri: "mount://m/d.pdf"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("not-found => %v, want ErrNotFound", err)
	}
}

func TestGetPartSpecFound(t *testing.T) {
	store := &fakePartSpecStore{spec: &parampb.PartSpec{Mpn: "LM1117"}, found: true}
	svc := NewDatasheetService(&fakeDocLoader{}, store, &fakeDocExtractor{}, &fakeAnnotationStore{})
	resp, err := svc.GetPartSpec(context.Background(), &webapi.GetPartSpecRequest{Uri: "mount://m/d.pdf"})
	if err != nil {
		t.Fatalf("GetPartSpec: %v", err)
	}
	if !resp.Found || resp.GetSpec().GetMpn() != "LM1117" || resp.Version != "v1" {
		t.Errorf("got found=%v spec=%v version=%q", resp.Found, resp.GetSpec(), resp.Version)
	}
}

func TestSavePartSpecConflictAndValidation(t *testing.T) {
	// A store conflict propagates as ErrConflict (the transport maps it to Aborted, "refetch").
	conflict := NewDatasheetService(&fakeDocLoader{}, &fakePartSpecStore{saveErr: ErrConflict}, &fakeDocExtractor{}, &fakeAnnotationStore{})
	_, err := conflict.SavePartSpec(context.Background(), &webapi.SavePartSpecRequest{Uri: "mount://m/d", Spec: &parampb.PartSpec{Mpn: "X"}})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("store conflict => %v, want ErrConflict", err)
	}
	// A nil spec is rejected before touching the store.
	empty := NewDatasheetService(&fakeDocLoader{}, &fakePartSpecStore{}, &fakeDocExtractor{}, &fakeAnnotationStore{})
	if _, err := empty.SavePartSpec(context.Background(), &webapi.SavePartSpecRequest{Uri: "mount://m/d"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("nil spec => %v, want ErrInvalidArgument", err)
	}
}

func TestExtractDocIRGated(t *testing.T) {
	// No producer configured -> ErrExtractNotEnabled (transport maps it to FailedPrecondition).
	off := NewDatasheetService(&fakeDocLoader{}, &fakePartSpecStore{}, &fakeDocExtractor{available: false}, &fakeAnnotationStore{})
	if _, err := off.ExtractDocIR(context.Background(), &webapi.ExtractDocIRRequest{Uri: "mount://m/d.pdf"}); !errors.Is(err, ErrExtractNotEnabled) {
		t.Errorf("disabled => %v, want ErrExtractNotEnabled", err)
	}
	// Configured -> returns the produced doc-IR.
	produced := &docpb.Document{ContentHash: "sha256:x", Producer: "docling"}
	on := NewDatasheetService(&fakeDocLoader{}, &fakePartSpecStore{}, &fakeDocExtractor{available: true, doc: produced}, &fakeAnnotationStore{})
	resp, err := on.ExtractDocIR(context.Background(), &webapi.ExtractDocIRRequest{Uri: "mount://m/d.pdf"})
	if err != nil || resp.GetDocument().GetContentHash() != "sha256:x" {
		t.Fatalf("extract: resp=%v err=%v", resp, err)
	}
}

func TestGetDocumentReportsExtractAvailable(t *testing.T) {
	on := NewDatasheetService(&fakeDocLoader{doc: nil}, &fakePartSpecStore{}, &fakeDocExtractor{available: true}, &fakeAnnotationStore{})
	resp, _ := on.GetDocument(context.Background(), &webapi.GetDocumentRequest{Uri: "mount://m/d.pdf"})
	if !resp.ExtractAvailable {
		t.Error("extract_available should be true when a producer is configured")
	}
	off := newDS(&fakeDocLoader{doc: nil}) // newDS uses a disabled extractor
	resp2, _ := off.GetDocument(context.Background(), &webapi.GetDocumentRequest{Uri: "mount://m/d.pdf"})
	if resp2.ExtractAvailable {
		t.Error("extract_available should be false with no producer")
	}
}

func TestSaveAnnotationsValidation(t *testing.T) {
	svc := NewDatasheetService(&fakeDocLoader{}, &fakePartSpecStore{}, &fakeDocExtractor{}, &fakeAnnotationStore{})
	// A nil set is rejected before the store.
	if _, err := svc.SaveAnnotations(context.Background(), &webapi.SaveAnnotationsRequest{Uri: "mount://m/d"}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("nil set => %v, want ErrInvalidArgument", err)
	}
	// An empty author is rejected: the author names the file and cannot be inferred.
	req := &webapi.SaveAnnotationsRequest{Set: &webapi.AnnotationSet{DocId: "LM1117"}}
	if _, err := svc.SaveAnnotations(context.Background(), req); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("empty author => %v, want ErrInvalidArgument", err)
	}
}

func TestSaveAndGetAnnotations(t *testing.T) {
	store := &fakeAnnotationStore{}
	svc := NewDatasheetService(&fakeDocLoader{}, &fakePartSpecStore{}, &fakeDocExtractor{}, store)
	set := &webapi.AnnotationSet{DocId: "LM1117", Author: "alice", Annotations: []*webapi.RegionAnnotation{{RegionId: "p4.t1", Type: "table"}}}
	if _, err := svc.SaveAnnotations(context.Background(), &webapi.SaveAnnotationsRequest{Uri: "mount://m/d.pdf", Set: set}); err != nil {
		t.Fatalf("SaveAnnotations: %v", err)
	}
	if store.author != "alice" || store.saved.GetAnnotations()[0].GetRegionId() != "p4.t1" {
		t.Errorf("store got author=%q saved=%v", store.author, store.saved)
	}
	// GetAnnotations returns the union the store provides (one set per author).
	store.sets = []*webapi.AnnotationSet{{Author: "alice"}, {Author: "bob"}}
	resp, err := svc.GetAnnotations(context.Background(), &webapi.GetAnnotationsRequest{Uri: "mount://m/d.pdf"})
	if err != nil {
		t.Fatalf("GetAnnotations: %v", err)
	}
	if len(resp.GetSets()) != 2 {
		t.Errorf("union = %d sets, want 2", len(resp.GetSets()))
	}
}

// Saving records what the author has. It is NOT a judgment about whether the spec is any good, so
// neither incompleteness nor structural incoherence may block a write: a rejected save costs work,
// and every mutation path would otherwise have to preserve an invariant or strand the document.
//
// Nothing downstream needs the gate. The sibling is <stem>.partspec.json and param.LoadSet reads
// *.textproto, so a draft cannot reach the corpus by sitting on disk; promotion is a separate step
// and that is where param.Validate belongs.
func TestSavePartSpecRecordsWhateverTheAuthorHas(t *testing.T) {
	svc := NewDatasheetService(&fakeDocLoader{}, &fakePartSpecStore{}, &fakeDocExtractor{}, &fakeAnnotationStore{})
	save := func(spec *parampb.PartSpec) error {
		_, err := svc.SavePartSpec(context.Background(), &webapi.SavePartSpecRequest{Uri: "mount://m/d", Spec: spec})
		return err
	}

	// No mpn, no parameters: the ordinary state of a datasheet someone has started transcribing.
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
