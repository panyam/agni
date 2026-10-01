// agnids is the datasheet service: the extraction workbench's page and API, and the derive step. It
// is a separate binary from agni because it is a separate service, deployed with its own mounts, its
// own writable store and the docling environment, and it is built from the datasheet module, which
// the engine never imports (C34, agni issue 744).
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "agnids",
		Short:         "The agni datasheet service: the extraction workbench and the derive step",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(serveCmd(), deriveCmd())
	return root
}
