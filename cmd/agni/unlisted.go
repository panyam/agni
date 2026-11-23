package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
)

// unlistedOut is where the CLI writes a note about a tier folder a descriptor doesn't list, as
// noteSource writes its notes (agni issue 1861).
var unlistedOut io.Writer = os.Stderr

// unlistedSeen holds the descriptors this process has noted, since the CLI opens a fresh project
// store for each read and would otherwise note one project on every read.
var unlistedSeen sync.Map

// noteUnlisted writes u's note once per process, naming the descriptor by its path on this machine
// when one of the run's mounts holds it.
func noteUnlisted(u projects.Unlisted) {
	if _, seen := unlistedSeen.LoadOrStore(u.Descriptor.String(), true); seen {
		return
	}
	where := u.Descriptor.String()
	if ws, err := workspace(); err == nil {
		for _, m := range ws.Mounts() {
			if m.Name == u.Descriptor.Mount {
				where = filepath.Join(m.Source, filepath.FromSlash(u.Descriptor.Path))
			}
		}
	}
	fmt.Fprintln(unlistedOut, u.Note(where))
}

// localUnlisted is the note for the project.yaml at descriptor on this machine, "" when it lists
// every tier folder beside it. packs push prints it, since a pack leaves out what project.yaml
// leaves out.
func localUnlisted(descriptor string) string {
	dir := filepath.Dir(descriptor)
	rels, err := walkLocal(dir)
	if err != nil {
		return ""
	}
	tiers, err := projects.UnlistedInFiles(rels, func(rel string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel))) // #nosec G304 -- a file in the project folder the user named
	})
	if err != nil || len(tiers) == 0 {
		return ""
	}
	return projects.Unlisted{Tiers: tiers}.Note(descriptor)
}

// notingImporter is projects.PackFromFiles, writing to log the note for a project.yaml that leaves
// out a tier folder beside it. A pack linked by --bootstrap or pushed by a client reads through it,
// so the server's log names the folder.
func notingImporter(log io.Writer) func([]string, func(string) ([]byte, error)) (string, *webapi.Pack, []string, error) {
	return func(files []string, read func(string) ([]byte, error)) (string, *webapi.Pack, []string, error) {
		id, pack, kept, err := projects.PackFromFiles(files, read)
		if err != nil || log == nil {
			return id, pack, kept, err
		}
		if tiers, terr := projects.UnlistedInFiles(files, read); terr == nil && len(tiers) > 0 {
			fmt.Fprintln(log, projects.Unlisted{Tiers: tiers}.Note("the project.yaml of packs/"+id))
		}
		return id, pack, kept, err
	}
}
