package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/panyam/oneauth/outbound"
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
  agni connections rm acme-token --remote http://localhost:8080

With --mine the connection is the caller's own. The CLI reads the token from the reference on this
machine and sends it once, and the server seals it under --connections-key. A design linked from a
private repo opens only for a caller whose own connection can read the repo (agni issue 1728).

  AGNI_TOKEN=... agni connections add my-github --mine --github-token env:GITHUB_TOKEN --remote https://agni.example

With --seal a server connection keeps its secret sealed too, for a server whose environment and
files this machine can't reach, such as one in a container (agni issue 1802). The CLI reads the token
or the App's key on this machine and sends it once. agni connections rotate replaces it, while mounts
keep reading through the connection.

  agni connections add hw --seal --github-token env:HW_TOKEN --remote https://agni.example
  agni connections rotate hw --github-token env:HW_TOKEN_NEXT --remote https://agni.example`,
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
	var mine, seal bool
	add := &cobra.Command{
		Use:   "add <id>",
		Short: "Keep a connection: a token with --github-token, or an App installation with --github-app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := &webapi.Connection{DisplayName: title, BaseUrl: baseURL}
			req := &webapi.CreateConnectionRequest{ConnectionId: args[0], Connection: conn}
			switch {
			case mine && token == "":
				return fmt.Errorf("--mine keeps your own GitHub token; name where it is with --github-token env:NAME or file:/abs/path")
			case mine && app != "":
				return fmt.Errorf("a GitHub App installation signs in as the server, so it can't be --mine")
			case mine && seal:
				return fmt.Errorf("--mine already seals your token; --seal is for a server connection")
			case mine:
				ref, err := outbound.ParseSecretRef(token)
				if err != nil {
					return fmt.Errorf("--github-token: %w", err)
				}
				secret, err := ref.Resolve()
				if err != nil {
					return fmt.Errorf("--github-token: %w", err)
				}
				conn.Kind, conn.Scope, req.UserToken = webapi.ConnectionKind_CONNECTION_KIND_GITHUB_USER, webapi.ConnectionScope_CONNECTION_SCOPE_PERSON, strings.TrimSpace(secret)
			case token != "" && app != "":
				return fmt.Errorf("--github-token and --github-app make different kinds of connection; pass one")
			case token != "" && seal:
				secret, err := readSecret("--github-token", token)
				if err != nil {
					return err
				}
				conn.Kind, req.ServerSecret = webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN, strings.TrimSpace(secret)
			case token != "":
				conn.Kind, conn.TokenSource = webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN, token
			case app != "" && seal:
				secret, err := readSecret("--key", key)
				if err != nil {
					return err
				}
				conn.Kind, conn.AppId, conn.InstallationId, req.ServerSecret = webapi.ConnectionKind_CONNECTION_KIND_GITHUB_APP, app, installation, secret
			case app != "":
				conn.Kind, conn.AppId, conn.InstallationId, conn.KeySource = webapi.ConnectionKind_CONNECTION_KIND_GITHUB_APP, app, installation, key
			default:
				return fmt.Errorf("name the sign-in: --github-token env:NAME, or --github-app <app-id> --installation <id> --key file:/path")
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			out, err := unary(cmd.Context(), c.CreateConnection, req)
			if err != nil {
				return err
			}
			printConnection(cmd.OutOrStdout(), out)
			return nil
		},
	}
	add.Flags().StringVar(&token, "github-token", "", "where a GitHub token is, env:NAME or file:/abs/path; the server reads it, or with --mine this command does")
	add.Flags().BoolVar(&mine, "mine", false, "keep the connection as your own: the server seals your token and checks it before it shows you a design linked from a private repo")
	add.Flags().BoolVar(&seal, "seal", false, "keep a server connection's secret sealed: this command reads the token or the App's --key, and the server seals it under --connections-key rather than reading it from its own environment or files")
	add.Flags().StringVar(&app, "github-app", "", "a GitHub App's id, for an installation of it")
	add.Flags().Int64Var(&installation, "installation", 0, "the GitHub App installation's id, with --github-app")
	add.Flags().StringVar(&key, "key", "", "where the GitHub App's private key is, env:NAME or file:/abs/path, with --github-app")
	add.Flags().StringVar(&title, "title", "", "what a page shows for the connection; the id when empty")
	add.Flags().StringVar(&baseURL, "base-url", "", "the GitHub API root, for GitHub Enterprise; the server's --github-api, or api.github.com, when empty")

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

	var nextToken, nextKey string
	rotate := &cobra.Command{
		Use:   "rotate <id>",
		Short: "Replace a sealed server connection's token, or its App's key, while mounts read through it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var secret string
			var err error
			switch {
			case (nextToken == "") == (nextKey == ""):
				return fmt.Errorf("name the new secret: --github-token env:NAME for a token, or --key file:/path for an App's key")
			case nextToken != "":
				if secret, err = readSecret("--github-token", nextToken); err != nil {
					return err
				}
				secret = strings.TrimSpace(secret)
			default:
				if secret, err = readSecret("--key", nextKey); err != nil {
					return err
				}
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			out, err := unary(cmd.Context(), c.RotateConnection, &webapi.RotateConnectionRequest{
				Name: service.ConnectionName(service.ConnectionID(args[0])), ServerSecret: secret,
			})
			if err != nil {
				return err
			}
			printConnection(cmd.OutOrStdout(), out)
			return nil
		},
	}
	rotate.Flags().StringVar(&nextToken, "github-token", "", "where the new GitHub token is on this machine, env:NAME or file:/abs/path")
	rotate.Flags().StringVar(&nextKey, "key", "", "where the GitHub App's new private key is on this machine, env:NAME or file:/abs/path")

	cmd.AddCommand(ls, add, rm, rotate)
	return cmd
}

// readSecret reads the secret ref names on this machine, for a connection the server seals.
func readSecret(flag, ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("%s: name where the secret is, env:NAME or file:/abs/path", flag)
	}
	r, err := outbound.ParseSecretRef(ref)
	if err != nil {
		return "", fmt.Errorf("%s: %w", flag, err)
	}
	secret, err := r.Resolve()
	if err != nil {
		return "", fmt.Errorf("%s: %w", flag, err)
	}
	if strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("%s %s is empty", flag, ref)
	}
	return secret, nil
}

// printConnection writes one connection as a tab-separated line: its id, its kind, where its secret
// is, and its status. A person's connection says whose it is in place of where its secret is.
func printConnection(w io.Writer, c *webapi.Connection) {
	kind := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(c.GetKind().String(), "CONNECTION_KIND_"), "_", "-"))
	secret, keySource := c.GetTokenSource(), c.GetKeySource()
	if c.GetSealed() {
		secret, keySource = "sealed", "sealed"
	}
	switch {
	case c.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_GITHUB_APP:
		secret = fmt.Sprintf("app %s installation %d key %s", c.GetAppId(), c.GetInstallationId(), keySource)
	case c.GetScope() == webapi.ConnectionScope_CONNECTION_SCOPE_PERSON:
		secret = "sealed, for " + c.GetOwner()
	}
	status := strings.ToLower(strings.TrimPrefix(c.GetStatus().String(), "CONNECTION_STATUS_"))
	fmt.Fprintf(w, "%s\t%s\t%s\t%s", service.ConnectionID(c.GetName()), kind, secret, status)
	if e := c.GetLastError(); e != "" {
		fmt.Fprintf(w, "\t%s", e)
	}
	fmt.Fprintln(w)
}
