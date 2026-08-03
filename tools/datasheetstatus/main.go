// Command datasheetstatus reports the Stage-A extraction freshness of a datasheet corpus laid out
// as datasheets/<vendor>/<PART>/ (WS13-009): per part, which source PDFs have a doc-IR sibling,
// whether that doc-IR still matches the PDF bytes and the installed toolchain, and whether a
// part-level PartSpec exists. It is report-only and never writes. The workspace `hw/Makefile`
// invokes it over the private, gitignored datasheets/ folder (the folder path is the only private
// input; this tool carries none), the same split as tools/pdf2doc.
//
// The freshness signals are read straight out of each doc-IR: Document.content_hash (the source
// bytes at extraction time) and Document.producer (the toolchain identity). A recipe/patch change
// is a derive-stage concern and does NOT make a doc-IR stale, so this tool reports Stage-A
// (PDF -> doc-IR) only.
//
// Usage:
//
//	datasheetstatus [--toolchain docling/X.Y.Z] [--list] <root>...
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
	"sort"
	"strings"

	"github.com/panyam/agni/doc"
)

// docSiblingSuffix is the doc-IR sibling extension, kept in step with the serve-side loader
// (cmd/agni/osdocloader.go): LM1117.pdf pairs with LM1117.doc.textproto.
const docSiblingSuffix = ".doc.textproto"

type pdfInfo struct {
	path   string // path as walked, used for display and for `make pdf2doc-all`
	name   string // base name
	status PDFStatus
}

type partInfo struct {
	name    string // directory relative to the scanned root, e.g. "onsemi/BSS138"
	pdfs    []pdfInfo
	hasSpec bool // a <...>.partspec.json exists in the part dir (part-level, WS13-009)
}

// rollup is the part's worst-case status: the one that most needs attention across its PDFs.
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
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: datasheetstatus [--toolchain docling/X.Y] [--list] <root>...")
		os.Exit(2)
	}

	var parts []partInfo
	for _, root := range flag.Args() {
		ps, err := scan(root, *toolchain)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", root, err)
			os.Exit(1)
		}
		parts = append(parts, ps...)
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].name < parts[j].name })

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
	report(parts)
}

// scan walks one root, groups the PDFs it finds by their containing directory into parts, and
// classifies each PDF's extraction status.
func scan(root, toolchain string) ([]partInfo, error) {
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
			part = &partInfo{name: rel, hasSpec: hasPartSpec(dir)}
			byDir[dir] = part
		}
		st, serr := statusOf(path, toolchain)
		if serr != nil {
			return serr
		}
		part.pdfs = append(part.pdfs, pdfInfo{path: path, name: filepath.Base(path), status: st})
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

// hasPartSpec reports whether the part directory holds a PartSpec (a <...>.partspec.json). The
// PartSpec is part-level (one per part dir), so its presence is a property of the dir, not a PDF.
func hasPartSpec(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".partspec.json") {
			return true
		}
	}
	return false
}

func report(parts []partInfo) {
	tally := map[PDFStatus]int{}
	withSpec := 0
	for _, p := range parts {
		spec := "no-spec"
		if p.hasSpec {
			spec = "spec"
			withSpec++
		}
		roll := p.rollup()
		tally[roll]++
		fmt.Printf("%-24s %-16s %s\n", p.name, roll, spec)
		if len(p.pdfs) > 1 {
			for _, f := range p.pdfs {
				fmt.Printf("  %-22s %s\n", f.name, f.status)
			}
		}
	}
	fmt.Printf("\n%d parts — %d fresh, %d not-extracted, %d stale-source, %d stale-toolchain; %d with spec\n",
		len(parts), tally[Fresh], tally[NotExtracted], tally[StaleSource], tally[StaleToolchain], withSpec)
}
