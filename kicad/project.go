package kicad

import (
	"io"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// ReadProject reads a KiCad project as one IR by combining the schematic (logical
// structure: part types, components with unit sections, sheets) with the board
// (connectivity: nets, plus the footprints components are placed as). Either reader may be
// nil: with only one present, its output is returned unchanged. The sources are recorded
// in provenance only; the caller owns file I/O (CONSTRAINTS C1) — open resolves each
// sub-sheet's (relative) Sheetfile reference for the hierarchy walk (WS1-018), and may be
// nil to read the root sheet alone.
//
// The join key is the reference designator: a schematic component and its board footprint
// share a ref_des. The schematic is authoritative for logical fields; the board supplies
// connectivity, footprints, each component's footprint_ref, and any board-only components
// (mounting holes, test points) that have no schematic symbol.
func ReadProject(schematic, board io.Reader, schematicSrc, boardSrc string, open func(relPath string) ([]byte, error)) (*ir.Design, error) {
	return ReadProjectWithSymbols(schematic, board, schematicSrc, boardSrc, open, nil)
}

// ReadProjectWithSymbols is ReadProject plus external symbol-library resolution
// (WS1-016); see ReadSchematicHierarchyNetsWithSymbols for the openSym contract.
func ReadProjectWithSymbols(schematic, board io.Reader, schematicSrc, boardSrc string, open func(relPath string) ([]byte, error), openSym func(string) ([]byte, error)) (*ir.Design, error) {
	var sch, pcb *ir.Design
	schComplete := false
	if schematic != nil {
		content, err := io.ReadAll(schematic)
		if err != nil {
			return nil, err
		}
		d, complete, err := ReadSchematicHierarchyNetsWithSymbols(schematicSrc, content, open, openSym)
		if err != nil {
			return nil, err
		}
		sch, schComplete = d, complete
	}
	if board != nil {
		d, err := Read(board, boardSrc)
		if err != nil {
			return nil, err
		}
		pcb = d
	}
	// The project file is the completeness witness (WS1-017): when the hierarchy walk
	// covered every sheet — trivially true for a sheetless root, and true for a walked
	// tree whose every Sheetfile opened — the external markings are stale and resolve to
	// global (netgraph.ResolveExternal). A partial walk (a missing sub-sheet file) keeps
	// them: those nets may genuinely continue into the unread sheets. A bare .kicad_sch
	// read never resolves: it may be one sheet of a larger design.
	if sch != nil && schComplete {
		netgraph.ResolveExternal(sch)
	}
	switch {
	case sch != nil && pcb != nil:
		return mergeProject(sch, pcb), nil
	case sch != nil:
		return sch, nil
	case pcb != nil:
		return pcb, nil
	default:
		return &ir.Design{IrVersion: "0", SourceFormat: "kicad"}, nil
	}
}

// mergeProject overlays board data onto the schematic's logical structure, keyed by
// ref_des. sch is the base (its libraries, components, sections, sheets, name are kept);
// from pcb it takes nets, footprints, each matched component's footprint_ref, and any
// board-only components.
func mergeProject(sch, pcb *ir.Design) *ir.Design {
	sch.SourceFormat = "kicad"

	byRef := make(map[string]*ir.Component, len(sch.Components))
	for _, c := range sch.Components {
		byRef[c.RefDes] = c
	}
	for _, pc := range pcb.Components {
		sc, ok := byRef[pc.RefDes]
		if !ok {
			sch.Components = append(sch.Components, pc) // board-only component
			byRef[pc.RefDes] = pc
			continue
		}
		sc.FootprintRef = pc.FootprintRef
		for k, v := range pc.Attributes { // fill logical attrs the schematic lacked
			if _, has := sc.Attributes[k]; !has {
				sc.Attributes[k] = v
			}
		}
	}

	sch.Nets = pcb.Nets
	sch.Footprints = pcb.Footprints
	return sch
}
