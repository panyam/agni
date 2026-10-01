package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/datasheet/corpus"
	"github.com/panyam/agni/datasheet/dsservice"
	dsapi "github.com/panyam/agni/datasheet/gen/go/agni/v1/dsapi"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/service"
)

// osDraftStore is the OS-backed dsservice.DraftStore over a corpus directory (agni issue 749). It
// holds both entity types: drafts under drafts/, keyed by MPN, and the published specs and index
// that `agnids publish` and PublishDraft write and PartSpecService reads.
//
// Save is compare-and-swap under a per-file lock, and publishing holds one lock for the whole corpus,
// since it rewrites the index. Both locks are per PROCESS, so two agnids on one corpus can still
// clobber each other.
type osDraftStore struct {
	dir     string
	locks   sync.Map // abs path -> *sync.Mutex
	publish sync.Mutex
}

func newOSDraftStore(dir string) *osDraftStore { return &osDraftStore{dir: dir} }

func (s *osDraftStore) lockFor(abs string) *sync.Mutex {
	m, _ := s.locks.LoadOrStore(abs, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// draftPath is the host path of the draft for mpn, laid out as corpus.DraftFile says.
func (s *osDraftStore) draftPath(mpn string) string {
	return filepath.Join(s.dir, filepath.FromSlash(corpus.DraftFile(mpn)))
}

// readDraft reads the draft at a host path.
func (s *osDraftStore) readDraft(abs string) (*dsapi.Draft, bool, error) {
	rel, err := filepath.Rel(s.dir, abs)
	if err != nil {
		return nil, false, err
	}
	return corpus.ReadDraft(os.DirFS(s.dir), filepath.ToSlash(rel))
}

// Get implements dsservice.DraftStore. Two MPNs can share a file name once unsafe characters are
// replaced ("A/B" and "A_B"), so a file holding a different MPN is not this one's draft.
func (s *osDraftStore) Get(_ context.Context, mpn string) (*dsapi.Draft, bool, error) {
	d, found, err := s.readDraft(s.draftPath(mpn))
	if err != nil || !found || !strings.EqualFold(d.GetMpn(), mpn) {
		return nil, false, err
	}
	return d, true, nil
}

// ListByDocument implements dsservice.DraftStore.
func (s *osDraftStore) ListByDocument(_ context.Context, documentURI string) ([]*dsapi.Draft, error) {
	all, err := corpus.Drafts(os.DirFS(s.dir))
	if err != nil {
		return nil, err
	}
	var out []*dsapi.Draft
	for _, d := range all {
		if slices.Contains(d.GetDocumentUris(), documentURI) {
			out = append(out, d)
		}
	}
	return out, nil
}

// Save implements dsservice.DraftStore: under the file's lock it reads the current version, requires
// it to equal baseVersion (empty means "expected absent"), then writes.
func (s *osDraftStore) Save(_ context.Context, d *dsapi.Draft, baseVersion string) (string, error) {
	abs := s.draftPath(d.GetMpn())
	lock := s.lockFor(abs)
	lock.Lock()
	defer lock.Unlock()

	cur, found, err := s.readDraft(abs)
	if err != nil {
		return "", err
	}
	if found && !strings.EqualFold(cur.GetMpn(), d.GetMpn()) {
		return "", fmt.Errorf("%w: %s already holds the draft for %q, whose file name %q shares", service.ErrInvalidArgument, filepath.Base(abs), cur.GetMpn(), d.GetMpn())
	}
	curVersion := ""
	if found {
		curVersion = cur.GetVersion()
	}
	if curVersion != baseVersion {
		return "", dsservice.ErrConflict
	}
	stored := proto.Clone(d).(*dsapi.Draft)
	stored.Version = ""
	out, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(stored)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := writeAtomic(abs, out); err != nil {
		return "", err
	}
	return corpus.VersionOf(out), nil
}

// Publish implements dsservice.DraftStore.
func (s *osDraftStore) Publish(ctx context.Context, mpn string) (*dsservice.Published, error) {
	d, found, err := s.Get(ctx, mpn)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%w: no draft for %q", service.ErrNotFound, mpn)
	}
	s.publish.Lock()
	defer s.publish.Unlock()
	p, err := publishSpec(s.dir, d.GetSpec())
	var pe *corpus.PromoteError
	if errors.As(err, &pe) {
		return nil, &dsservice.PublishRefused{Reason: pe.Reason, Problems: pe.Problems}
	}
	if err != nil {
		return nil, err
	}
	return &dsservice.Published{Replaced: p.Replaces, Generation: p.Index.Generation}, nil
}

// publishSpec writes a validated spec into the corpus at dir and records it in the index, the one
// sequence both PublishDraft and `agnids publish` use. corpus.PromoteSpec decides; this writes the
// spec, re-loads the corpus and restores what was there if it no longer loads, so a publication
// cannot leave a corpus that fails every check, and only then writes the index.
func publishSpec(dir string, spec *parampb.PartSpec) (*corpus.Promoted, error) {
	fsys := os.DirFS(dir)
	prev, err := readIndex(fsys)
	if err != nil {
		return nil, err
	}
	p, err := corpus.PromoteSpec(spec, fsys, prev)
	if err != nil {
		return nil, err
	}
	dst := filepath.Join(dir, filepath.FromSlash(p.File))
	prevText, prevErr := os.ReadFile(dst) // a replaced file is restored, a new one removed
	if err := writeAtomic(dst, p.Text); err != nil {
		return nil, err
	}
	if _, err := param.LoadSet(fsys); err != nil {
		if prevErr == nil {
			_ = writeAtomic(dst, prevText)
		} else {
			_ = os.Remove(dst)
		}
		return nil, fmt.Errorf("the corpus stopped loading after writing %s, so it was put back: %w", dst, err)
	}
	if err := writeIndex(dir, p.Index); err != nil {
		return nil, fmt.Errorf("%s was written but the index was not, so run `agnids index %s`: %w", dst, dir, err)
	}
	return p, nil
}
