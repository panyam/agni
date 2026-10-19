package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/core/render"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// uriLoader records the artifact each read was asked of, which is what an rpc that skips TierURIs
// gets wrong: it hands the loader the request's spelling rather than the tier it should read.
type uriLoader struct {
	*recordingLoader
	design, report []string
}

func (l *uriLoader) Design(ctx context.Context, u artifact.URI, opts ...ReadOption) (*ir.Design, error) {
	l.mu.Lock()
	l.design = append(l.design, u.String())
	l.mu.Unlock()
	return l.recordingLoader.Design(ctx, u, opts...)
}

func (l *uriLoader) Report(ctx context.Context, u artifact.URI, faithful bool, opts ...ReadOption) (*graph.ConversionReport, error) {
	l.mu.Lock()
	l.report = append(l.report, u.String())
	l.mu.Unlock()
	return l.recordingLoader.Report(ctx, u, faithful, opts...)
}

// Both rpcs passed the request URI straight to the loader, so naming the design folder read nothing:
// the layout report came back empty and the viewer's parts panel got invalid_argument on every open
// (agni issue 736). Every spelling of one design has to read the same tier.
func TestLayoutReportAndComponentParamsReadTheDesignsTiers(t *testing.T) {
	spellings := []string{"mount://m/d", "mount://m/d/board.edn", "mount://m/d/board.eds"}
	projects := &ProjectResolver{Store: declaringStore{design: withCompanion()}}
	for _, spelling := range spellings {
		t.Run(spelling, func(t *testing.T) {
			l := &uriLoader{recordingLoader: newRecordingLoader()}
			ds := NewDesignService(l, noNative{}, render.DefaultStyle, projects)
			snap := snapOf(t, ds.Snapshots(), spelling)
			l.design = nil // the ingest's own read, so what is counted below is GetComponentParams'
			if _, err := ds.GetLayoutReport(context.Background(), &webapi.GetLayoutReportRequest{Snapshot: snap}); err != nil {
				t.Fatalf("GetLayoutReport: %v", err)
			}
			if len(l.report) != 1 || l.report[0] != "mount://m/d/board.eds" {
				t.Errorf("GetLayoutReport read %v, want the declared schematic companion alone", l.report)
			}
			cs := NewCheckService(l, nil, nil, "", nil, projects)
			if _, err := cs.GetComponentParams(context.Background(), &webapi.GetComponentParamsRequest{Snapshot: snap}); err != nil {
				t.Fatalf("GetComponentParams: %v", err)
			}
			if len(l.design) == 0 {
				t.Fatal("GetComponentParams read no netlist")
			}
			for _, got := range l.design {
				if got != "mount://m/d/board.edn" {
					t.Errorf("GetComponentParams read %q, want the declared entry", got)
				}
			}
		})
	}
}
