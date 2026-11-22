package service

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/panyam/goapplib/storage/mounts"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// MountsOf names the mounts whose source is source, in the driver's form such as
// "github://acme/board", whatever ref each mount was made with (agni issue 1726). A link carries its
// own ref, so a push to any branch can move an artifact read through a mount made with none.
type MountsOf func(ctx context.Context, source string) ([]string, error)

// WithSourceChanges lets SourceChanged find the mounts a change names (agni issue 1726).
func (s *DesignService) WithSourceChanges(mountsOf MountsOf) *DesignService {
	s.mountsOf = mountsOf
	return s
}

// SourceMove is what SourceChanged or PollSources did to one design.
type SourceMove struct {
	// Design is "designs/{design}".
	Design string
	// Artifacts are the ids of the artifacts that follow the moved ref, sorted.
	Artifacts []string
	// Synced is true when the design read the new files. Otherwise the artifacts carry the new
	// version as newer_version.
	Synced bool
	// Err is why a sync failed, when the design was meant to sync. The artifacts are marked instead.
	Err error
}

// SourceChanged acts on a ref that moved in a source, as a push webhook reports it (agni issue
// 1726). Every stored artifact linked through a mount of c.Source that follows c.Ref moves: one
// whose link names c.Ref, or names no ref when c.Ref is the default branch. A deleted ref moves
// nothing.
//
// A source with no refs, such as a SharePoint drive, reports a change with Default set and no Ref
// (agni issue 1729). It moves every artifact that links through a mount in that source with no ref.
//
// The change only says that something moved. Each matching mount and ref is resolved again, once,
// and an artifact moves to what the source names now. So a late delivery of an older push, or a
// forged one, can't move or mark a design backward. An artifact the push names a version it already
// holds, or is already marked with, is left alone before anything is asked, so a redelivery reads
// nothing. A design set to mark, or one whose sync fails, has each artifact's newer_version set.
//
// It runs as the server, with no viewer, so it sees every design. A link through a connection that
// is one person's own can't be read with no one viewing, so it is marked with c.Version.
func (s *DesignService) SourceChanged(ctx context.Context, c mounts.Change) ([]SourceMove, error) {
	if s.readers == nil || s.mountsOf == nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedPrecondition, noLinks)
	}
	if !changeMoves(c) {
		return nil, nil
	}
	names, err := s.mountsOf(ctx, c.Source)
	if err != nil {
		return nil, err
	}
	return s.catchUp(unguarded(ctx), c.Version, func(src *webapi.ArtifactSource) bool {
		follows := followsChange(src, c)
		seen := c.Version != "" && (src.GetVersion() == c.Version || src.GetNewerVersion() == c.Version)
		return slices.Contains(names, src.GetMount()) && follows && !seen
	})
}

// changeMoves reports whether c can move a link: it isn't a deleted ref, and it names a ref or comes
// from a source with no refs, such as a SharePoint drive, which reports Default and no Ref (agni
// issue 1729).
func changeMoves(c mounts.Change) bool {
	return !c.Deleted && (c.Ref != "" || c.Default)
}

// followsChange reports whether a link at src follows the ref c moved: it names c.Ref, or names no
// ref and c is the source's default branch or a change in a source with no refs.
func followsChange(src *webapi.ArtifactSource, c mounts.Change) bool {
	if src.GetRef() == c.Ref && c.Ref != "" {
		return true
	}
	return src.GetRef() == "" && c.Default && (c.RefKind == mounts.RefBranch || c.Ref == "")
}

// PollSources makes one pass over every stored design that follows a branch or a folder, for a
// source that can't send events or a server that can't receive them (agni issue 1726). It resolves
// each mount and ref once, however many designs read it: one request on GitHub, and a walk of the
// file times in a folder. A design whose artifacts are behind syncs or is marked, as SourceChanged
// does. A source that can't answer now is asked again on the next pass.
func (s *DesignService) PollSources(ctx context.Context) ([]SourceMove, error) {
	if s.readers == nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedPrecondition, noLinks)
	}
	return s.catchUp(unguarded(ctx), "", func(*webapi.ArtifactSource) bool { return true })
}

