package main

import (
	"fmt"
	"io"

	"github.com/panyam/agni/stdlib/profiles"
)

// warnOverBroadProfiles reports overlay profiles whose signal matchers claim an implausible share of
// this design's nets, or whose own roles collide on a net (WS3-101). It is the surface where a
// profile and a design meet: validateSignalMatcher rejects a universally-broad pattern at load time,
// but a merely LOOSE one can only be judged against a board.
//
// It writes to stderr and never fails the command. An over-broad matcher is a mistake in the profile
// the author wrote, not a defect in the board, so it must not become a Finding, change the exit code,
// or reach the findings stream that --format json and --results-out serialize.
//
// Best-effort by design: it reads the design a second time, which is why it runs only under
// --profile-path (the overlay-authoring path), and a read error is swallowed. The real read is the
// service's, and if the design cannot be read the command is about to say so properly.
func warnOverBroadProfiles(w io.Writer, path string, ps []profiles.Profile) {
	if len(ps) == 0 {
		return
	}
	d, err := readDesign(path)
	if err != nil {
		return
	}
	// Projected inline rather than through a helper taking *ir.Design: C19's ratchet targets exactly
	// that shape, and rightly — a helper handed the design to scan is the thing that should have read
	// an index instead. There is no net-name index to read here, and this function is already the
	// top-level consumer that did the read, so the projection stays where the read is.
	names := make([]string, 0, len(d.GetNets()))
	for _, n := range d.GetNets() {
		names = append(names, n.GetName())
	}
	for _, p := range ps {
		for _, msg := range profiles.Diagnose(names, p) {
			fmt.Fprintln(w, "warning: "+msg)
		}
	}
}
