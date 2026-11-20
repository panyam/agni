package service

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/stdlib/rules/intent"
	"github.com/panyam/goapplib/storage/mounts"
)

// DesignService serves the design resource, "designs/{design}", over a DesignCatalog (agni issue
// 1694). It does no I/O itself. It validates requests, applies AIP-158 pagination and the supported
// AIP-160 filters, and computes each series' latest design. The catalog decides which store holds a
// design and refuses a write to a mounted one.
type DesignService struct {
	catalog  *DesignCatalog
	blobs    BlobSource
	projects *ProjectService
	// reports, checks and packs serve a design's reports (agni issue 1704). WithReports sets them.
	reports ReportStore
	checks  *CheckService
	packs   PackLookup
	// proposer and mountRead read mounted folders into designs (agni issue 1736).
	proposer  DesignProposer
	mountRead FileReader
	// readers read the mounts a linked artifact names, and uploadsMount names the mount uploaded
	// artifacts come from (agni issue 1723). WithLinks sets them.
	readers      MountReaders
	uploadsMount string
	// now stamps a link's read time; time.Now when nil.
	now func() time.Time
	// packer keeps an added design's project as a pack, and is set by WithProjectPacks.
	packer ProjectPacker
	// mountsOf finds the mounts of a source that moved, and is set by WithSourceChanges (agni issue
	// 1726).
	mountsOf MountsOf
}

// NewDesignService returns a DesignService over catalog, reading uploaded file contents from blobs.
// A nil catalog serves no designs and refuses every write. A nil blobs holds no contents, so every
// write asks for all of them.
func NewDesignService(catalog *DesignCatalog, blobs BlobSource) *DesignService {
	if catalog == nil {
		catalog = NewDesignCatalog(nil)
	}
	if blobs == nil {
		blobs = func(context.Context, string) ([]byte, bool) { return nil, false }
	}
	s := &DesignService{catalog: catalog, blobs: blobs}
	catalog.revise = s.revisionOf
	return s
}

// WithProjects lets ListDesigns filter by `project = "projects/{p}"`, and returns s.
func (s *DesignService) WithProjects(projects *ProjectService) *DesignService {
	s.projects = projects
	return s
}

// CreateDesign makes a new design with no artifacts.
func (s *DesignService) CreateDesign(ctx context.Context, req *webapi.CreateDesignRequest) (*webapi.Design, error) {
	if !newIDPattern.MatchString(req.GetDesignId()) {
		return nil, fmt.Errorf("%w: design id %q must be lowercase letters, digits, '.', '_' or '-', starting with a letter or digit", ErrInvalidArgument, req.GetDesignId())
	}
	d := req.GetDesign()
	packs, err := packNames(d.GetPacks())
	if err != nil {
		return nil, err
	}
	rec := DesignRecord{Title: d.GetTitle(), Series: d.GetSeries(), FlattenHierarchy: d.GetFlattenHierarchy(), Intent: d.GetConfig().GetIntent(), Packs: packs}
	if err := checkDesignConfig(d.GetConfig()); err != nil {
		return nil, err
	}
	out, err := s.catalog.CreateDesign(ctx, req.GetDesignId(), rec)
	if err != nil {
		return nil, err
	}
	return s.withLatest(ctx, out)
}

