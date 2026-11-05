// Package allegro reads a Cadence Allegro board database, a binary `.brd`, into the neutral IR and
// the board geometry sidecar.
//
// Cadence publishes no specification for the format. The layouts read here are from KiCad's
// reverse-engineered notes and importer. 17.2 is checked block by block against a real board and an
// ASCII extract Allegro wrote from it (the BeagleBone AI, agni-samples' allegro artifact). 16.6 is
// checked against the Parallella parasdr's ODB++X and Clay R6's Capture netlist (agni issue 1388).
// Allegro 16.6 and 17.2 are read, each through its own layout; a board from another release is
// refused with the release named (agni issue 1355).
//
// Fidelity: lossy-bounded. The netlist is each placed, designated footprint, with each pin's net
// taken from the net's chain of connected pads. A net with no pin on it is dropped, as is an
// unnamed net, whose one pad reads as unconnected. Each component carries its footprint name, its
// value and its device type; pin names and pin use codes are not read yet (agni issue 1361). The
// board geometry carries the copper layer stack renamed into the contract's F.Cu/In<n>.Cu/B.Cu
// vocabulary, placements with footprint-local pads sized from the first copper layer of their
// padstack, ETCH tracks with their arcs kept as arcs, vias with their span, zone boundary
// outlines with their nets, the board outline, and silkscreen and assembly graphics and text. A
// board with no outline of its own takes the outline rectangle of a placed mechanical symbol. A pad
// Allegro draws as a shape symbol reads as custom, with no outline. Per-layer pad shapes, antipads
// and thermal reliefs (1356), computed fills, plane voids and standalone copper shapes (1357),
// constraint sets (1359), match groups (1360), keepouts and teardrops are not kept. Coordinates are
// integer nanometres (unit_nm=1), in Allegro's own Y-up frame.
package allegro

import (
	"bytes"
	"fmt"
	"io"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Read parses a .brd into an ir.Design. sourceFile is provenance only.
func Read(r io.Reader, sourceFile string) (*ir.Design, error) {
	p, err := load(r, sourceFile)
	if err != nil {
		return nil, err
	}
	return p.toDesign(sourceFile), nil
}

// ReadBoardGeometry parses a .brd into its board geometry. sourceFile is provenance only.
func ReadBoardGeometry(r io.Reader, sourceFile string) (*geom.BoardGeometry, error) {
	p, err := load(r, sourceFile)
	if err != nil {
		return nil, err
	}
	return p.toBoard(sourceFile)
}

// IsAllegro reports whether a file's first bytes are an Allegro database rather than an Eagle
// board, which shares the .brd extension and is XML. head needs at least 0x100 bytes.
func IsAllegro(head []byte) bool {
	return len(head) >= 0xFB && string(head[0xF8:0xFB]) == "all"
}

// IsEagle reports whether a file's first bytes are an Eagle XML board.
func IsEagle(head []byte) bool {
	h := bytes.TrimLeft(head, " \t\r\n\xEF\xBB\xBF")
	return bytes.HasPrefix(h, []byte("<?xml")) && bytes.Contains(head, []byte("<eagle"))
}

func load(r io.Reader, sourceFile string) (*board, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if IsEagle(b[:min(len(b), 1024)]) {
		return nil, fmt.Errorf("%q: an Eagle board, which agni does not read; only Cadence Allegro .brd boards are read", sourceFile)
	}
	p, err := parse(b)
	if err != nil {
		return nil, fmt.Errorf("%q: %w", sourceFile, err)
	}
	return p, nil
}
