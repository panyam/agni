package formats

import (
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/altium"
)

// registerAltium registers .PcbDoc, an Altium Designer board. Extension matching ignores case, so
// the one lowercase key covers Altium's .PcbDoc spelling.
func registerAltium() {
	Register(&Format{Ext: ".pcbdoc", Name: "altium", Design: readAltium, Board: readAltiumBoard})
}

func readAltium(l *Loader, p string) (*ir.Design, error) {
	f, err := l.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return altium.Read(f, p)
}

func readAltiumBoard(l *Loader, p string) (*geom.BoardGeometry, error) {
	f, err := l.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return altium.ReadBoardGeometry(f, p)
}
