package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

// healthcheckCmd probes a running server's /healthz and exits non-zero unless it answers 200. The
// container image's HEALTHCHECK runs it because the debian-slim runtime ships neither curl nor wget,
// and installing one would add a package to every deployment for a single request. It reads only
// the status code and checks no other route.
func healthcheckCmd() *cobra.Command {
	var addr string
	var timeout time.Duration
	c := &cobra.Command{
		Use:   "healthcheck",
		Short: "Probe a running server's /healthz and exit non-zero if it is unhealthy",
		Long: "healthcheck GETs /healthz on a running agni server and exits 0 only on a 200.\n" +
			"It is what the container image's HEALTHCHECK runs, so the image needs no curl or wget.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := &http.Client{Timeout: timeout}
			url := "http://" + addr + "/healthz"
			resp, err := client.Get(url)
			if err != nil {
				return fmt.Errorf("%s: %w", url, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("%s: got HTTP %d, want 200", url, resp.StatusCode)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
	// Defaults to the loopback form of serve's own default --addr. serve accepts ":8080" (all
	// interfaces), which is not a dialable host, so the probe names localhost explicitly.
	c.Flags().StringVar(&addr, "addr", "localhost:8080", "address of the server to probe")
	c.Flags().DurationVar(&timeout, "timeout", 3*time.Second, "how long to wait for a response")
	return c
}
