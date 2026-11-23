package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/internal/flagfile"
	"github.com/panyam/agni/service"
)

// applyServeConfig sets serve's flags from the --config file at path (agni issue 1853), and returns
// the bootstrap plan when the file holds one inline. A flag typed on the command line wins. A
// `bootstrap:` that is a string names a file, as --bootstrap does. An unknown key, or a value its
// flag refuses, stops the server before it listens.
func applyServeConfig(cmd *cobra.Command, path string) (*service.BootstrapPlan, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path) // #nosec G304 -- the operator names the file in --config, as they name --tls-cert
	if err != nil {
		return nil, fmt.Errorf("--config: %w", err)
	}
	entries, err := flagfile.Parse(b, "bootstrap")
	if err != nil {
		return nil, fmt.Errorf("--config %s: %w", path, err)
	}
	local := cmd.LocalFlags()
	var plan *service.BootstrapPlan
	for _, e := range entries {
		switch {
		case e.Flag == "config" || e.Flag == "help":
			return nil, fmt.Errorf("--config %s: %s: there is no flag --%s", path, e.Key, e.Flag)
		case local.Lookup(e.Flag) == nil && cmd.Flags().Lookup(e.Flag) != nil:
			return nil, fmt.Errorf("--config %s: %s: --%s is read before the file, so give it as a flag or in agni.yaml", path, e.Key, e.Flag)
		case e.Raw == nil || cmd.Flags().Changed(e.Flag):
			continue
		case e.Raw.Kind == yaml.ScalarNode:
			if err := local.Set(e.Flag, e.Raw.Value); err != nil {
				return nil, fmt.Errorf("--config %s: %s: %w", path, e.Key, err)
			}
		default:
			inline, err := yaml.Marshal(e.Raw)
			if err != nil {
				return nil, fmt.Errorf("--config %s: %s: %w", path, e.Key, err)
			}
			if plan, err = service.ParseBootstrap(inline); err != nil {
				return nil, fmt.Errorf("--config %s: %s: %w", path, e.Key, err)
			}
		}
	}
	if err := flagfile.Apply(local, entries); err != nil {
		return nil, fmt.Errorf("--config %s: %w", path, err)
	}
	return plan, nil
}
