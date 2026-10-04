package service

import (
	"context"
	"github.com/panyam/agni/artifact"
	"sort"

	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/stdlib/profiles"
)

// GetInterfaceCoverage projects the run's interface profiles onto the loaded design's coverage
// matrix (WS9-041), with one entry per DETECTED interface listing each required signal, its matched
// net and its state. It reuses profiles.Coverage, which runs the same datalog the profile rules
// compile to, so the coverage panel and the findings never disagree. A design with no detected
// interface yields an empty list rather than an error, since the rules are silent there too.
//
// It reads the design the way CheckDesign does, through the design's overlay and declared tiers, and
// walks the profiles that run's rules came from (Overlay.ProfileIndex). Until agni issue 833 it read
// the bare file against the built-ins alone, so a project's lexicon, its own profiles and its intent
// were absent from the panel while present in the findings beside it.
func (s *CheckService) GetInterfaceCoverage(ctx context.Context, req *webapi.GetInterfaceCoverageRequest) (*webapi.GetInterfaceCoverageResponse, error) {
	u, err := ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	ov, err := s.projects.Overlay(ctx, u, req.GetOverlay(), s.baseConvention)
	if err != nil {
		return nil, err
	}
	nu, bu, _, err := s.projects.TierURIs(ctx, u, artifact.URI{}, req.GetAsNamed())
	if err != nil {
		return nil, err
	}
	m, _, err := BuildModelCached(ctx, s.loader, nu, bu, ov, s.specs)
	if err != nil {
		return nil, err
	}
	index := ov.ProfileIndex(s.profiles)
	names := make([]string, 0, len(index))
	for name := range index {
		names = append(names, name)
	}
	sort.Strings(names)
	resp := &webapi.GetInterfaceCoverageResponse{}
	for _, name := range names {
		for _, p := range index[name] {
			cov := profiles.Coverage(ctx, p, m)
			if cov == nil {
				continue
			}
			ic := &webapi.InterfaceCoverage{Profile: cov.Profile, AnchorNet: cov.Anchor}
			for _, sig := range cov.Signals {
				ic.Signals = append(ic.Signals, &webapi.SignalCoverage{Name: sig.Name, Net: sig.Net, State: sig.State})
			}
			resp.Interfaces = append(resp.Interfaces, ic)
		}
	}
	return resp, nil
}
