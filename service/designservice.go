package service

import (
	"context"
	"fmt"
	"regexp"

	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// DesignService serves the design resource, "designs/{design}", over a DesignCatalog (agni issue
// 1694). It does no I/O itself. It validates requests, applies AIP-158 pagination and the supported
// AIP-160 filters, and computes each series' latest design. The catalog decides which store holds a
// design and refuses a write to a mounted one.
type DesignService struct {
	catalog  *DesignCatalog
	blobs    BlobSource
	projects *ProjectService
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
	return &DesignService{catalog: catalog, blobs: blobs}
}

// WithProjects lets ListDesigns filter by `project = "projects/{p}"`, and returns s.
func (s *DesignService) WithProjects(projects *ProjectService) *DesignService {
	s.projects = projects
	return s
}

// CreateDesign stores a new design from its files.
func (s *DesignService) CreateDesign(ctx context.Context, req *webapi.CreateDesignRequest) (*webapi.Design, error) {
	if !newIDPattern.MatchString(req.GetDesignId()) {
		return nil, fmt.Errorf("%w: design id %q must be lowercase letters, digits, '.', '_' or '-', starting with a letter or digit", ErrInvalidArgument, req.GetDesignId())
	}
	d, err := s.catalog.CreateDesign(ctx, req.GetDesignId(), req.GetFiles(), s.blobs)
	if err != nil {
		return nil, err
	}
	return s.withLatest(ctx, d)
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

// UpdateDesign replaces a stored design's whole file set.
func (s *DesignService) UpdateDesign(ctx context.Context, req *webapi.UpdateDesignRequest) (*webapi.Design, error) {
	d, err := s.catalog.ReplaceDesign(ctx, req.GetName(), req.GetFiles(), s.blobs)
	if err != nil {
		return nil, err
	}
	return s.withLatest(ctx, d)
}

// DeleteDesign removes a stored design.
func (s *DesignService) DeleteDesign(ctx context.Context, req *webapi.DeleteDesignRequest) error {
	return s.catalog.DeleteDesign(ctx, req.GetName())
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

var designFilterPattern = regexp.MustCompile(`^\s*(series|project)\s*=\s*"([^"]*)"\s*$`)

// parseFilter reads the two supported AIP-160 filters, `series = "..."` and `project = "..."`. Any
// other filter is an error rather than an ignored argument, as parseProjectFilter explains.
func (s *DesignService) parseFilter(ctx context.Context, filter string) (func(*webapi.Design) bool, error) {
	if strings.TrimSpace(filter) == "" {
		return func(*webapi.Design) bool { return true }, nil
	}
	m := designFilterPattern.FindStringSubmatch(filter)
	if m == nil {
		return nil, fmt.Errorf(`%w: filter %q is not supported; the supported filters are series = "<series>" and project = "projects/<project>"`, ErrInvalidArgument, filter)
	}
	if m[1] == "series" {
		series := m[2]
		return func(d *webapi.Design) bool { return d.GetSeries() == series }, nil
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
