// Command datasheetstatus reports the Stage-A (PDF -> doc-IR) extraction freshness of a datasheet
// corpus laid out as <vendor>/<PART>/ (WS13-009). Per part it reports which PDFs have a doc-IR
// sibling, whether that doc-IR still matches the PDF bytes and the installed toolchain, and, given
// the corpus store, which drafts cite the part's PDFs and whether each is published (agni issue
// 749). It never writes. `make datasheets-status` runs it over DATASHEET_DIR and CORPUS_DIR.
//
// Freshness is read from each doc-IR's Document.content_hash (the source bytes at extraction time)
// and Document.producer (the toolchain). A recipe or patch change is a derive-stage concern and
// does NOT make a doc-IR stale.
//
// Usage:
//
//	datasheetstatus [--toolchain docling/X.Y.Z] [--corpus <dir> [--mount ds]] [--list] <root>...
//
// Default output is a per-part table plus a summary tally. With --list it prints, one per line,
// the PDF paths that need (re)extraction (not-extracted or stale-source) for `make pdf2doc-all`.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/datasheet/corpus"
	"github.com/panyam/agni/datasheet/doc"
)

// docSiblingSuffix is the doc-IR sibling extension, so LM1117.pdf pairs with LM1117.doc.textproto.
// Keep it in step with cmd/agni/osdocloader.go.
const docSiblingSuffix = ".doc.textproto"

type pdfInfo struct {
	path   string // path as walked, used for display and for `make pdf2doc-all`
	name   string // base name
	uri    string // the mount:// URI a draft cites this PDF by
	status PDFStatus
}

type partInfo struct {
	name   string // directory relative to the scanned root, e.g. "onsemi/BSS138"
	pdfs   []pdfInfo
	drafts []draftInfo // the drafts citing any of the part's PDFs
}

// draftInfo is one draft citing a part, and whether its MPN has a published spec.
type draftInfo struct {
	mpn       string
	published bool
}

// rollup is the status most needing attention across the part's PDFs.
func (p partInfo) rollup() PDFStatus {
	worst := Fresh
	for _, f := range p.pdfs {
		if f.status.rank() < worst.rank() {
			worst = f.status
		}
	}
	return worst
}

func main() {
	toolchain := flag.String("toolchain", "", `producer the installed extractor would stamp now, e.g. "docling/2.5.1"; empty disables stale-toolchain detection`)
	list := flag.Bool("list", false, "print only the PDF paths needing (re)extraction, one per line")
	corpusDir := flag.String("corpus", "", "the corpus store (agnids serve --corpus), to report which drafts cite each part and whether each is published; empty leaves the column out")
	mount := flag.String("mount", "ds", "the mount name the datasheet root is served under, which the URIs drafts cite begin with (make dsserve mounts DATASHEET_DIR as ds)")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: datasheetstatus [--toolchain docling/X.Y] [--corpus <dir> [--mount ds]] [--list] <root>...")
		os.Exit(2)
	}
	if *corpusDir != "" && flag.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "--corpus takes one root, since --mount names the one mount its PDFs are cited under")
		os.Exit(2)
	}

	var parts []partInfo
	for _, root := range flag.Args() {
		ps, err := scan(root, *mount, *toolchain)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", root, err)
			os.Exit(1)
		}
		parts = append(parts, ps...)
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].name < parts[j].name })
	if *corpusDir != "" {
		if err := attachDrafts(parts, os.DirFS(*corpusDir)); err != nil {
			fmt.Fprintf(os.Stderr, "--corpus %s: %v\n", *corpusDir, err)
			os.Exit(1)
		}
	}

	if *list {
		for _, p := range parts {
			for _, f := range p.pdfs {
				if f.status.needsExtraction() {
					fmt.Println(f.path)
				}
			}
		}
		return
	}
	report(parts, *corpusDir != "")
}

