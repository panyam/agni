package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// digestResolver resolves config and reports a digest for what it read, which is what a resolver has
// to do for an overlay built on it to be identifiable. digest is the value it reports; empty models
// a resolver that does not report one.
type digestResolver struct{ digest string }

func (r digestResolver) ResolveConfig(context.Context, *webapi.AnalysisConfig, string) (ResolvedConfig, error) {
	return ResolvedConfig{Profiles: true, Digest: r.digest}, nil
}

// idFor composes an overlay the way the service does and returns its identity, failing the test on a
// composition error so a caller can compare values directly.
func idFor(t *testing.T, p *webapi.Project, d *webapi.Design, req *webapi.OverlayConfig, resolver ConfigResolver, base string) (string, bool) {
	t.Helper()
	o, err := OverlayFor(context.Background(), resolver, nil, p, d, req, Overlay{}, base)
	if err != nil {
		t.Fatalf("OverlayFor: %v", err)
	}
	return o.Identity()
}

func project(name string) *webapi.Project { return &webapi.Project{Name: name} }

// Equal inputs compose to one identity. Without this the rest of the file proves nothing, since a
// scheme that returned a fresh value every call would pass every "it changed" assertion below.
func TestOverlayIdentityIsStableForEqualInputs(t *testing.T) {
	a, okA := idFor(t, project("projects/p"), nil, nil, nil, "house")
	b, okB := idFor(t, project("projects/p"), nil, nil, nil, "house")
	if !okA || !okB {
		t.Fatalf("identity refused for a fully identifiable overlay: %v %v", okA, okB)
	}
	if a != b {
		t.Errorf("two compositions of one configuration gave %q and %q, want one value", a, b)
	}
	if a == "" {
		t.Error("identity is empty, which a caller could mistake for a key")
	}
}

// Every input has to move it. An input that does not is an input a cache would ignore, which is how
// a run configured one way gets answered with work done under another.
func TestEveryOverlayInputMovesTheIdentity(t *testing.T) {
	baseline, ok := idFor(t, project("projects/p"), &webapi.Design{Name: "d"},
		&webapi.OverlayConfig{}, digestResolver{digest: "sha256:cfg"}, "house")
	if !ok {
		t.Fatal("baseline identity refused")
	}
	for _, tc := range []struct {
		name string
		id   func() (string, bool)
	}{
		{"the base convention", func() (string, bool) {
			return idFor(t, project("projects/p"), &webapi.Design{Name: "d"},
				&webapi.OverlayConfig{}, digestResolver{digest: "sha256:cfg"}, "OTHER")
		}},
		{"the project", func() (string, bool) {
			return idFor(t, project("projects/OTHER"), &webapi.Design{Name: "d"},
				&webapi.OverlayConfig{}, digestResolver{digest: "sha256:cfg"}, "house")
		}},
		{"the design", func() (string, bool) {
			return idFor(t, project("projects/p"), &webapi.Design{Name: "OTHER"},
				&webapi.OverlayConfig{}, digestResolver{digest: "sha256:cfg"}, "house")
		}},
		{"the request", func() (string, bool) {
			return idFor(t, project("projects/p"), &webapi.Design{Name: "d"},
				&webapi.OverlayConfig{IgnoreProject: false, Config: &webapi.AnalysisConfig{IntentUri: "mount://m/i.yaml"}},
				digestResolver{digest: "sha256:cfg"}, "house")
		}},
		// The one that is not a proto. Config a resolver READ changes the run without changing any
		// message the request or the descriptor carries, which is the whole reason a digest exists.
		{"the bytes the resolver read", func() (string, bool) {
			return idFor(t, project("projects/p"), &webapi.Design{Name: "d"},
				&webapi.OverlayConfig{}, digestResolver{digest: "sha256:DIFFERENT"}, "house")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.id()
			if !ok {
				t.Fatal("identity refused")
			}
			if got == baseline {
				t.Errorf("changing %s left the identity at %q, so a cache keyed on it would reuse work done under different configuration", tc.name, got)
			}
		})
	}
}

// A resolver that does not say what it read leaves the overlay unidentifiable, and unidentifiable
// has to read as a refusal rather than as an empty key: every such overlay would otherwise share
// one cache entry.
func TestOverlayWithoutAConfigDigestRefusesToIdentify(t *testing.T) {
	got, ok := idFor(t, project("projects/p"), nil, nil, digestResolver{digest: ""}, "house")
	if ok {
		t.Errorf("identity = %q, want a refusal: the resolver never said what it read", got)
	}
	if got != "" {
		t.Errorf("a refused identity returned %q, want the empty string", got)
	}
}

// An overlay nobody composed carries no inputs to hash. A hand-built one is the shape an embedder
// passes as a deployment default, and it must not answer as though it were identified.
func TestHandBuiltOverlayRefusesToIdentify(t *testing.T) {
	if got, ok := (Overlay{}).Identity(); ok {
		t.Errorf("a hand-built overlay identified as %q, want a refusal", got)
	}
}
