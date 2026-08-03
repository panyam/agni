// Package myfmt is the format-reader slot of the overlay template. Copy this package, rename it,
// and replace the toy parser with your proprietary format's reader. It registers a ".myfmt"
// reader with the engine's public formats registry (formats.Register); blank-importing it
// (import _ ".../myfmt") makes the extension resolve through the engine's Loader and CLI.
//
// See docs/OVERLAY_AUTHORING.md for the full walkthrough.
package myfmt

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/panyam/agni/formats"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// init registers the reader by import side effect. To register explicitly from your binary's
// main instead (no blank import), delete this init and export a Register func the main calls.
func init() {
	formats.Register(&formats.Format{
		// TODO: your file extension (lowercase, with the dot) and a UI label.
		Ext:  ".myfmt",
		Name: "myfmt",
		// The registry entry owns the file open (C1: the Loader owns I/O); Read stays io.Reader-pure.
		// Set Geometry / Board too if your format carries a faithful schematic or a board layout.
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

// Read parses your format into an ir.Design. This toy version reads one component per
// non-blank line ("<refdes> <kind>"); replace the body with your real parser. It takes an
// io.Reader and never opens a file itself, so the engine's Loader owns file I/O (C1).
func Read(r io.Reader, src string) (*ir.Design, error) {
	d := &ir.Design{IrVersion: "0", SourceFormat: "myfmt", Prov: &ir.Provenance{SourceFile: src}}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// TODO: parse your format. This toy grammar is "<refdes> <kind>" per line.
		f := strings.Fields(line)
		d.Components = append(d.Components, &ir.Component{
			RefDes:     f[0],
			Attributes: map[string]string{},
			Prov:       &ir.Provenance{SourceFile: src, NativeId: f[0]},
		})
	}
	return d, sc.Err()
}
