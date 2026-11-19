package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// ReportStore keeps the reports run on stored designs (agni issue 1704). A report is a check of one
// design against packs, kept as "designs/{design}/reports/{report}". Like ReviewStore it is a port,
// and the folder adapter is `internal/projects.DirReports`.
//
// Errors:
//   - ErrInvalidArgument: a malformed design or report id.
//   - ErrAlreadyExists: a create whose report id the design already holds.
//   - ErrNotFound: a report the store does not keep.
type ReportStore interface {
	// CreateReport keeps r under design. An empty id has the store mint one that sorts by creation
	// time. It returns r with name and create_time set, and the store owns both.
	CreateReport(ctx context.Context, design, id string, r *webapi.Report) (*webapi.Report, error)
	// Report returns one report as kept.
	Report(ctx context.Context, design, id string) (*webapi.Report, error)
	// Reports returns every report of design, newest first. A design with none returns none.
	Reports(ctx context.Context, design string) ([]*webapi.Report, error)
	// DeleteReport removes one report.
	DeleteReport(ctx context.Context, design, id string) error
	// DeleteReports removes every report of design. A design with none is not an error.
	DeleteReports(ctx context.Context, design string) error
}

const reportsSegment = "/reports/"

// ReportName builds a report's resource name, "designs/{design}/reports/{report}".
func ReportName(design, id string) string {
	return StandaloneDesignName(design) + reportsSegment + id
}

// ReportID splits "designs/{design}/reports/{report}" into its two ids.
func ReportID(name string) (design, report string, ok bool) {
	parent, r, found := strings.Cut(name, reportsSegment)
	if !found {
		return "", "", false
	}
	d, ok := StandaloneDesignID(parent)
	if !ok || !validResourceID(r) {
		return "", "", false
	}
	return d, r, true
}

// WithReports lets the service keep reports in store, running each with checks, and returns s (agni
// issue 1704). checks must share the resolver whose Designs and Packs this service's snapshots read.
// Without it, every report method answers FAILED_PRECONDITION.
func (s *DesignService) WithReports(store ReportStore, checks *CheckService, packs PackLookup) *DesignService {
	s.reports, s.checks, s.packs = store, checks, packs
	return s
}

var errNoReports = fmt.Errorf("%w: this server keeps no reports; start it with --store", ErrFailedPrecondition)

// CreateReport checks a stored design against packs and keeps the run.
func (s *DesignService) CreateReport(ctx context.Context, req *webapi.CreateReportRequest) (*webapi.Report, error) {
	if s.reports == nil || s.checks == nil {
		return nil, errNoReports
	}
	if id := req.GetReportId(); id != "" && !newIDPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: report id %q must be lowercase letters, digits, '.', '_' or '-', starting with a letter or digit", ErrInvalidArgument, id)
	}
	design, err := s.reportable(ctx, req.GetParent())
	if err != nil {
		return nil, err
	}
	packs, err := s.reportPacks(ctx, req.GetProject(), req.GetReport().GetPacks())
	if err != nil {
		return nil, err
	}
	read, err := ResolvePacks(ctx, s.packs, packs)
	if err != nil {
		return nil, err
	}
	before, err := s.designHash(ctx, req.GetParent())
	if err != nil {
		return nil, err
	}
	results, err := s.runReport(ctx, req.GetParent(), packs)
	if err != nil {
		return nil, err
	}
	// A write between the hash and the read would record the run against files it never read.
	if after, err := s.designHash(ctx, req.GetParent()); err != nil {
		return nil, err
	} else if after != before {
		return nil, fmt.Errorf("%w: %s changed while its report ran; ask again", ErrFailedPrecondition, req.GetParent())
	}
	r := &webapi.Report{Packs: packs, Project: req.GetProject(), DesignHash: before, Results: results}
	for _, p := range read {
		r.PackHashes = append(r.PackHashes, p.GetHash())
	}
	kept, err := s.reports.CreateReport(ctx, design, req.GetReportId(), r)
	if err != nil {
		return nil, err
	}
	return inView(kept, webapi.ReportView_REPORT_VIEW_FULL, webapi.ReportView_REPORT_VIEW_FULL), nil
}

