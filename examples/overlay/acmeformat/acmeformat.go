// Package acmeformat is a demonstration out-of-module format reader for the open-core overlay
// skeleton (WS12-001). It parses a toy ".acme" netlist into the agni IR and registers itself
// with the engine's public formats registry (WS12-003). Blank-importing it for the side effect
// (import _ ".../acmeformat") makes ".acme" resolve through every engine surface — the CLI
// reader dispatch, the file-tree label, the Loader — with no fork of the engine.
//
// A real overlay's reader would be a proprietary schematic/netlist format the house does not
// release; the point here is only the wiring, so the format is deliberately trivial.
package acmeformat

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/panyam/agni/readers/formats"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// init registers the .acme reader. An overlay chooses import-side-effect registration (like the
// standard library's image format readers) so a consumer wires the format in with one blank
// import; the alternative is an explicit call from the composing binary's main (see WS12-003).
func init() {
	formats.Register(&formats.Format{
		Ext:  ".acme",
		Name: "acme",
		// The registry entry owns the file open (C1: the Loader owns I/O); Read stays io.Reader-pure.
		Design: func(_ *formats.Loader, path string) (*ir.Design, error) {
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			return Read(f, path)
		},
	})
}

// pinDirections maps the toy format's pin-type spelling to the IR enum. Only the types this
// example needs are here; an unrecognized one is an error rather than a silent UNSPECIFIED,
// because a pin that reads as untyped is exactly what a pin-type rule would then miss.
var pinDirections = map[string]ir.PinDirection{
	"power_in":  ir.PinDirection_PIN_DIRECTION_POWER_IN,
	"power_out": ir.PinDirection_PIN_DIRECTION_POWER_OUT,
	"input":     ir.PinDirection_PIN_DIRECTION_INPUT,
	"output":    ir.PinDirection_PIN_DIRECTION_OUTPUT,
	"inout":     ir.PinDirection_PIN_DIRECTION_INOUT,
}

// partByName finds an already-declared PartType so a `pin` line can attach to it. Linear because a
// toy netlist is small and the reader stays dependency-free.
//
// Pins belong to a PART TYPE in the IR, not to a placed component: one part definition is shared by
// every placement of it, and a ComponentSection points at it by PartRef. This toy format synthesizes
// one part type PER COMPONENT (named after the ref-des) so `pin <ref> ...` reads naturally, at the
// cost of not sharing a definition between two placements of the same part. A real reader emits one
// part type per library part and points many components at it.
func partByName(d *ir.Design, name string) *ir.PartType {
	for _, lib := range d.Libraries {
		for _, p := range lib.Parts {
			if p.Name == name {
				return p
			}
		}
	}
	return nil
}

// Read parses the toy .acme netlist into an ir.Design. The grammar is three line kinds, with
// blank lines and '#' comments ignored:
//
//	component <ref> <kind> [value]
//	pin <ref> <designator> <name> [power_in|power_out|input|output|inout]
//	net <name> <ref>.<pin> [<ref>.<pin> ...]
//
// The `pin` line declares a component's PART-TYPE pins, which is a different thing from a net
// connection: a connection says a pin is wired somewhere, a pin declaration says the pin exists
// and what it is called. The engine's pin relations (pin, pin.role, pin.type, pin.net) project
// from the declared pins, so a format that emits only connections leaves every one of them empty
// and a pin-level rule silently finds nothing. That is why this toy format carries them.
//
// It takes an io.Reader and never opens a file itself, so the engine's Loader owns file I/O (C1).
func Read(r io.Reader, src string) (*ir.Design, error) {
	d := &ir.Design{IrVersion: "0", SourceFormat: "acme", Prov: &ir.Provenance{SourceFile: src}}
	lib := &ir.PartLibrary{Name: "acme", Prov: &ir.Provenance{SourceFile: src}}
	d.Libraries = append(d.Libraries, lib)
	nets := map[string]*ir.Net{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		switch f[0] {
		case "component":
			if len(f) < 3 {
				return nil, fmt.Errorf("acme %s: component needs <ref> <kind>: %q", src, line)
			}
			// One synthesized part type per component, named after the ref-des, so a later
			// `pin` line has something to attach to and the section resolves to it. Without a
			// PartRef the engine's part index never resolves the section, so the component has
			// no declared pins and every pin relation is empty for it.
			part := &ir.PartType{Name: f[1], Kind: f[2], Prov: &ir.Provenance{SourceFile: src}}
			lib.Parts = append(lib.Parts, part)
			sec := &ir.ComponentSection{PartRef: f[1], Attributes: map[string]string{"kind": f[2]}}
			if len(f) > 3 {
				sec.Attributes["value"] = f[3]
			}
			d.Components = append(d.Components, &ir.Component{
				RefDes:     f[1],
				Attributes: map[string]string{},
				Sections:   []*ir.ComponentSection{sec},
				Prov:       &ir.Provenance{SourceFile: src, NativeId: f[1]},
			})
		case "pin":
			if len(f) < 4 {
				return nil, fmt.Errorf("acme %s: pin needs <ref> <designator> <name> [type]: %q", src, line)
			}
			part := partByName(d, f[1])
			if part == nil {
				return nil, fmt.Errorf("acme %s: pin references unknown component %q (declare it first): %q", src, f[1], line)
			}
			p := &ir.Pin{Designator: f[2], Name: f[3], Prov: &ir.Provenance{SourceFile: src}}
			if len(f) > 4 {
				dir, ok := pinDirections[f[4]]
				if !ok {
					return nil, fmt.Errorf("acme %s: unknown pin type %q: %q", src, f[4], line)
				}
				p.Direction = dir
			}
			part.Pins = append(part.Pins, p)
		case "net":
			if len(f) < 2 {
				return nil, fmt.Errorf("acme %s: net needs a name: %q", src, line)
			}
			n := nets[f[1]]
			if n == nil {
				n = &ir.Net{Name: f[1], Prov: &ir.Provenance{SourceFile: src}}
				nets[f[1]] = n
				d.Nets = append(d.Nets, n)
			}
			for _, conn := range f[2:] {
				ref, pin, ok := strings.Cut(conn, ".")
				if !ok {
					return nil, fmt.Errorf("acme %s: net connection %q is not <ref>.<pin>", src, conn)
				}
				n.Connections = append(n.Connections, &ir.Connection{ComponentRef: ref, PinRef: pin})
			}
		default:
			return nil, fmt.Errorf("acme %s: unknown line %q (want 'component' or 'net')", src, line)
		}
	}
	return d, sc.Err()
}
