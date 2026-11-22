package projects

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/records"
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

func (s *DirStore) revisionsOf(id string) string { return records.Join(revisionsDir, id) }

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
	p := records.Join(dir, rec.Rev+".yaml")
	var r revisionYAML
	if raw, err := s.recs.Get(context.Background(), p); err == nil {
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
		// A ref names one revision, so it moves here from any other that held it (agni issue 1774).
		// Linking a design's artifacts one at a time names each step, and the last one keeps it.
		others, err := s.loadRevisions(id)
		if err != nil {
			return revisionYAML{}, err
		}
		for name, o := range others {
			if name == rec.Rev || !slices.Contains(o.Refs, ref) {
				continue
			}
			o.Refs = slices.DeleteFunc(o.Refs, func(x string) bool { return x == ref })
			if err := s.writeRevision(dir, name, o); err != nil {
				return revisionYAML{}, err
			}
		}
	}
	return r, s.writeRevision(dir, rec.Rev, r)
}

// writeRevision writes r as revision name in dir, replacing the record whole.
func (s *DirStore) writeRevision(dir, name string, r revisionYAML) error {
	out, err := yaml.Marshal(&r)
	if err != nil {
		return err
	}
	return s.recs.Put(context.Background(), records.Join(dir, name+".yaml"), out)
}

// loadRevisions reads design id's kept revisions by id. None is an empty map. Callers hold s.mu.
func (s *DirStore) loadRevisions(id string) (map[string]revisionYAML, error) {
	entries, err := s.recs.List(context.Background(), s.revisionsOf(id))
	if err != nil {
		return nil, err
	}
	out := map[string]revisionYAML{}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name, ".yaml")
		if !ok || e.Dir {
			continue
		}
		raw, err := s.recs.Get(context.Background(), records.Join(s.revisionsOf(id), e.Name))
		if err != nil {
			return nil, err
		}
		var r revisionYAML
		if err := yaml.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name, err)
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

// RevisionArtifacts returns design id's artifacts as of the kept revision whose id or one of whose
// refs is rev (agni issue 1774). A rev it doesn't keep is ErrNotFound.
func (s *DirStore) RevisionArtifacts(_ context.Context, id, rev string) ([]*webapi.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.loadRevisions(id)
	if err != nil {
		return nil, err
	}
	for name, r := range all {
		if name == rev || slices.Contains(r.Refs, rev) {
			out := make([]*webapi.Artifact, 0, len(r.Record.Artifacts))
			for _, a := range r.Record.Artifacts {
				out = append(out, s.artifactOf(id, a))
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("%w: no kept revision %s", service.ErrNotFound, service.RevisionName(service.StandaloneDesignName(id), rev))
}

// WithRevisionPins makes a prune keep the revisions pins names for a design, and returns s (agni
// issue 1774). The server pins the revisions its reports ran on, so a report always opens its files.
func (s *DirStore) WithRevisionPins(pins func(ctx context.Context, id string) (map[string]bool, error)) *DirStore {
	s.pins = pins
	return s
}

// namedRef is the ref every linked artifact of rec with history is pinned at, or "" when one
// follows a branch, two are at different refs, or none has history. A link one artifact at a time
// then names the revision only once every artifact reached the ref (agni issue 1774).
func namedRef(rec recordYAML) string {
	ref := ""
	for _, a := range rec.Artifacts {
		src := a.Source
		if src == nil || src.Version == "" {
			continue
		}
		if src.Follow || src.Ref == "" || (ref != "" && src.Ref != ref) {
			return ""
		}
		ref = src.Ref
	}
	return ref
}

// pruneRevisions deletes design id's kept revisions that nothing names: not current, with no ref a
// read named, and not pinned (agni issue 1774). A link that moves artifacts one at a time keeps a
// revision after each, and this drops the half-moved ones once the design moves on. Their files go
// at the next sweep of the uploads. Callers hold s.mu.
func (s *DirStore) pruneRevisions(ctx context.Context, id, current string) error {
	all, err := s.loadRevisions(id)
	if err != nil || len(all) == 0 {
		return err
	}
	pinned := map[string]bool{}
	if s.pins != nil {
		if pinned, err = s.pins(ctx, id); err != nil {
			return err
		}
	}
	for name, r := range all {
		if name == current || len(r.Refs) > 0 || pinned[name] {
			continue
		}
		if err := s.recs.Delete(ctx, records.Join(s.revisionsOf(id), name+".yaml")); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}