// GetDesign returns one design by resource name.
func (s *DesignService) GetDesign(ctx context.Context, req *webapi.GetDesignRequest) (*webapi.Design, error) {
	d, err := s.catalog.Design(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	return s.withLatest(ctx, d)
}

// ListDesigns returns the designs this server holds, ordered by name. A name two places hold is
// left out and returned in unreachable.
func (s *DesignService) ListDesigns(ctx context.Context, req *webapi.ListDesignsRequest) (*webapi.ListDesignsResponse, error) {
	keep, err := s.parseFilter(ctx, req.GetFilter())
	if err != nil {
		return nil, err
	}
	all, unreachable, err := s.catalog.Designs(ctx)
	if err != nil {
		return nil, err
	}
	markLatest(all)
	var kept []*webapi.Design
	for _, d := range all {
		if keep(d) {
			kept = append(kept, d)
		}
	}
	page, next := paginate(len(kept), req.GetPageSize(), req.GetPageToken(), func(i int) string { return kept[i].GetName() })
	resp := &webapi.ListDesignsResponse{NextPageToken: next, Unreachable: unreachable}
	for _, i := range page {
		resp.Designs = append(resp.Designs, kept[i])
	}
	return resp, nil
}

// UpdateDesign changes a stored design's title, series, flatten_hierarchy, intent, packs or
// on_source_change, as update_mask names. The intent's path is `config.intent`, and an empty mask leaves the intent alone.
func (s *DesignService) UpdateDesign(ctx context.Context, req *webapi.UpdateDesignRequest) (*webapi.Design, error) {
	in := req.GetDesign()
	paths := req.GetUpdateMask().GetPaths()
	if len(paths) == 0 {
		paths = []string{"title", "series", "flatten_hierarchy"}
	}
	for _, p := range paths {
		if p != "title" && p != "series" && p != "flatten_hierarchy" && p != "config.intent" && p != "packs" && p != "on_source_change" {
			return nil, fmt.Errorf("%w: update_mask path %q; a design's title, series, flatten_hierarchy, config.intent, packs and on_source_change can be changed", ErrInvalidArgument, p)
		}
		if p == "packs" {
			if _, err := packNames(in.GetPacks()); err != nil {
				return nil, err
			}
		}
	}
	if slices.Contains(paths, "config.intent") {
		if err := checkDesignConfig(&webapi.AnalysisConfig{Intent: in.GetConfig().GetIntent()}); err != nil {
			return nil, err
		}
	}
	d, err := s.catalog.UpdateDesign(ctx, in.GetName(), func(r *DesignRecord) {
		for _, p := range paths {
			switch p {
			case "title":
				r.Title = in.GetTitle()
			case "series":
				r.Series = in.GetSeries()
			case "flatten_hierarchy":
				r.FlattenHierarchy = in.GetFlattenHierarchy()
			case "config.intent":
				r.Intent = in.GetConfig().GetIntent()
			case "packs":
				r.Packs = in.GetPacks()
			case "on_source_change":
				r.OnSourceChange = in.GetOnSourceChange()
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return s.withLatest(ctx, d)
}

// checkDesignConfig refuses config a stored design cannot keep. A stored design keeps its intent,
// and the rest of its config comes from the packs it is read under. An intent that does not compile
// is refused here, so a bad one fails the write rather than every later read.
func checkDesignConfig(cfg *webapi.AnalysisConfig) error {
	if cfg == nil {
		return nil
	}
	rest := proto.Clone(cfg).(*webapi.AnalysisConfig)
	rest.Intent = nil
	if !proto.Equal(rest, &webapi.AnalysisConfig{}) {
		return fmt.Errorf("%w: a stored design keeps its intent alone; put conventions, checklists, profiles and params in a pack", ErrInvalidArgument)
	}
	if _, err := intentSource(cfg, "design"); err != nil {
		return err
	}
	return nil
}

// DeleteDesign removes a stored design and its reports.
func (s *DesignService) DeleteDesign(ctx context.Context, req *webapi.DeleteDesignRequest) error {
	if err := s.catalog.DeleteDesign(ctx, req.GetName()); err != nil {
		return err
	}
	if s.reports == nil {
		return nil
	}
	id, _ := StandaloneDesignID(req.GetName())
	return s.reports.DeleteReports(ctx, id)
}

// GetArtifact returns one artifact of a stored design.
func (s *DesignService) GetArtifact(ctx context.Context, req *webapi.GetArtifactRequest) (*webapi.Artifact, error) {
	d, _, ok := ArtifactID(req.GetName())
	if !ok {
		return nil, fmt.Errorf("%w: %q is not an artifact resource name, designs/{design}/artifacts/{artifact}", ErrInvalidArgument, req.GetName())
	}
	all, err := s.catalog.Artifacts(ctx, StandaloneDesignName(d))
	if err != nil {
		return nil, err
	}
	for _, a := range all {
		if a.GetName() == req.GetName() {
			return a, nil
		}
	}
	return nil, fmt.Errorf("%w: no artifact %s", ErrNotFound, req.GetName())
}

// ListArtifacts returns a design's artifacts, ordered by name. A mounted design has none.
func (s *DesignService) ListArtifacts(ctx context.Context, req *webapi.ListArtifactsRequest) (*webapi.ListArtifactsResponse, error) {
	all, err := s.catalog.Artifacts(ctx, req.GetParent())
	if err != nil {
		return nil, err
	}
	page, next := paginate(len(all), req.GetPageSize(), req.GetPageToken(), func(i int) string { return all[i].GetName() })
	resp := &webapi.ListArtifactsResponse{NextPageToken: next}
	for _, i := range page {
		resp.Artifacts = append(resp.Artifacts, all[i])
	}
	return resp, nil
}

// UpdateArtifact replaces an artifact's files, or creates it with allow_missing.
func (s *DesignService) UpdateArtifact(ctx context.Context, req *webapi.UpdateArtifactRequest) (*webapi.Artifact, error) {
	d, w, err := artifactWrite(req)
	if err != nil {
		return nil, err
	}
	if err := s.refuseLinked(ctx, StandaloneDesignName(d), []string{w.ID}); err != nil {
		return nil, err
	}
	out, err := s.catalog.WriteArtifacts(ctx, StandaloneDesignName(d), []ArtifactWrite{w}, s.blobs)
	if err != nil {
		return nil, err
	}
	return out[0], nil
}

// BatchUpdateArtifacts applies several UpdateArtifact requests to one design in one write.
func (s *DesignService) BatchUpdateArtifacts(ctx context.Context, req *webapi.BatchUpdateArtifactsRequest) (*webapi.BatchUpdateArtifactsResponse, error) {
	parent, ok := StandaloneDesignID(req.GetParent())
	if !ok {
		return nil, fmt.Errorf("%w: %q is not a design resource name, designs/{design}", ErrInvalidArgument, req.GetParent())
	}
	if len(req.GetRequests()) == 0 {
		return nil, fmt.Errorf("%w: the batch holds no requests", ErrInvalidArgument)
	}
	seen := map[string]bool{}
	var writes []ArtifactWrite
	for _, r := range req.GetRequests() {
		d, w, err := artifactWrite(r)
		if err != nil {
			return nil, err
		}
		if d != parent {
			return nil, fmt.Errorf("%w: %s is not an artifact of %s", ErrInvalidArgument, r.GetArtifact().GetName(), req.GetParent())
		}
		if seen[w.ID] {
			return nil, fmt.Errorf("%w: %s is named twice in the batch", ErrInvalidArgument, r.GetArtifact().GetName())
		}
		seen[w.ID] = true
		writes = append(writes, w)
	}
	if err := s.refuseLinked(ctx, req.GetParent(), slices.Collect(maps.Keys(seen))); err != nil {
		return nil, err
	}
	out, err := s.catalog.WriteArtifacts(ctx, req.GetParent(), writes, s.blobs)
	if err != nil {
		return nil, err
	}
	return &webapi.BatchUpdateArtifactsResponse{Artifacts: out}, nil
}

// ImportDesignFiles groups a folder's files into artifacts with PlanArtifacts and writes them all in
// one write, creating or replacing each.
func (s *DesignService) ImportDesignFiles(ctx context.Context, req *webapi.ImportDesignFilesRequest) (*webapi.ImportDesignFilesResponse, error) {
	if _, ok := StandaloneDesignID(req.GetParent()); !ok {
		return nil, fmt.Errorf("%w: %q is not a design resource name, designs/{design}", ErrInvalidArgument, req.GetParent())
	}
	if len(req.GetFiles()) == 0 {
		return nil, fmt.Errorf("%w: the import holds no files", ErrInvalidArgument)
	}
	byPath := map[string]*webapi.SnapshotFile{}
	var paths, missing []string
	for _, f := range req.GetFiles() {
		if _, dup := byPath[f.GetPath()]; dup {
			return nil, fmt.Errorf("%w: %q is listed twice", ErrInvalidArgument, f.GetPath())
		}
		byPath[f.GetPath()] = f
		paths = append(paths, f.GetPath())
		if _, ok := s.blobs(ctx, f.GetSha256()); !ok {
			missing = append(missing, f.GetSha256())
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, &BlobsMissingError{Hashes: slices.Compact(missing)}
	}
	plan, err := PlanArtifacts(paths, func(p string) ([]byte, error) {
		f, ok := byPath[p]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, p)
		}
		b, _ := s.blobs(ctx, f.GetSha256())
		return b, nil
	})
	if err != nil {
		return nil, err
	}
	declared, err := declaredIntent(paths, func(p string) ([]byte, error) {
		b, _ := s.blobs(ctx, byPath[p].GetSha256())
		return b, nil
	})
	if err != nil {
		return nil, err
	}
	id, _ := StandaloneDesignID(req.GetParent())
	if err := s.refuseLinked(ctx, req.GetParent(), plan.IDs); err != nil {
		return nil, err
	}
	symbols, err := declaredSymbols(paths, func(p string) ([]byte, error) {
		b, _ := s.blobs(ctx, byPath[p].GetSha256())
		return b, nil
	})
	if err != nil {
		return nil, err
	}
	inSymbols := func(f string) bool {
		return slices.ContainsFunc(symbols, func(dir string) bool { return strings.HasPrefix(f, dir+"/") })
	}
	var writes []ArtifactWrite
	for _, a := range plan.IDs {
		w := ArtifactWrite{ID: a, AllowMissing: true}
		for _, pf := range plan.Files[a] {
			if inSymbols(pf.Folder) {
				continue
			}
			f := byPath[pf.Folder]
			w.Files = append(w.Files, &webapi.SnapshotFile{Path: pf.Rel, Sha256: f.GetSha256(), Size: f.GetSize()})
		}
		writes = append(writes, w)
	}
	// A symbol folder the design.yaml names is an artifact of its own, as AddDesigns keeps one (agni
	// issue 1756), so a gEDA, Lepton or xschem design reads its parts' pins.
	for i, dir := range symbols {
		w := ArtifactWrite{ID: "symbols", AllowMissing: true}
		if i > 0 {
			w.ID = fmt.Sprintf("symbols-%d", i+1)
		}
		for _, p := range paths {
			if rel, ok := strings.CutPrefix(p, dir+"/"); ok {
				f := byPath[p]
				w.Files = append(w.Files, &webapi.SnapshotFile{Path: rel, Sha256: f.GetSha256(), Size: f.GetSize()})
			}
		}
		if len(w.Files) > 0 {
			writes = append(writes, w)
		}
	}
	plan.Unread = slices.DeleteFunc(plan.Unread, inSymbols)
	arts, err := s.catalog.WriteArtifacts(ctx, StandaloneDesignName(id), writes, s.blobs)
	if err != nil {
		return nil, err
	}
	// The folder's design.yaml declares the design's intent, which a stored design keeps as its own
	// (agni issue 1697). One with no intent leaves the kept intent as it was, as `designs push` does.
	if declared != nil {
		if _, err := s.catalog.UpdateDesign(ctx, StandaloneDesignName(id), func(r *DesignRecord) { r.Intent = declared }); err != nil {
			return nil, err
		}
	}
	return &webapi.ImportDesignFilesResponse{Artifacts: arts, Unread: plan.Unread}, nil
}

// declaredSymbols reads the symbol folders a top-level design.yaml among the files names under
// `symbols:`, each cleaned and relative to the folder. A folder outside the files, such as
// ../shared/symbols, can't be among them and is left out. No design.yaml names none.
func declaredSymbols(files []string, read func(string) ([]byte, error)) ([]string, error) {
	if !slices.Contains(files, DesignDescriptorName) {
		return nil, nil
	}
	raw, err := read(DesignDescriptorName)
	if err != nil {
		return nil, err
	}
	var d struct {
		Symbols []string `yaml:"symbols"`
	}
	if yaml.Unmarshal(raw, &d) != nil {
		return nil, nil
	}
	var out []string
	for _, dir := range d.Symbols {
		c, err := mounts.CleanPath(dir)
		if err != nil || c == "" {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

// declaredIntent reads the intent a top-level design.yaml among the files declares, nil when there is
// no design.yaml or it declares none. An intent section that does not bind is INVALID_ARGUMENT, since
// keeping the design without it would check the board as if it declared nothing.
func declaredIntent(files []string, read func(string) ([]byte, error)) (*configpb.DesignIntent, error) {
	if !slices.Contains(files, DesignDescriptorName) {
		return nil, nil
	}
	raw, err := read(DesignDescriptorName)
	if err != nil {
		return nil, err
	}
	var d struct {
		Intent yaml.Node `yaml:"intent"`
	}
	if err := yaml.Unmarshal(raw, &d); err != nil || d.Intent.IsZero() || d.Intent.Tag == "!!null" {
		return nil, nil
	}
	in, err := intent.DecodeSection(&d.Intent)
	if err != nil {
		return nil, fmt.Errorf("%w: %s's intent: %v", ErrInvalidArgument, DesignDescriptorName, err)
	}
	return in, nil
}

// DeleteArtifact removes one artifact of a stored design.
func (s *DesignService) DeleteArtifact(ctx context.Context, req *webapi.DeleteArtifactRequest) error {
	return s.catalog.DeleteArtifact(ctx, req.GetName())
}

// artifactWrite checks one UpdateArtifact request and returns its design id and the write.
func artifactWrite(req *webapi.UpdateArtifactRequest) (string, ArtifactWrite, error) {
	a := req.GetArtifact()
	d, id, ok := ArtifactID(a.GetName())
	if !ok || !newIDPattern.MatchString(id) {
		return "", ArtifactWrite{}, fmt.Errorf("%w: %q is not an artifact resource name, designs/{design}/artifacts/{artifact}, with an id of lowercase letters, digits, '.', '_' or '-'", ErrInvalidArgument, a.GetName())
	}
	if len(a.GetFiles()) == 0 {
		return "", ArtifactWrite{}, fmt.Errorf("%w: %s lists no files; DeleteArtifact removes an artifact", ErrInvalidArgument, a.GetName())
	}
	return d, ArtifactWrite{ID: id, Files: a.GetFiles(), AllowMissing: req.GetAllowMissing()}, nil
}

// withLatest sets d.series_latest against the other designs in its series.
func (s *DesignService) withLatest(ctx context.Context, d *webapi.Design) (*webapi.Design, error) {
	if d.GetSeries() == "" {
		return d, nil
	}
	all, _, err := s.catalog.Designs(ctx)
	if err != nil {
		return nil, err
	}
	markLatest(all)
	for _, o := range all {
		if o.GetName() == d.GetName() {
			d.SeriesLatest = o.GetSeriesLatest()
		}
	}
	return d, nil
}

// markLatest sets series_latest on the design each series updated last. Two designs updated at one
// instant are ordered by name, so the answer does not depend on listing order.
func markLatest(all []*webapi.Design) {
	latest := map[string]*webapi.Design{}
	for _, d := range all {
		d.SeriesLatest = false
		if d.GetSeries() == "" {
			continue
		}
		cur, ok := latest[d.GetSeries()]
		if !ok || newer(d, cur) {
			latest[d.GetSeries()] = d
		}
	}
	for _, d := range latest {
		d.SeriesLatest = true
	}
}

func newer(a, b *webapi.Design) bool {
	at, bt := a.GetUpdateTime().AsTime(), b.GetUpdateTime().AsTime()
	if !at.Equal(bt) {
		return at.After(bt)
	}
	return a.GetName() > b.GetName()
}

var designFilterPattern = regexp.MustCompile(`^\s*(series|project|source)\s*=\s*"([^"]*)"\s*$`)

// parseFilter reads the three supported AIP-160 filters, `series = "..."`, `project = "..."` and
// `source = "mount://m/path"`. The last keeps the designs with an artifact read from that folder of
// a mount, or from a folder under it (agni issue 1736). Any other filter is an error rather than an
// ignored argument, as parseProjectFilter explains.
func (s *DesignService) parseFilter(ctx context.Context, filter string) (func(*webapi.Design) bool, error) {
	if strings.TrimSpace(filter) == "" {
		return func(*webapi.Design) bool { return true }, nil
	}
	m := designFilterPattern.FindStringSubmatch(filter)
	if m == nil {
		return nil, fmt.Errorf(`%w: filter %q is not supported; the supported filters are series = "<series>", project = "projects/<project>" and source = "mount://<mount>/<path>"`, ErrInvalidArgument, filter)
	}
	switch m[1] {
	case "series":
		series := m[2]
		return func(d *webapi.Design) bool { return d.GetSeries() == series }, nil
	case "source":
		u, err := ParseArtifactURI(m[2])
		if err != nil {
			return nil, err
		}
		return func(d *webapi.Design) bool {
			arts, err := s.catalog.Artifacts(ctx, d.GetName())
			if err != nil {
				return false
			}
			return slices.ContainsFunc(arts, func(a *webapi.Artifact) bool {
				src := a.GetSource()
				return src.GetMount() == u.Mount && (u.Path == "" || src.GetPath() == u.Path || strings.HasPrefix(src.GetPath(), u.Path+"/"))
			})
		}, nil
	}
	if s.projects == nil {
		return nil, fmt.Errorf("%w: no project %q", ErrNotFound, m[2])
	}
	p, err := s.projects.GetProject(ctx, &webapi.GetProjectRequest{Name: m[2]})
	if err != nil {
		return nil, err
	}
	listed := map[string]bool{}
	for _, n := range p.GetDesigns() {
		listed[n] = true
	}
	return func(d *webapi.Design) bool { return listed[d.GetName()] }, nil
}
