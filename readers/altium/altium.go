// Package altium reads an Altium Designer board, a binary .PcbDoc, into the neutral IR and the
// board geometry sidecar.
//
// A .PcbDoc is a Microsoft compound file (CFB), read after Microsoft's published MS-CFB
// specification. Altium publishes no specification for the records inside it. The layouts read here
// are from KiCad's reverse-engineered importer and its format notes, checked against five foxBMS
// boards and the ODB++ job Altium exported from each (agni-samples' altium and odb-altium
// artifacts).
//
// Fidelity: lossy-bounded. The netlist is every component with a designator and at least one named
// pad, which is the set Altium's own ODB++ export keeps, so mounting holes and fiducials are left
// out. The designator is the board's designator text, not the schematic's source designator, which
// repeats across the channels of a multi-channel design. Each pin's net comes from its pad's net
// index, and pads of one part sharing a name make one pin. A component carries its footprint, its
// comment as Value, and its schematic parameters (Part Number among them) from PrimitiveParameters.
// Assembly variants live in the project file and are not read. The board geometry carries the
// copper stack in physical order, renamed into the contract's F.Cu/In<n>.Cu/B.Cu vocabulary, every
// placement with footprint-local pads sized from the top copper, tracks, arc tracks as arcs, vias,
// polygon pour outlines with their nets as zones, the copper regions a pour filled and solid regions
// as copper shapes with their holes (Regions6, agni 1410), the board outline, and silkscreen lines,
// arcs and text. Region arcs are approximated by chords, as Regions6 stores them. A graphical
// component's copper (a layer marker), the tracks a hatched pour drew, fills (Fills6), per-layer
// pad stacks, mechanical layers, rules and classes are not kept, and a drilled pad no part owns is
// kept as a via.
// Coordinates are integer nanometres (unit_nm=1) from the board's origin, Y up.
package altium

import (
	"fmt"
	"io"

	"github.com/panyam/agni/internal/cfb"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Read parses a .PcbDoc into an ir.Design. sourceFile is provenance only.
func Read(r io.Reader, sourceFile string) (*ir.Design, error) {
	p, err := load(r, sourceFile)
	if err != nil {
		return nil, err
	}
	return p.toDesign(sourceFile), nil
}

// ReadBoardGeometry parses a .PcbDoc into its board geometry. sourceFile is provenance only.
func ReadBoardGeometry(r io.Reader, sourceFile string) (*geom.BoardGeometry, error) {
	p, err := load(r, sourceFile)
	if err != nil {
		return nil, err
	}
	return p.toBoard(sourceFile)
}

func load(r io.Reader, sourceFile string) (*pcb, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	c, err := cfb.Open(b)
	if err != nil {
		return nil, fmt.Errorf("%q: %w; an ASCII .PcbDoc is not read yet", sourceFile, err)
	}
	p, err := decode(c)
	if err != nil {
		return nil, fmt.Errorf("%q: %w", sourceFile, err)
	}
	return p, nil
}
