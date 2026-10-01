// Package myfmt is the format-reader slot of the extension template. Copy this package, rename it,
// and replace the toy parser with your own format's reader. It registers a ".myfmt" reader with
// formats.Register, so blank-importing it makes the engine's Loader and CLI resolve the format.
//
// See docsite/content/build/extending.md for the full walkthrough.
package myfmt

import (
	"bufio"
	"io"
	"os"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/formats"
)

// init registers the reader by import side effect. To register explicitly from your binary's
// main instead (no blank import), delete this init and export a Register func the main calls.
func init() {
	formats.Register(&formats.Format{
		// TODO: your file extension (lowercase, with the dot) and a UI label.
		Ext:  ".myfmt",
		Name: "myfmt",
		// The registry entry opens the file so Read stays io.Reader-pure (C1). Set Geometry or
		// Board too if your format carries a faithful schematic or a board layout.
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
// non-blank line ("<refdes> <kind>"); replace the body with your real parser. It never opens a
// file itself.
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
