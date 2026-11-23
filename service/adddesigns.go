package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/goapplib/storage/mounts"
	"gopkg.in/yaml.v3"
)

// DesignProposer proposes the designs a mounted folder's files make, as
// WorkspaceService.ProposeDesigns does.
type DesignProposer interface {
	ProposeDesigns(ctx context.Context, req *webapi.ProposeDesignsRequest) (*webapi.ProposeDesignsResponse, error)
	// ListDir lists one folder of a mount, which AddDesigns walks to copy a design's symbol folders.
	ListDir(ctx context.Context, req *webapi.ListDirRequest) (*webapi.ListDirResponse, error)
}

// ProjectPacker keeps the project a mounted design.yaml extends as a pack and returns its name,
// "packs/{pack}", as `agni designs push` does from a folder (agni issue 1756). project is the
// project.yaml's URI. A pack the server holds with the same hash is left as it is.
type ProjectPacker func(ctx context.Context, project mounts.URI) (string, error)

// WithProjectPacks lets AddDesigns keep each design's project as a pack and read the design under
// it, and returns s. Without it, an added design names no packs.
func (s *DesignService) WithProjectPacks(p ProjectPacker) *DesignService {
	s.packer = p
	return s
}

// WithMountFiles lets AddDesigns make designs from mounted folders, and returns s (agni issue 1736).
// propose groups a folder's files into designs, and read reads one file of a mount. Without it,
// AddDesigns answers FAILED_PRECONDITION.
func (s *DesignService) WithMountFiles(propose DesignProposer, read FileReader) *DesignService {
	s.proposer, s.mountRead = propose, read
	return s
}

// addedDesign is one design AddDesigns makes: its id and record, and the artifacts it writes with
// the contents read for them, by hash.
type addedDesign struct {
	id       string
	record   DesignRecord
	writes   []ArtifactWrite
	contents map[string][]byte
	// project is the project.yaml the design's descriptor extends, zero for none.
	project mounts.URI
	// uncopied are the symbol folders no_symbol_copy left out.
	uncopied []string
}

