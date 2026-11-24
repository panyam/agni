package projects

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/panyam/goapplib/storage/mounts"
)

// tierFolders are the folder names a descriptor's file tiers were found by before agni 1136, in the
// order a note lists them.
var tierFolders = []string{"profiles", "params", "symbols", "lib", "queries"}

// Unlisted is a descriptor that sits beside folders named like its file tiers and lists none of
// them (agni issue 1861). Releases before agni 1136 found a profiles/ or params/ folder by its name,
// so a project written for them still loads, and runs without those tiers and with no error.
type Unlisted struct {
	// Descriptor is the project.yaml or design.yaml, as a mount:// URI, zero for a project.yaml read
	// from files. A caller that knows where the mount sits on disk names it by its path instead.
	Descriptor mounts.URI
	// Tiers are the folder names, in tierFolders order.
	Tiers []string
}

// Note is the line a surface prints for u, naming the descriptor as where.
func (u Unlisted) Note(where string) string {
	folders := make([]string, len(u.Tiers))
	lines := make([]string, len(u.Tiers))
	for i, t := range u.Tiers {
		folders[i] = t + "/"
		lines[i] = fmt.Sprintf("%q", t+": ["+t+"]")
	}
	file := ProjectDescriptor
	if u.Descriptor.Path != "" {
		file = path.Base(u.Descriptor.Path)
	}
	if len(u.Tiers) == 1 {
		return fmt.Sprintf("note: %s sits beside %s, and %s doesn't list it, so this run leaves it out; add %s to %s to read it", folders[0], where, file, lines[0], file)
	}
	return fmt.Sprintf("note: %s sit beside %s, and %s lists none of them, so this run leaves them out; add %s to %s to read them", andList(folders), where, file, andList(lines), file)
}

// andList joins items as "a, b and c".
func andList(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

// unlistedTiers returns the tier folders isDir finds beside a descriptor whose sources name none of
// them. A folder counts as listed when any tier names it or a path under it, so `symbols:
// [lib/symbols]` lists lib/. A mount:// source names another place and lists nothing here.
func unlistedTiers(src Sources, isDir func(name string) bool) []string {
	var refs []string
	for _, tier := range [][]string{src.Profiles, src.Params, src.Symbols, src.Lib, src.Queries} {
		for _, r := range tier {
			if !strings.HasPrefix(strings.TrimSpace(r), mounts.Scheme) {
				refs = append(refs, CleanRel(r))
			}
		}
	}
	var out []string
	for _, name := range tierFolders {
		listed := slices.ContainsFunc(refs, func(r string) bool { return r == name || strings.HasPrefix(r, name+"/") })
		if !listed && isDir(name) {
			out = append(out, name)
		}
	}
	return out
}

// UnlistedInFiles is UnlistedInFolder over a folder whose paths are all of files, relative and
// slash-separated, as PackFromFiles takes them.
func UnlistedInFiles(files []string, read func(string) ([]byte, error)) ([]string, error) {
	return UnlistedInFolder(ListFiles(files), read)
}

// UnlistedInFolder reads the project.yaml at the top of a project folder and returns the tier
// folders in it that project.yaml lists none of, for a pack read through a lister rather than a
// tree. list and read are PackFromFolder's. A folder with no project.yaml returns nil, since
// PackFromFolder refuses it on its own.
func UnlistedInFolder(list func(rel string) ([]string, error), read func(string) ([]byte, error)) ([]string, error) {
	top, err := list(ProjectDescriptor)
	if err != nil || !slices.Contains(top, ProjectDescriptor) {
		return nil, err
	}
	raw, err := read(ProjectDescriptor)
	if err != nil {
		return nil, err
	}
	_, _, src, err := ParseProject(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	var lerr error
	tiers := unlistedTiers(src, func(name string) bool {
		got, err := list(name)
		if err != nil {
			lerr = err
		}
		return slices.ContainsFunc(got, func(f string) bool { return strings.HasPrefix(f, name+"/") })
	})
	return tiers, lerr
}

// noteUnlisted tells the store's listener about the tier folders beside the descriptor in dir that
// src lists none of. It runs where the descriptor is parsed, so a cached descriptor notes once.
func (s *FSStore) noteUnlisted(t Tree, dir, descriptor string, src Sources) {
	if s.unlisted == nil {
		return
	}
	tiers := unlistedTiers(src, func(name string) bool {
		fi, err := fs.Stat(t.FS, path.Join(walkRoot(dir), name))
		return err == nil && fi.IsDir()
	})
	if len(tiers) == 0 {
		return
	}
	u, err := mounts.NewURI(t.Mount, descriptor)
	if err != nil {
		return
	}
	s.unlisted(Unlisted{Descriptor: u, Tiers: tiers})
}

// OnUnlisted makes s call fn for each descriptor it parses that sits beside a tier folder it doesn't
// list (agni issue 1861), and returns s. fn runs again only when the descriptor changes.
func (s *FSStore) OnUnlisted(fn func(Unlisted)) *FSStore {
	s.unlisted = fn
	return s
}
