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
	var aws, gcp bool
	var accessKeyID, roleARN, region, tenant, clientID, entraAuth, cert string
	add := &cobra.Command{
		Use:   "add <id>",
		Short: "Keep a connection: --github-token, --github-app, --aws, --gcp or --entra-tenant",
		Long: `add keeps a connection the server reads a private source through.

  agni connections add gh --github-token env:GH_TOKEN --remote URL
  agni connections add acme --github-app 123 --installation 456 --key file:/srv/app.pem --remote URL
  agni connections add aws --aws --role-arn arn:aws:iam::123456789012:role/reader --region us-east-1 --remote URL
  agni connections add aws-keys --aws --access-key-id AKIA... --key env:AWS_SECRET --remote URL
  agni connections add gcs --gcp --key file:/srv/sa.json --remote URL
  agni connections add sp --entra-tenant contoso.onmicrosoft.com --entra-client <app-id> --entra-auth secret --key env:ENTRA_SECRET --remote URL

--aws with no keys uses the server's AWS default chain, such as an instance or IRSA role, and --gcp
with no key uses Application Default Credentials. --entra-auth is secret, certificate (with
--cert), managed-identity or workload-identity (with --key naming the federated token file). AWS,
GCP and Entra connections sign in as the server, so none can be --mine.

--seal reads the token, or the secret --key names, on this machine and sends it once, and the server
seals it under --connections-key. It suits a server in a container, whose environment and files this
machine can't name (agni issue 1802). A managed or workload identity has no secret to seal.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn := &webapi.Connection{DisplayName: title, BaseUrl: baseURL}
			req := &webapi.CreateConnectionRequest{ConnectionId: args[0], Connection: conn}
			// sealKey sends the secret --key names, read on this machine, for the server to seal in
			// place of the reference (agni issue 1802).
			sealKey := func() error {
				if !seal {
					return nil
				}
				if key == "" {
					return fmt.Errorf("--seal seals the secret --key names, and this connection names none")
				}
				secret, err := readSecret("--key", key)
				if err != nil {
					return err
				}
				conn.KeySource, req.ServerSecret = "", secret
				return nil
			}
			cloud := 0
			for _, on := range []bool{aws, gcp, tenant != "", token != "" || app != ""} {
				if on {
					cloud++
				}
			}
			switch {
			case cloud > 1:
				return fmt.Errorf("--github-token, --github-app, --aws, --gcp and --entra-tenant make different kinds of connection; pass one")
			case mine && (aws || gcp || tenant != ""):
				return fmt.Errorf("an AWS, GCP or Entra connection signs in as the server, so it can't be --mine")
			case aws:
				if (accessKeyID == "") != (key == "") {
					return fmt.Errorf("--access-key-id and --key go together for AWS; pass both, or neither for the server's default chain")
				}
				conn.Kind, conn.AccessKeyId, conn.KeySource, conn.RoleArn, conn.Region = webapi.ConnectionKind_CONNECTION_KIND_AWS, accessKeyID, key, roleARN, region
				if err := sealKey(); err != nil {
					return err
				}
			case gcp:
				conn.Kind, conn.KeySource = webapi.ConnectionKind_CONNECTION_KIND_GCP, key
				if err := sealKey(); err != nil {
					return err
				}
			case tenant != "":
				auth, ok := map[string]webapi.EntraAuth{
					"secret": webapi.EntraAuth_ENTRA_AUTH_SECRET, "certificate": webapi.EntraAuth_ENTRA_AUTH_CERTIFICATE,
					"managed-identity": webapi.EntraAuth_ENTRA_AUTH_MANAGED_IDENTITY, "workload-identity": webapi.EntraAuth_ENTRA_AUTH_WORKLOAD_IDENTITY,
				}[entraAuth]
				if !ok {
					return fmt.Errorf("--entra-auth %q: want secret, certificate, managed-identity or workload-identity", entraAuth)
				}
				conn.Kind, conn.TenantId, conn.ClientId, conn.EntraAuth, conn.KeySource, conn.CertSource = webapi.ConnectionKind_CONNECTION_KIND_ENTRA, tenant, clientID, auth, key, cert
				if err := sealKey(); err != nil {
					return err
				}
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
				return fmt.Errorf("name the sign-in: --github-token env:NAME, --github-app <app-id> --installation <id> --key file:/path, --aws, --gcp, or --entra-tenant <tenant> --entra-client <app-id> --entra-auth <how>")
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
	add.Flags().StringVar(&key, "key", "", "where a secret is, env:NAME or file:/abs/path: the GitHub App's private key, the AWS secret access key, the GCP service account key, or the Entra client secret, certificate key or federated token file")
	add.Flags().BoolVar(&aws, "aws", false, "an AWS connection for s3:// mounts (agni issue 1730)")
	add.Flags().StringVar(&accessKeyID, "access-key-id", "", "an AWS access key's id, with --aws and --key")
	add.Flags().StringVar(&roleARN, "role-arn", "", "an AWS role to assume before signing, with --aws")
	add.Flags().StringVar(&region, "region", "", "the AWS region, such as us-east-1, with --aws")
	add.Flags().BoolVar(&gcp, "gcp", false, "a Google Cloud connection for gs:// mounts (agni issue 1730)")
	add.Flags().StringVar(&tenant, "entra-tenant", "", "an Entra tenant, as a GUID or a domain, for an Entra connection for msgraph:// mounts (agni issue 1729)")
	add.Flags().StringVar(&clientID, "entra-client", "", "the Entra app registration's application id, or a user-assigned managed identity's client id")
	add.Flags().StringVar(&entraAuth, "entra-auth", "secret", "how an Entra connection signs in: secret, certificate, managed-identity or workload-identity")
	add.Flags().StringVar(&cert, "cert", "", "where an Entra certificate is, env:NAME or file:/abs/path, with --entra-auth certificate")
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
	case c.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_AWS:
		secret = "default chain"
		if keySource != "" {
			secret = fmt.Sprintf("key %s secret %s", c.GetAccessKeyId(), keySource)
		}
		if c.GetRoleArn() != "" {
			secret += " role " + c.GetRoleArn()
		}
	case c.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_GCP:
		secret = "application default credentials"
		if keySource != "" {
			secret = "key " + keySource
		}
	case c.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_ENTRA:
		how := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(c.GetEntraAuth().String(), "ENTRA_AUTH_"), "_", "-"))
		secret = fmt.Sprintf("tenant %s client %s %s", c.GetTenantId(), c.GetClientId(), how)
		if keySource != "" {
			secret += " " + keySource
		}
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
