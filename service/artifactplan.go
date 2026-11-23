package service

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/formats"
)

// ArtifactTypeOf names what an artifact entered at entry holds (agni issue 1696). A drawn schematic is
// a schematic even when it also nets, a board is a board even when it carries a netlist, and a
// netlist is the rest that reads.
func ArtifactTypeOf(entry string) webapi.ArtifactType {
	switch {
	case formats.HasFaithful(entry):
		return webapi.ArtifactType_ARTIFACT_TYPE_SCHEMATIC
	case formats.HasBoard(entry):
		return webapi.ArtifactType_ARTIFACT_TYPE_BOARD
	case formats.HasNetlist(entry):
		return webapi.ArtifactType_ARTIFACT_TYPE_NETLIST
	}
	return webapi.ArtifactType_ARTIFACT_TYPE_OTHER
}

// PlannedFile is one file of a planned artifact: where it is in the folder, and its path inside the
// artifact.
type PlannedFile struct {
	Folder string
	Rel    string
}

// ArtifactPlan is how a folder's files become a design's artifacts.
type ArtifactPlan struct {
	// IDs orders the artifacts, the one analysis reads first.
	IDs []string
	// Files holds each artifact's files by id.
	Files map[string][]PlannedFile
	// Unread names the folder's files no reader opens.
	Unread []string
	// Declared says a design.yaml named the entry, so IDs[0] is the artifact analysis reads, as a
	// DesignRecord's Entry.
	Declared bool
}

// PlanArtifacts groups a folder's files into artifacts (agni issues 1696, 1719). It proposes the
// folder's design with ProposeFromFiles, leaving out any design.yaml or project.yaml. A top-level
// design.yaml picks the design its entry names, and adds the companions it declares that the
// proposals read apart, such as an EDIF schematic beside an ODB++ board (agni issue 1858). Each companion,
// with the files under it when it is a folder such as an ODB++ job, becomes an artifact named for its
// type, and keeps its paths relative to its own folder. The entry's artifact holds every other file
// the design reads, with paths relative to the folder, since a KiCad root sheet finds its child
// sheets by them. Files making no design, or more than one, are ErrInvalidArgument.
func PlanArtifacts(files []string, read func(string) ([]byte, error)) (ArtifactPlan, error) {
	var kept []string
	for _, f := range files {
		if b := path.Base(f); b != DesignDescriptorName && b != ProjectDescriptorName {
			kept = append(kept, f)
		}
	}
	resp := ProposeFromFiles("", kept, read)
	designs := resp.GetDesigns()
	var strays []string
	decl := declaredDesign(files, read)
	if decl.entry != "" && len(designs) > 1 {
		// A folder often holds sheets its root never places, or views of the design no file name
		// pairs, such as an ODB++ board and an EDIF schematic. Its design.yaml names the entry and
		// the companions, as the CLI reads it, so that design keeps those views and the others' files
		// are reported as not kept (agni issues 1719, 1858).
		if i := slices.IndexFunc(designs, func(d *webapi.ProposedDesign) bool { return namedBy(decl.entry, d.GetDesign().GetEntryUri()) }); i >= 0 {
			kept := proto.Clone(designs[i]).(*webapi.ProposedDesign)
			for j, o := range designs {
				if j == i {
					continue
				}
				e := o.GetDesign().GetEntryUri()
				if slices.ContainsFunc(decl.companions, func(c string) bool { return namedBy(c, e) }) {
					kept.Design.CompanionUris = append(kept.Design.CompanionUris, e)
					kept.Files = append(kept.Files, o.GetFiles()...)
					continue
				}
				strays = append(strays, o.GetFiles()...)
			}
			designs = []*webapi.ProposedDesign{kept}
		}
	}
	switch n := len(designs); {
	case n == 0:
		return ArtifactPlan{}, fmt.Errorf("%w: these files hold no design agni reads", ErrInvalidArgument)
	case n > 1:
		var entries []string
		for _, d := range designs {
			entries = append(entries, d.GetDesign().GetEntryUri())
		}
		return ArtifactPlan{}, fmt.Errorf("%w: these files hold %d designs, read from %s; send each design's files on their own, or put artifacts one by one", ErrInvalidArgument, n, strings.Join(entries, ", "))
	}
	pd := designs[0]
	plan := ArtifactPlan{Files: map[string][]PlannedFile{}, Declared: decl.entry != "" && namedBy(decl.entry, pd.GetDesign().GetEntryUri())}
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
	entryID := name(pd.GetDesign().GetEntryUri())
	plan.IDs = append(plan.IDs, entryID)
	owner := map[string]string{}
	prefix := map[string]string{}
	for _, c := range pd.GetDesign().GetCompanionUris() {
		id := name(c)
		plan.IDs = append(plan.IDs, id)
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
		plan.Files[id] = append(plan.Files[id], PlannedFile{Folder: f, Rel: strings.TrimPrefix(f, prefix[id])})
	}
	for _, u := range resp.GetUnread() {
		plan.Unread = append(plan.Unread, u.GetPath())
	}
	plan.Unread = append(plan.Unread, strays...)
	slices.Sort(plan.Unread)
	return plan, nil
}

// declared is what a top-level design.yaml names: its entry and its companions, as cleaned paths.
type declared struct {
	entry      string
	companions []string
}

// declaredDesign reads the entry and companions a top-level design.yaml among the files names,
// loosely, as ProposeDesigns reads a descriptor. None, or one that does not parse, gives no entry.
func declaredDesign(files []string, read func(string) ([]byte, error)) declared {
	if !slices.Contains(files, DesignDescriptorName) {
		return declared{}
	}
	raw, err := read(DesignDescriptorName)
	if err != nil {
		return declared{}
	}
	var d struct {
		Entry      string   `yaml:"entry"`
		Companions []string `yaml:"companions"`
	}
	if yaml.Unmarshal(raw, &d) != nil || d.Entry == "" {
		return declared{}
	}
	clean := func(p string) string { return path.Clean(strings.ReplaceAll(p, "\\", "/")) }
	out := declared{entry: clean(d.Entry)}
	for _, c := range d.Companions {
		out.companions = append(out.companions, clean(c))
	}
	return out
}

// namedBy says whether a path a descriptor declares names the design read from entry. An ODB++ job
// is declared by its folder or archive, and proposed by a file inside it.
func namedBy(declared, entry string) bool {
	return entry == declared || strings.HasPrefix(entry, declared+"/")
}

// artifactWord names an artifact by what its entry holds: netlist, schematic, board or files.
func artifactWord(entry string) string {
	switch t := ArtifactTypeOf(entry); t {
	case webapi.ArtifactType_ARTIFACT_TYPE_OTHER, webapi.ArtifactType_ARTIFACT_TYPE_UNSPECIFIED:
		return "files"
	default:
		return strings.ToLower(strings.TrimPrefix(t.String(), "ARTIFACT_TYPE_"))
	}
}