// reportable returns the id of a design the store holds. A design only a mounted reader holds has
// no artifacts to hash, so it takes no report.
func (s *DesignService) reportable(ctx context.Context, name string) (string, error) {
	id, ok := StandaloneDesignID(name)
	if !ok {
		return "", fmt.Errorf("%w: %q is not a design resource name, designs/{design}", ErrInvalidArgument, name)
	}
	_, stored, err := s.catalog.holder(ctx, name)
	if err != nil {
		return "", err
	}
	if !stored {
		return "", fmt.Errorf("%w: %s is declared in a mounted folder, and a report needs a design the server stores; add it with AddDesigns", ErrFailedPrecondition, name)
	}
	return id, nil
}

// reportPacks lists the packs a report applies: the project's default packs, then the request's. A
// pack named in both applies once, where the project put it.
func (s *DesignService) reportPacks(ctx context.Context, project string, named []string) ([]string, error) {
	var out []string
	if project != "" {
		if s.projects == nil {
			return nil, fmt.Errorf("%w: this server resolves no projects, so it cannot read %s", ErrFailedPrecondition, project)
		}
		p, err := s.projects.GetProject(ctx, &webapi.GetProjectRequest{Name: project})
		if err != nil {
			return nil, err
		}
		out = append(out, p.GetDefaultPacks()...)
	}
	for _, n := range named {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	for _, n := range out {
		if _, ok := PackID(n); !ok {
			return nil, fmt.Errorf("%w: %q is not a pack resource name, packs/{pack}", ErrInvalidArgument, n)
		}
	}
	return out, nil
}

// runReport ingests the design under packs, checks the whole catalog, and drops the snapshot.
func (s *DesignService) runReport(ctx context.Context, design string, packs []string) (*webapi.CheckDesignResponse, error) {
	snaps := s.checks.Snapshots()
	snap, err := snaps.Ingest(ctx, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Design: design, Packs: packs}})
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = snaps.DeleteSnapshot(context.WithoutCancel(ctx), &webapi.DeleteSnapshotRequest{Name: snap.GetName()})
	}()
	return s.checks.CheckDesign(ctx, &webapi.CheckDesignRequest{Snapshot: snap.GetName()})
}

