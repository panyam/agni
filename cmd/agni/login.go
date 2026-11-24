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

// loginCmd signs in to a server with GitHub's device flow, through the server (agni issue 1772),
// or with a local account's username and password (agni issue 1879).
func loginCmd() *cobra.Command {
	var remote, with, user string
	var passwordStdin bool
	c := &cobra.Command{
		Use:   "login --remote <url> [--with github|oidc|local] [--user <name> --password-stdin]",
		Short: "Sign in to an agni server from a terminal, with GitHub, its OpenID Connect provider or a local account, and keep the token it mints",
		Long: "Sign in to an agni server with GitHub (agni issue 1772). The server asks GitHub for a code, " +
			"you open the address it prints and type the code, and the server mints you a token once GitHub " +
			"says you approved. The token is kept in your config folder, and every --remote call to that " +
			"server sends it when AGNI_TOKEN isn't set. The server also keeps your GitHub token as your own " +
			"connection, so a design linked from a private repo you can read opens for you. The server needs " +
			"--github-client-id. With --with oidc it signs in through the server's OpenID Connect provider " +
			"instead (agni issue 1811), and keeps no connection. Without --with it uses the server's only " +
			"provider, and a server offering both asks you to choose.\n\n" +
			"With --with local it signs in as a local account that the operator made with agni users add " +
			"(agni issue 1879). Name the account with --user and give its password on stdin with " +
			"--password-stdin, such as printf '%s\\n' \"$PASSWORD\" | agni login --remote <url> --with local " +
			"--user alice --password-stdin. The server needs --local-accounts. It keeps the token the same way.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u, err := url.Parse(remote)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return fmt.Errorf("--remote %q: want a URL like https://agni.example.com", remote)
			}
			server := strings.TrimRight(remote, "/")
			var token, subject string
			if with == "local" {
				if user == "" {
					return fmt.Errorf("--with local needs --user, the account's username")
				}
				password, err := readPassword(cmd, passwordStdin)
				if err != nil {
					return err
				}
				got, err := authgate.PasswordLogin(cmd.Context(), server, user, password)
				if err != nil {
					return err
				}
				token, subject = got.Token, got.Subject
			} else {
				if user != "" || passwordStdin {
					return fmt.Errorf("--user and --password-stdin go with --with local")
				}
				got, err := authgate.DeviceLogin(cmd.Context(), server, with, func(s authgate.DeviceStart) {
					fmt.Fprintf(cmd.ErrOrStderr(), "Open %s and enter the code %s\n", s.VerificationURI, s.UserCode)
				})
				if err != nil {
					return err
				}
				token, subject = got.Token, got.Subject
			}
			p, err := credentials.Save(server, token)
			if err != nil {
				return fmt.Errorf("signed in as %s, but keeping the token failed: %w", subject, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "signed in to %s as %s; the token is in %s\n", server, subject, p)
			return nil
		},
	}
	c.Flags().StringVar(&remote, "remote", "", "the agni serve to sign in to, such as https://agni.example.com (required)")
	_ = c.MarkFlagRequired("remote")
	c.Flags().StringVar(&with, "with", "", "the provider to sign in with: github, oidc for the server's OpenID Connect provider, or local for an account made with agni users add. Empty uses the server's only device-flow provider")
	c.Flags().StringVar(&user, "user", "", "with --with local, the account's username")
	c.Flags().BoolVar(&passwordStdin, "password-stdin", false, "with --with local, read the password from the first line of stdin")
	return c
}
