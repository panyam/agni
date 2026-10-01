package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/stdlib/profiles"
)

// noteSupersededRules reports which catalog rules an overlay source replaced (WS3-056). Supersession
// stops an overlay double-reporting an interface it re-binds, but it works by REMOVING rules, and a
// removed rule produces no output. Without this note, a report whose core profile rules were dropped
// reads the same as one where they ran and found nothing.
//
// It writes to stderr and never fails the command. This describes how the run was CONFIGURED, not
// something found on the board, so it stays out of the findings stream that --format json and
// --results-out serialize.
func noteSupersededRules(w io.Writer, c *check.Catalog) {
	for _, s := range c.Superseded() {
		fmt.Fprintf(w, "note: %s supersedes %d rule(s): %s\n", s.By, len(s.Rules), strings.Join(s.Rules, ", "))
	}
}

// warnOverBroadProfiles reports overlay profiles whose signal matchers claim an implausible share of
// this design's nets, or whose own roles collide on a net (WS3-101). validateSignalMatcher rejects a
// universally-broad pattern at load time, but a merely LOOSE one can only be judged against a board.
//
// It writes to stderr and never fails the command, for the reason noteSupersededRules gives. An
// over-broad matcher is a mistake in the profile, not a defect in the board, so it must not become a
// Finding or change the exit code.
//
// It reads the design a second time, so it runs only under --profile-path, and a read error is
// swallowed because the service's own read will report it.
func warnOverBroadProfiles(w io.Writer, path string, ps []profiles.Profile) {
	if len(ps) == 0 {
		return
	}
	d, err := readDesign(path)
	if err != nil {
		return
	}
	// Projected inline because C19's ratchet flags a helper taking *ir.Design. There is no net-name
	// index to read, and this function is the consumer that did the read.
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