// catchUp resolves the mount and ref of each followed artifact that want takes, once per pair, and
// syncs or marks the designs whose artifacts are behind. A pair that fails to resolve marks its
// artifacts with fallback, or skips them when fallback is empty.
func (s *DesignService) catchUp(ctx context.Context, fallback string, want func(*webapi.ArtifactSource) bool) ([]SourceMove, error) {
	type key struct{ mount, ref string }
	type answer struct {
		version string
		err     error
	}
	answers := map[key]answer{}
	moved := movedSet{}
	var lastErr error
	err := s.eachFollowed(ctx, func(design string, a *webapi.Artifact) {
		src := a.GetSource()
		if !want(src) {
			return
		}
		k := key{src.GetMount(), src.GetRef()}
		ans, ok := answers[k]
		if !ok {
			r, err := s.readers(ctx, src.GetMount())
			if err == nil {
				ans.version, _, err = r.Resolve(ctx, src.GetRef())
			}
			if err != nil {
				ans.err = fmt.Errorf("%s: %w", SourceString(src), err)
				lastErr = ans.err
			}
			answers[k] = ans
		}
		v := ans.version
		if ans.err != nil {
			v = fallback
		}
		// A design already marked at v isn't written again on every pass.
		if v == "" || v == src.GetVersion() || v == src.GetNewerVersion() {
			return
		}
		moved.add(design, a, v)
	})
	if err != nil {
		return nil, err
	}
	out, err := s.move(ctx, moved)
	if err == nil && lastErr != nil && len(out) == 0 {
		err = lastErr
	}
	return out, err
}

// movedSet maps a design's name to the ids of its artifacts whose source moved, and the version
// each moved to.
type movedSet map[string]map[string]string

func (m movedSet) add(design string, a *webapi.Artifact, version string) {
	if m[design] == nil {
		m[design] = map[string]string{}
	}
	_, id, _ := ArtifactID(a.GetName())
	m[design][id] = version
}

// eachFollowed calls f with every linked artifact that follows its source, in every stored design.
func (s *DesignService) eachFollowed(ctx context.Context, f func(design string, a *webapi.Artifact)) error {
	designs, _, err := s.catalog.Designs(ctx)
	if err != nil {
		return err
	}
	for _, d := range designs {
		arts, err := s.catalog.Artifacts(ctx, d.GetName())
		if err != nil {
			// A mounted design has no artifacts and no record to read them from.
			continue
		}
		for _, a := range arts {
			if s.linked(a) && a.GetSource().GetFollow() {
				f(d.GetName(), a)
			}
		}
	}
	return nil
}

// move syncs or marks each design in moved, by its on_source_change.
func (s *DesignService) move(ctx context.Context, moved movedSet) ([]SourceMove, error) {
	var out []SourceMove
	for _, design := range slices.Sorted(maps.Keys(moved)) {
		ids := slices.Sorted(maps.Keys(moved[design]))
		m := SourceMove{Design: design, Artifacts: ids}
		d, err := s.catalog.Design(ctx, design)
		if err != nil {
			return out, err
		}
		if d.GetOnSourceChange() != webapi.SourceChange_SOURCE_CHANGE_MARK {
			if _, m.Err = s.SyncDesign(ctx, &webapi.SyncDesignRequest{Name: design, Artifacts: ids}); m.Err == nil {
				m.Synced = true
				out = append(out, m)
				continue
			}
		}
		now := s.clock()
		if _, err := s.catalog.UpdateDesign(ctx, design, func(r *DesignRecord) {
			for id, v := range moved[design] {
				if v != "" {
					r.Newer[id] = NewerVersion{Version: v, Seen: now}
				}
			}
		}); err != nil {
			return out, fmt.Errorf("marking %s: %w", strings.Join(ids, ", "), err)
		}
		out = append(out, m)
	}
	return out, nil
}
