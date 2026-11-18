package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
)

func packsCmd() *cobra.Command {
	var remoteURL string
	cmd := &cobra.Command{
		Use:   "packs",
		Short: "Keep packs of rules on an agni server, and list and remove them",
		Long: `packs manages the packs an agni serve started with --store keeps (agni issue 1704). A pack is
named packs/<id> and holds the rules a design is judged by: a naming convention, checklists, interface
profiles, datasheet parameters and shared intent. It holds what a project.yaml holds, and push reads
one. A pack's profile and params folders go to the server with it, so the server needs --uploads too.

  agni packs push ./jetson-project --id jetson --remote http://localhost:8080
  agni check designs/bb-1.1.3 --pack jetson --remote http://localhost:8080`,
	}
	cmd.PersistentFlags().StringVar(&remoteURL, "remote", "", "the agni serve that keeps the packs, such as http://localhost:8080 (required)")
	client := func(cmd *cobra.Command) (*packsClient, error) {
		if remoteURL == "" {
			return nil, fmt.Errorf("agni packs needs --remote, the URL of an agni serve started with --store")
		}
		r, err := dialRemote(cmd.Context(), remoteURL)
		if err != nil {
			return nil, err
		}
		return &packsClient{
			packs: webapiconnect.NewPackServiceClient(r.http, r.url),
			up:    &designsClient{snaps: webapiconnect.NewSnapshotServiceClient(r.http, r.url)},
		}, nil
	}

	var pushID, pushTitle string
	push := &cobra.Command{
		Use:   "push <project.yaml or its folder>",
		Short: "Keep a project.yaml's rules on the server as a pack",
		Long: `push reads a project.yaml and keeps its conventions, checklists and intent as a pack, with every
file its profiles, params, symbols, lib and queries name. A path relative to the project.yaml is sent
with its files, and a mount:// URI is kept as written. A project.yaml that extends another is refused,
since a pack layers by the order a check names packs: push the other as its own pack and name both.

The pack is created when the server lacks it, and replaced whole when it has it.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			return c.push(cmd.Context(), cmd.OutOrStdout(), args[0], pushID, pushTitle)
		},
	}
	push.Flags().StringVar(&pushID, "id", "", "the pack's id on the server; the project.yaml's name when empty")
	push.Flags().StringVar(&pushTitle, "title", "", "the pack's title; the project.yaml's title when empty")

	list := &cobra.Command{
		Use:   "list",
		Short: "List the packs the server keeps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			for token, pages := "", 0; ; pages++ {
				resp, err := unary(cmd.Context(), c.packs.ListPacks, &webapi.ListPacksRequest{PageToken: token})
				if err != nil {
					return err
				}
				for _, p := range resp.GetPacks() {
					printPack(cmd.OutOrStdout(), p)
				}
				if token = resp.GetNextPageToken(); token == "" {
					return nil
				}
				if pages > 10000 {
					return fmt.Errorf("the server's listing has not ended after %d pages", pages)
				}
			}
		},
	}

	get := &cobra.Command{
		Use:   "get <id>",
		Short: "Print a pack, its tiers and its files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			p, err := unary(cmd.Context(), c.packs.GetPack, &webapi.GetPackRequest{Name: packName(args[0])})
			if err != nil {
				return err
			}
			printPack(cmd.OutOrStdout(), p)
			printPackDetail(cmd.OutOrStdout(), p)
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete a pack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			_, err = unary(cmd.Context(), c.packs.DeletePack, &webapi.DeletePackRequest{Name: packName(args[0])})
			return err
		},
	}

	cmd.AddCommand(push, list, get, rm)
	return cmd
}

// packsClient is the server a packs subcommand talks to. up uploads the contents the server lacks.
type packsClient struct {
	packs webapiconnect.PackServiceClient
	up    *designsClient
}

// packName turns <id> or packs/<id> into the pack's resource name.
func packName(arg string) string {
	return service.PackName(strings.TrimPrefix(arg, "packs/"))
}

// packNames turns each --pack value into a resource name.
func packNames(args []string) []string {
	var out []string
	for _, a := range args {
		out = append(out, packName(a))
	}
	return out
}

func printPack(w io.Writer, p *webapi.Pack) {
	fmt.Fprintf(w, "%s\t%s\t%d files\t%s\n", p.GetName(), p.GetTitle(), len(p.GetFiles()), p.GetHash())
}

func printPackDetail(w io.Writer, p *webapi.Pack) {
	cfg := p.GetConfig()
	if c := cfg.GetConventions(); c != nil {
		fmt.Fprintf(w, "  conventions: %s\n", c.GetName())
	}
	for _, c := range cfg.GetChecklists() {
		fmt.Fprintf(w, "  checklist: %s\n", c.GetName())
	}
	if cfg.GetIntent() != nil {
		fmt.Fprintln(w, "  intent: declared")
	}
	for _, t := range []struct {
		name string
		refs []string
	}{{"profiles", cfg.GetProfileUris()}, {"params", cfg.GetParamUris()}, {"symbols", cfg.GetSymbolPathUris()}, {"lib", cfg.GetLibraryUris()}, {"queries", cfg.GetQuerySetUris()}} {
		for _, r := range t.refs {
			fmt.Fprintf(w, "  %s: %s\n", t.name, r)
		}
	}
}

// push keeps a project.yaml's rules on the server as a pack.
func (c *packsClient) push(ctx context.Context, w io.Writer, arg, id, title string) error {
	pack, files, descriptor, err := localPack(arg, id, title)
	if err != nil {
		return err
	}
	out, err := c.write(ctx, pack, files)
	if err != nil {
		return fmt.Errorf("%s: %w", descriptor, err)
	}
	printPack(w, out)
	printPackDetail(w, out)
	return nil
}

// localPack reads the project.yaml at arg, or in the folder arg names, into the pack push sends, as
// the server reads one (projects.PackFromFiles). files holds project.yaml and the files its relative
// tiers name, and the pack's files carry their hashes. It returns the descriptor's path for messages.
// An empty id or title takes the project's own.
func localPack(arg, id, title string) (*webapi.Pack, fileSet, string, error) {
	descriptor := arg
	if fi, err := os.Stat(arg); err != nil {
		return nil, fileSet{}, "", err
	} else if fi.IsDir() {
		descriptor = filepath.Join(arg, projects.ProjectDescriptor)
	} else if filepath.Base(arg) != projects.ProjectDescriptor {
		return nil, fileSet{}, "", fmt.Errorf("%s is not a %s", arg, projects.ProjectDescriptor)
	}
	dir := filepath.Dir(descriptor)
	rels, err := walkLocal(dir)
	if err != nil {
		return nil, fileSet{}, "", err
	}
	declared, pack, kept, err := projects.PackFromFiles(rels, func(rel string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel))) // #nosec G304 -- a file in the project folder the user named
	})
	if err != nil {
		return nil, fileSet{}, "", fmt.Errorf("%s: %w", descriptor, err)
	}
	var files fileSet
	for _, rel := range append([]string{projects.ProjectDescriptor}, kept...) {
		if err := files.add(filepath.Join(dir, filepath.FromSlash(rel)), rel); err != nil {
			return nil, fileSet{}, "", err
		}
	}
	if id == "" {
		id = declared
	}
	if title != "" {
		pack.Title = title
	}
	pack.Name = packName(id)
	pack.Files = files.files[1:]
	return pack, files, descriptor, nil
}

// write keeps pack on the server from the folder's files, creating it or replacing it whole, and
// uploads the contents the server asks for.
func (c *packsClient) write(ctx context.Context, pack *webapi.Pack, files fileSet) (*webapi.Pack, error) {
	return retryUpload(ctx, c.up, files, func() (*webapi.Pack, error) {
		return unary(ctx, c.packs.ImportPackFiles, &webapi.ImportPackFilesRequest{
			PackId: strings.TrimPrefix(pack.GetName(), "packs/"), Title: pack.GetTitle(), Files: files.files,
		})
	})
}

// getPack reads one pack the server keeps, by resource name.
func (r *remoteServer) getPack(ctx context.Context, name string) (*webapi.Pack, error) {
	resp, err := webapiconnect.NewPackServiceClient(r.http, r.url).GetPack(ctx, connect.NewRequest(&webapi.GetPackRequest{Name: name}))
	if err != nil {
		return nil, rpcFailed(err)
	}
	return resp.Msg, nil
}
