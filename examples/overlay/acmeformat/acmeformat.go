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

// Read parses the toy .acme netlist into an ir.Design. The grammar is two line kinds, with
// blank lines and '#' comments ignored:
//
//	component <ref> <kind> [value]
//	net <name> <ref>.<pin> [<ref>.<pin> ...]
//
// It takes an io.Reader and never opens a file itself, so the engine's Loader owns file I/O (C1).
func Read(r io.Reader, src string) (*ir.Design, error) {
	d := &ir.Design{IrVersion: "0", SourceFormat: "acme", Prov: &ir.Provenance{SourceFile: src}}
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
			sec := &ir.ComponentSection{Attributes: map[string]string{"kind": f[2]}}
			if len(f) > 3 {
				sec.Attributes["value"] = f[3]
			}
			d.Components = append(d.Components, &ir.Component{
				RefDes:     f[1],
				Attributes: map[string]string{},
				Sections:   []*ir.ComponentSection{sec},
				Prov:       &ir.Provenance{SourceFile: src, NativeId: f[1]},
			})
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
