package service

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/formats"
)

// DesignDescriptorName is the file that declares a folder's design. internal/projects owns the name
// and a test there holds this copy to it.
const DesignDescriptorName = "design.yaml"

// ProposeDesigns groups the files under a folder into the designs they make (agni issue 854). It
// lists the folder through the Workspace port and reads KiCad sheets through the FileReader, so it
// needs WithDesignFiles. Paths in the answer are mount-relative.
func (s *WorkspaceService) ProposeDesigns(ctx context.Context, req *webapi.ProposeDesignsRequest) (*webapi.ProposeDesignsResponse, error) {
	if s.files == nil {
		return nil, fmt.Errorf("%w: this host does not propose designs", ErrInvalidArgument)
	}
	u, err := ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	var files []string
	budget := designFilesMaxDirs
	if err := s.walkFiles(ctx, u.Mount, u.Path, func(p string) { files = append(files, p) }, &budget); err != nil {
		return nil, err
	}
	read := func(p string) ([]byte, error) {
		fu, err := artifact.New(u.Mount, p)
		if err != nil {
			return nil, err
		}
		return s.files.ReadFile(ctx, fu)
	}
	return ProposeFromFiles(u.Mount, files, read), nil
}

// ProposeFromFiles applies the grouping rules to a list of mount-relative file paths. read returns a
// file's bytes, and is used only to find a KiCad sheet's child sheets.
//
// The rules, in the order they claim files:
//
//   - A folder holding a `design.yaml` is DECLARED, and every file under it belongs to that design.
//     The proposal reports the descriptor rather than guessing over it.
//   - A `project.yaml`'s folder lends its descriptor and its profiles, params, symbols and lib
//     directories to the designs under it, so those are support files, not unread ones.
//   - (Netlists go first, so a netlist claims the KiCad schematic and board of its own name as its
//     views before either can become a design of its own; see the netlist rule below.)
//   - A `.kicad_pro` claims the `.kicad_sch` of its stem as the entry, that sheet's child sheets, and
//     the `.kicad_pcb` of its stem as the board companion.
//   - A `.kicad_sch` that no other sheet names as a child is a root, and makes a design the same way.
//   - In each folder, netlists whose names share a first word (board.edn, board-rev-b.edn) are ONE
//     design's revisions, never merged: the entry is the first with a `.eds` of its stem (else the
//     first), each `.eds` of a netlist's stem is that netlist's schematic companion, and every other
//     netlist is a revision. A netlist's companions are the `.eds`, `.kicad_sch` and `.kicad_pcb` of
//     its own stem. Netlists with different first words are separate designs. Inferring this is safe
//     here, where it is not on a server (agni 528), because the visitor sees it and edits it first.
//   - A `.eds` with no netlist of its stem is a design of its own, with a note, since a drawing's
//     counts are not a netlist's.
//   - Any other file a reader opens is a design of its own. KiCad libraries belong to the KiCad
//     design beside or above them.
//   - Everything left is unread, with the reason.
func ProposeFromFiles(mount string, files []string, read func(string) ([]byte, error)) *webapi.ProposeDesignsResponse {
	p := &proposer{mount: mount, have: map[string]bool{}, claimed: map[string]bool{}, read: read}
	for _, f := range files {
		p.have[f] = true
	}
	sorted := append([]string(nil), files...)
	sort.Strings(sorted)
	p.files = sorted

	p.declared()
	p.projects()
	p.kicadDirs = map[string]*webapi.ProposedDesign{}
	p.edif()
	p.kicad()
	p.loose()
	p.libraries()
	p.leftovers()

	sort.SliceStable(p.resp.Designs, func(i, j int) bool {
		return p.resp.Designs[i].GetFolder() < p.resp.Designs[j].GetFolder()
	})
	p.noteSharedFolders()
	return &p.resp
}

type proposer struct {
	mount   string
	files   []string
	have    map[string]bool
	claimed map[string]bool
	read    func(string) ([]byte, error)
	resp    webapi.ProposeDesignsResponse
	// kicadDirs are the folders holding a KiCad design, for its libraries to attach to.
	kicadDirs map[string]*webapi.ProposedDesign
}

func (p *proposer) claim(f string) { p.claimed[f] = true }
func (p *proposer) free(f string) bool {
	return p.have[f] && !p.claimed[f]
}

func (p *proposer) uri(f string) string {
	u, err := artifact.New(p.mount, f)
	if err != nil {
		return f
	}
	return u.String()
}

