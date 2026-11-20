package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// SplitRevision splits a design's name read at a revision, "designs/{design}@{rev}", into the
// design's name and the revision (agni issue 1725). A name with no "@" has an empty revision. The
// revision is a ref, such as a tag, branch or commit, or a revision id ListDesignRevisions gave.
func SplitRevision(name string) (design, rev string) {
	design, rev, _ = strings.Cut(name, "@")
	return design, rev
}

// RevisionName is a design's name read at rev, "designs/{design}@{rev}".
func RevisionName(design, rev string) string { return design + "@" + rev }

// RevisionStore keeps the revisions of a design read from its sources (agni issue 1725). DirStore is
// one. A revision is the design's record with some artifacts' files swapped for their files at
// another version. It shares the design's view folder and blobs, and leaves the design's own record
// as it is.
type RevisionStore interface {
	// PutRevision keeps design id with writes applied over its current artifacts as a revision, and
	// returns it with the design as of it. ref is kept on the revision when it is not empty. The same
	// files give the same revision, so a second put adds the ref and nothing else.
	PutRevision(ctx context.Context, id, ref string, writes []ArtifactWrite, blobs BlobSource) (*webapi.DesignRevision, *webapi.Design, error)
	// Revision returns the kept revision whose id, or one of whose refs, is rev, with the design as
	// of it. A rev it doesn't hold is ErrNotFound.
	Revision(ctx context.Context, id, rev string) (*webapi.DesignRevision, *webapi.Design, error)
	// Revisions returns design id's kept revisions, newest first.
	Revisions(ctx context.Context, id string) ([]*webapi.DesignRevision, error)
}

// validRevision refuses a revision no ref can be: empty, too long, or holding a space or a
// control character.
func validRevision(rev string) error {
	if rev == "" || len(rev) > 255 || strings.IndexFunc(rev, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return fmt.Errorf("%w: revision %q must be a tag, branch, commit or revision id", ErrInvalidArgument, rev)
	}
	return nil
}

// revisions is the catalog's store as a RevisionStore, or nil when it keeps none.
func (c *DesignCatalog) revisionStore() RevisionStore {
	rs, _ := c.store.(RevisionStore)
	return rs
}

// revisionOf reads the stored design name at rev (agni issue 1725). A kept revision whose id or ref
// is rev answers at once. Otherwise rev is resolved on each linked artifact's mount, its files at
// that version are fetched where the server lacks them, and the result is kept as a revision. An
// artifact whose source has no history, or that is uploaded, keeps its current files.
func (s *DesignService) revisionOf(ctx context.Context, name, rev string) (*webapi.Design, error) {
	if err := validRevision(rev); err != nil {
		return nil, err
	}
	id, _ := StandaloneDesignID(name)
	rs := s.catalog.revisionStore()
	if rs == nil {
		return nil, fmt.Errorf("%w: this server keeps no revisions; start it with --store and --uploads", ErrFailedPrecondition)
	}
	if _, d, err := rs.Revision(ctx, id, rev); err == nil {
		return d, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if s.readers == nil {
		return nil, fmt.Errorf("%w: no revision %s", ErrNotFound, RevisionName(name, rev))
	}
	arts, err := s.catalog.Artifacts(ctx, name)
	if err != nil {
		return nil, err
	}
	var writes []ArtifactWrite
	history, moves := false, false
	now := timestamppb.New(s.clock())
	for _, a := range arts {
		if !s.linked(a) {
			continue
		}
		src := a.GetSource()
		r, err := s.readers(ctx, src.GetMount())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", a.GetName(), err)
		}
		version, follow, err := r.Resolve(ctx, rev)
		switch {
		case errors.Is(err, ErrInvalidArgument):
			// A source with no refs, such as a folder, has no revisions, and its files stay.
			continue
		case errors.Is(err, ErrNotFound):
			return nil, fmt.Errorf("%w: no revision %s: %s's source %s has no ref %q", ErrNotFound, RevisionName(name, rev), a.GetName(), src.GetMount(), rev)
		case err != nil:
			return nil, fmt.Errorf("%s: %w", a.GetName(), err)
		case version == "":
			continue
		}
		history, moves = true, moves || follow
		if version == src.GetVersion() {
			continue
		}
		files, _, err := s.readSource(ctx, r, version, src.GetPath())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", a.GetName(), err)
		}
		_, aid, _ := ArtifactID(a.GetName())
		writes = append(writes, ArtifactWrite{ID: aid, Files: files, Source: &webapi.ArtifactSource{
			Mount: src.GetMount(), Path: src.GetPath(), Ref: src.GetRef(), Follow: src.GetFollow(), Version: version,
			ReadTime: now, ReadHash: manifestHash(files), AccessSource: src.GetAccessSource(),
		}})
	}
	if !history {
		return nil, fmt.Errorf("%w: no revision %s: no artifact of it links to a source with history", ErrNotFound, RevisionName(name, rev))
	}
	// A branch names another version tomorrow, so only a tag or a commit is kept as the revision's
	// name.
	keep := rev
	if moves {
		keep = ""
	}
	_, d, err := rs.PutRevision(ctx, id, keep, writes, s.blobs)
	return d, err
}

// ListDesignRevisions returns the revisions of a stored design the server has read, newest first.
func (s *DesignService) ListDesignRevisions(ctx context.Context, req *webapi.ListDesignRevisionsRequest) (*webapi.ListDesignRevisionsResponse, error) {
	if _, err := s.catalog.Design(ctx, req.GetParent()); err != nil {
		return nil, err
	}
	resp := &webapi.ListDesignRevisionsResponse{}
	rs := s.catalog.revisionStore()
	if rs == nil {
		return resp, nil
	}
	id, _ := StandaloneDesignID(req.GetParent())
	all, err := rs.Revisions(ctx, id)
	if err != nil {
		return nil, err
	}
	start := 0
	if t := req.GetPageToken(); t != "" {
		start = -1
		for i, r := range all {
			if r.GetName() == t {
				start = i
				break
			}
		}
		if start < 0 {
			return nil, fmt.Errorf("%w: page_token %q names no revision of %s", ErrInvalidArgument, t, req.GetParent())
		}
	}
	size := int(req.GetPageSize())
	switch {
	case size <= 0:
		size = 50
	case size > 200:
		size = 200
	}
	end := min(start+size, len(all))
	resp.Revisions = all[start:end]
	if end < len(all) {
		resp.NextPageToken = all[end].GetName()
	}
	return resp, nil
}

// sourceVersions maps each artifact id to the version its source was read at, for one that names
// one.
func sourceVersions(arts []*webapi.Artifact) map[string]string {
	out := map[string]string{}
	for _, a := range arts {
		if v := a.GetSource().GetVersion(); v != "" {
			_, id, _ := ArtifactID(a.GetName())
			out[id] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
