package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"gopkg.in/yaml.v3"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/service"
	"github.com/panyam/agni/stdlib/rules/intent"
)

// designsCmd groups the commands that keep designs on a server (agni issue 1696). A design on a
// server holds artifacts, each one source such as a schematic with its sheets, a board, or a BOM.
// Every subcommand talks to the server named by --remote, since a laptop keeps its designs as
// folders and needs none of this.
func designsCmd() *cobra.Command {
	var remoteURL string
	cmd := &cobra.Command{
		Use:   "designs",
		Short: "Create, list and change the designs an agni server keeps, and put their artifacts",
		Long: `designs manages the designs an agni serve started with --store and --uploads keeps. A design is
named designs/<id> and holds artifacts. An artifact is one source of the design: a schematic with its
sheets, a netlist, a board, or files no reader opens, such as a BOM. The server reads each artifact
to find its type, and reads the design from its netlist, else its schematic, else its board.

  agni designs create bb-1.1.3 --series jetson --remote http://localhost:8080
  agni designs put bb-1.1.3 schematic ./kicad/ --remote http://localhost:8080
  agni designs push ./jetson-1.1.3 --id bb-1.1.3 --series jetson --remote http://localhost:8080`,
	}
	cmd.PersistentFlags().StringVar(&remoteURL, "remote", "", "the agni serve that keeps the designs, such as http://localhost:8080 (required)")
	client := func(cmd *cobra.Command) (*designsClient, error) {
		if remoteURL == "" {
			return nil, fmt.Errorf("agni designs needs --remote, the URL of an agni serve started with --store and --uploads")
		}
		r, err := dialRemote(cmd.Context(), remoteURL)
		if err != nil {
			return nil, err
		}
		return &designsClient{
			designs: webapiconnect.NewDesignServiceClient(r.http, r.url),
			snaps:   webapiconnect.NewSnapshotServiceClient(r.http, r.url),
			packs:   webapiconnect.NewPackServiceClient(r.http, r.url),
		}, nil
	}

	var title, series string
	create := &cobra.Command{
		Use:   "create <id>",
		Short: "Make a design with no artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			d, err := unary(cmd.Context(), c.designs.CreateDesign, &webapi.CreateDesignRequest{DesignId: args[0], Design: &webapi.Design{Title: title, Series: series}})
			if err != nil {
				return err
			}
			printDesign(cmd.OutOrStdout(), d)
			return nil
		},
	}
	create.Flags().StringVar(&title, "title", "", "the design's title; the id when empty")
	create.Flags().StringVar(&series, "series", "", "the series the design is a version in, such as memory_unit")

	var listSeries, listProject string
	list := &cobra.Command{
		Use:   "list",
		Short: "List the designs the server holds, stored and mounted",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			filter := ""
			switch {
			case listSeries != "" && listProject != "":
				return fmt.Errorf("--series and --project cannot be given together")
			case listSeries != "":
				filter = fmt.Sprintf("series = %q", listSeries)
			case listProject != "":
				filter = fmt.Sprintf("project = %q", projectName(listProject))
			}
			for token, pages := "", 0; ; pages++ {
				resp, err := unary(cmd.Context(), c.designs.ListDesigns, &webapi.ListDesignsRequest{Filter: filter, PageToken: token})
				if err != nil {
					return err
				}
				for _, d := range resp.GetDesigns() {
					printDesign(cmd.OutOrStdout(), d)
				}
				for _, u := range resp.GetUnreachable() {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\tunreachable: two mounted folders declare it\n", u)
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
	list.Flags().StringVar(&listSeries, "series", "", "list only the designs in this series")
	list.Flags().StringVar(&listProject, "project", "", "list only the designs this project lists, as <id> or projects/<id>")

	get := &cobra.Command{
		Use:   "get <id>",
		Short: "Print a design and its artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			d, err := unary(cmd.Context(), c.designs.GetDesign, &webapi.GetDesignRequest{Name: designName(args[0])})
			if err != nil {
				return err
			}
			printDesign(cmd.OutOrStdout(), d)
			if _, rev := service.SplitRevision(d.GetName()); rev != "" {
				// A revision's files are listed by agni designs revisions; its artifacts are the design's.
				return nil
			}
			return c.printArtifacts(cmd.Context(), cmd.OutOrStdout(), d.GetName())
		},
	}

	revisions := &cobra.Command{
		Use:   "revisions <id>",
		Short: "List the revisions of a design the server has read, newest first",
		Long: `revisions lists the design as of each version of its linked sources the server has read (agni
issue 1725). A link or a sync keeps one, and so does reading the design at a ref, designs/<id>@<ref>.
Each line gives the revision's name, when the server first read it, the tags and commits that name it,
and each linked artifact's source version. Any of them reads the design at that revision:

  agni diff designs/bb@1.1.2 designs/bb@1.1.3 --remote http://localhost:8080`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			req := &webapi.ListDesignRevisionsRequest{Parent: designName(args[0])}
			for {
				resp, err := unary(cmd.Context(), c.designs.ListDesignRevisions, req)
				if err != nil {
					return err
				}
				for _, r := range resp.GetRevisions() {
					printRevision(cmd.OutOrStdout(), r)
				}
				if req.PageToken = resp.GetNextPageToken(); req.PageToken == "" {
					return nil
				}
			}
		},
	}

	artifacts := &cobra.Command{
		Use:   "artifacts <id>",
		Short: "List a design's artifacts, each with its type, entry and files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			return c.printArtifacts(cmd.Context(), cmd.OutOrStdout(), designName(args[0]))
		},
	}

	put := &cobra.Command{
		Use:   "put <id> <artifact> <file-or-folder>",
		Short: "Create an artifact, or replace all its files",
		Long: `put sends a file, or every file under a folder with its path relative to that folder, as one
artifact of the design. An artifact of that name is replaced whole, so a schematic never holds sheets
from two releases. Only the contents the server lacks are uploaded.`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			files, err := localFiles(args[2])
			if err != nil {
				return err
			}
			req := &webapi.UpdateArtifactRequest{Artifact: files.artifact(service.ArtifactName(designID(args[0]), args[1])), AllowMissing: true}
			a, err := retryUpload(cmd.Context(), c, files, func() (*webapi.Artifact, error) { return unary(cmd.Context(), c.designs.UpdateArtifact, req) })
			if err != nil {
				return err
			}
			printArtifact(cmd.OutOrStdout(), a)
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm <id> [<artifact>]",
		Short: "Delete a design, or one of its artifacts",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			if len(args) == 2 {
				_, err = unary(cmd.Context(), c.designs.DeleteArtifact, &webapi.DeleteArtifactRequest{Name: service.ArtifactName(designID(args[0]), args[1])})
			} else {
				_, err = unary(cmd.Context(), c.designs.DeleteDesign, &webapi.DeleteDesignRequest{Name: designName(args[0])})
			}
			return err
		},
	}

	var pushID, pushTitle, pushSeries string
	var noPack bool
	push := &cobra.Command{
		Use:   "push <folder>",
		Short: "Keep a design folder on the server, one artifact per source",
		Long: `push sends a folder's files, and the server finds the design in them the way agni propose does
and puts each of its sources as an artifact: the file analysis reads, with every file it reads, then each view of it, such as the board.
Each artifact is named for its type, so a KiCad folder becomes a schematic and a board. A folder
holding two designs is refused, naming both.

The design is created when the server lacks it, and its artifacts are replaced in one write when it
has it. An artifact the folder no longer holds is left in place; agni designs rm removes it. A
design.yaml in the folder sets the default title and series, and its intent is kept as the design's
own. A design.yaml with no intent leaves the stored intent as it was.

When the design.yaml extends a project.yaml, push first keeps that project as a pack, the way agni
packs push does, under the project's name. A pack the server already holds with the same hash is
left alone. Check the design with --pack and that name. --no-pack skips it.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			pack := ""
			if !noPack {
				if pack, err = c.pushExtends(cmd.Context(), cmd.OutOrStdout(), args[0]); err != nil {
					return err
				}
			}
			if err := c.push(cmd.Context(), cmd.OutOrStdout(), args[0], pushID, pushTitle, pushSeries); err != nil {
				return err
			}
			if pack != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: check it under its project's rules with --pack %s\n", strings.TrimPrefix(pack, "packs/"))
			}
			return nil
		},
	}
	push.Flags().BoolVar(&noPack, "no-pack", false, "do not keep the project.yaml the design.yaml extends as a pack")
	push.Flags().StringVar(&pushID, "id", "", "the design's id on the server; the folder's design.yaml name, else the folder's name")
	push.Flags().StringVar(&pushTitle, "title", "", "the design's title, when the server makes the design")
	push.Flags().StringVar(&pushSeries, "series", "", "the design's series, when the server makes the design")

	var addAll bool
	var addOnly []string
	var addSeries string
	var noSymbolCopy bool
	add := &cobra.Command{
		Use:   "add <mount>/<path>",
		Short: "Keep the boards in a folder of a server's mount as designs",
		Long: `add proposes the designs the server finds in a folder of one of its mounts, one design per
board, and keeps the ones named as designs (agni issue 1736). The server reads each board's files
once, keeps its own copy, and records where they came from. It writes nothing into the mount.

With neither --yes nor --only, add lists the proposals and keeps nothing. --yes keeps them all, and
--only keeps the ones whose folder, id or entry it names (repeatable), and
--only <folder|id|entry>=<id> gives that one an id of its own.

A design keeps the symbol folders its design.yaml names as an artifact of its own, so it reads
alone, and --no-symbol-copy leaves them out. When the design.yaml extends a project, the server
keeps that project as a pack and the design reads under it, so the project's own symbol folders are
shared by every design added from it rather than copied.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			r, err := dialRemote(cmd.Context(), remoteURL)
			if err != nil {
				return err
			}
			uri := "mount://" + strings.Trim(strings.TrimPrefix(args[0], "mount://"), "/")
			proposed, err := unary(cmd.Context(), webapiconnect.NewWorkspaceServiceClient(r.http, r.url).ProposeDesigns, &webapi.ProposeDesignsRequest{Uri: uri})
			if err != nil {
				return err
			}
			// Each --only names a proposal by its folder, its default id or its entry, and may give
			// it an id of its own after '=' (agni issue 1756).
			only := map[string]string{}
			for _, o := range addOnly {
				pick, id, _ := strings.Cut(o, "=")
				only[pick] = id
			}
			var picks []*webapi.DesignToAdd
			for _, p := range proposed.GetDesigns() {
				id := service.DefaultDesignID(p, proposed.GetDesigns())
				named, picked := "", addAll
				entry := p.GetDesign().GetEntryUri()
				for _, k := range []string{p.GetFolder(), id, entry, strings.TrimPrefix(entry, uri+"/")} {
					if v, ok := only[k]; ok {
						named, picked = v, true
						break
					}
				}
				if !picked {
					fmt.Fprintf(cmd.OutOrStdout(), "proposed\t%s\t%s\t%d files\t%s\n", displayFolder(p.GetFolder()), id, len(p.GetFiles()), p.GetNote())
					continue
				}
				if named != "" {
					id = named
				}
				picks = append(picks, &webapi.DesignToAdd{Folder: p.GetFolder(), Entry: p.GetDesign().GetEntryUri(), DesignId: id, Series: addSeries})
			}
			if len(picks) == 0 {
				if len(proposed.GetDesigns()) == 0 {
					return fmt.Errorf("%s holds no design", uri)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "note: nothing kept; pass --yes to keep every proposal, or --only <folder or id>")
				return nil
			}
			resp, err := unary(cmd.Context(), c.designs.AddDesigns, &webapi.AddDesignsRequest{Uri: uri, Designs: picks, NoSymbolCopy: noSymbolCopy})
			if err != nil {
				return err
			}
			for _, u := range resp.GetUncopiedSymbols() {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s names a symbol folder that was not copied; it reads those parts only under a pack holding them\n", u)
			}
			for _, d := range resp.GetDesigns() {
				printDesign(cmd.OutOrStdout(), d)
			}
			return nil
		},
	}
	add.Flags().BoolVar(&addAll, "yes", false, "keep every proposal as a design")
	add.Flags().StringArrayVar(&addOnly, "only", nil, "keep the proposal at this folder, with this id, or with this entry, as <folder|id|entry>[=<new id>] to name its id (repeatable)")
	add.Flags().StringVar(&addSeries, "series", "", "the series every kept design joins")
	add.Flags().BoolVar(&noSymbolCopy, "no-symbol-copy", false, "leave out the symbol folders each design.yaml names, for a library a pack already holds")

	link := &cobra.Command{
		Use:   "link <id> <artifact> <mount>[/<path>][@<ref>]",
		Short: "Point an artifact at a file or folder in a mount, and read its files",
		Long: `link points an artifact at a path in a mount, such as a github mount made with agni mounts add,
and the server reads its files and keeps them (agni issue 1723). A path naming a design's entry, such
as a KiCad root sheet, takes the files a reader opens from it. A path naming a board takes the board.
A path naming a folder takes every file under it. A ref naming a branch, or no ref, is followed, and
agni designs sync moves it. A tag or commit is pinned. Linking again replaces the source.

  agni designs link bb schematic jetson/jetson-agx-thor-baseboard.kicad_sch@3411086e --remote http://localhost:8080`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			src, err := parseLinkSource(args[2])
			if err != nil {
				return err
			}
			a, err := unary(cmd.Context(), c.designs.LinkArtifact, &webapi.LinkArtifactRequest{Name: service.ArtifactName(designID(args[0]), args[1]), Source: src, AllowMissing: true})
			if err != nil {
				return err
			}
			printArtifact(cmd.OutOrStdout(), a)
			return nil
		},
	}

	unlink := &cobra.Command{
		Use:   "unlink <id> <artifact>",
		Short: "Keep a linked artifact's files as uploaded ones, so put can replace them",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			a, err := unary(cmd.Context(), c.designs.UnlinkArtifact, &webapi.UnlinkArtifactRequest{Name: service.ArtifactName(designID(args[0]), args[1])})
			if err != nil {
				return err
			}
			printArtifact(cmd.OutOrStdout(), a)
			return nil
		},
	}

	var dryRun bool
	sync := &cobra.Command{
		Use:   "sync <id> [<artifact>...]",
		Short: "Move a design's followed artifacts to their sources' current versions",
		Long: `sync asks each followed artifact's source for its current version and reads only the files the
server lacks. Artifacts linked to one mount at one ref move together, in one write. A pinned artifact
is listed and never moved. --dry-run says what a sync would change and writes nothing.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			resp, err := unary(cmd.Context(), c.designs.SyncDesign, &webapi.SyncDesignRequest{Name: designName(args[0]), Artifacts: args[1:], ValidateOnly: dryRun})
			if err != nil {
				return err
			}
			for _, a := range resp.GetArtifacts() {
				printSync(cmd.OutOrStdout(), a)
			}
			return nil
		},
	}
	sync.Flags().BoolVar(&dryRun, "dry-run", false, "report what a sync would change, and write nothing")

	cmd.AddCommand(create, list, get, artifacts, put, rm, push, add, link, unlink, sync, revisions)
	return cmd
}

// designsClient is the server a designs subcommand talks to.
type designsClient struct {
	designs webapiconnect.DesignServiceClient
	snaps   webapiconnect.SnapshotServiceClient
	packs   webapiconnect.PackServiceClient
}

// pushExtends keeps the project.yaml that dir's design.yaml extends as a pack, under the project's
// name, and returns the pack's name (agni issue 1746). A design.yaml extending nothing returns "". A
// pack the server holds with the same hash is not written again. An extends naming a mount:// URI
// names a project this machine cannot read, so it is noted and skipped.
func (c *designsClient) pushExtends(ctx context.Context, w io.Writer, dir string) (string, error) {
	extends := readLocalDescriptor(filepath.Join(dir, service.DesignDescriptorName)).Extends
	if extends == "" {
		return "", nil
	}
	if strings.Contains(extends, "://") {
		fmt.Fprintf(w, "note: the design extends %s, which push cannot read; keep it with agni packs push\n", extends)
		return "", nil
	}
	pack, files, descriptor, err := localPack(filepath.Join(dir, filepath.FromSlash(extends)), "", "")
	if err != nil {
		return "", fmt.Errorf("the design's extends: %w (pass --no-pack to push the design alone)", err)
	}
	have, err := unary(ctx, c.packs.GetPack, &webapi.GetPackRequest{Name: pack.GetName()})
	switch {
	case err == nil && have.GetHash() == service.PackHash(pack):
		fmt.Fprintf(w, "%s\tunchanged\n", pack.GetName())
		return pack.GetName(), nil
	case err != nil && connect.CodeOf(err) != connect.CodeNotFound:
		return "", err
	}
	out, err := (&packsClient{packs: c.packs, up: c}).write(ctx, pack, files)
	if err != nil {
		return "", fmt.Errorf("%s: %w", descriptor, err)
	}
	printPack(w, out)
	return out.GetName(), nil
}

func designName(arg string) string {
	return service.StandaloneDesignName(designID(arg))
}

func designID(arg string) string {
	return strings.TrimPrefix(arg, "designs/")
}

func projectName(arg string) string {
	if strings.HasPrefix(arg, "projects/") {
		return arg
	}
	return "projects/" + arg
}

func printDesign(w io.Writer, d *webapi.Design) {
	var notes []string
	if d.GetTitle() != "" && d.GetTitle() != designID(d.GetName()) {
		notes = append(notes, fmt.Sprintf("%q", d.GetTitle()))
	}
	if s := d.GetSeries(); s != "" {
		if d.GetSeriesLatest() {
			s += ", latest"
		}
		notes = append(notes, "series "+s)
	}
	if t := d.GetUpdateTime(); t != nil {
		notes = append(notes, "updated "+t.AsTime().UTC().Format("2006-01-02 15:04:05"))
	}
	if d.GetEntryUri() == "" {
		notes = append(notes, "no artifact a reader opens yet")
	} else {
		notes = append(notes, "reads "+d.GetEntryUri())
	}
	if src := d.GetAccessSources(); len(src) > 0 {
		notes = append(notes, "opens for who can read "+strings.Join(src, " and "))
	}
	fmt.Fprintf(w, "%s\t%s\n", d.GetName(), strings.Join(notes, "; "))
}

// printRevision writes one revision as a tab-separated line: its name, when the server first read
// it, the refs that name it, and each artifact's source version, the current one marked.
func printRevision(w io.Writer, r *webapi.DesignRevision) {
	notes := []string{"read " + r.GetCreateTime().AsTime().UTC().Format("2006-01-02 15:04:05")}
	if r.GetCurrent() {
		notes = append(notes, "current")
	}
	if refs := r.GetRefs(); len(refs) > 0 {
		notes = append(notes, "refs "+strings.Join(refs, ", "))
	}
	ids := slices.Sorted(maps.Keys(r.GetSourceVersions()))
	for _, id := range ids {
		notes = append(notes, id+" at "+r.GetSourceVersions()[id])
	}
	fmt.Fprintf(w, "%s\t%s\n", r.GetName(), strings.Join(notes, "; "))
}

func printArtifact(w io.Writer, a *webapi.Artifact) {
	typ := strings.ToLower(strings.TrimPrefix(a.GetType().String(), "ARTIFACT_TYPE_"))
	entry := a.GetEntry()
	if entry == "" {
		entry = "-"
	}
	fmt.Fprintf(w, "%s\t%s\tentry %s\t%d files\tcontent %s", a.GetName(), typ, entry, len(a.GetFiles()), artifactDigest(a))
	if src := a.GetSource(); src.GetMount() != "" && src.GetMount() != uploadsMount {
		fmt.Fprintf(w, "\tfrom %s", service.SourceString(src))
		if v := src.GetVersion(); v != "" && v != src.GetRef() {
			fmt.Fprintf(w, " at %s", v)
		}
		// Only a link records a read hash. A design added from a mounted folder was read once, and
		// neither follows nor pins anything (agni issue 1736).
		switch {
		case src.GetReadHash() == "":
		case src.GetFollow():
			fmt.Fprint(w, ", followed")
		default:
			fmt.Fprint(w, ", pinned")
		}
	}
	fmt.Fprintln(w)
}

// parseLinkSource reads `<mount>[/<path>][@<ref>]`. The ref follows the last '@'.
func parseLinkSource(s string) (*webapi.ArtifactSource, error) {
	src := &webapi.ArtifactSource{}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s, src.Ref = s[:i], s[i+1:]
		if src.Ref == "" {
			return nil, fmt.Errorf("%q names an empty ref after '@'", s+"@")
		}
	}
	src.Mount, src.Path, _ = strings.Cut(s, "/")
	if src.Mount == "" {
		return nil, fmt.Errorf("%q names no mount; write <mount>[/<path>][@<ref>]", s)
	}
	return src, nil
}

// printSync writes one artifact's line of a sync.
func printSync(w io.Writer, a *webapi.ArtifactSync) {
	from, to := a.GetFromVersion(), a.GetToVersion()
	what := "unchanged"
	if a.GetChanged() {
		what = "changed"
	}
	if from != "" || to != "" {
		what += "\t" + shortVersion(from) + " -> " + shortVersion(to)
	}
	fmt.Fprintf(w, "%s\t%s\tfetched %d files, %d bytes\n", a.GetArtifact(), what, a.GetFilesFetched(), a.GetBytesFetched())
}

func shortVersion(v string) string {
	if len(v) > 12 {
		return v[:12]
	}
	if v == "" {
		return "-"
	}
	return v
}

// artifactDigest is a short hash over an artifact's paths and their contents' hashes, so two listings
// show whether its files changed.
func artifactDigest(a *webapi.Artifact) string {
	var lines []string
	for _, f := range a.GetFiles() {
		lines = append(lines, f.GetPath()+"\x00"+f.GetSha256())
	}
	slices.Sort(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])[:12]
}

func (c *designsClient) printArtifacts(ctx context.Context, w io.Writer, design string) error {
	for token, pages := "", 0; ; pages++ {
		resp, err := unary(ctx, c.designs.ListArtifacts, &webapi.ListArtifactsRequest{Parent: design, PageToken: token})
		if err != nil {
			return err
		}
		for _, a := range resp.GetArtifacts() {
			printArtifact(w, a)
		}
		if token = resp.GetNextPageToken(); token == "" {
			return nil
		}
		if pages > 10000 {
			return fmt.Errorf("the server's artifact listing has not ended after %d pages", pages)
		}
	}
}

// fileSet is files read on this machine, each with its path relative to the artifact.
type fileSet struct {
	files  []*webapi.SnapshotFile
	byHash map[string][]byte
}

func (s fileSet) artifact(name string) *webapi.Artifact {
	return &webapi.Artifact{Name: name, Files: s.files}
}

// add reads one file and records it under rel.
func (s *fileSet) add(abs, rel string) error {
	b, err := os.ReadFile(abs) // #nosec G304 -- a file under the folder the user named to send
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	h := "sha256:" + hex.EncodeToString(sum[:])
	s.files = append(s.files, &webapi.SnapshotFile{Path: rel, Sha256: h, Size: int64(len(b))})
	if s.byHash == nil {
		s.byHash = map[string][]byte{}
	}
	s.byHash[h] = b
	return nil
}

// localFiles reads a file, or every file under a folder by its path relative to the folder. Hidden
// files and folders are left out, as the server refuses them.
func localFiles(arg string) (fileSet, error) {
	var s fileSet
	fi, err := os.Stat(arg)
	if err != nil {
		return s, err
	}
	if !fi.IsDir() {
		return s, s.add(arg, filepath.Base(arg))
	}
	rels, err := walkLocal(arg)
	if err != nil {
		return s, err
	}
	for _, rel := range rels {
		if err := s.add(filepath.Join(arg, filepath.FromSlash(rel)), rel); err != nil {
			return s, err
		}
	}
	if len(s.files) == 0 {
		return s, fmt.Errorf("%s holds no files", arg)
	}
	return s, nil
}

// walkLocal lists the files under dir, slash-separated and relative to it, skipping hidden ones.
func walkLocal(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != dir && strings.HasPrefix(de.Name(), ".") {
			if de.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if de.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	slices.Sort(out)
	return out, err
}

// retryUpload makes a write, and when the server answers that it lacks contents, uploads them from
// have and makes the write again. A write may name its contents in rounds, as ImportPackFiles asks
// for project.yaml before the files it names, so it repeats while each answer asks for contents not
// sent yet.
func retryUpload[T any](ctx context.Context, c *designsClient, have fileSet, write func() (T, error)) (T, error) {
	sent := map[string]bool{}
	for {
		out, err := write()
		var ce *connect.Error
		if err == nil || !errors.As(err, &ce) {
			return out, err
		}
		var missing []string
		for _, d := range ce.Details() {
			if v, derr := d.Value(); derr == nil {
				if bm, ok := v.(*webapi.BlobsMissing); ok {
					for _, h := range bm.GetSha256() {
						if !sent[h] {
							missing = append(missing, h)
						}
					}
				}
			}
		}
		if len(missing) == 0 {
			return out, err
		}
		read := func(h string) ([]byte, error) {
			b, ok := have.byHash[h]
			if !ok {
				return nil, fmt.Errorf("the server asked for contents this push did not send (%s)", h)
			}
			return b, nil
		}
		if err := (remoteSnapshot{c.snaps}).upload(ctx, missing, read); err != nil {
			return out, err
		}
		for _, h := range missing {
			sent[h] = true
		}
	}
}

// push keeps a folder's design on the server.
func (c *designsClient) push(ctx context.Context, w io.Writer, dir, id, title, series string) error {
	rels, err := walkLocal(dir)
	if err != nil {
		return err
	}
	var all fileSet
	// A design.yaml goes too. The server stores none of it, and reads its entry to pick the design
	// when the folder holds sheets its root never places.
	for _, r := range rels {
		if err := all.add(filepath.Join(dir, filepath.FromSlash(r)), r); err != nil {
			return err
		}
	}
	if len(all.files) == 0 {
		return fmt.Errorf("%s holds no files", dir)
	}
	declared := readLocalDescriptor(filepath.Join(dir, service.DesignDescriptorName))
	declaredIntent, err := declared.intent()
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Join(dir, service.DesignDescriptorName), err)
	}
	if id == "" {
		id = declared.Name
	}
	if title == "" {
		title = declared.Title
	}
	if series == "" {
		series = declared.Series
	}
	if id == "" {
		id = strings.ToLower(filepath.Base(filepath.Clean(dir)))
	}
	name := designName(id)
	created := false
	if _, err := unary(ctx, c.designs.GetDesign, &webapi.GetDesignRequest{Name: name}); connect.CodeOf(err) == connect.CodeNotFound {
		var cfg *webapi.AnalysisConfig
		if declaredIntent != nil {
			cfg = &webapi.AnalysisConfig{Intent: declaredIntent}
		}
		if _, err := unary(ctx, c.designs.CreateDesign, &webapi.CreateDesignRequest{DesignId: designID(id), Design: &webapi.Design{Title: title, Series: series, Config: cfg}}); err != nil {
			return err
		}
		created = true
	} else if err != nil {
		return err
	} else if title != "" || series != "" || declaredIntent != nil {
		var paths []string
		if title != "" {
			paths = append(paths, "title")
		}
		if series != "" {
			paths = append(paths, "series")
		}
		in := &webapi.Design{Name: name, Title: title, Series: series}
		if declaredIntent != nil {
			paths = append(paths, "config.intent")
			in.Config = &webapi.AnalysisConfig{Intent: declaredIntent}
		}
		if _, err := unary(ctx, c.designs.UpdateDesign, &webapi.UpdateDesignRequest{Design: in, UpdateMask: &fieldmaskpb.FieldMask{Paths: paths}}); err != nil {
			return err
		}
	}
	// The server groups the files into artifacts (ImportDesignFiles, agni issue 1719), as the viewer's
	// upload does, so the two cannot group one folder two ways.
	req := &webapi.ImportDesignFilesRequest{Parent: name, Files: all.files}
	resp, err := retryUpload(ctx, c, all, func() (*webapi.ImportDesignFilesResponse, error) {
		return unary(ctx, c.designs.ImportDesignFiles, req)
	})
	if err != nil {
		if created {
			// A design this push made and could not fill would be left empty, so it goes too.
			_, _ = unary(context.WithoutCancel(ctx), c.designs.DeleteDesign, &webapi.DeleteDesignRequest{Name: name})
		}
		return fmt.Errorf("%s: %w", dir, err)
	}
	d, err := unary(ctx, c.designs.GetDesign, &webapi.GetDesignRequest{Name: name})
	if err != nil {
		return err
	}
	printDesign(w, d)
	for _, a := range resp.GetArtifacts() {
		printArtifact(w, a)
	}
	if u := resp.GetUnread(); len(u) > 0 {
		fmt.Fprintf(w, "not kept, as no reader opens them: %s\n", strings.Join(u, ", "))
	}
	return nil
}

// localDescriptor is the part of a local design.yaml push reads for defaults. It reads loosely, as
// ProposeDesigns does, since push sends no descriptor and internal/projects checks one strictly where
// one is read as a design.
type localDescriptor struct {
	Name    string    `yaml:"name"`
	Title   string    `yaml:"title"`
	Series  string    `yaml:"series"`
	Extends string    `yaml:"extends"`
	Intent  yaml.Node `yaml:"intent"`
}

// intent binds the descriptor's intent section, nil when it has none. A section that does not bind
// is an error, since pushing the design without it would check the board as if it declared nothing.
func (d localDescriptor) intent() (*configpb.DesignIntent, error) {
	if d.Intent.IsZero() || d.Intent.Tag == "!!null" {
		return nil, nil
	}
	return intent.DecodeSection(&d.Intent)
}

// readLocalDescriptor reads a design.yaml's name, title and series. A missing or unreadable one gives
// no defaults.
func readLocalDescriptor(p string) localDescriptor {
	var d localDescriptor
	if raw, err := os.ReadFile(p); err == nil { // #nosec G304 -- the design.yaml in the folder the user named
		_ = yaml.Unmarshal(raw, &d)
	}
	return d
}

// displayFolder names a proposal's folder, "." for the folder that was asked about.
func displayFolder(f string) string {
	if f == "" {
		return "."
	}
	return f
}
