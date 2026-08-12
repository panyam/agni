package main

import (
	"strings"
	"testing"
)

// TestCheckBoardPath: `check` can attach a board that is not a declared companion, which `review` and
// `query` have been able to do since WS3-089.
//
// The asymmetry mattered because of which command it hit. "Does this layout pass the fab's rules" is
// the question `check` exists to answer, and it was the one command that could not be pointed at a
// layout: a netlist read on its own carries no copper, so every board-tier rule found nothing and the
// run reported clean.
func TestCheckBoardPath(t *testing.T) {
	const (
		netlist = "../../examples/tutorial-project/designs/gateway/gateway.edn"
		board   = "../../examples/tutorial-project/designs/gateway/gateway.kicad_pcb"
	)
	// as-named reads exactly the netlist, so the design's own declared board companion does not supply
	// the copper and --board-path is the only thing that can. It is set directly because it is a ROOT
	// persistent flag, and a bare checkCmd() does not carry it.
	saved := readAsNamed
	readAsNamed = true
	defer func() { readAsNamed = saved }()

	withBoard := runCLI(t, checkCmd(), "--board-path", board, netlist)
	if !strings.Contains(withBoard, "track-width") {
		t.Errorf("an attached board should let board-tier rules run, got:\n%s", withBoard)
	}
	without := runCLI(t, checkCmd(), netlist)
	if strings.Contains(without, "track-width") {
		t.Errorf("a netlist alone carries no copper, so the board rule must not fire, got:\n%s", without)
	}
}