// AddDesigns makes stored designs from the proposals a mounted folder makes that the request names.
// Each proposal gets one result. Without allow_partial, one refused proposal fails the request and
// nothing is kept. A proposal whose design exists follows on_exists (agni issue 1860).
func (s *DesignService) AddDesigns(ctx context.Context, req *webapi.AddDesignsRequest) (*webapi.AddDesignsResponse, error) {
	if s.proposer == nil || s.mountRead == nil {
		return nil, fmt.Errorf("%w: this server reads no mounted folders into designs", ErrFailedPrecondition)
	}
	u, err := ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	if err := s.catalog.guard.URI(ctx, u); err != nil {
		return nil, err
	}
	if len(req.GetDesigns()) == 0 {
		return nil, fmt.Errorf("%w: name the proposals to add, each by its folder", ErrInvalidArgument)
	}
	proposed, err := s.proposer.ProposeDesigns(ctx, &webapi.ProposeDesignsRequest{Uri: req.GetUri()})
	if err != nil {
		return nil, err
	}
	resp := &webapi.AddDesignsResponse{}
	// planned is a proposal that passed planning, with its result and whether it updates a design.
	type planned struct {
		add    addedDesign
		result *webapi.AddResult
		update bool
	}
	var plans []planned
	ids := map[string]bool{}
	for _, want := range req.GetDesigns() {
		result := &webapi.AddResult{Folder: want.GetFolder(), Entry: want.GetEntry()}
		resp.Results = append(resp.Results, result)
		add, exists, err := s.planOne(ctx, u.Mount, req, proposed.GetDesigns(), want, ids)
		if add.id != "" {
			result.Design = StandaloneDesignName(add.id)
		}
		if err != nil {
			if !req.GetAllowPartial() {
				return nil, err
			}
			result.Outcome, result.Reason = webapi.AddOutcome_ADD_OUTCOME_REFUSED, err.Error()
			continue
		}
		if exists {
			switch req.GetOnExists() {
			case webapi.OnExists_ON_EXISTS_SKIP:
				result.Outcome = webapi.AddOutcome_ADD_OUTCOME_SKIPPED
				continue
			case webapi.OnExists_ON_EXISTS_UPDATE:
				same, err := s.holdsFiles(ctx, add)
				if err != nil {
					return nil, err
				}
				if same {
					result.Outcome = webapi.AddOutcome_ADD_OUTCOME_UNCHANGED
					continue
				}
				plans = append(plans, planned{add: add, result: result, update: true})
				continue
			default:
				err := fmt.Errorf("%w: %s", ErrAlreadyExists, StandaloneDesignName(add.id))
				if !req.GetAllowPartial() {
					return nil, err
				}
				result.Outcome, result.Reason = webapi.AddOutcome_ADD_OUTCOME_REFUSED, err.Error()
				continue
			}
		}
		plans = append(plans, planned{add: add, result: result})
	}
	if req.GetValidateOnly() {
		for _, p := range plans {
			p.result.Outcome = webapi.AddOutcome_ADD_OUTCOME_ADDED
			if p.update {
				p.result.Outcome = webapi.AddOutcome_ADD_OUTCOME_UPDATED
			}
			d := &webapi.Design{Name: StandaloneDesignName(p.add.id), Title: p.add.record.Title, Series: p.add.record.Series}
			for _, w := range p.add.writes {
				d.Artifacts = append(d.Artifacts, ArtifactName(p.add.id, w.ID))
			}
			resp.Designs = append(resp.Designs, d)
		}
		return resp, nil
	}
	for i, p := range plans {
		if p.update || p.add.project.IsZero() || s.packer == nil {
			continue
		}
		pack, err := s.packer(ctx, p.add.project)
		if err != nil {
			err = fmt.Errorf("%s: keeping its project %s as a pack: %w", StandaloneDesignName(p.add.id), p.add.project, err)
			if !req.GetAllowPartial() {
				return nil, err
			}
			p.result.Outcome, p.result.Reason = webapi.AddOutcome_ADD_OUTCOME_REFUSED, err.Error()
			continue
		}
		plans[i].add.record.Packs = []string{pack}
	}
	var written []planned
	for _, p := range plans {
		if p.result.Outcome == webapi.AddOutcome_ADD_OUTCOME_REFUSED {
			continue
		}
		write := s.writeAdded
		outcome := webapi.AddOutcome_ADD_OUTCOME_ADDED
		if p.update {
			write, outcome = s.writeUpdated, webapi.AddOutcome_ADD_OUTCOME_UPDATED
		}
		if err := write(ctx, p.add); err != nil {
			if !req.GetAllowPartial() {
				return nil, err
			}
			p.result.Outcome, p.result.Reason = webapi.AddOutcome_ADD_OUTCOME_REFUSED, err.Error()
			continue
		}
		p.result.Outcome = outcome
		for _, u := range p.add.uncopied {
			resp.UncopiedSymbols = append(resp.UncopiedSymbols, StandaloneDesignName(p.add.id)+": "+u)
		}
		written = append(written, p)
	}
	// Read back once every design is written, so a series' latest is marked among all of them.
	for _, p := range written {
		d, err := s.catalog.Design(ctx, StandaloneDesignName(p.add.id))
		if err != nil {
			return nil, err
		}
		if d, err = s.withLatest(ctx, d); err != nil {
			return nil, err
		}
		resp.Designs = append(resp.Designs, d)
	}
	return resp, nil
}

// planOne plans the proposal want names: its id, its artifacts and its symbol folders. exists says
// whether the server already holds a design with its id. ids holds the ids earlier proposals in the
// request took, and planOne adds this one's.
func (s *DesignService) planOne(ctx context.Context, mount string, req *webapi.AddDesignsRequest, proposed []*webapi.ProposedDesign, want *webapi.DesignToAdd, ids map[string]bool) (addedDesign, bool, error) {
	p, err := pickProposal(req.GetUri(), proposed, want)
	if err != nil {
		return addedDesign{}, false, err
	}
	if want.GetDesignId() == "" {
		want = &webapi.DesignToAdd{Folder: want.GetFolder(), Entry: want.GetEntry(), DesignId: DefaultDesignID(p, proposed), Title: want.GetTitle(), Series: want.GetSeries()}
	}
	add, err := s.planAdd(ctx, mount, p, want)
	if err != nil {
		return addedDesign{id: add.id}, false, err
	}
	if err := s.planSymbols(ctx, mount, p, &add, req.GetNoSymbolCopy()); err != nil {
		return addedDesign{id: add.id}, false, err
	}
	if ids[add.id] {
		return add, false, fmt.Errorf("%w: two proposals would both be %s", ErrInvalidArgument, StandaloneDesignName(add.id))
	}
	ids[add.id] = true
	if _, err := s.catalog.Design(ctx, StandaloneDesignName(add.id)); err == nil {
		return add, true, nil
	} else if !errors.Is(err, ErrNotFound) {
		return add, false, err
	}
	return add, false, nil
}