func under(dir, f string) bool { return dir == "" || strings.HasPrefix(f, dir+"/") }
func dirOf(f string) string    { return parentPath(f) }
func stemOf(f string) string   { return strings.TrimSuffix(path.Base(f), path.Ext(f)) }
func sibling(f, ext string) string {
	return joinPath(dirOf(f), stemOf(f)+ext)
}
func ext(f string) string { return strings.ToLower(path.Ext(f)) }

func (p *proposer) declared() {
	var dirs []string
	for _, f := range p.files {
		if path.Base(f) == DesignDescriptorName {
			dirs = append(dirs, dirOf(f))
		}
	}
	for _, d := range dirs {
		var owned []string
		for _, f := range p.files {
			if under(d, f) && p.free(f) {
				owned = append(owned, f)
				p.claim(f)
			}
		}
		text, _ := p.read(joinPath(d, DesignDescriptorName))
		p.resp.Designs = append(p.resp.Designs, &webapi.ProposedDesign{
			Folder:     d,
			Design:     &webapi.Design{Name: path.Base(d), Uri: p.uri(d)},
			DesignYaml: string(text),
			Files:      owned,
			Declared:   true,
			Note:       "declared by its design.yaml",
		})
	}
}

// projectTiers are the directories a project's config is read from by default, which
// internal/projects's descriptor defaults name.
var projectTiers = []string{"profiles", "params", "symbols", "lib"}

func (p *proposer) projects() {
	for _, f := range p.files {
		if path.Base(f) != ProjectDescriptorName || !p.free(f) {
			continue
		}
		d := dirOf(f)
		p.support(f)
		for _, t := range projectTiers {
			for _, g := range p.files {
				if under(joinPath(d, t), g) && p.free(g) {
					p.support(g)
				}
			}
		}
	}
}

func (p *proposer) support(f string) {
	p.claim(f)
	p.resp.Support = append(p.resp.Support, f)
}

// sheetRef finds the child sheets a KiCad schematic names.
var sheetRef = regexp.MustCompile(`\(property\s+"Sheetfile"\s+"([^"]+)"`)

// sheetClosure is root and every sheet it reaches through child-sheet references, among the files
// present.
func (p *proposer) sheetClosure(root string) []string {
	seen := map[string]bool{root: true}
	out := []string{root}
	for i := 0; i < len(out); i++ {
		b, err := p.read(out[i])
		if err != nil {
			continue
		}
		for _, m := range sheetRef.FindAllSubmatch(b, -1) {
			child := path.Clean(joinPath(dirOf(out[i]), strings.ReplaceAll(string(m[1]), "\\", "/")))
			if p.have[child] && !seen[child] {
				seen[child] = true
				out = append(out, child)
			}
		}
	}
	return out
}

func (p *proposer) kicad() {
	for _, f := range p.files {
		if ext(f) == ".kicad_pro" && p.free(f) {
			entry := sibling(f, ".kicad_sch")
			if !p.free(entry) {
				entry = f
			}
			p.kicadDesign(entry, []string{f})
		}
	}
	// Roots among the sheets left: a sheet no other free sheet names as a child.
	children := map[string]bool{}
	var sheets []string
	for _, f := range p.files {
		if ext(f) == ".kicad_sch" && p.free(f) {
			sheets = append(sheets, f)
		}
	}
	for _, s := range sheets {
		for _, c := range p.sheetClosure(s)[1:] {
			children[c] = true
		}
	}
	for _, s := range sheets {
		if !children[s] && p.free(s) {
			p.kicadDesign(s, nil)
		}
	}
}

func (p *proposer) kicadDesign(entry string, extra []string) {
	files := append([]string(nil), extra...)
	var companions []string
	if ext(entry) == ".kicad_sch" {
		for _, s := range p.sheetClosure(entry) {
			if p.free(s) {
				files = append(files, s)
			}
		}
		if pcb := sibling(entry, ".kicad_pcb"); p.free(pcb) {
			companions = append(companions, pcb)
			files = append(files, pcb)
		}
	} else {
		files = append(files, entry)
	}
	for _, f := range files {
		p.claim(f)
	}
	d := p.add(entry, companions, nil, files, "")
	if _, ok := p.kicadDirs[dirOf(entry)]; !ok {
		p.kicadDirs[dirOf(entry)] = d
	}
}