// attachDrafts records, per part, the drafts citing any of its PDFs and whether each draft's MPN has
// a published spec in the corpus index.
func attachDrafts(parts []partInfo, store fs.FS) error {
	drafts, err := corpus.Drafts(store)
	if err != nil {
		return err
	}
	ix, err := corpus.Read(store)
	if err != nil && err != corpus.ErrNoIndex {
		return err
	}
	for i := range parts {
		for _, d := range drafts {
			cites := false
			for _, f := range parts[i].pdfs {
				if slices.Contains(d.GetDocumentUris(), f.uri) {
					cites = true
					break
				}
			}
			if !cites {
				continue
			}
			published := false
			if ix != nil {
				_, published = ix.Lookup(d.GetMpn())
			}
			parts[i].drafts = append(parts[i].drafts, draftInfo{mpn: d.GetMpn(), published: published})
		}
	}
	return nil
}

// scan walks one root, groups the PDFs it finds by their containing directory into parts, and
// classifies each PDF's extraction status.
func scan(root, mount, toolchain string) ([]partInfo, error) {
	byDir := map[string]*partInfo{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.ToLower(filepath.Ext(path)) != ".pdf" {
			return nil
		}
		dir := filepath.Dir(path)
		part := byDir[dir]
		if part == nil {
			rel, rerr := filepath.Rel(root, dir)
			if rerr != nil {
				rel = dir
			}
			part = &partInfo{name: rel}
			byDir[dir] = part
		}
		st, serr := statusOf(path, toolchain)
		if serr != nil {
			return serr
		}
		uri := ""
		if rel, rerr := filepath.Rel(root, path); rerr == nil {
			if u, uerr := artifact.New(mount, filepath.ToSlash(rel)); uerr == nil {
				uri = u.String()
			}
		}
		part.pdfs = append(part.pdfs, pdfInfo{path: path, name: filepath.Base(path), uri: uri, status: st})
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]partInfo, 0, len(byDir))
	for _, p := range byDir {
		sort.Slice(p.pdfs, func(i, j int) bool { return p.pdfs[i].name < p.pdfs[j].name })
		out = append(out, *p)
	}
	return out, nil
}

// statusOf classifies one PDF by hashing its bytes and, if a doc-IR sibling exists, reading the
// stored hash and producer out of it.
func statusOf(pdfPath, toolchain string) (PDFStatus, error) {
	pdfHash, err := hashPDF(pdfPath)
	if err != nil {
		return "", err
	}
	sib := strings.TrimSuffix(pdfPath, filepath.Ext(pdfPath)) + docSiblingSuffix
	f, err := os.Open(sib)
	if err != nil {
		if os.IsNotExist(err) {
			return classify(false, pdfHash, "", "", toolchain), nil
		}
		return "", err
	}
	defer f.Close()
	dir, err := doc.Load(f)
	if err != nil {
		return "", fmt.Errorf("%s: %w", sib, err)
	}
	return classify(true, pdfHash, dir.ContentHash, dir.Producer, toolchain), nil
}

func report(parts []partInfo, withDrafts bool) {
	tally := map[PDFStatus]int{}
	drafted, published := 0, 0
	for _, p := range parts {
		roll := p.rollup()
		tally[roll]++
		line := fmt.Sprintf("%-24s %-16s", p.name, roll)
		if withDrafts {
			line += " " + draftColumn(p.drafts)
			if len(p.drafts) > 0 {
				drafted++
			}
			if slices.ContainsFunc(p.drafts, func(d draftInfo) bool { return d.published }) {
				published++
			}
		}
		fmt.Println(strings.TrimRight(line, " "))
		if len(p.pdfs) > 1 {
			for _, f := range p.pdfs {
				fmt.Printf("  %-22s %s\n", f.name, f.status)
			}
		}
	}
	fmt.Printf("\n%d parts: %d fresh, %d not-extracted, %d stale-source, %d stale-toolchain",
		len(parts), tally[Fresh], tally[NotExtracted], tally[StaleSource], tally[StaleToolchain])
	if withDrafts {
		fmt.Printf("; %d with a draft, %d with a published spec", drafted, published)
	}
	fmt.Println()
}

// draftColumn renders a part's drafts as "LM1117-3.3 (published), LM1117-5.0 (draft)", or
// "no-draft".
func draftColumn(ds []draftInfo) string {
	if len(ds) == 0 {
		return "no-draft"
	}
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		state := "draft"
		if d.published {
			state = "published"
		}
		out = append(out, fmt.Sprintf("%s (%s)", d.mpn, state))
	}
	return strings.Join(out, ", ")
}
