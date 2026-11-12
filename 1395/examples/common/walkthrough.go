package common

import (
	"github.com/panyam/demokit"
	"github.com/panyam/demokit/notebookbridge"
	"github.com/panyam/demokit/tui"
)

// SetupRenderer selects the demokit renderer from the --mode flag, for every example. --mode=tui
// gives styled boxes, --mode=notebook the Bubble Tea cells, and no flag gives plain text. Call it
// just before demo.Execute().
func SetupRenderer(d *demokit.Demo) {
	switch demokit.Mode() {
	case "tui":
		d.WithRenderer(tui.New())
	case "notebook":
		d.WithRenderer(notebookbridge.New())
	}
}