func (p *proposer) edif() {
	// Netlists group by folder and by the first word of their name, so gateway.edn and
	// gateway-rev-b.edn are one design's revisions while basic.edn and bus.edn beside them are not.
	groups := map[string][]string{}
	var keys []string
	for _, f := range p.files {
		if (ext(f) == ".edn" || ext(f) == ".edf" || ext(f) == ".edif") && p.free(f) {
			k := dirOf(f) + "\x00" + firstWord(stemOf(f))
			if _, ok := groups[k]; !ok {
				keys = append(keys, k)
			}
			groups[k] = append(groups[k], f)
		}
	}
	for _, k := range keys {
		nets := groups[k]
		// A netlist's companions are the views of the same name beside it: a schematic export, a
		// KiCad schematic (with its child sheets, read through it) and a KiCad board.
		companionOf := func(n string) []string {
			var out []string
			for _, e := range []string{".eds", ".kicad_sch", ".kicad_pcb"} {
				if c := sibling(n, e); p.free(c) {
					out = append(out, c)
				}
			}
			return out
		}
		readsOf := func(n string) []string {
			out := []string{n}
			for _, c := range companionOf(n) {
				if ext(c) == ".kicad_sch" {
					out = append(out, p.sheetClosure(c)...)
				} else {
					out = append(out, c)
				}
			}
			if pro := sibling(n, ".kicad_pro"); p.free(pro) {
				out = append(out, pro)
			}
			return out
		}
		// The entry is the netlist named exactly what the group shares (board.edn among its
		// revisions), else the first with a view beside it, else the first.
		entry := nets[0]
		for _, n := range nets {
			if len(companionOf(n)) > 0 {
				entry = n
				break
			}
		}
		for _, n := range nets {
			if strings.ToLower(stemOf(n)) == firstWord(stemOf(n)) {
				entry = n
				break
			}
		}
		var files []string
		var revisions []*webapi.DesignRevision
		for _, n := range nets {
			files = append(files, readsOf(n)...)
			if n != entry {
				rev := &webapi.DesignRevision{EntryUri: p.uri(n)}
				for _, c := range companionOf(n) {
					rev.CompanionUris = append(rev.CompanionUris, p.uri(c))
				}
				revisions = append(revisions, rev)
			}
		}
		companions := companionOf(entry)
		for _, f := range files {
			p.claim(f)
		}
		files = dedupe(files)
		note := ""
		if len(revisions) > 0 {
			note = fmt.Sprintf("%d more netlist(s) in this folder are listed as revisions of %s, not merged into it", len(revisions), path.Base(entry))
		}
		d := p.add(entry, companions, revisions, files, note)
		for _, c := range companions {
			if strings.HasPrefix(ext(c), ".kicad") {
				if _, ok := p.kicadDirs[dirOf(entry)]; !ok {
					p.kicadDirs[dirOf(entry)] = d
				}
			}
		}
	}
	for _, f := range p.files {
		if ext(f) == ".eds" && p.free(f) {
			p.claim(f)
			p.add(f, nil, nil, []string{f}, "a schematic export with no netlist beside it: it draws and answers queries, but its counts are per drawn sheet, not a netlist's")
		}
	}
}

// loose makes a design of each remaining file a reader opens, other than a board, which needs a
// schematic or netlist to belong to.
func (p *proposer) loose() {
	for _, f := range p.files {
		if !p.free(f) || ext(f) == ".kicad_pcb" || formats.ByExt(f) == nil {
			continue
		}
		p.claim(f)
		p.add(f, nil, nil, []string{f}, "")
	}
}

