package main

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/core/timing"
	"github.com/spf13/cobra"
)

// withTiming gives a command --timing, and --explain when it evaluates queries (agni issue 914). The
// command's service calls run in-process under its context, so a recorder put on that context before
// the command runs collects what a served request would report in its Agni-Timing header. The
// breakdown goes to stderr, so a timed run still pipes its answer.
func withTiming(c *cobra.Command, explain bool) *cobra.Command {
	var timed, explained bool
	c.Flags().BoolVar(&timed, "timing", false, "print where the run's time went to stderr: each stage, which cache tier answered each read, and the slowest rules")
	if explain {
		c.Flags().BoolVar(&explained, "explain", false, "print each query's evaluation plan to stderr: the order each rule body ran in, where its work went, and how each relation was read")
	}
	inner := c.RunE
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if !timed && !explained {
			return inner(cmd, args)
		}
		rec := timing.New()
		rec.Explain = explained
		cmd.SetContext(timing.With(cmd.Context(), rec))
		err := inner(cmd, args)
		s := rec.Snapshot()
		if !timed {
			// --explain alone prints the plans and none of the timings.
			s = timing.Snapshot{Plans: s.Plans}
			for _, p := range s.Plans {
				fmt.Fprintf(cmd.ErrOrStderr(), "plan for %s\n%s\n", p.Query, strings.TrimRight(p.Text, "\n"))
			}
			return err
		}
		fmt.Fprint(cmd.ErrOrStderr(), "timing: "+s.Text(20))
		return err
	}
	return c
}
