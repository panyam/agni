package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"gopkg.in/yaml.v3"

	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/service"
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
		Long: `designs manages the designs an agni serve started with --store-dir keeps. A design is
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
			return nil, fmt.Errorf("agni designs needs --remote, the URL of an agni serve started with --store-dir")
		}
		r, err := dialRemote(cmd.Context(), remoteURL)
		if err != nil {
			return nil, err
		}
		return &designsClient{
			designs: webapiconnect.NewDesignServiceClient(r.http, r.url),
			snaps:   webapiconnect.NewSnapshotServiceClient(r.http, r.url),
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
			return c.printArtifacts(cmd.Context(), cmd.OutOrStdout(), d.GetName())
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
	push := &cobra.Command{
		Use:   "push <folder>",
		Short: "Keep a design folder on the server, one artifact per source",
		Long: `push finds the design in a folder the way agni propose does, and puts each of its sources as an
artifact: the file analysis reads, with every file it reads, then each view of it, such as the board.
Each artifact is named for its type, so a KiCad folder becomes a schematic and a board. A folder
holding two designs is refused, naming both.

The design is created when the server lacks it, and its artifacts are replaced in one write when it
has it. An artifact the folder no longer holds is left in place; agni designs rm removes it. A
design.yaml in the folder sets the default title and series, and its intent is not sent.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			return c.push(cmd.Context(), cmd.OutOrStdout(), args[0], pushID, pushTitle, pushSeries)
		},
	}
	push.Flags().StringVar(&pushID, "id", "", "the design's id on the server; the folder's design.yaml name, else the folder's name")
	push.Flags().StringVar(&pushTitle, "title", "", "the design's title, when the server makes the design")
	push.Flags().StringVar(&pushSeries, "series", "", "the design's series, when the server makes the design")

	cmd.AddCommand(create, list, get, artifacts, put, rm, push)
	return cmd
}

// designsClient is the server a designs subcommand talks to.
type designsClient struct {
	designs webapiconnect.DesignServiceClient
	snaps   webapiconnect.SnapshotServiceClient
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
	fmt.Fprintf(w, "%s\t%s\n", d.GetName(), strings.Join(notes, "; "))
}

