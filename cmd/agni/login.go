package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/internal/authgate"
	"github.com/panyam/agni/internal/credentials"
)

// remoteToken is the token a --remote call sends to server: AGNI_TOKEN when it is set, and
// otherwise the one `agni login` kept for server (agni issue 1772).
func remoteToken(server string) string {
	if token := os.Getenv(envToken); token != "" {
		return token
	}
	return credentials.Token(server)
}

// loginCmd signs in to a server with GitHub's device flow, through the server (agni issue 1772).
func loginCmd() *cobra.Command {
	var remote, with string
	c := &cobra.Command{
		Use:   "login --remote <url> [--with github|oidc]",
		Short: "Sign in to an agni server from a terminal, with GitHub or its OpenID Connect provider, and keep the token it mints",
		Long: "Sign in to an agni server with GitHub (agni issue 1772). The server asks GitHub for a code, " +
			"you open the address it prints and type the code, and the server mints you a token once GitHub " +
			"says you approved. The token is kept in your config folder, and every --remote call to that " +
			"server sends it when AGNI_TOKEN isn't set. The server also keeps your GitHub token as your own " +
			"connection, so a design linked from a private repo you can read opens for you. The server needs " +
			"--github-client-id. With --with oidc it signs in through the server's OpenID Connect provider " +
			"instead (agni issue 1811), and keeps no connection. Without --with it uses the server's only " +
			"provider, and a server offering both asks you to choose.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u, err := url.Parse(remote)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return fmt.Errorf("--remote %q: want a URL like https://agni.example.com", remote)
			}
			server := strings.TrimRight(remote, "/")
			got, err := authgate.DeviceLogin(cmd.Context(), server, with, func(s authgate.DeviceStart) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Open %s and enter the code %s\n", s.VerificationURI, s.UserCode)
			})
			if err != nil {
				return err
			}
			p, err := credentials.Save(server, got.Token)
			if err != nil {
				return fmt.Errorf("signed in as %s, but keeping the token failed: %w", got.Subject, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "signed in to %s as %s; the token is in %s\n", server, got.Subject, p)
			return nil
		},
	}
	c.Flags().StringVar(&remote, "remote", "", "the agni serve to sign in to, such as https://agni.example.com (required)")
	_ = c.MarkFlagRequired("remote")
	c.Flags().StringVar(&with, "with", "", "the provider to sign in with: github, or oidc for the server's OpenID Connect provider. Empty uses the server's only one")
	return c
}
