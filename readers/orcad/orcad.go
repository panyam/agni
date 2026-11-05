// Package orcad reads an OrCAD Capture design (.DSN) into the neutral IR.
//
// A .DSN is a Microsoft compound file. Its Library stream holds the format version and the string
// table that every other stream indexes. Its Cache holds the symbols and packages the design
// places. Each schematic folder under Views holds its pages and, for the root, the Hierarchy
// stream, the occurrence tree that names every part along every instantiation path. Cadence
// publishes no specification. The layouts read here are from OpenOrCadParser's documentation (MIT)
// and KiCad's format notes, read for format facts only, and they are checked against Clay R6, whose
// netlist Capture wrote beside it (agni issue 1399).
//
// A hierarchical block may keep its pages in another .DSN, which it names by a path relative to
// its own design. Read takes an Opener for those, so the reader opens no file itself.
//
// Fidelity: lossy-bounded. The netlist is every placed part, named by its occurrence designator
// where the design has one, flattened through every block the design places. A pin joins a net
// through the wire its stored position sits on, another pin at the same point, the net id it
// stores, or a PACK_SHORT group of its part. A port, power symbol or off-page connector joins the
// wire ends inside its stored box: a power name joins across the design, a port joins its block's
// pin, and an off-page name joins within its schematic folder. A net takes the shallowest name it
// carries, qualified by its block path when another net shares the name, and a net with no name
// takes an N-number from its wire's id. The geometry is one sheet per page, each block's page
// under the page that places it, with every part's symbol, its displayed fields and its pins'
// names and numbers as the part shows them, every wire and net alias, power symbols, ports and
// off-page connectors with their names, block rectangles with their pins, computed junction dots,
// and the title block. Ellipses that are not circles and beziers are drawn as chords. A mirrored
// placement is read as the format notes describe it, and no public design checks it. Buses join no
// net, and bus entries, no-connect markers (an explicitly unconnected pin reads as unconnected),
// bitmaps, OLE objects, CIS variants and net names that match Capture's own (agni issue 1406) are
// not read. Only the modern format family is read (major version 3 and up; Clay R6's Capture 16.6
// wrote 3.2), and a legacy design is refused with its version named.
package orcad

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/cfb"
)

// Opener opens a block's design. from is the name Read was given for the design that places the
// block, or a name an earlier call returned. ref is the block's path as the design records it,
// relative to from's folder and possibly in Windows form (".\MPU9250\MPU9250.DSN"). The returned
// name is the opened file's, used for provenance and to resolve the blocks it places in turn. A
// nil Opener, or an error, leaves the block out and lists it in the design's diagnostics.
type Opener func(from, ref string) (name string, data []byte, err error)

// Read parses an OrCAD Capture design into an ir.Design, flattening every block it places.
// sourceFile is the design's name, for provenance and for open.
func Read(r io.Reader, sourceFile string, open Opener) (*ir.Design, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	top, err := load(b, sourceFile)
	if err != nil {
		return nil, err
	}
	return flatten(top, open)
}

// IsDesign reports whether b is a compound file, which a Capture design is and a SPECCTRA
// design (.dsn, plain text) is not.
func IsDesign(b []byte) bool { return cfb.IsCompoundFile(b) }

// dsn is one loaded .DSN.
type dsn struct {
	name    string
	lib     *library
	cache   *cache
	folders map[string][]*page // schematic folder (lower case) -> its pages in name order
	root    string             // the root folder's key
	occ     *scope             // the root's occurrence tree
}

func load(b []byte, name string) (*dsn, error) {
	if !cfb.IsCompoundFile(b) {
		return nil, fmt.Errorf("%s: not an OrCAD Capture design (no compound-file signature; a SPECCTRA .dsn is plain text)", path.Base(name))
	}
	f, err := cfb.Open(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path.Base(name), err)
	}
	stream := func(p string) ([]byte, error) {
		data, ok, err := f.Stream(p)
		if err == nil && !ok {
			err = fmt.Errorf("no %s stream", p)
		}
		return data, err
	}
	lb, err := stream("Library")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path.Base(name), err)
	}
	d := &dsn{name: name, folders: map[string][]*page{}}
	if d.lib, err = parseLibrary(lb); err != nil {
		return nil, fmt.Errorf("%s: %w", path.Base(name), err)
	}
	cb, err := stream("Cache")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path.Base(name), err)
	}
	if d.cache, err = parseCache(cb, d.lib); err != nil {
		return nil, fmt.Errorf("%s: %w", path.Base(name), err)
	}
	views, _ := f.Children("Views")
	sort.Strings(views)
	for _, folder := range views {
		pages, ok := f.Children("Views/" + folder + "/Pages")
		if !ok {
			continue
		}
		sort.Strings(pages)
		key := strings.ToLower(folder)
		for _, pn := range pages {
			pb, err := stream("Views/" + folder + "/Pages/" + pn)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path.Base(name), err)
			}
			pg, err := parsePage(pb, d.lib)
			if err != nil {
				return nil, fmt.Errorf("%s: page %s/%s: %w", path.Base(name), folder, pn, err)
			}
			if pg.name == "" {
				pg.name = pn
			}
			d.folders[key] = append(d.folders[key], pg)
		}
	}
	if len(d.folders) == 0 {
		return nil, fmt.Errorf("%s: the design holds no schematic pages", path.Base(name))
	}
	// The Library names the root folder. A design that names none, or names a folder it lacks,
	// is rooted at its first folder by name, as Capture's own fallback is.
	rootFolder := ""
	for _, folder := range views {
		if _, ok := d.folders[strings.ToLower(folder)]; !ok {
			continue
		}
		if rootFolder == "" || strings.EqualFold(folder, d.lib.root) {
			rootFolder = folder
		}
	}
	d.root = strings.ToLower(rootFolder)
	if hb, ok, err := f.Stream("Views/" + rootFolder + "/Hierarchy/Hierarchy"); err == nil && ok {
		if d.occ, err = parseHierarchy(hb); err != nil {
			return nil, fmt.Errorf("%s: %w", path.Base(name), err)
		}
	}
	return d, nil
}