func dedupe(fs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range fs {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// firstWord is a file stem up to its first separator, lowercased, which is what two revisions of one
// board share in practice (board.edn, board-rev-b.edn, board_v2.edn).
func firstWord(stem string) string {
	stem = strings.ToLower(stem)
	if i := strings.IndexAny(stem, "-_. "); i > 0 {
		return stem[:i]
	}
	return stem
}

// kicadLibrary is a file a KiCad design reads without being a design file.
func kicadLibrary(f string) bool {
	switch ext(f) {
	case ".kicad_sym", ".kicad_mod", ".kicad_prl", ".kicad_dru":
		return true
	}
	b := path.Base(f)
	return b == "sym-lib-table" || b == "fp-lib-table"
}

func (p *proposer) libraries() {
	for _, f := range p.files {
		if !p.free(f) || !kicadLibrary(f) {
			continue
		}
		for d := dirOf(f); ; d = parentPath(d) {
			if pd, ok := p.kicadDirs[d]; ok {
				p.claim(f)
				pd.Files = append(pd.Files, f)
				sort.Strings(pd.Files)
				break
			}
			if d == "" {
				break
			}
		}
	}
}

func (p *proposer) leftovers() {
	for _, f := range p.files {
		if !p.free(f) {
			continue
		}
		reason := "not a design file"
		switch {
		case ext(f) == ".kicad_pcb":
			reason = fmt.Sprintf("a board with no schematic or netlist named %s beside it", stemOf(f))
		case kicadLibrary(f):
			reason = "a KiCad library with no KiCad design beside or above it"
		case ext(f) == ".zip":
			reason = "not a readable zip archive"
		}
		p.resp.Unread = append(p.resp.Unread, &webapi.UnreadFile{Path: f, Reason: reason})
	}
}

// add records one proposal and its descriptor.
func (p *proposer) add(entry string, companions []string, revisions []*webapi.DesignRevision, files []string, note string) *webapi.ProposedDesign {
	d := &webapi.Design{Name: designID(stemOf(entry)), Uri: p.uri(dirOf(entry)), EntryUri: p.uri(entry), Revisions: revisions}
	for _, c := range companions {
		d.CompanionUris = append(d.CompanionUris, p.uri(c))
	}
	sort.Strings(files)
	pd := &webapi.ProposedDesign{Folder: dirOf(entry), Design: d, Files: files, Note: note, DesignYaml: p.descriptor(entry, companions, revisions)}
	p.resp.Designs = append(p.resp.Designs, pd)
	return pd
}

// descriptor writes the design.yaml that declares a proposal, paths relative to its folder.
func (p *proposer) descriptor(entry string, companions []string, revisions []*webapi.DesignRevision) string {
	dir := dirOf(entry)
	rel := func(f string) string {
		if dir == "" {
			return f
		}
		return strings.TrimPrefix(f, dir+"/")
	}
	fromURI := func(u string) string {
		if a, err := artifact.Parse(u); err == nil {
			return rel(a.Path)
		}
		return u
	}
	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\nentry: %s\n", yamlScalar(designID(stemOf(entry))), yamlScalar(rel(entry)))
	if len(companions) > 0 {
		b.WriteString("companions:\n")
		for _, c := range companions {
			fmt.Fprintf(&b, "  - %s\n", yamlScalar(rel(c)))
		}
	}
	if len(revisions) > 0 {
		b.WriteString("revisions:\n")
		for _, r := range revisions {
			fmt.Fprintf(&b, "  - entry: %s\n", yamlScalar(fromURI(r.GetEntryUri())))
			if len(r.GetCompanionUris()) > 0 {
				b.WriteString("    companions:\n")
				for _, c := range r.GetCompanionUris() {
					fmt.Fprintf(&b, "      - %s\n", yamlScalar(fromURI(c)))
				}
			}
		}
	}
	return b.String()
}

// designID turns a file stem into a name a descriptor accepts: lowercase letters, digits, '-', '_'
// and '.', starting with a letter or digit (internal/projects checks the same).
func designID(stem string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(stem) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	id := strings.Trim(b.String(), "-_.")
	for strings.Contains(id, "--") {
		id = strings.ReplaceAll(id, "--", "-")
	}
	if id == "" {
		return "design"
	}
	return id
}

// yamlScalar quotes a value YAML would otherwise read as something other than the string it is.
func yamlScalar(s string) string {
	if s == "" || strings.ContainsAny(s, ":#{}[],&*!|>'\"%@`") || strings.TrimSpace(s) != s {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// noteSharedFolders marks every proposal past the first in a folder, since a folder holds one
// design.yaml and so can declare only one of them.
func (p *proposer) noteSharedFolders() {
	first := map[string]bool{}
	for _, d := range p.resp.Designs {
		if d.GetDeclared() {
			first[d.GetFolder()] = true
			continue
		}
		if first[d.GetFolder()] {
			msg := "another design shares this folder, and a folder declares one; open this one by its file, or move it to a folder of its own"
			if d.Note != "" {
				d.Note += ". " + strings.ToUpper(msg[:1]) + msg[1:]
			} else {
				d.Note = msg
			}
			continue
		}
		first[d.GetFolder()] = true
	}
}
