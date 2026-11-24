package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/internal/flagfile"
	"github.com/panyam/agni/service"
)

// applyServeConfig sets serve's flags from the --config file at path (agni issue 1853), and returns
// the bootstrap plan when the file holds one inline. A flag typed on the command line wins. A
// `bootstrap:` that is a string names a file, as --bootstrap does. An unknown key, or a value its
// flag refuses, stops the server before it listens. An `access:` list sets --grant, one value per
// entry, unless --grant was typed (agni issue 1889).
func applyServeConfig(cmd *cobra.Command, path string) (*service.BootstrapPlan, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path) // #nosec G304 -- the operator names the file in --config, as they name --tls-cert
	if err != nil {
		return nil, fmt.Errorf("--config: %w", err)
	}
	entries, err := flagfile.Parse(b, "bootstrap", "access")
	if err != nil {
		return nil, fmt.Errorf("--config %s: %w", path, err)
	}
	local := cmd.LocalFlags()
	var plan *service.BootstrapPlan
	var access *flagfile.Entry
	for i, e := range entries {
		switch {
		case e.Flag == "access":
			access = &entries[i]
			continue
		case e.Flag == "grant" && slices.ContainsFunc(entries, func(o flagfile.Entry) bool { return o.Flag == "access" }):
			return nil, fmt.Errorf("--config %s: %s and access both set --grant; keep one", path, e.Key)
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
	if access != nil && !cmd.Flags().Changed("grant") {
		grants, err := accessGrants(access.Raw)
		if err != nil {
			return nil, fmt.Errorf("--config %s: %w", path, err)
		}
		for _, g := range grants {
			if err := local.Set("grant", g); err != nil {
				return nil, fmt.Errorf("--config %s: %s: %w", path, access.Key, err)
			}
		}
	}
	entries = slices.DeleteFunc(entries, func(e flagfile.Entry) bool { return e.Flag == "access" })
	if err := flagfile.Apply(local, entries); err != nil {
		return nil, fmt.Errorf("--config %s: %w", path, err)
	}
	return plan, nil
}
