package projects

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// revisionsDir is the folder under the records folder that holds each design's kept revisions,
// <records>/revisions/<design>/<revision id>.yaml (agni issue 1725). The store lists only .yaml
// files in the records folder as designs, so a folder there is never read as one.
const revisionsDir = "revisions"

// revisionYAML is a kept revision: the design's record with the files of that revision, and the
// refs a read named that gave it.
type revisionYAML struct {
	Refs       []string   `yaml:"refs,omitempty"`
	CreateTime time.Time  `yaml:"create_time"`
	Record     recordYAML `yaml:"record"`
}

var _ service.RevisionStore = (*DirStore)(nil)

func (s *DirStore) revisionsOf(id string) string { return filepath.Join(s.records, revisionsDir, id) }

// PutRevision keeps design id with writes applied as a revision, building its view, and leaves the
// design's record as it is.
func (s *DirStore) PutRevision(ctx context.Context, id, ref string, writes []service.ArtifactWrite, blobs service.BlobSource) (*webapi.DesignRevision, *webapi.Design, error) {
	if s.blobs == nil {
		return nil, nil, fmt.Errorf("%w: this server keeps no uploaded files; start it with --uploads", service.ErrFailedPrecondition)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.load(id)
	if err != nil {
		return nil, nil, err
	}
	next, _, err := s.apply(ctx, id, rec, writes, blobs)
	if err != nil {
		return nil, nil, err
	}
	_, _, raw, err := describe(id, next)
	if err != nil {
		return nil, nil, err
	}
	if raw == nil {
		return nil, nil, fmt.Errorf("%w: %s has no artifact a reader opens", service.ErrFailedPrecondition, service.StandaloneDesignName(id))
	}
	if next.Rev, err = s.buildView(id, next, raw); err != nil {
		return nil, nil, err
	}
	kept, err := s.keepRevision(id, next, ref)
	if err != nil {
		return nil, nil, err
	}
	return s.revisionOf(id, next.Rev, kept, rec.Rev)
}

// Revision returns the kept revision whose id or one of whose refs is rev.
func (s *DirStore) Revision(_ context.Context, id, rev string) (*webapi.DesignRevision, *webapi.Design, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.load(id)
	if err != nil {
		return nil, nil, err
	}
	all, err := s.loadRevisions(id)
	if err != nil {
		return nil, nil, err
	}
	for name, r := range all {
		if name == rev || slices.Contains(r.Refs, rev) {
			return s.revisionOf(id, name, r, rec.Rev)
		}
	}
	return nil, nil, fmt.Errorf("%w: no kept revision %s", service.ErrNotFound, service.RevisionName(service.StandaloneDesignName(id), rev))
}

// Revisions returns design id's kept revisions, newest first, and by id among those kept at one time.
func (s *DirStore) Revisions(_ context.Context, id string) ([]*webapi.DesignRevision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.load(id)
	if err != nil {
		return nil, err
	}
	all, err := s.loadRevisions(id)
	if err != nil {
		return nil, err
	}
	out := make([]*webapi.DesignRevision, 0, len(all))
	for name, r := range all {
		rv, _, err := s.revisionOf(id, name, r, rec.Rev)
		if err != nil {
			return nil, err
		}
		out = append(out, rv)
	}
	slices.SortFunc(out, func(a, b *webapi.DesignRevision) int {
		if c := b.GetCreateTime().AsTime().Compare(a.GetCreateTime().AsTime()); c != 0 {
			return c
		}
		return strings.Compare(a.GetRevisionId(), b.GetRevisionId())
	})
	return out, nil
}

// keepRevision writes rec as the revision named by its view, adding ref to one already kept. Callers
// hold s.mu.
func (s *DirStore) keepRevision(id string, rec recordYAML, ref string) (revisionYAML, error) {
	dir := s.revisionsOf(id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return revisionYAML{}, err
	}
	p := filepath.Join(dir, rec.Rev+".yaml")
	var r revisionYAML
	if raw, err := os.ReadFile(p); err == nil { // #nosec G304 -- p is the records folder, a design id and a hex revision id
		if err := yaml.Unmarshal(raw, &r); err != nil {
			return revisionYAML{}, fmt.Errorf("%s: %w", p, err)
		}
		if ref == "" || slices.Contains(r.Refs, ref) {
			return r, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return revisionYAML{}, err
	} else {
		r = revisionYAML{CreateTime: s.clock(), Record: rec}
	}
	if ref != "" {
		r.Refs = append(r.Refs, ref)
		slices.Sort(r.Refs)
	}
	out, err := yaml.Marshal(&r)
	if err != nil {
		return revisionYAML{}, err
	}
	tmp, err := os.CreateTemp(dir, ".put-*")
	if err != nil {
		return revisionYAML{}, err
	}
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return revisionYAML{}, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return revisionYAML{}, err
	}
	return r, os.Rename(tmp.Name(), p)
}

// loadRevisions reads design id's kept revisions by id. None is an empty map. Callers hold s.mu.
func (s *DirStore) loadRevisions(id string) (map[string]revisionYAML, error) {
	entries, err := os.ReadDir(s.revisionsOf(id))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]revisionYAML{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]revisionYAML{}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".yaml")
		if !ok || e.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.revisionsOf(id), e.Name())) // #nosec G304 -- a file the store wrote under its records folder
		if err != nil {
			return nil, err
		}
		var r revisionYAML
		if err := yaml.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out[name] = r
	}
	return out, nil
}

// revisionOf is a kept revision named name, and the design as of it. current is the revision id the
// design's record holds now.
func (s *DirStore) revisionOf(id, name string, r revisionYAML, current string) (*webapi.DesignRevision, *webapi.Design, error) {
	rec := r.Record
	rec.Rev = name
	d, err := s.designOf(id, rec)
	if err != nil {
		return nil, nil, err
	}
	d.Name = service.RevisionName(d.GetName(), name)
	rv := &webapi.DesignRevision{
		Name: d.GetName(), RevisionId: name, Refs: r.Refs, CreateTime: timestamppb.New(r.CreateTime), Current: name == current,
	}
	for _, a := range rec.Artifacts {
		if a.Source != nil && a.Source.Version != "" {
			if rv.SourceVersions == nil {
				rv.SourceVersions = map[string]string{}
			}
			rv.SourceVersions[a.ID] = a.Source.Version
		}
	}
	return rv, d, nil
}

// revisionRefs adds to refs the blobs every kept revision of every design names, and returns the
// view each one keeps by design, so Sweep keeps them. Callers hold s.mu.
func (s *DirStore) revisionRefs(ids []string, refs map[string]bool) (map[string]map[string]bool, error) {
	views := map[string]map[string]bool{}
	for _, id := range ids {
		all, err := s.loadRevisions(id)
		if err != nil {
			return nil, err
		}
		for name, r := range all {
			if views[id] == nil {
				views[id] = map[string]bool{}
			}
			views[id][name] = true
			for _, a := range r.Record.Artifacts {
				for _, f := range a.Files {
					refs[f.Sha256] = true
				}
			}
		}
	}
	return views, nil
}
