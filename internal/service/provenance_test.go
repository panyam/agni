package service

import "testing"

// TestOverlayProvenanceUnionsBothSources is the fix for the bug this replaced: a results document's
// RunConfig was built from the SERVICE's startup config while the run had used the resolved overlay, so
// a design in a project declaring params/, profiles/ and conventions.yaml scored against all three and
// recorded none of them.
//
// Union rather than override, because a deployment's --profile-path and a project's own profiles both
// genuinely reach the catalog. A document claiming only one would misdescribe its own catalog snapshot.
func TestOverlayProvenanceUnionsBothSources(t *testing.T) {
	// someSpecs is the package's existing non-nil provider helper; contents are irrelevant, since the
	// Params flag reports that a corpus was ATTACHED, not that a part is seeded.
	corpus := someSpecs()
	for name, tc := range map[string]struct {
		overlay    Overlay
		deployment RunProvenance
		want       RunProvenance
	}{
		"project supplies everything, deployment nothing": {
			overlay: Overlay{Specs: corpus, Profiles: true, Intent: true, conventionName: "gateway"},
			want:    RunProvenance{Params: true, Profiles: true, Intent: true, Conventions: "gateway"},
		},
		"deployment supplies everything, design has no project": {
			deployment: RunProvenance{Params: true, Profiles: true, Intent: true, Conventions: "house"},
			want:       RunProvenance{Params: true, Profiles: true, Intent: true, Conventions: "house"},
		},
		"both, and each tier is on when either side has it": {
			overlay:    Overlay{Profiles: true},
			deployment: RunProvenance{Params: true},
			want:       RunProvenance{Params: true, Profiles: true},
		},
		"neither": {},
		// The convention is the one tier that does NOT union, because a request-supplied one already
		// REPLACED whatever was in place by the time this is called (WS3-124). The deployment's name is
		// only the answer when nothing replaced it.
		"project convention wins over the deployment default": {
			overlay:    Overlay{conventionName: "gateway"},
			deployment: RunProvenance{Conventions: "house"},
			want:       RunProvenance{Conventions: "gateway"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := tc.overlay.Provenance(tc.deployment)
			if got != tc.want {
				t.Errorf("Provenance() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
