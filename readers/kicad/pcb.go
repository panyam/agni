package kicad

import (
	"fmt"
	"io"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/refdes"
)

// kicadNativeIDKind tags a KiCad uuid in Provenance. Unlike the EDIF rename &id, a KiCad
// uuid is stable across exports, so it is a sound native id (though the semantic keys --
// ref_des, net name, pad -- remain the cross-revision join key).
const kicadNativeIDKind = "kicad-uuid"

// Read parses a KiCad .kicad_pcb board into an ir.Design.
//
// Fidelity: lossy-bounded (netlist subset). We extract physical components (from
// footprints), the footprints they place (the provisional physical tier), and net
// connectivity (from pad net assignments). The logical structure -- part types, pins, and
// multi-unit sections -- lives in the .kicad_sch schematic and is a separate reader; a PCB
// component therefore has no ComponentSections. sourceFile is recorded in provenance only;
// the caller owns file I/O so the core stays runtime-agnostic (CONSTRAINTS C1).
func Read(r io.Reader, sourceFile string) (*ir.Design, error) {
	root, err := parse(r)
	if err != nil {
		return nil, err
	}
	if root.Head() != "kicad_pcb" {
		return nil, fmt.Errorf("kicad: not a .kicad_pcb file (root is %q)", root.Head())
	}
	return extractPCB(root, sourceFile), nil
}

// extractPCB walks the board tree and builds the netlist IR: nets, physical components
// (footprints), the footprint library, and pad-to-net connections.
func extractPCB(root *node, src string) *ir.Design {
	d := &ir.Design{
		IrVersion:    "0",
		SourceFormat: "kicad-pcb",
		Attributes:   map[string]string{},
		Prov:         &ir.Provenance{SourceFile: src},
	}
	if g := root.Child("generator"); g != nil {
		d.Attributes["kicad_generator"] = atomOf(g.Arg(1))
	}
	if v := root.Child("version"); v != nil {
		d.Attributes["kicad_version"] = atomOf(v.Arg(1))
	}
	if tb := root.Child("title_block"); tb != nil {
		d.Name = atomOf(tb.Child("title").Arg(1))
	}

	// Nets are keyed by name, built from the pads that reference them. This is robust to
	// both KiCad formats: the older (net <number> "<name>") pad form and the KiCad 10
	// (net "<name>") name-only form. The top-level (net ...) table is not needed. An empty
	// name is the "no net" sentinel and is skipped.
	netByName := map[string]*ir.Net{}
	// Dedup connections per net so a pad number that appears twice on the same net (e.g. a
	// split thermal pad) counts once.
	connSeen := map[*ir.Net]map[string]bool{}
	seenFp := map[string]bool{}
	for _, fp := range root.Children("footprint") {
		ref := propValue(fp, "Reference")
		// A placeholder reference ("REF**", "C?1845") is annotation state, not an identity, so
		// keying it merges distinct parts' pads onto one component (WS1-024). Placeholder
		// footprints are skipped like Reference-less graphics by BOTH this reader and the
		// board-geometry one, so the two artifacts keep agreeing on the component set. The
		// predicate is internal/refdes's rather than this reader's; see its package comment.
		if ref == "" || refdes.IsPlaceholder(ref) {
			continue
		}
		fpid := atomOf(fp.Arg(1))
		comp := &ir.Component{
			RefDes:       ref,
			FootprintRef: fpid,
			Attributes:   map[string]string{},
			Prov:         &ir.Provenance{SourceFile: src, NativeId: uuidOf(fp), NativeIdKind: kicadNativeIDKind},
		}
		if val := propValue(fp, "Value"); val != "" {
			comp.Attributes["Value"] = val
		}
		// A placed footprint is one physical section (a board has no multi-unit split). Emit it
		// so section-aware consumers (diff's per-section part_ref, check's pin walk) see the same
		// structure they get from EDIF/schematic readers; the "part" is the footprint.
		comp.Sections = []*ir.ComponentSection{{
			Index:      0,
			PartRef:    fpid,
			LibraryRef: libPrefix(fpid),
			Attributes: map[string]string{},
			Prov:       &ir.Provenance{SourceFile: src, NativeId: uuidOf(fp), NativeIdKind: kicadNativeIDKind},
		}}
		d.Components = append(d.Components, comp)

		if fpid != "" && !seenFp[fpid] {
			seenFp[fpid] = true
			d.Footprints = append(d.Footprints, &ir.Footprint{
				Name:    fpid,
				Library: libPrefix(fpid),
				Prov:    &ir.Provenance{SourceFile: src},
			})
		}

		for _, pad := range fp.Children("pad") {
			name := padNetName(pad)
			if name == "" {
				continue // unconnected pad
			}
			net := netByName[name]
			if net == nil {
				net = &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: src}}
				netByName[name] = net
				d.Nets = append(d.Nets, net)
			}
			pin := atomOf(pad.Arg(1))
			key := ref + "." + pin
			if connSeen[net] == nil {
				connSeen[net] = map[string]bool{}
			}
			if connSeen[net][key] {
				continue
			}
			connSeen[net][key] = true
			net.Connections = append(net.Connections, &ir.Connection{
				ComponentRef: ref,
				PinRef:       pin,
				Prov:         &ir.Provenance{SourceFile: src},
			})
		}
	}
	return d
}

// padNetName returns the net name a pad is on, or "" if it has none. Handles both the
// older (net <number> "<name>") form and the KiCad 10 (net "<name>") form by taking the
// last argument (always the name).
func padNetName(pad *node) string {
	nref := pad.Child("net")
	if nref == nil || len(nref.Kids) < 2 {
		return ""
	}
	return atomOf(nref.Arg(len(nref.Kids) - 1))
}

// propValue returns the value of the first (property "<key>" "<value>" ...) child of n.
func propValue(n *node, key string) string {
	for _, p := range n.Children("property") {
		if atomOf(p.Arg(1)) == key {
			return atomOf(p.Arg(2))
		}
	}
	return ""
}

// propNode returns the (property "key" ...) child node, or nil. Unlike propValue it exposes
// the whole node so callers can read the property's position/effects, not just its value.
func propNode(n *node, key string) *node {
	for _, p := range n.Children("property") {
		if atomOf(p.Arg(1)) == key {
			return p
		}
	}
	return nil
}

// uuidOf returns a node's (uuid "...") child value, or "".
func uuidOf(n *node) string {
	if u := n.Child("uuid"); u != nil {
		return atomOf(u.Arg(1))
	}
	return ""
}

// libPrefix returns the library part of a "Lib:Name" id (footprint id or symbol lib_id),
// or "" if unqualified.
func libPrefix(id string) string {
	if i := strings.IndexByte(id, ':'); i >= 0 {
		return id[:i]
	}
	return ""
}
