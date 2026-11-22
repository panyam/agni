package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
)

// mountsCmd lists a server's mounts and makes and deletes the ones it keeps at runtime (agni issue
// 1749). Every subcommand talks to the server named by --remote. A mount from the server's own
// --mount flags is listed but never deleted.
func mountsCmd() *cobra.Command {
	var remoteURL string
	cmd := &cobra.Command{
		Use:   "mounts",
		Short: "List an agni server's mounts, and add or remove a GitHub mount while it runs",
		Long: `mounts lists the folders an agni serve serves, and adds or removes the mounts it makes at runtime.
A runtime mount is a GitHub repo snapshotted at one commit. The server needs --store and --uploads
to make one, and serves it again after a restart.

  agni mounts ls --remote http://localhost:8080
  agni mounts add jetson github://antmicro/jetson-agx-thor-baseboard --ref main --remote http://localhost:8080
  agni mounts rm jetson --remote http://localhost:8080

A private repo reads through a connection the server keeps, named with --connection (see agni
connections).`,
	}
	cmd.PersistentFlags().StringVar(&remoteURL, "remote", "", "the agni serve whose mounts these are, such as http://localhost:8080 (required)")
	client := func(cmd *cobra.Command) (webapiconnect.WorkspaceServiceClient, error) {
		if remoteURL == "" {
			return nil, fmt.Errorf("agni mounts needs --remote, the URL of an agni serve")
		}
		r, err := dialRemote(cmd.Context(), remoteURL)
		if err != nil {
			return nil, err
		}
		return webapiconnect.NewWorkspaceServiceClient(r.http, r.url), nil
	}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "List the server's mounts, each with its kind, source, commit, and whether it is fixed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			resp, err := unary(cmd.Context(), c.ListMounts, &webapi.ListMountsRequest{})
			if err != nil {
				return err
			}
			for _, m := range resp.GetMounts() {
				printMount(cmd.OutOrStdout(), m)
			}
			return nil
		},
	}

	var ref, connection string
	add := &cobra.Command{
		Use:   "add <name> <source>",
		Short: "Mount a GitHub repo, a bucket or a SharePoint folder, snapshotted on the server",
		Long: `add snapshots a source on the server and serves it as a mount. The source is one of:

  github://<owner>/<repo>                          a GitHub repo, at --ref
  s3://<bucket>/<prefix>                           an S3 bucket prefix (agni issue 1730)
  gs://<bucket>/<prefix>                           a Cloud Storage bucket prefix (agni issue 1730)
  msgraph://sites/<site>/drives/<drive>/root/<dir> a SharePoint or OneDrive folder (agni issue 1729)

A bucket or a drive has no refs, so the snapshot pins each file at the version it has now. A
private repo or bucket reads through --connection, and Microsoft Graph always does.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			m, err := unary(cmd.Context(), c.CreateMount, &webapi.CreateMountRequest{Name: args[0], Source: args[1], Ref: ref, Connection: connection})
			if err != nil {
				return err
			}
			printMount(cmd.OutOrStdout(), m)
			return nil
		},
	}
	add.Flags().StringVar(&ref, "ref", "", "the tag, branch or commit to snapshot, for a repo; the repo's default branch when empty")
	add.Flags().StringVar(&connection, "connection", "", "the connection to read a private source through, as <id> or connections/<id>; see agni connections")

	rm := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a mount the server made at runtime",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			if _, err := unary(cmd.Context(), c.DeleteMount, &webapi.DeleteMountRequest{Name: args[0]}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", args[0])
			return nil
		},
	}

	var hookURL string
	webhook := &cobra.Command{
		Use:   "webhook <name> --url https://<server>/hooks/github",
		Short: "Ask GitHub or Microsoft Graph to send a mount's changes to this server",
		Long: `webhook makes the push webhook on a github mount's repo, so a push to a branch a linked design
follows syncs the design (agni issue 1726). GitHub signs each delivery with the server's
--github-webhook-secret. --url is the server's /hooks/github as GitHub reaches it, since the server
can't tell its own public address. The mount's connection needs a token allowed to manage the repo's
webhooks. Running it again updates the hook rather than making a second one.

On a SharePoint or OneDrive mount it subscribes to the drive's changes instead, with --url the
server's /hooks/msgraph, which Graph requires to be https (agni issue 1729). Graph checks the URL at
once, so the server must already answer there with --msgraph-client-state. The server renews the
subscription before Graph ends it, about 29 days on. An S3 or GCS mount sends no events; --poll
follows it.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if hookURL == "" {
				return fmt.Errorf("--url names where GitHub sends the pushes, such as https://agni.example.com/hooks/github")
			}
			c, err := client(cmd)
			if err != nil {
				return err
			}
			h, err := unary(cmd.Context(), c.CreateMountWebhook, &webapi.CreateMountWebhookRequest{Name: args[0], Url: hookURL})
			if err != nil {
				return err
			}
			did := "updated"
			if h.GetCreated() {
				did = "made"
			}
			if id := h.GetSubscriptionId(); id != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s Graph subscription %s on %s, sending changes to %s until %s\n", h.GetMount(), did, id, h.GetSource(), h.GetUrl(), h.GetExpireTime().AsTime().UTC().Format(time.RFC3339))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s webhook %d on %s, sending %s to %s\n", h.GetMount(), did, h.GetId(), h.GetSource(), strings.Join(h.GetEvents(), ", "), h.GetUrl())
			return nil
		},
	}
	webhook.Flags().StringVar(&hookURL, "url", "", "where the host sends the changes: this server's /hooks/github or /hooks/msgraph as the host reaches it")

	cmd.AddCommand(ls, add, rm, webhook)
	return cmd
}

// printMount writes one mount as a tab-separated line: its name, its kind, where it came from, and
// whether it is fixed by the server's flags or made at runtime.
func printMount(w io.Writer, m *webapi.Mount) {
	kind := strings.ToLower(strings.TrimPrefix(m.GetKind().String(), "MOUNT_KIND_"))
	from := m.GetSource()
	if c := m.GetCommit(); c != "" {
		from += "@" + c
	}
	if m.GetSourceHidden() {
		from = "a private source you can't read"
	}
	life := "runtime"
	if m.GetFixed() {
		life = "fixed"
	}
	fmt.Fprintf(w, "%s\t%s\t%s\t%s", m.GetName(), kind, from, life)
	if c := m.GetConnection(); c != "" {
		fmt.Fprintf(w, "\tvia %s", strings.TrimPrefix(c, "connections/"))
	}
	fmt.Fprintln(w)
}
