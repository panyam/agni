package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/internal/native"
)

// nativeCmd groups commands that hand a design file to its own EDA tool: `render` produces the
// tool's reference SVG (an independent oracle for agni's render), and `open` launches the
// tool's GUI. Both dispatch by file extension via internal/native; formats with no native tool
// (EDIF, IPC-2581, ODB++) report that plainly. See docs/NATIVE_VERIFICATION.md.
func nativeCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "native",
		Short: "Render or open a design with its own EDA tool (KiCad, xschem, Lepton)",
	}
	c.AddCommand(nativeRenderCmd(), nativeOpenCmd())
	return c
}

func nativeRenderCmd() *cobra.Command {
	var out string
	var page int
	cmd := &cobra.Command{
		Use:   "render <file>",
		Short: "Render a design to SVG using its native tool (kicad-cli, xschem, lepton-cli)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			abs, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			svg, err := native.RenderFile(cmd.Context(), abs, page)
			if err != nil {
				return nativeHint(err, abs)
			}
			if out == "" {
				_, err := cmd.OutOrStdout().Write([]byte(svg))
				return err
			}
			if err := os.WriteFile(out, []byte(svg), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s (native render of %s)\n", out, filepath.Base(abs))
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "", "write SVG here (default stdout)")
	cmd.Flags().IntVar(&page, "page", 1, "1-based page/sheet to render (multi-sheet schematics)")
	return cmd
}

func nativeOpenCmd() *cobra.Command {
	var printOnly bool
	cmd := &cobra.Command{
		Use:   "open <file>",
		Short: "Open a design in its native GUI tool (KiCad, xschem, lepton-schematic)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			abs, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			bin, cmdArgs, err := native.OpenArgs(abs)
			if err != nil {
				return nativeHint(err, abs)
			}
			if printOnly {
				fmt.Fprintln(cmd.OutOrStdout(), strings.Join(append([]string{bin}, cmdArgs...), " "))
				return nil
			}
			// Check the launcher is installed before exec, so a missing tool gives the same
			// actionable hint the render path does rather than a raw "file not found".
			if _, err := exec.LookPath(bin); err != nil {
				return nativeHint(native.ErrNotFound, abs)
			}
			run := exec.CommandContext(cmd.Context(), bin, cmdArgs...)
			run.Stdout, run.Stderr = cmd.OutOrStdout(), cmd.ErrOrStderr()
			// Block until the GUI exits. A macOS `open -a` launcher returns immediately (the app
			// outlives it), but a direct X11 binary (xschem, lepton-schematic) must stay attached
			// so that when this runs over `ssh -X` the forwarded display survives until the window
			// is closed; detaching would tear the tunnel down and kill the GUI.
			return run.Run()
		},
	}
	cmd.Flags().BoolVar(&printOnly, "print", false, "print the launch command instead of running it")
	return cmd
}

// nativeHint turns the native package's gate sentinels into an actionable CLI message naming
// the file's extension, so a user knows whether to install a tool or that the format has none.
func nativeHint(err error, abs string) error {
	switch {
	case errors.Is(err, native.ErrNoTool):
		return fmt.Errorf("no native tool for %s files (EDIF/IPC-2581/ODB++ have none; see docs/NATIVE_VERIFICATION.md)", filepath.Ext(abs))
	case errors.Is(err, native.ErrNotFound):
		return fmt.Errorf("the native tool for %s is not installed / not on PATH; xschem and Lepton are Linux/X11 tools with no macOS build, so use Dockerfile.nattools (see docs/NATIVE_VERIFICATION.md)", filepath.Ext(abs))
	default:
		return err
	}
}
