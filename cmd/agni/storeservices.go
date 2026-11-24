package main

import (
	"context"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"strings"
	"time"

	"github.com/panyam/goapplib/storage/blobs"
	"github.com/panyam/goapplib/storage/mounts"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/internal/storeurl"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
)

// sweepGrace is how old an unnamed blob or view must be before a sweep removes it. It spares a file
// a client uploaded a moment ago and has not yet named in a write.
const sweepGrace = time.Hour

// sweepInterval is how often a running server sweeps --uploads (agni issue 1742). A test shortens it.
var sweepInterval = time.Hour

// storeServices builds the project, design and pack services `agni serve` answers with (agni issues
// 1694, 1704, 1732, 1736, 1739). Its designs are the records it keeps: with --store, the designs and
// projects its callers create or add from a mounted folder, as records in st. With --uploads it keeps their
// files too, by hash under uploadsDir, and serves each design's view as the mount "uploads". Uploaded
// blobs go to the same folder, so they outlive a restart, and a sweep at startup and every hour
// after removes what nothing names. The sweep stops when ctx ends. With --store it keeps packs too, and a pack's files go to --uploads beside the designs'.
// With --store and checks, it keeps each design's reports, each run by checks. spawn starts each
// goroutine it leaves running, so the caller can wait for them once ctx ends (agni issue 1869).
func storeServices(ctx context.Context, spawn func(func()), ms *mountset.Table, st *storeurl.Store, uploadsDir string, upBlobs blobs.Linker, projectStore service.ProjectStore, resolver *service.ProjectResolver, checks *service.CheckService, guard *service.AccessGuard, log io.Writer) (*service.ProjectService, *service.DesignService, *service.PackService, error) {
	projectSvc := service.NewProjectService(projectStore)
	var store service.DesignStore
	var packStore service.PackStore
	var reportStore service.ReportStore
	if st != nil {
		recs := map[string]records.Store{}
		for _, name := range []string{storeDesignsDir, storeProjectsDir, storePacksDir, storeReportsDir, storeSnapshotsDir} {
			r, err := storeRecords(st, name)
			if err != nil {
				return nil, nil, nil, err
			}
			recs[name] = r
		}
		var bd *projects.BlobDir
		if upBlobs != nil {
			bd = projects.NewBlobDirOver(upBlobs)
		}
		dirStore, err := projects.NewStoreOver(recs[storeDesignsDir], uploadsDir, uploadsMount, bd)
		if err != nil {
			return nil, nil, nil, err
		}
		projectRecs := projects.NewProjects(recs[storeProjectsDir])
		packs := projects.NewPacks(recs[storePacksDir], dirStore.Blobs(), projects.ViewsRoot(uploadsDir), uploadsMount)
		reports := projects.NewReports(recs[storeReportsDir])
		// A view the uploads folder lacks, as after a bucket --uploads lost its local copy, is built
		// again from the blobs (agni issue 1730).
		for name, rebuild := range map[string]func(context.Context) (int, error){"design": dirStore.RebuildViews, "pack": packs.RebuildViews} {
			n, err := rebuild(ctx)
			if err != nil {
				return nil, nil, nil, err
			}
			if n > 0 {
				fmt.Fprintf(log, "note: rebuilt %d %s views from the blobs in --uploads\n", n, name)
			}
		}
		snapshots := &osSnapshotStore{recs: recs[storeSnapshotsDir]}
		// A revision a report ran on stays while the report does, so the report keeps naming files the
		// server holds (agni issue 1774).
		dirStore.WithRevisionPins(func(ctx context.Context, id string) (map[string]bool, error) {
			kept, err := reports.Reports(ctx, id)
			if err != nil {
				return nil, err
			}
			pinned := map[string]bool{}
			for _, r := range kept {
				if r.GetRevision() != "" {
					pinned[r.GetRevision()] = true
				}
			}
			return pinned, nil
		})
		store, packStore, reportStore = dirStore, packs, reports
		// A Datastore store can strand a large record's parts when a server dies mid-write, and sweeps
		// them as the uploads are swept (agni issue 1823).
		if sw, ok := st.Records.(records.Sweeper); ok {
			spawn(func() { sweepRecords(ctx, sw, log) })
		}
		projectSvc.WithRecords(projectRecs)
		if blobs := dirStore.Blobs(); blobs != nil && resolver != nil {
			resolver.Blobs = blobs
			// The sweep runs at startup and then every sweepInterval while the server runs, so a
			// server up for weeks doesn't keep every replaced artifact's files (agni issue 1742).
			sweep := func() error {
				return sweepUploads(ms, dirStore, packs, snapshots, log)
			}
			if err := sweep(); err != nil {
				return nil, nil, nil, err
			}
			spawn(func() {
				tick := time.NewTicker(sweepInterval)
				defer tick.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-tick.C:
						if err := sweep(); err != nil {
							fmt.Fprintf(log, "note: %v\n", err)
						}
					}
				}
			})
		}
	}
	// The designs are the store's records alone. A folder in a mount becomes a design only when a
	// person adds it (AddDesigns), so its design.yaml makes none (agni issue 1736).
	catalog := service.NewDesignCatalog(store)
	// A link records the private source it read with or without --auth (agni issue 1778).
	if ms != nil {
		catalog.WithSources(ms)
	}
	catalog.WithGuard(guard)
	if resolver != nil {
		// A snapshot may name its design, designs/{d}, and Ingest looks it up here (agni issue 1695).
		resolver.Designs = catalog
	}
	// A linked pack read from a private source opens only for who can read it (agni issue 1801).
	packSvc := service.NewPackService(packStore, resolver.Blob).WithImporter(notingImporter(log)).WithGuard(guard)
	if resolver != nil {
		// A snapshot may name the packs it is read under, packs/{k} (agni issue 1704).
		resolver.Packs = packSvc
	}
	designSvc := service.NewDesignService(catalog, resolver.Blob).WithProjects(projectSvc)
	// A mounted project lists the stored designs read from its folder (agni issue 1756).
	projectSvc.WithSourcedDesigns(designSvc.SourcedFrom)
	if reportStore != nil && checks != nil {
		// A report runs a check under packs and keeps it under its design (agni issue 1704).
		designSvc.WithReports(reportStore, checks, packSvc)
	}
	// AddDesigns proposes a mounted folder's designs and reads their files (agni issue 1736).
	designSvc.WithMountFiles(service.NewWorkspaceService(osWorkspace(ms)).WithDesignFiles(resolver, osFiles(ms)), osFiles(ms))
	if packStore != nil && resolver != nil && resolver.Blobs != nil {
		// An added design's project becomes a pack it reads under, as `agni designs push` keeps one
		// (agni issue 1756).
		designSvc.WithProjectPacks(projectPacker(ms, packSvc, resolver.Blobs))
	}
	return projectSvc, designSvc, packSvc, nil
}