// designHash names a stored design's content: its intent and each artifact's files, by path and
// hash. A change to any file, to the set of artifacts, or to the intent moves it.
func (s *DesignService) designHash(ctx context.Context, name string) (string, error) {
	d, err := s.catalog.Design(ctx, name)
	if err != nil {
		return "", err
	}
	arts, err := s.catalog.Artifacts(ctx, name)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	b, _ := proto.MarshalOptions{Deterministic: true}.Marshal(d.GetConfig().GetIntent())
	h.Write(b)
	for _, a := range arts {
		fmt.Fprintf(h, "\x01%s", a.GetName())
		for _, f := range a.GetFiles() {
			fmt.Fprintf(h, "\x00%s\x00%s", f.GetPath(), f.GetSha256())
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// GetReport returns one report, saying what changed since it ran.
func (s *DesignService) GetReport(ctx context.Context, req *webapi.GetReportRequest) (*webapi.Report, error) {
	if s.reports == nil {
		return nil, errNoReports
	}
	design, id, ok := ReportID(req.GetName())
	if !ok {
		return nil, fmt.Errorf("%w: %q is not a report resource name, designs/{design}/reports/{report}", ErrInvalidArgument, req.GetName())
	}
	r, err := s.reports.Report(ctx, design, id)
	if err != nil {
		return nil, err
	}
	out, err := s.withChanges(ctx, r, newHashes(s))
	if err != nil {
		return nil, err
	}
	return inView(out, req.GetView(), webapi.ReportView_REPORT_VIEW_FULL), nil
}

// inView returns r as view asks, with fallback standing for an unspecified view. Every view sets
// finding_count, and BASIC leaves results out.
func inView(r *webapi.Report, view, fallback webapi.ReportView) *webapi.Report {
	if view == webapi.ReportView_REPORT_VIEW_UNSPECIFIED {
		view = fallback
	}
	r.FindingCount = int32(min(len(r.GetResults().GetFindings()), math.MaxInt32)) // #nosec G115 -- clamped
	if view == webapi.ReportView_REPORT_VIEW_BASIC {
		r.Results = nil
	}
	return r
}

// ListReports returns a design's reports, newest first, each saying what changed since it ran.
func (s *DesignService) ListReports(ctx context.Context, req *webapi.ListReportsRequest) (*webapi.ListReportsResponse, error) {
	if s.reports == nil {
		return nil, errNoReports
	}
	design, ok := StandaloneDesignID(req.GetParent())
	if !ok {
		return nil, fmt.Errorf("%w: %q is not a design resource name, designs/{design}", ErrInvalidArgument, req.GetParent())
	}
	if err := noFilter("reports", req.GetFilter()); err != nil {
		return nil, err
	}
	all, err := s.reports.Reports(ctx, design)
	if err != nil {
		return nil, err
	}
	page, next := paginate(len(all), req.GetPageSize(), req.GetPageToken(), func(i int) string { return all[i].GetName() })
	resp := &webapi.ListReportsResponse{NextPageToken: next}
	hashes := newHashes(s)
	for _, i := range page {
		r, err := s.withChanges(ctx, all[i], hashes)
		if err != nil {
			return nil, err
		}
		resp.Reports = append(resp.Reports, inView(r, req.GetView(), webapi.ReportView_REPORT_VIEW_BASIC))
	}
	return resp, nil
}

// DeleteReport removes one report.
func (s *DesignService) DeleteReport(ctx context.Context, req *webapi.DeleteReportRequest) error {
	if s.reports == nil {
		return errNoReports
	}
	design, id, ok := ReportID(req.GetName())
	if !ok {
		return fmt.Errorf("%w: %q is not a report resource name, designs/{design}/reports/{report}", ErrInvalidArgument, req.GetName())
	}
	return s.reports.DeleteReport(ctx, design, id)
}

// currentHashes looks up each design and pack hash once per request, so a listing of many reports
// hashes a design once.
type currentHashes struct {
	s      *DesignService
	design map[string]string
	pack   map[string]string
}

func newHashes(s *DesignService) *currentHashes {
	return &currentHashes{s: s, design: map[string]string{}, pack: map[string]string{}}
}

// designNow is the design's hash now, "" when it is gone.
func (c *currentHashes) designNow(ctx context.Context, name string) (string, error) {
	if h, ok := c.design[name]; ok {
		return h, nil
	}
	h, err := c.s.designHash(ctx, name)
	if errors.Is(err, ErrNotFound) {
		h, err = "", nil
	}
	if err != nil {
		return "", err
	}
	c.design[name] = h
	return h, nil
}

// packNow is the pack's hash now, "" when it is gone or the host keeps no packs.
func (c *currentHashes) packNow(ctx context.Context, name string) (string, error) {
	if h, ok := c.pack[name]; ok {
		return h, nil
	}
	h := ""
	if c.s.packs != nil {
		p, err := c.s.packs.GetPack(ctx, &webapi.GetPackRequest{Name: name})
		switch {
		case err == nil:
			h = p.GetHash()
		case !errors.Is(err, ErrNotFound):
			return "", err
		}
	}
	c.pack[name] = h
	return h, nil
}

// withChanges returns a copy of r with design_changed and packs_changed set from the hashes now.
func (s *DesignService) withChanges(ctx context.Context, r *webapi.Report, hashes *currentHashes) (*webapi.Report, error) {
	out := proto.Clone(r).(*webapi.Report)
	design, _, _ := ReportID(r.GetName())
	now, err := hashes.designNow(ctx, StandaloneDesignName(design))
	if err != nil {
		return nil, err
	}
	out.DesignChanged = now != r.GetDesignHash()
	out.PacksChanged = nil
	for i, p := range r.GetPacks() {
		h, err := hashes.packNow(ctx, p)
		if err != nil {
			return nil, err
		}
		if i >= len(r.GetPackHashes()) || h != r.GetPackHashes()[i] {
			out.PacksChanged = append(out.PacksChanged, p)
		}
	}
	return out, nil
}