// holdsFiles reports whether the stored design a's id names already holds each artifact a would
// write, with the same files, so an update can leave it alone. An artifact a doesn't write is not
// compared, as an update leaves it.
func (s *DesignService) holdsFiles(ctx context.Context, a addedDesign) (bool, error) {
	arts, err := s.catalog.Artifacts(ctx, StandaloneDesignName(a.id))
	if err != nil {
		return false, err
	}
	have := map[string]string{}
	for _, art := range arts {
		_, id, _ := ArtifactID(art.GetName())
		have[id] = fileSet(art.GetFiles())
	}
	for _, w := range a.writes {
		if got, ok := have[w.ID]; !ok || got != fileSet(w.Files) {
			return false, nil
		}
	}
	return true, nil
}

// fileSet is a canonical string of files' paths and hashes, in path order.
func fileSet(files []*webapi.SnapshotFile) string {
	lines := make([]string, 0, len(files))
	for _, f := range files {
		lines = append(lines, f.GetPath()+"\x00"+f.GetSha256())
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

// pickProposal finds the proposal want names, by its folder and, when set, its entry. A folder holding
// several proposals needs the entry, and a request naming only the folder is refused with each
// entry listed, rather than adding whichever came last.
func pickProposal(uri string, proposed []*webapi.ProposedDesign, want *webapi.DesignToAdd) (*webapi.ProposedDesign, error) {
	var inFolder []*webapi.ProposedDesign
	var folders []string
	for _, p := range proposed {
		folders = append(folders, fmt.Sprintf("%q", p.GetFolder()))
		if p.GetFolder() == want.GetFolder() {
			inFolder = append(inFolder, p)
		}
	}
	if len(inFolder) == 0 {
		return nil, fmt.Errorf("%w: %s proposes no design at %q; it proposes %s", ErrInvalidArgument, uri, want.GetFolder(), strings.Join(slices.Compact(folders), ", "))
	}
	var entries []string
	for _, p := range inFolder {
		if want.GetEntry() == p.GetDesign().GetEntryUri() {
			return p, nil
		}
		entries = append(entries, fmt.Sprintf("%q", p.GetDesign().GetEntryUri()))
	}
	if want.GetEntry() == "" && len(inFolder) == 1 {
		return inFolder[0], nil
	}
	if want.GetEntry() == "" {
		return nil, fmt.Errorf("%w: %s proposes %d designs at %q; name one by its entry: %s", ErrInvalidArgument, uri, len(inFolder), want.GetFolder(), strings.Join(entries, ", "))
	}
	return nil, fmt.Errorf("%w: %s proposes no design at %q with entry %q; its entries there are %s", ErrInvalidArgument, uri, want.GetFolder(), want.GetEntry(), strings.Join(entries, ", "))
}

// planAdd reads one proposal's files from the mount and groups them into artifacts, each with its
// source. Files outside the proposal's folder, such as a project's symbol library, are left out,
// as a dropped folder holds only its own files.
func (s *DesignService) planAdd(ctx context.Context, mount string, p *webapi.ProposedDesign, want *webapi.DesignToAdd) (addedDesign, error) {
	id := want.GetDesignId()
	if !newIDPattern.MatchString(id) {
		return addedDesign{}, fmt.Errorf("%w: design id %q must be lowercase letters, digits, '.', '_' or '-', starting with a letter or digit", ErrInvalidArgument, id)
	}
	title := want.GetTitle()
	if title == "" {
		title = p.GetDesign().GetTitle()
	}
	folder := strings.Trim(p.GetFolder(), "/")
	prefix := ""
	if folder != "" {
		prefix = folder + "/"
	}
	contents := map[string][]byte{}
	var rels []string
	for _, f := range p.GetFiles() {
		// The folder's design.yaml goes to PlanArtifacts too, which reads the entry and companions
		// it declares and keeps it out of every artifact (agni issue 1858).
		rel, ok := strings.CutPrefix(f, prefix)
		if !ok || (path.Base(f) == DesignDescriptorName && rel != DesignDescriptorName) {
			continue
		}
		fu, err := mounts.NewURI(mount, f)
		if err != nil {
			return addedDesign{}, err
		}
		b, err := s.mountRead.ReadFile(ctx, fu)
		if err != nil {
			return addedDesign{}, ClassifyLoadErr(err)
		}
		contents[rel] = b
		rels = append(rels, rel)
	}
	slices.Sort(rels)
	plan, err := PlanArtifacts(rels, func(r string) ([]byte, error) {
		b, ok := contents[r]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, r)
		}
		return b, nil
	})
	if err != nil {
		return addedDesign{}, err
	}
	add := addedDesign{id: id, record: DesignRecord{Title: title, Series: want.GetSeries()}, contents: map[string][]byte{}}
	if plan.Declared {
		add.record.Entry = plan.IDs[0]
	}
	for _, a := range plan.IDs {
		w := ArtifactWrite{ID: a, AllowMissing: true}
		root := ""
		for i, pf := range plan.Files[a] {
			b := contents[pf.Folder]
			sum := sha256.Sum256(b)
			h := "sha256:" + hex.EncodeToString(sum[:])
			add.contents[h] = b
			w.Files = append(w.Files, &webapi.SnapshotFile{Path: pf.Rel, Sha256: h, Size: int64(len(b))})
			if i == 0 {
				root = strings.TrimSuffix(strings.TrimSuffix(pf.Folder, pf.Rel), "/")
			}
		}
		w.Source = &webapi.ArtifactSource{Mount: mount, Path: strings.Trim(path.Join(folder, root), "/")}
		add.writes = append(add.writes, w)
	}
	return add, nil
}

