package main

import (
	"path/filepath"

	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/service"
)

// storeServices builds the project and design services `agni serve` answers with (agni issue 1694).
// Every mounted folder's designs are read-only. With --store-dir, the server also keeps the designs
// and projects its callers create, designs in the mount "store" and project records beside it.
func storeServices(ms []mounts.Mount, storeDir string, projectStore service.ProjectStore, resolver *service.ProjectResolver) (*service.ProjectService, *service.DesignService, error) {
	var mountedTrees []projects.Tree
	for _, t := range projectTrees(ms) {
		if storeDir == "" || t.Mount != storeMount {
			mountedTrees = append(mountedTrees, t)
		}
	}
	projectSvc := service.NewProjectService(projectStore)
	var store service.DesignStore
	if storeDir != "" {
		abs, err := filepath.Abs(storeDir)
		if err != nil {
			return nil, nil, err
		}
		dirStore, err := projects.NewDirStore(filepath.Join(abs, storeDesignsDir), storeMount)
		if err != nil {
			return nil, nil, err
		}
		records, err := projects.NewDirProjects(filepath.Join(abs, storeProjectsDir))
		if err != nil {
			return nil, nil, err
		}
		store = dirStore
		projectSvc.WithRecords(records)
	}
	catalog := service.NewDesignCatalog(store, projects.NewMountedDesigns(mountedTrees...))
	return projectSvc, service.NewDesignService(catalog, resolver.Blob).WithProjects(projectSvc), nil
}
