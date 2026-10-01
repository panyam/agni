package service

import (
	"context"
	"github.com/panyam/agni/artifact"

	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/stdlib/profiles"
)

// GetInterfaceCoverage projects the built-in interface profiles onto the loaded design's coverage
// matrix (WS9-041), with one entry per DETECTED interface listing each required signal, its matched
// net and its state. It reuses profiles.Coverage, which runs the same datalog the profile rules
// compile to, so the coverage panel and the findings never disagree. A design with no detected
// interface yields an empty list rather than an error, since the rules are silent there too.
func (s *CheckService) GetInterfaceCoverage(ctx context.Context, req *webapi.GetInterfaceCoverageRequest) (*webapi.GetInterfaceCoverageResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	m, err := BuildModel(ctx, s.loader, u, artifact.URI{}, s.specs)
	if err != nil {
		return nil, err
	}
	resp := &webapi.GetInterfaceCoverageResponse{}
	for _, p := range profiles.Profiles {
		cov := profiles.Coverage(p, m)
		if cov == nil {
			continue
		}
		ic := &webapi.InterfaceCoverage{Profile: cov.Profile, AnchorNet: cov.Anchor}
		for _, sig := range cov.Signals {
			ic.Signals = append(ic.Signals, &webapi.SignalCoverage{Name: sig.Name, Net: sig.Net, State: sig.State})
		}
		resp.Interfaces = append(resp.Interfaces, ic)
	}
	return resp, nil
}