// writeUpdated writes an existing design's artifacts from a, and leaves its record as it is. An
// artifact the design holds that a doesn't write stays.
func (s *DesignService) writeUpdated(ctx context.Context, a addedDesign) error {
	read := func(_ context.Context, h string) ([]byte, bool) {
		b, ok := a.contents[h]
		return b, ok
	}
	_, err := s.catalog.WriteArtifacts(ctx, StandaloneDesignName(a.id), a.writes, read)
	return err
}

// writeAdded makes one design and writes its artifacts. A design it made and could not fill is
// deleted, so a failed add leaves nothing behind.
func (s *DesignService) writeAdded(ctx context.Context, a addedDesign) error {
	name := StandaloneDesignName(a.id)
	if _, err := s.catalog.CreateDesign(ctx, a.id, a.record); err != nil {
		return err
	}
	read := func(_ context.Context, h string) ([]byte, bool) {
		b, ok := a.contents[h]
		return b, ok
	}
	if _, err := s.catalog.WriteArtifacts(ctx, name, a.writes, read); err != nil {
		_ = s.catalog.DeleteDesign(context.WithoutCancel(ctx), name)
		return err
	}
	return nil
}

var idRun = regexp.MustCompile(`[^a-z0-9._-]+`)

// DefaultDesignID is the id AddDesigns gives a proposal the request names no id for. It is the
// proposal's folder name, since two releases of one board sit in two folders under one entry name.
// A folder holding several proposals gives each its design's name. When that still leaves two
// proposals with one id, as two folders that each hold boards of the same names do, each id takes
// its folder's path in front (agni issue 1756).
func DefaultDesignID(p *webapi.ProposedDesign, all []*webapi.ProposedDesign) string {
	id := baseDesignID(p, all)
	clash := slices.ContainsFunc(all, func(o *webapi.ProposedDesign) bool { return o != p && baseDesignID(o, all) == id })
	if folder := strings.Trim(p.GetFolder(), "/"); clash && folder != "" {
		return designIDFrom(strings.ReplaceAll(folder, "/", "-") + "-" + id)
	}
	return id
}

// baseDesignID is the id a proposal takes before any clash: its folder's name, or its design's name
// when its folder holds others too or is the mount's root.
func baseDesignID(p *webapi.ProposedDesign, all []*webapi.ProposedDesign) string {
	folder := strings.Trim(p.GetFolder(), "/")
	shared := slices.ContainsFunc(all, func(o *webapi.ProposedDesign) bool { return o != p && strings.Trim(o.GetFolder(), "/") == folder })
	if folder == "" || shared {
		return designIDFrom(p.GetDesign().GetName())
	}
	return designIDFrom(folder)
}

// designIDFrom turns a name into a design id: its last path segment lowercased, each run of other
// characters as one '-', and trimmed of leading punctuation.
func designIDFrom(name string) string {
	id := idRun.ReplaceAllString(strings.ToLower(path.Base(name)), "-")
	return strings.TrimLeft(id, "-._")
}

