package main

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/internal/version"
)

// versionCmd prints what this build is.
//
// The identity itself comes from internal/version, which already resolves it for the provenance
// stamp a results document carries (WS3-103). This command deliberately adds no second opinion:
// a build that reported one version to a human and a different one into an archived report would
// be worse than having no command at all, so the human-facing surface reads the same function the
// document does.
//
// What it adds is the surrounding detail worth pasting into a bug report, which provenance has no
// field for: the toolchain and the platform.
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version, revision, and toolchain of this build",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "agni %s\n", version.Version())
			if t := commitTime(); t != "" {
				fmt.Fprintf(out, "  built:    %s\n", t)
			}
			fmt.Fprintf(out, "  go:       %s\n", runtime.Version())
			fmt.Fprintf(out, "  platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}

// commitTime reads the commit timestamp the toolchain embeds when it can see a repository.
// Absent for a `go install module@version` build and for the container image (whose build
// context excludes .git), where the version string is the identity and a timestamp adds nothing.
func commitTime() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range bi.Settings {
		if s.Key == "vcs.time" {
			return s.Value
		}
	}
	return ""
}
