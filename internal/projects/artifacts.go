package projects

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/service"
)

// artifactRole is what one artifact holds, found by reading its files (agni issue 1696). entry and
// companions are relative to the artifact.
type artifactRole struct {
	id         string
	typ        webapi.ArtifactType
	entry      string
	companions []string
}

// classifyArtifact finds an artifact's entry, companions and type. It groups the files with the same
// rules ProposeDesigns uses on a dropped folder, so a KiCad root and its sheets read as one design and
// an ODB++ job as another. Files making two designs are an error, since the artifact would hold two
// sources. Files making none hold no design, unless there is one file a reader opens, such as a lone
// `.kicad_pcb`, which the proposer leaves to its schematic.
func classifyArtifact(id string, files []string, read func(string) ([]byte, error)) (artifactRole, error) {
	resp := service.ProposeFromFiles("", files, read)
	role := artifactRole{id: id, typ: webapi.ArtifactType_ARTIFACT_TYPE_OTHER}
	switch len(resp.GetDesigns()) {
	case 0:
		var readable []string
		for _, f := range files {
			if formats.ByExt(f) != nil {
				readable = append(readable, f)
			}
		}
		if len(readable) != 1 {
			return role, nil
		}
		role.entry = readable[0]
	case 1:
		d := resp.GetDesigns()[0].GetDesign()
		role.entry = d.GetEntryUri()
		role.companions = d.GetCompanionUris()
	default:
		var entries []string
		for _, d := range resp.GetDesigns() {
			entries = append(entries, d.GetDesign().GetEntryUri())
		}
		return role, fmt.Errorf("%w: artifact %s holds %d designs, read from %s; put each in an artifact of its own", service.ErrInvalidArgument, id, len(entries), strings.Join(entries, ", "))
	}
	role.typ = typeOf(role.entry)
	return role, nil
}

// ArtifactTypeOf names what an artifact entered at entry holds, as the store types it. The CLI's
// push names artifacts by it.
func ArtifactTypeOf(entry string) webapi.ArtifactType { return typeOf(entry) }

// typeOf names what an entry holds. A drawn schematic is a schematic even when it also nets, a board
// is a board even when it carries a netlist, and a netlist is the rest that reads.
func typeOf(entry string) webapi.ArtifactType {
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

// composeDesign picks the file analysis reads from a design's artifacts, and names the rest as its
// views (CONSTRAINTS C21). A netlist wins, then a schematic, then a board. Two artifacts of the
// winning type are an error, and so are two boards, since a design has one copper tier. Paths come
// back relative to the design, "{artifact}/{path}". No artifact a reader opens gives an empty entry.
func composeDesign(roles []artifactRole) (entry string, companions []string, err error) {
	byType := map[webapi.ArtifactType][]artifactRole{}
	for _, r := range roles {
		byType[r.typ] = append(byType[r.typ], r)
	}
	var source *artifactRole
	for _, t := range []webapi.ArtifactType{webapi.ArtifactType_ARTIFACT_TYPE_NETLIST, webapi.ArtifactType_ARTIFACT_TYPE_SCHEMATIC, webapi.ArtifactType_ARTIFACT_TYPE_BOARD} {
		rs := byType[t]
		if len(rs) > 1 {
			return "", nil, fmt.Errorf("%w: artifacts %s are each a %s, and a design reads one; delete one, or keep it in another design", service.ErrFailedPrecondition, ids(rs), typeWord(t))
		}
		if len(rs) == 1 {
			source = &rs[0]
			break
		}
	}
	if source == nil {
		return "", nil, nil
	}
	in := func(r artifactRole, p string) string { return path.Join(r.id, p) }
	entry = in(*source, source.entry)
	for _, c := range source.companions {
		companions = append(companions, in(*source, c))
	}
	for _, r := range roles {
		if r.id == source.id || r.entry == "" {
			continue
		}
		companions = append(companions, in(r, r.entry))
		for _, c := range r.companions {
			companions = append(companions, in(r, c))
		}
	}
	var boards []string
	for _, f := range append([]string{entry}, companions...) {
		if typeOf(f) == webapi.ArtifactType_ARTIFACT_TYPE_BOARD {
			boards = append(boards, f)
		}
	}
	if len(boards) > 1 {
		return "", nil, fmt.Errorf("%w: the design would hold two boards, %s, and a design has one", service.ErrFailedPrecondition, strings.Join(boards, " and "))
	}
	slices.Sort(companions)
	return entry, companions, nil
}

func ids(rs []artifactRole) string {
	var out []string
	for _, r := range rs {
		out = append(out, r.id)
	}
	return strings.Join(out, " and ")
}

func typeWord(t webapi.ArtifactType) string {
	return strings.ToLower(strings.TrimPrefix(t.String(), "ARTIFACT_TYPE_"))
}
