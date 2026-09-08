package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/service"
)

// C32's Verify: one design resolves to the same tiers however it is named and whichever surface
// reads it.
//
// It lives in cmd/agni because this is the only package that can see BOTH surfaces. The CLI's
// resolver is unexported here and the service is importable, where service cannot import package
// main. internal/constraints is the other candidate and is wrong for this one: its own doc says it
// holds rules "whose violation is a line of source", and C32's violation was a function that was
// never CALLED, which no sweep over source can see.
//
// The rule was already broken when C32 was written. service.SourcesFor had two callers, both outside
// the served path, so the CLI attached a design's declared companions and the server did not: same
// file, same mount, same spelling, and a computed layout on one side against 82 real sheets on the
// other, with no error either way (agni issues 656 and 658).
const c32Design = "../../examples/tutorial-project/designs/gateway"

// tiersFromCLI resolves through the path cmd/agni takes: a typed path, a mount table, the CLI's own
// resolver.
func tiersFromCLI(t *testing.T, path string) service.DesignSources {
	t.Helper()
	src, err := resolve(t, path)
	if err != nil {
		t.Fatalf("cli resolve %s: %v", path, err)
	}
	return src.DesignSources
}

// tiersFromService resolves through the path a served request takes: an artifact URI and the
// ProjectResolver the services hold.
func tiersFromService(t *testing.T, ref string) service.DesignSources {
	t.Helper()
	u, err := artifact.Parse(ref)
	if err != nil {
		t.Fatalf("parse %s: %v", ref, err)
	}
	res, err := cliProjects().Sources(context.Background(), u, false)
	if err != nil {
		t.Fatalf("service resolve %s: %v", ref, err)
	}
	return res.DesignSources
}

// TestC32SurfacesResolveOneDesignAlike: every spelling of a design, through both surfaces, names the
// same artifact for each tier.
//
// The fixture declares a netlist entry with a schematic companion and a board companion, which is the
// shape that broke: a netlist carries no faithful geometry of its own, so a surface that does not
// resolve falls back to an auto-layout while the other draws the real sheets.
func TestC32SurfacesResolveOneDesignAlike(t *testing.T) {
	for _, spelling := range []string{
		c32Design,
		filepath.Join(c32Design, "gateway.edn"),
		filepath.Join(c32Design, "gateway.kicad_sch"),
	} {
		t.Run(filepath.Base(spelling), func(t *testing.T) {
			cli := tiersFromCLI(t, spelling)
			svc := tiersFromService(t, cli.NetlistURI) // the ref a served client would be handed
			if cli.NetlistURI != svc.NetlistURI {
				t.Errorf("netlist tier: cli %q, service %q", cli.NetlistURI, svc.NetlistURI)
			}
			if cli.GeometryURI != svc.GeometryURI {
				t.Errorf("geometry tier: cli %q, service %q", cli.GeometryURI, svc.GeometryURI)
			}
			if cli.BoardURI != svc.BoardURI {
				t.Errorf("board tier: cli %q, service %q", cli.BoardURI, svc.BoardURI)
			}
			// Agreement alone is not the property. Both surfaces resolving NOTHING agree perfectly,
			// and that is exactly the state this constraint exists to catch, so the tiers must also
			// have actually moved off the entry.
			if !strings.HasSuffix(cli.GeometryURI, ".kicad_sch") {
				t.Errorf("geometry tier = %q, want the declared schematic companion", cli.GeometryURI)
			}
			if !strings.HasSuffix(cli.BoardURI, ".kicad_pcb") {
				t.Errorf("board tier = %q, want the declared board companion", cli.BoardURI)
			}
		})
	}
}

// THE POSITIVE CONTROL, and the reason this file is not just an equality assertion.
//
// A test that only compares the two surfaces passes on a tree where resolution is entirely broken:
// both return the ref they were handed, and the refs match. `gateway-rev-b.edn` is a later revision
// sitting in the same folder and deliberately NOT declared a companion, so it must be read exactly as
// named on both surfaces. If the test above ever starts passing vacuously, this one is what still
// distinguishes "resolved to the design" from "resolved to nothing".
func TestC32LeavesAnUndeclaredSiblingAlone(t *testing.T) {
	sibling := filepath.Join(c32Design, "gateway-rev-b.edn")
	cli := tiersFromCLI(t, sibling)
	if !strings.HasSuffix(cli.NetlistURI, "gateway-rev-b.edn") {
		t.Errorf("an undeclared revision was redirected: netlist %q", cli.NetlistURI)
	}
	if cli.GeometryURI != cli.NetlistURI || cli.BoardURI != cli.NetlistURI {
		t.Errorf("an undeclared revision picked up tiers: %+v", cli)
	}
	svc := tiersFromService(t, cli.NetlistURI)
	if svc.GeometryURI != svc.NetlistURI {
		t.Errorf("the service redirected an undeclared revision: %+v", svc)
	}
	// And the discriminator: the design's OWN entry does resolve, so "read as named" above is a
	// decision rather than a resolver that never works.
	entry := tiersFromCLI(t, filepath.Join(c32Design, "gateway.edn"))
	if entry.GeometryURI == entry.NetlistURI {
		t.Fatal("the entry resolved to no companion, so the control above proves nothing")
	}
}