func printArtifact(w io.Writer, a *webapi.Artifact) {
	typ := strings.ToLower(strings.TrimPrefix(a.GetType().String(), "ARTIFACT_TYPE_"))
	entry := a.GetEntry()
	if entry == "" {
		entry = "-"
	}
	fmt.Fprintf(w, "%s\t%s\tentry %s\t%d files\tcontent %s\n", a.GetName(), typ, entry, len(a.GetFiles()), artifactDigest(a))
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
	b, err := os.ReadFile(abs)
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
// have and makes the write again.
func retryUpload[T any](ctx context.Context, c *designsClient, have fileSet, write func() (T, error)) (T, error) {
	out, err := write()
	var ce *connect.Error
	if err == nil || !errors.As(err, &ce) {
		return out, err
	}
	var missing []string
	for _, d := range ce.Details() {
		if v, derr := d.Value(); derr == nil {
			if bm, ok := v.(*webapi.BlobsMissing); ok {
				missing = append(missing, bm.GetSha256()...)
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
	return write()
}

// pushPlan is the artifacts a folder's design becomes, by id.
type pushPlan struct {
	ids   []string
	files map[string]fileSet
	all   fileSet
}

// planPush groups a folder's files into artifacts. It proposes the folder's design as agni propose
// does, ignoring any design.yaml or project.yaml. Each companion, with the files under it when it
// is a folder such as an ODB++ job, becomes an artifact named for its type, and the entry's
// artifact holds every other file the design reads.
func planPush(dir string) (pushPlan, []string, error) {
	rels, err := walkLocal(dir)
	if err != nil {
		return pushPlan{}, nil, err
	}
	var files []string
	for _, r := range rels {
		if b := path.Base(r); b != service.DesignDescriptorName && b != service.ProjectDescriptorName {
			files = append(files, r)
		}
	}
	read := func(p string) ([]byte, error) { return os.ReadFile(filepath.Join(dir, filepath.FromSlash(p))) }
	resp := service.ProposeFromFiles("", files, read)
	switch n := len(resp.GetDesigns()); {
	case n == 0:
		return pushPlan{}, nil, fmt.Errorf("%s holds no design agni reads", dir)
	case n > 1:
		var entries []string
		for _, d := range resp.GetDesigns() {
			entries = append(entries, d.GetDesign().GetEntryUri())
		}
		return pushPlan{}, nil, fmt.Errorf("%s holds %d designs, read from %s; push each from a folder of its own, or put artifacts one by one", dir, n, strings.Join(entries, ", "))
	}
	pd := resp.GetDesigns()[0]
	owner := map[string]string{}
	prefix := map[string]string{}
	taken := map[string]bool{}
	name := func(entry string) string {
		base := artifactWord(entry)
		id := base
		for i := 2; taken[id]; i++ {
			id = fmt.Sprintf("%s-%d", base, i)
		}
		taken[id] = true
		return id
	}
	plan := pushPlan{files: map[string]fileSet{}}
	entryID := name(pd.GetDesign().GetEntryUri())
	plan.ids = append(plan.ids, entryID)
	for _, c := range pd.GetDesign().GetCompanionUris() {
		id := name(c)
		plan.ids = append(plan.ids, id)
		// A companion keeps its path relative to its own folder, and an ODB++ job keeps its job
		// folder, since the reader finds the job by it.
		root := c
		if r := formats.DesignRoot(c); r != "" && r != "." {
			root = r
		}
		if d := path.Dir(root); d != "." {
			prefix[id] = d + "/"
		}
		for _, f := range pd.GetFiles() {
			if f == root || strings.HasPrefix(f, root+"/") {
				owner[f] = id
			}
		}
	}
	for _, f := range pd.GetFiles() {
		id, ok := owner[f]
		if !ok {
			id = entryID
		}
		rel := strings.TrimPrefix(f, prefix[id])
		abs := filepath.Join(dir, filepath.FromSlash(f))
		set := plan.files[id]
		if err := set.add(abs, rel); err != nil {
			return pushPlan{}, nil, err
		}
		plan.files[id] = set
		if err := plan.all.add(abs, rel); err != nil {
			return pushPlan{}, nil, err
		}
	}
	var unread []string
	for _, u := range resp.GetUnread() {
		unread = append(unread, u.GetPath())
	}
	return plan, unread, nil
}

// artifactWord names an artifact by what its entry holds: netlist, schematic, board or files.
func artifactWord(entry string) string {
	switch t := projects.ArtifactTypeOf(entry); t {
	case webapi.ArtifactType_ARTIFACT_TYPE_OTHER, webapi.ArtifactType_ARTIFACT_TYPE_UNSPECIFIED:
		return "files"
	default:
		return strings.ToLower(strings.TrimPrefix(t.String(), "ARTIFACT_TYPE_"))
	}
}

// push keeps a folder's design on the server.
func (c *designsClient) push(ctx context.Context, w io.Writer, dir, id, title, series string) error {
	plan, unread, err := planPush(dir)
	if err != nil {
		return err
	}
	if id == "" || title == "" || series == "" {
		declared := readLocalDescriptor(filepath.Join(dir, service.DesignDescriptorName))
		if id == "" {
			id = declared.Name
		}
		if title == "" {
			title = declared.Title
		}
		if series == "" {
			series = declared.Series
		}
	}
	if id == "" {
		id = strings.ToLower(filepath.Base(filepath.Clean(dir)))
	}
	name := designName(id)
	if _, err := unary(ctx, c.designs.GetDesign, &webapi.GetDesignRequest{Name: name}); connect.CodeOf(err) == connect.CodeNotFound {
		if _, err := unary(ctx, c.designs.CreateDesign, &webapi.CreateDesignRequest{DesignId: designID(id), Design: &webapi.Design{Title: title, Series: series}}); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if title != "" || series != "" {
		var paths []string
		if title != "" {
			paths = append(paths, "title")
		}
		if series != "" {
			paths = append(paths, "series")
		}
		if _, err := unary(ctx, c.designs.UpdateDesign, &webapi.UpdateDesignRequest{Design: &webapi.Design{Name: name, Title: title, Series: series}, UpdateMask: &fieldmaskpb.FieldMask{Paths: paths}}); err != nil {
			return err
		}
	}
	req := &webapi.BatchUpdateArtifactsRequest{Parent: name}
	for _, a := range plan.ids {
		req.Requests = append(req.Requests, &webapi.UpdateArtifactRequest{Artifact: plan.files[a].artifact(service.ArtifactName(designID(id), a)), AllowMissing: true})
	}
	resp, err := retryUpload(ctx, c, plan.all, func() (*webapi.BatchUpdateArtifactsResponse, error) {
		return unary(ctx, c.designs.BatchUpdateArtifacts, req)
	})
	if err != nil {
		return err
	}
	d, err := unary(ctx, c.designs.GetDesign, &webapi.GetDesignRequest{Name: name})
	if err != nil {
		return err
	}
	printDesign(w, d)
	for _, a := range resp.GetArtifacts() {
		printArtifact(w, a)
	}
	if len(unread) > 0 {
		fmt.Fprintf(w, "not sent, as no reader opens them: %s\n", strings.Join(unread, ", "))
	}
	return nil
}

// localDescriptor is the part of a local design.yaml push reads for defaults. It reads loosely, as
// ProposeDesigns does, since push sends no descriptor and internal/projects checks one strictly where
// one is read as a design.
type localDescriptor struct {
	Name   string `yaml:"name"`
	Title  string `yaml:"title"`
	Series string `yaml:"series"`
}

// readLocalDescriptor reads a design.yaml's name, title and series. A missing or unreadable one gives
// no defaults.
func readLocalDescriptor(p string) localDescriptor {
	var d localDescriptor
	if raw, err := os.ReadFile(p); err == nil {
		_ = yaml.Unmarshal(raw, &d)
	}
	return d
}
