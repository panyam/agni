package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/service"
)

// connectionsCmd lists, adds and removes the connections an agni server reads private sources
// through (agni issue 1724). A connection names where its secret is, never the secret.
func connectionsCmd() *cobra.Command {
	var remoteURL string
	cmd := &cobra.Command{
		Use:   "connections",
		Short: "List, add and remove the sign-ins an agni server reads private repos through",
		Long: `connections manages the sign-ins an agni serve with --store keeps. A runtime mount names one with
agni mounts add --connection, and the server reads the mount's repo with its token. A connection
names where its secret is, as env:NAME or file:/abs/path, and the server reads the secret from there
when it needs a token, so no secret is stored or sent back.

  agni connections add acme-token --github-token env:GITHUB_TOKEN --remote http://localhost:8080
  agni connections add acme-app --github-app 12345 --installation 678 --key file:/run/secrets/app.pem --remote http://localhost:8080
  agni connections ls --remote http://localhost:8080
  agni connections rm acme-token --remote http://localhost:8080`,
	}
	cmd.PersistentFlags().StringVar(&remoteURL, "remote", "", "the agni serve whose connections these are, such as http://localhost:8080 (required)")
	client := func(cmd *cobra.Command) (webapiconnect.ConnectionServiceClient, error) {
		if remoteURL == "" {
			return nil, fmt.Errorf("agni connections needs --remote, the URL of an agni serve")
		}
		r, err := dialRemote(cmd.Context(), remoteURL)
		if err != nil {
			return nil, err
		}
		return webapiconnect.NewConnectionServiceClient(r.http, r.url), nil
	}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List the server's connections, each with its kind and where its secret is",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			resp, err := unary(cmd.Context(), c.ListConnections, &webapi.ListConnectionsRequest{})
			if err != nil {
				return err
			}
			for _, conn := range resp.GetConnections() {
				printConnection(cmd.OutOrStdout(), conn)
			}
			return nil
		},
	}

	var token, app, key, title, baseURL string
	var installation int64
	add := &cobra.Command{
		Use:   "add <id>",
		Short: "Keep a connection: a token with --github-token, or an App installation with --github-app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := &webapi.Connection{DisplayName: title, BaseUrl: baseURL}
			switch {
			case token != "" && app != "":
				return fmt.Errorf("--github-token and --github-app make different kinds of connection; pass one")
			case token != "":
				conn.Kind, conn.TokenSource = webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN, token
			case app != "":
				conn.Kind, conn.AppId, conn.InstallationId, conn.KeySource = webapi.ConnectionKind_CONNECTION_KIND_GITHUB_APP, app, installation, key
			default:
				return fmt.Errorf("name the sign-in: --github-token env:NAME, or --github-app <app-id> --installation <id> --key file:/path")
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			out, err := unary(cmd.Context(), c.CreateConnection, &webapi.CreateConnectionRequest{ConnectionId: args[0], Connection: conn})
			if err != nil {
				return err
			}
			printConnection(cmd.OutOrStdout(), out)
			return nil
		},
	}
	add.Flags().StringVar(&token, "github-token", "", "where a GitHub token is, env:NAME or file:/abs/path")
	add.Flags().StringVar(&app, "github-app", "", "a GitHub App's id, for an installation of it")
	add.Flags().Int64Var(&installation, "installation", 0, "the GitHub App installation's id, with --github-app")
	add.Flags().StringVar(&key, "key", "", "where the GitHub App's private key is, env:NAME or file:/abs/path, with --github-app")
	add.Flags().StringVar(&title, "title", "", "what a page shows for the connection; the id when empty")
	add.Flags().StringVar(&baseURL, "base-url", "", "the GitHub API root, for GitHub Enterprise; api.github.com when empty")

	rm := &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a connection no mount reads through",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			if _, err := unary(cmd.Context(), c.DeleteConnection, &webapi.DeleteConnectionRequest{Name: service.ConnectionName(service.ConnectionID(args[0]))}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", service.ConnectionID(args[0]))
			return nil
		},
	}

	cmd.AddCommand(ls, add, rm)
	return cmd
}

// printConnection writes one connection as a tab-separated line: its id, its kind, where its secret
// is, and its status.
func printConnection(w io.Writer, c *webapi.Connection) {
	kind := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(c.GetKind().String(), "CONNECTION_KIND_"), "_", "-"))
	secret := c.GetTokenSource()
	if c.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_GITHUB_APP {
		secret = fmt.Sprintf("app %s installation %d key %s", c.GetAppId(), c.GetInstallationId(), c.GetKeySource())
	}
	status := strings.ToLower(strings.TrimPrefix(c.GetStatus().String(), "CONNECTION_STATUS_"))
	fmt.Fprintf(w, "%s\t%s\t%s\t%s", service.ConnectionID(c.GetName()), kind, secret, status)
	if e := c.GetLastError(); e != "" {
		fmt.Fprintf(w, "\t%s", e)
	}
	fmt.Fprintln(w)
}
