package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/panyam/agni"
	"github.com/panyam/agni/fshost"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
	_ "github.com/panyam/agni/stdlib/relations"     // the fact base every datalog rule reads
	_ "github.com/panyam/agni/stdlib/rules/builtin" // the built-in rule catalog a report runs
)

// tour holds the services a server answers designs, projects and packs with, over a store in a
// temporary folder and the bundled demo project mounted read-only. uploaded is the server's upload
// store, the contents a client sent with UploadBlobs, by hash.
type tour struct {
	designs  *service.DesignService
	projects *service.ProjectService
	packs    *service.PackService
	checks   *service.CheckService
	uploaded map[string][]byte
	root     string
}

// newTour builds the services the way `agni serve --store --uploads` does (cmd/agni/storeservices.go).
func newTour(root, mounted string) (*tour, error) {
	store, err := projects.NewDirStore(filepath.Join(root, "designs"), filepath.Join(root, "uploads"), "uploads")
	if err != nil {
		return nil, err
	}
	records, err := projects.NewDirProjects(filepath.Join(root, "projects"))
	if err != nil {
		return nil, err
	}
	tree := projects.Tree{Mount: "fixtures", FS: os.DirFS(mounted)}
	t := &tour{uploaded: map[string][]byte{}, root: root}
	t.projects = service.NewProjectService(projects.NewFSStore(tree)).WithRecords(records)
	catalog := service.NewDesignCatalog(store, projects.NewMountedDesigns(tree))
	t.designs = service.NewDesignService(catalog, t.blob).WithProjects(t.projects)
	packs, err := projects.NewDirPacks(filepath.Join(root, "packs"), store.Blobs(), projects.ViewsRoot(filepath.Join(root, "uploads")), "uploads")
	if err != nil {
		return nil, err
	}
	t.packs = service.NewPackService(packs, t.blob)
	// A report runs a check, so the tour serves the uploads mount through fshost, the way the lexicon
	// example serves a folder, and checks a stored design by name under its packs (agni issue 1704).
	reports, err := projects.NewDirReports(filepath.Join(root, "reports"))
	if err != nil {
		return nil, err
	}
	eng, err := agni.New()
	if err != nil {
		return nil, err
	}
	up := fshost.Mount{Name: "uploads", FS: os.DirFS(projects.ViewsRoot(filepath.Join(root, "uploads")))}
	host := fshost.New(up)
	resolver := &service.ProjectResolver{
		Store:   projects.NewFSStore(projects.Tree{Mount: up.Name, FS: up.FS}),
		Config:  &fshost.ConfigResolver{Mounts: []fshost.Mount{up}},
		Designs: catalog,
		Packs:   t.packs,
	}
	t.checks = service.NewCheckService(host, eng.Catalog(), nil, "", host, resolver)
	t.designs.WithReports(reports, t.checks, t.packs)
	return t, nil
}

func (t *tour) blob(_ context.Context, h string) ([]byte, bool) {
	b, ok := t.uploaded[h]
	return b, ok
}

// manifest is what a client sends first, each file's path, hash and size, and the contents by hash
// for when the server asks for them.
func manifest(files map[string][]byte) ([]*webapi.SnapshotFile, map[string][]byte) {
	var out []*webapi.SnapshotFile
	contents := map[string][]byte{}
	for p, b := range files {
		sum := sha256.Sum256(b)
		h := "sha256:" + hex.EncodeToString(sum[:])
		out = append(out, &webapi.SnapshotFile{Path: p, Sha256: h, Size: int64(len(b))})
		contents[h] = b
	}
	return out, contents
}

// keep creates a design and puts its netlist artifact the way a client does: it sends the artifact's
// manifest, sends the contents the server says it lacks, and asks again. It returns how many contents
// were asked for.
func (t *tour) keep(ctx context.Context, id, title, netlist string) (*webapi.Design, int, error) {
	b, err := os.ReadFile(netlist) // #nosec G304 -- a bundled fixture the example names
	if err != nil {
		return nil, 0, err
	}
	if _, err := t.designs.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: id, Design: &webapi.Design{Title: title, Series: "mixer"}}); err != nil {
		return nil, 0, err
	}
	files, contents := manifest(map[string][]byte{filepath.Base(netlist): b})
	req := &webapi.UpdateArtifactRequest{Artifact: &webapi.Artifact{Name: service.ArtifactName(id, "netlist"), Files: files}, AllowMissing: true}
	asked := 0
	_, err = t.designs.UpdateArtifact(ctx, req)
	var bm *service.BlobsMissingError
	if errors.As(err, &bm) {
		for _, h := range bm.Hashes {
			t.uploaded[h] = contents[h]
		}
		asked = len(bm.Hashes)
		_, err = t.designs.UpdateArtifact(ctx, req)
	}
	if err != nil {
		return nil, asked, err
	}
	d, err := t.designs.GetDesign(ctx, &webapi.GetDesignRequest{Name: service.StandaloneDesignName(id)})
	return d, asked, err
}

// houseRules is a pack's convention: the house names a rail by its voltage after an underscore.
func houseRules(pattern string) *webapi.AnalysisConfig {
	return &webapi.AnalysisConfig{Conventions: &configpb.NamingConvention{Name: "house", Lexicon: &configpb.NamingLexicon{
		Net: &configpb.NetNameVocab{Rail: &configpb.VocabPatterns{Patterns: []string{pattern}}}}}}
}

// keepPack keeps the house rules as packs/house, then changes its rail pattern, and returns the pack
// before and after (agni issue 1704).
func (t *tour) keepPack(ctx context.Context) (before, after *webapi.Pack, err error) {
	if before, err = t.packs.CreatePack(ctx, &webapi.CreatePackRequest{PackId: "house", Pack: &webapi.Pack{Title: "House rules", Config: houseRules("_[0-9]+V[0-9]$")}}); err != nil {
		return nil, nil, err
	}
	after, err = t.packs.UpdatePack(ctx, &webapi.UpdatePackRequest{Pack: &webapi.Pack{Name: "packs/house", Config: houseRules("^(VCC|VDD)_")}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"config"}}})
	return before, after, err
}

// report checks mixer-rev-b under packs/house and keeps the run, then replaces its netlist with rev
// A's and reads the report again (agni issue 1704). It returns the report as kept and as read after.
func (t *tour) report(ctx context.Context) (kept, after *webapi.Report, err error) {
	kept, err = t.designs.CreateReport(ctx, &webapi.CreateReportRequest{Parent: "designs/mixer-rev-b", Report: &webapi.Report{Packs: []string{"packs/house"}}})
	if err != nil {
		return nil, nil, err
	}
	b, err := os.ReadFile("../common/designs/rev-a.edn")
	if err != nil {
		return nil, nil, err
	}
	files, contents := manifest(map[string][]byte{"rev-b.edn": b})
	for h, c := range contents {
		t.uploaded[h] = c
	}
	if _, err := t.designs.UpdateArtifact(ctx, &webapi.UpdateArtifactRequest{Artifact: &webapi.Artifact{Name: service.ArtifactName("mixer-rev-b", "netlist"), Files: files}}); err != nil {
		return nil, nil, err
	}
	after, err = t.designs.GetReport(ctx, &webapi.GetReportRequest{Name: kept.GetName()})
	return kept, after, err
}
