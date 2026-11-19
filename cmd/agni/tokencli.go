package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/panyam/oneauth/core"
	oafs "github.com/panyam/oneauth/stores/fs"

	"github.com/panyam/agni/internal/storeurl"
)

// tokenCmd makes, lists and revokes the tokens an `agni serve --auth tokens` checks (agni issue
// 1627). It works on the --store folder directly rather than through the server, so making a token
// takes access to the server's machine, and no API call can make one.
func tokenCmd() *cobra.Command {
	var storeURL string
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Make, list and revoke the tokens an agni serve --auth tokens accepts",
		Long: `token manages the tokens a shared agni serve checks, kept in its --store folder. Run it on the
server's machine, with the same --store the server was started with. A token's name is who it is
for, a person or a tool, and the server records it as the caller on what it stores, such as a
review run.

  agni token create alice --store file:///srv/agni/meta
  agni token ls --store file:///srv/agni/meta
  agni token revoke oa_3f2c... --store file:///srv/agni/meta

create prints the token once. The store keeps only its hash. A revoked token stops working within
a minute, since the server trusts a token it checked for that long.`,
	}
	cmd.PersistentFlags().StringVar(&storeURL, "store", "", "the --store of the agni serve these tokens are for, such as file:///srv/agni/meta (required)")
	store := func() (*oafs.FSAPIKeyStore, error) {
		if storeURL == "" {
			return nil, fmt.Errorf("agni token needs --store, the same one the server was started with")
		}
		dir, _, err := storeurl.Folders(storeURL, "")
		if err != nil {
			return nil, err
		}
		return tokenStore(dir), nil
	}

	create := &cobra.Command{
		Use:   "create <name>",
		Short: "Make a token for a person or a tool, and print it once",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store()
			if err != nil {
				return err
			}
			resp, err := s.CreateAPIKey(cmd.Context(), &core.CreateAPIKeyRequest{Subject: args[0], Name: args[0]})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), resp.FullKey)
			fmt.Fprintf(cmd.ErrOrStderr(), "made token %s for %s; it is shown only this once\n", resp.APIKey.KeyID, args[0])
			return nil
		},
	}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List the tokens, newest first, each with its id, who it is for, and when it was made",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := store()
			if err != nil {
				return err
			}
			keys, err := listTokens(cmd.Context(), s)
			if err != nil {
				return err
			}
			for _, k := range keys {
				state := "active"
				if k.Revoked {
					state = "revoked"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", k.KeyID, k.Subject, k.CreatedAt.UTC().Format(time.RFC3339), state)
			}
			return nil
		},
	}

	revoke := &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke a token by the id agni token ls shows",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store()
			if err != nil {
				return err
			}
			if _, err := s.RevokeAPIKey(cmd.Context(), &core.RevokeAPIKeyRequest{KeyID: args[0]}); err != nil {
				return fmt.Errorf("revoke %s: %w", args[0], err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "revoked %s\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(create, ls, revoke)
	return cmd
}
