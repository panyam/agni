package formats

import (
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/allegro"
)

// registerAllegro registers .brd, the Cadence Allegro board database. Eagle writes .brd too, as XML,
// and the reader names an Eagle board in its refusal rather than failing to parse it.
func registerAllegro() {
	Register(&Format{Ext: ".brd", Name: "allegro", Design: readAllegro, Board: readAllegroBoard})
}

func readAllegro(l *Loader, p string) (*ir.Design, error) {
	f, err := l.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return allegro.Read(f, p)
}

func readAllegroBoard(l *Loader, p string) (*geom.BoardGeometry, error) {
	f, err := l.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return allegro.ReadBoardGeometry(f, p)
}