// SourcedFrom names the stored designs any of whose artifacts were read from mount at dir or below
// it, sorted, each "designs/{design}" (agni issue 1756). A served mounted project lists these as its
// designs. An empty dir is the mount's root.
func (s *DesignService) SourcedFrom(ctx context.Context, mount, dir string) ([]string, error) {
	designs, _, err := s.catalog.Designs(ctx)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, d := range designs {
		arts, err := s.catalog.Artifacts(ctx, d.GetName())
		if err != nil {
			continue
		}
		if slices.ContainsFunc(arts, func(a *webapi.Artifact) bool {
			src := a.GetSource()
			return src.GetMount() == mount && (dir == "" || src.GetPath() == dir || strings.HasPrefix(src.GetPath(), dir+"/"))
		}) {
			out = append(out, d.GetName())
		}
	}
	slices.Sort(out)
	return out, nil
}

// planSymbols reads the proposal's design.yaml for the project it extends and the symbol folders
// it names (agni issue 1756). Each symbol folder becomes an artifact of type ARTIFACT_TYPE_SYMBOLS,
// named symbols, symbols-2 and so on, with its files copied by hash and its source in the mount, so
// the design reads alone. A folder may sit outside the design's own folder, as a library a sibling
// design keeps does. With noCopy the folders are listed in add.uncopied instead. A proposal with no
// design.yaml has neither.
func (s *DesignService) planSymbols(ctx context.Context, mount string, p *webapi.ProposedDesign, add *addedDesign, noCopy bool) error {
	descriptor, err := mounts.NewURI(mount, path.Join(strings.Trim(p.GetFolder(), "/"), DesignDescriptorName))
	if err != nil {
		return err
	}
	raw, err := s.mountRead.ReadFile(ctx, descriptor)
	if err != nil {
		return nil
	}
	var d struct {
		Extends string   `yaml:"extends"`
		Symbols []string `yaml:"symbols"`
	}
	if yaml.Unmarshal(raw, &d) != nil {
		return nil
	}
	if d.Extends != "" {
		if add.project, err = descriptor.Resolve(d.Extends); err != nil {
			return fmt.Errorf("%w: %s extends %q: %v", ErrInvalidArgument, descriptor, d.Extends, err)
		}
	}
	taken := map[string]bool{}
	for _, w := range add.writes {
		taken[w.ID] = true
	}
	for _, ref := range d.Symbols {
		dir, err := descriptor.Resolve(ref)
		if err != nil {
			return fmt.Errorf("%w: %s names the symbol folder %q: %v", ErrInvalidArgument, descriptor, ref, err)
		}
		if noCopy {
			add.uncopied = append(add.uncopied, dir.String())
			continue
		}
		files, err := s.walk(ctx, dir)
		if err != nil {
			return fmt.Errorf("%s names the symbol folder %s: %w", descriptor, dir, err)
		}
		id := "symbols"
		for i := 2; taken[id]; i++ {
			id = fmt.Sprintf("symbols-%d", i)
		}
		taken[id] = true
		w := ArtifactWrite{ID: id, AllowMissing: true, Source: &webapi.ArtifactSource{Mount: dir.Mount, Path: dir.Path}}
		for _, f := range files {
			b, err := s.mountRead.ReadFile(ctx, f)
			if err != nil {
				return ClassifyLoadErr(err)
			}
			sum := sha256.Sum256(b)
			h := "sha256:" + hex.EncodeToString(sum[:])
			add.contents[h] = b
			w.Files = append(w.Files, &webapi.SnapshotFile{Path: strings.TrimPrefix(f.Path, dir.Path+"/"), Sha256: h, Size: int64(len(b))})
		}
		if len(w.Files) == 0 {
			continue
		}
		add.writes = append(add.writes, w)
	}
	return nil
}

// walk lists every file under dir in its mount, through the proposer's ListDir.
func (s *DesignService) walk(ctx context.Context, dir mounts.URI) ([]mounts.URI, error) {
	resp, err := s.proposer.ListDir(ctx, &webapi.ListDirRequest{Uri: dir.String()})
	if err != nil {
		return nil, err
	}
	var out []mounts.URI
	for _, e := range resp.GetEntries() {
		u, err := dir.Join(e.GetName())
		if err != nil {
			return nil, err
		}
		if !e.GetIsDir() {
			out = append(out, u)
			continue
		}
		sub, err := s.walk(ctx, u)
		if err != nil {
			return nil, err
		}
		out = append(out, sub...)
	}
	return out, nil
}
