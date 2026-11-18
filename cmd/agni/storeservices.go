package main

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/service"
)

// sweepGrace is how old an unnamed blob or view must be before the startup sweep removes it. It
// spares a file a client uploaded a moment before a restart and has not yet named in a write.
const sweepGrace = time.Hour

// storeServices builds the project, design and pack services `agni serve` answers with (agni issues
// 1694, 1704, 1732, 1739). Every mounted folder's designs are read-only. With --store, the server also keeps the
// designs and projects its callers create, as records under storeDir. With --uploads it keeps their
// files too, by hash under uploadsDir, and serves each design's view as the mount "uploads". Uploaded
// blobs go to the same folder, so they outlive a restart, and a sweep at startup removes what nothing
// names. With --store it keeps packs too, and a pack's files go to --uploads beside the designs'.
func storeServices(ms []mounts.Mount, storeDir, uploadsDir string, projectStore service.ProjectStore, resolver *service.ProjectResolver, log io.Writer) (*service.ProjectService, *service.DesignService, *service.PackService, error) {
	var mountedTrees []projects.Tree
	for _, t := range projectTrees(ms) {
		if uploadsDir == "" || t.Mount != uploadsMount {
			mountedTrees = append(mountedTrees, t)
		}
	}
	projectSvc := service.NewProjectService(projectStore)
	var store service.DesignStore
	var packStore service.PackStore
	if storeDir != "" {
		dirStore, err := projects.NewDirStore(filepath.Join(storeDir, storeDesignsDir), uploadsDir, uploadsMount)
		if err != nil {
			return nil, nil, nil, err
		}
		records, err := projects.NewDirProjects(filepath.Join(storeDir, storeProjectsDir))
		if err != nil {
			return nil, nil, nil, err
		}
		packs, err := projects.NewDirPacks(filepath.Join(storeDir, storePacksDir), dirStore.Blobs(), projects.ViewsRoot(uploadsDir), uploadsMount)
		if err != nil {
			return nil, nil, nil, err
		}
		store, packStore = dirStore, packs
		projectSvc.WithRecords(records)
		if blobs := dirStore.Blobs(); blobs != nil && resolver != nil {
			resolver.Blobs = blobs
			keep, err := snapshotHashes(filepath.Join(storeDir, storeSnapshotsDir))
			if err != nil {
				return nil, nil, nil, err
			}
			packHashes, err := packs.Hashes(context.Background())
			if err != nil {
				return nil, nil, nil, err
			}
			maps.Copy(keep, packHashes)
			views, removed, err := dirStore.Sweep(keep, sweepGrace)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("sweeping --uploads: %w", err)
			}
			if views+removed > 0 {
				fmt.Fprintf(log, "note: swept %d old design views and %d files nothing names from --uploads\n", views, removed)
			}
		}
	}
	catalog := service.NewDesignCatalog(store, projects.NewMountedDesigns(mountedTrees...))
	if resolver != nil {
		// A snapshot may name its design, designs/{d}, and Ingest looks it up here (agni issue 1695).
		resolver.Designs = catalog
	}
	packSvc := service.NewPackService(packStore, resolver.Blob)
	if resolver != nil {
		// A snapshot may name the packs it is read under, packs/{k} (agni issue 1704).
		resolver.Packs = packSvc
	}
	return projectSvc, service.NewDesignService(catalog, resolver.Blob).WithProjects(projectSvc), packSvc, nil
}

// snapshotHashes reads every stored snapshot definition in dir and returns the hashes of the files
// each one sent, so the sweep keeps them. A missing dir holds none.
func snapshotHashes(dir string) (map[string]bool, error) {
	out := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name())) // #nosec G304 -- a definition in the store's own folder
		if err != nil {
			return nil, err
		}
		def := &webapi.Snapshot{}
		if err := protojson.Unmarshal(b, def); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		for _, f := range def.GetFiles() {
			out[f.GetSha256()] = true
		}
	}
	return out, nil
}
