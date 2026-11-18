package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

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

// localPack reads the project.yaml at arg, or in the folder arg names, into the pack push sends, with
// the files its relative tiers name. It returns the descriptor's path for messages. An empty id or
// title takes the project's own.
func localPack(arg, id, title string) (*webapi.Pack, fileSet, string, error) {
	descriptor := arg
	if fi, err := os.Stat(arg); err != nil {
		return nil, fileSet{}, "", err
	} else if fi.IsDir() {
		descriptor = filepath.Join(arg, projects.ProjectDescriptor)
	}
	raw, err := os.ReadFile(descriptor) // #nosec G304 -- the project.yaml the user named to send
	if err != nil {
		return nil, fileSet{}, "", err
	}
	declared, project, src, err := projects.ParseProject(bytes.NewReader(raw))
	if err != nil {
		return nil, fileSet{}, "", fmt.Errorf("%s: %w", descriptor, err)
	}
	if src.Extends != "" {
		return nil, fileSet{}, "", fmt.Errorf("%s extends %s, and a pack extends nothing; push that one as its own pack and name both with --pack", descriptor, src.Extends)
	}
	if id == "" {
		id = declared
	}
	if title == "" {
		title = project.GetTitle()
	}
	cfg := project.GetConfig()
	dir := filepath.Dir(descriptor)
	var files fileSet
	tier := func(refs []string) ([]string, error) {
		var out []string
		for _, r := range refs {
			if strings.Contains(r, "://") {
				out = append(out, r)
				continue
			}
			rel := path.Clean(filepath.ToSlash(r))
			if err := addTier(&files, dir, rel); err != nil {
				return nil, err
			}
			out = append(out, rel)
		}
		return out, nil
	}
	if cfg.ProfileUris, err = tier(src.Profiles); err != nil {
		return nil, fileSet{}, "", err
	}
	if cfg.ParamUris, err = tier(src.Params); err != nil {
		return nil, fileSet{}, "", err
	}
	if cfg.SymbolPathUris, err = tier(src.Symbols); err != nil {
		return nil, fileSet{}, "", err
	}
	if cfg.LibraryUris, err = tier(src.Lib); err != nil {
		return nil, fileSet{}, "", err
	}
	if cfg.QuerySetUris, err = tier(src.Queries); err != nil {
		return nil, fileSet{}, "", err
	}
	return &webapi.Pack{Name: packName(id), Title: title, Config: cfg, Files: files.files}, files, descriptor, nil
}

// write creates pack, or replaces the server's pack of that name whole, uploading the contents the
// server lacks.
func (c *packsClient) write(ctx context.Context, pack *webapi.Pack, files fileSet) (*webapi.Pack, error) {
	return retryUpload(ctx, c.up, files, func() (*webapi.Pack, error) {
		_, err := unary(ctx, c.packs.GetPack, &webapi.GetPackRequest{Name: pack.GetName()})
		if connect.CodeOf(err) == connect.CodeNotFound {
			return unary(ctx, c.packs.CreatePack, &webapi.CreatePackRequest{PackId: strings.TrimPrefix(pack.GetName(), "packs/"), Pack: pack})
		}
		if err != nil {
			return nil, err
		}
		return unary(ctx, c.packs.UpdatePack, &webapi.UpdatePackRequest{Pack: pack, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title", "config", "files"}}})
	})
}

// addTier adds the file or every file under the folder at rel, relative to dir, each under its path
// relative to dir.
func addTier(files *fileSet, dir, rel string) error {
	if rel == ".." || strings.HasPrefix(rel, "../") || path.IsAbs(rel) {
		return fmt.Errorf("%s leaves the project's folder; a pack sends only files beside its project.yaml, so name it with a mount:// URI", rel)
	}
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	fi, err := os.Stat(abs)
	if err != nil {
		return err
	}
	have := func(p string) bool {
		return slices.ContainsFunc(files.files, func(f *webapi.SnapshotFile) bool { return f.GetPath() == p })
	}
	if !fi.IsDir() {
		if have(rel) {
			return nil
		}
		return files.add(abs, rel)
	}
	rels, err := walkLocal(abs)
	if err != nil {
		return err
	}
	if len(rels) == 0 {
		return fmt.Errorf("%s holds no files", rel)
	}
	for _, r := range rels {
		// Two tiers may name one folder, as profiles and params sometimes share one, and its files go once.
		if have(path.Join(rel, r)) {
			continue
		}
		if err := files.add(filepath.Join(abs, filepath.FromSlash(r)), path.Join(rel, r)); err != nil {
			return err
		}
	}
	return nil
}

// getPack reads one pack the server keeps, by resource name.
func (r *remoteServer) getPack(ctx context.Context, name string) (*webapi.Pack, error) {
	resp, err := webapiconnect.NewPackServiceClient(r.http, r.url).GetPack(ctx, connect.NewRequest(&webapi.GetPackRequest{Name: name}))
	if err != nil {
		return nil, rpcFailed(err)
	}
	return resp.Msg, nil
}
