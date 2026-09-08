package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// outFileFlag registers the -o/--out flag on a command that writes a rendered artifact.
//
// The spelling is `render`'s, verbatim, because a second convention for the same idea is worse than
// either convention: `-o -` and an omitted flag both mean stdout, so adding the flag changes no
// existing invocation and no committed capture.
func outFileFlag(cmd *cobra.Command, out *string) {
	cmd.Flags().StringVarP(out, "out", "o", "-", "write the --format output to this file (- for stdout). "+
		"Distinct from --results-out, which writes the self-contained check-result DOCUMENT that "+
		"`agni results` re-renders later; this writes what you would otherwise have redirected")
}

// redirectOut points the command's own output at a file for the rest of the run, and returns the
// closer. It is a no-op for stdout, so a caller may always defer the result.
//
// Redirecting the COMMAND rather than each write site is what keeps this small: every format already
// writes through cmd.OutOrStdout(), so one call covers text, csv, json, markdown, report and html
// without a per-format branch, and a format added later inherits it. That is only safe because the
// root command sets SilenceUsage and SilenceErrors, so cobra's own usage and error text cannot land
// in the middle of a report.
//
// The written-file note goes to STDERR, matching render, so `-o` composes with a pipe: the artifact
// is in the file and the human line is not in whatever reads stdout next.
func redirectOut(cmd *cobra.Command, out string) (func(), error) {
	if out == "" || out == "-" {
		return func() {}, nil
	}
	f, err := os.Create(out)
	if err != nil {
		return nil, err
	}
	cmd.SetOut(f)
	return func() {
		f.Close()
		fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s\n", out)
	}, nil
}
