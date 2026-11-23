package projects

import (
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// ChecklistDir is a checklist a descriptor declares as a folder of item files, with items_dir in
// place of areas (agni issue 1812). Index is the checklist's place in the config's Checklists, and
// Dir is the folder relative to the descriptor.
type ChecklistDir struct {
	Index int
	Name  string
	Dir   string
}

// ResolveChecklistDirs reads each folder checklist src declares into cfg's Checklists, through
// review.ItemsDir, which says how a folder of item files reads (agni issue 1812). list returns every
// file under a folder, as slash-separated paths relative to the descriptor's folder, and read returns
// one file's contents. It returns every path it read, files and folders, so a cache can watch them.
func ResolveChecklistDirs(cfg *webapi.AnalysisConfig, dirs []ChecklistDir, list func(dir string) ([]string, error), read func(string) ([]byte, error)) ([]string, error) {
	var deps []string
	for _, d := range dirs {
		man, got, err := review.ItemsDir(d.Name, cfg.Checklists[d.Index].GetManifest().GetName(), d.Dir, list, read)
		if err != nil {
			return nil, err
		}
		cfg.Checklists[d.Index].Manifest = service.ManifestProto(man)
		deps = append(deps, got...)
	}
	return deps, nil
}
