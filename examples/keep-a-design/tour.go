package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
)

// tour holds the two services a server answers designs and projects with, over a store in a
// temporary folder and the bundled demo project mounted read-only. uploaded is the server's upload
// store, the contents a client sent with UploadBlobs, by hash.
type tour struct {
	designs  *service.DesignService
	projects *service.ProjectService
	uploaded map[string][]byte
	root     string
}

// newTour builds the services the way `agni serve --store-dir` does (cmd/agni/storeservices.go).
func newTour(root, mounted string) (*tour, error) {
	store, err := projects.NewDirStore(filepath.Join(root, "designs"), "store")
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