// sweepRecords sweeps a store's stranded records now and every sweepInterval until ctx ends.
func sweepRecords(ctx context.Context, sw records.Sweeper, log io.Writer) {
	sweep := func() {
		if n, err := sw.Sweep(ctx, sweepGrace); err != nil {
			fmt.Fprintf(log, "note: sweeping --store: %v\n", err)
		} else if n > 0 {
			fmt.Fprintf(log, "note: swept %d stranded parts of large records from --store\n", n)
		}
	}
	sweep()
	tick := time.NewTicker(sweepInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			sweep()
		}
	}
}

// storeRecords is the records of one kind under --store, under the folder name. A file:// store
// makes that folder now, so a path there that is no folder fails at startup. A sqlite store keeps
// them as rows under the name.
func storeRecords(store *storeurl.Store, name string) (records.Store, error) {
	if store.Dir == "" {
		return store.Sub(name), nil
	}
	d, err := records.NewDir(filepath.Join(store.Dir, name))
	if err != nil {
		return nil, fmt.Errorf("--store: %w", err)
	}
	return d, nil
}

// sweepUploads removes the design views, pack views and blobs in --uploads that nothing names and
// that are older than sweepGrace. A blob is named by a design's record, a stored snapshot, a pack,
// or a runtime mount's snapshot manifest through the mounts registry. It logs what it removed.
func sweepUploads(ms *mountset.Table, dirStore *projects.DirStore, packs *projects.DirPacks, snapshots *osSnapshotStore, log io.Writer) error {
	ctx := context.Background()
	keep, err := snapshots.Hashes(ctx)
	if err != nil {
		return fmt.Errorf("sweeping --uploads: %w", err)
	}
	packHashes, err := packs.Hashes(ctx)
	if err != nil {
		return fmt.Errorf("sweeping --uploads: %w", err)
	}
	maps.Copy(keep, packHashes)
	var more []blobs.Referrer
	if ms != nil && ms.Blobs() != nil {
		// A runtime mount's snapshot is files in the blobs too (agni issues 1735, 1759).
		more = append(more, ms.Registry().BlobRefs(ms.Blobs()))
	}
	packViews, err := packs.Sweep(ctx, sweepGrace)
	if err != nil {
		return fmt.Errorf("sweeping --uploads: %w", err)
	}
	views, removed, err := dirStore.Sweep(keep, sweepGrace, more...)
	if err != nil {
		return fmt.Errorf("sweeping --uploads: %w", err)
	}
	if views+packViews+removed > 0 {
		fmt.Fprintf(log, "note: swept %d old design views, %d old pack views and %d files nothing names from --uploads\n", views, packViews, removed)
	}
	return nil
}

// projectPacker keeps a mounted project.yaml as a pack, as `agni designs push` keeps the project a
// folder's design.yaml extends (agni issue 1756). It reads the project through its host path with
// localPack, puts the files' contents in the uploads, and imports the pack. A pack the server holds
// with the same hash is left as it is.
func projectPacker(ms *mountset.Table, packs *service.PackService, blobs service.UploadStore) service.ProjectPacker {
	return func(ctx context.Context, project mounts.URI) (string, error) {
		host, err := ms.Resolve(project)
		if err != nil {
			return "", err
		}
		pack, files, _, err := localPack(host, "", "")
		if err != nil {
			return "", fmt.Errorf("%w: %v", service.ErrInvalidArgument, err)
		}
		if have, err := packs.GetPack(ctx, &webapi.GetPackRequest{Name: pack.GetName()}); err == nil && have.GetHash() == service.PackHash(pack) {
			return pack.GetName(), nil
		}
		for _, b := range files.byHash {
			if _, err := blobs.Put(b); err != nil {
				return "", err
			}
		}
		out, err := packs.ImportPackFiles(ctx, &webapi.ImportPackFilesRequest{PackId: strings.TrimPrefix(pack.GetName(), "packs/"), Title: pack.GetTitle(), Files: files.files})
		if err != nil {
			return "", err
		}
		return out.GetName(), nil
	}
}
