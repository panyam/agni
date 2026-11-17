package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// declaringStore resolves every ref to one design, a netlist entry with a schematic companion. That
// shape broke. A nil design models a ref belonging to no declared design. That is the ordinary case
// for a mounted folder, and such a ref must read exactly what it names.
type declaringStore struct {
	ProjectStore
	design *webapi.Design
}

func (s declaringStore) ResolveDesign(context.Context, artifact.URI) (*webapi.Design, *webapi.Project, error) {
	return s.design, nil, nil
}

func withCompanion() *webapi.Design {
	return &webapi.Design{
		Uri:           "mount://m/d",
		EntryUri:      "mount://m/d/board.edn",
		CompanionUris: []string{"mount://m/d/board.eds"},
	}
}

func mustURI(t *testing.T, s string) artifact.URI {
	t.Helper()
	u, err := artifact.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return u
}

// The three spellings of one design must resolve to the same tiers. They did not, because the
// served path never called SourcesFor, so a netlist entry yielded no faithful geometry and the
// design fell back to an auto-layout while the CLI drew the companion's sheets (agni issue 656,
// constraint C32).
func TestSourcesAgreeAcrossEverySpelling(t *testing.T) {
	r := &ProjectResolver{Store: declaringStore{design: withCompanion()}}
	for _, spelling := range []string{"mount://m/d", "mount://m/d/board.edn", "mount://m/d/board.eds"} {
		t.Run(spelling, func(t *testing.T) {
			got, err := r.Sources(context.Background(), mustURI(t, spelling), false)
			if err != nil {
				t.Fatal(err)
			}
			if got.NetlistURI != "mount://m/d/board.edn" {
				t.Errorf("netlist tier = %q, want the declared entry", got.NetlistURI)
			}
			if got.GeometryURI != "mount://m/d/board.eds" {
				t.Errorf("geometry tier = %q, want the declared companion", got.GeometryURI)
			}
		})
	}
}

// THE POSITIVE CONTROL. This guards against both surfaces resolving nothing. An assertion that only
// checks agreement passes on a design where nothing is attached. This pins the other direction.
// With no design, every tier is the ref, and the test above would fail here.
func TestSourcesLeaveAnUndeclaredRefAlone(t *testing.T) {
	r := &ProjectResolver{Store: declaringStore{design: nil}}
	ref := "mount://m/loose/board.edn"
	got, err := r.Sources(context.Background(), mustURI(t, ref), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.NetlistURI != ref || got.GeometryURI != ref || got.BoardURI != ref {
		t.Errorf("an undeclared ref must be read exactly as named, got %+v", got.DesignSources)
	}
	if got.FromDeclaration {
		t.Error("nothing was declared, so no declaration was applied")
	}
	// This is the control proper. If resolution silently stopped working, the test above would
	// report the ref in every tier too, and only this assertion separates "read as named" from
	// "resolved nothing".
	if got.GeometryURI == "mount://m/d/board.eds" {
		t.Fatal("a loose ref resolved to another design's companion")
	}
}

// as_named is the CLI's opt-out and has to survive the wire, because the CLI is itself a client of
// these services. Resolving unconditionally here once silently overrode it. TestCheckBoardPath
// caught that by finding board rules firing on a netlist read as-named.
func TestSourcesHonourAsNamed(t *testing.T) {
	r := &ProjectResolver{Store: declaringStore{design: withCompanion()}}
	ref := "mount://m/d/board.eds"
	got, err := r.Sources(context.Background(), mustURI(t, ref), true)
	if err != nil {
		t.Fatal(err)
	}
	if got.NetlistURI != ref || got.GeometryURI != ref {
		t.Errorf("as-named must read exactly the artifact named, got %+v", got.DesignSources)
	}
}

// A folder is not a readable artifact, so as-named has no meaning for one and must not resolve every
// tier to a directory.
func TestAsNamedDoesNotApplyToTheDesignItself(t *testing.T) {
	got := ResolveSources(withCompanion(), "mount://m/d", true, true)
	if got.NetlistURI != "mount://m/d/board.edn" {
		t.Errorf("naming the design must still read its entry, got %q", got.NetlistURI)
	}
}

// A later netlist beside the entry is read exactly as named, with no board and no schematic of the
// entry's. A later version is a design in its own folder (agni 1692), so nothing in this folder's
// descriptor can hand it a board.
func TestALaterNetlistBesideTheEntryReadsAsNamed(t *testing.T) {
	ref := "mount://m/d/board-rev-b.edn"
	got := ResolveSources(withCompanion(), ref, false, false)
	if got.NetlistURI != ref || got.BoardURI != ref || got.GeometryURI != ref || got.FromDeclaration {
		t.Errorf("a later netlist resolved to %+v, want it read as named", got)
	}
}
