// Package docindex answers questions INSIDE one datasheet: given a phrase, which passages or table
// cells of this document are about it, and where exactly are they.
//
// It exists because the doc-IR is addressable but not searchable. A page/block/cell can be located
// precisely once you know which one you want, and "where does this document state the VCC range" has
// no form at all, so a person looking for a fact the engine could not find has to scroll.
//
// # This index must never be reachable from a check
//
// Two lookups get conflated the moment a retrieval index exists, and conflating them is how a
// confident wrong answer reaches a design review:
//
//   - A FACT lookup is exact, keyed by part and symbol. param.LoadSet already refuses a near-miss
//     MPN on the stated grounds that a near-miss is a different part until a human says otherwise.
//   - PASSAGE retrieval is fuzzy by construction, and exists to put a person or an extractor in
//     front of the right paragraph.
//
// Same document, two trust levels. The engine reads the first; people and extractors read the
// second. This package therefore lives under datasheet/ beside the authoring surfaces and is
// imported by none of core/check, core/review or core/query. If a check ever needs to consult it,
// that is a design error rather than a convenient shortcut.
//
// # Derived, never a second source of truth
//
// An Index is built from a doc-IR and holds no state the doc-IR does not. It can be thrown away and
// rebuilt at any time, which is what keeps a stale index a performance problem rather than a
// correctness one.
package docindex

import (
	"sort"
	"strings"
	"unicode"

	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
)

// Hit is one passage the index matched, located precisely enough to highlight.
//
// A hit that resolved only to a page would be useless for the job this serves: verification has to be
// possible at a glance, or a reviewer waves through whatever they are shown. So every hit names the
// doc-IR region it came from, and a table hit names the cell.
type Hit struct {
	Page int32
	// RegionID is the doc-IR text block or table id, which is what a viewer highlights.
	RegionID string
	// Row and Col locate a table cell, and are -1 for a text block. A cell is the unit worth citing
	// in a datasheet, because most facts worth finding are in one.
	Row, Col int32
	// Text is the matched content verbatim, so a caller quotes the document rather than paraphrasing.
	Text string
	// Context is what makes a cell mean something: its row label and column header. "3.6" is not a
	// fact; "VCCA / MAX = 3.6" is. Empty for a text block, which carries its own context.
	Context string
	Score   float64
}

// entry is one indexed unit before scoring.
type entry struct {
	hit    Hit
	tokens map[string]int
	length int
}

// Index is a searchable view of one document. Safe for concurrent reads; build it once per document.
type Index struct {
	entries []entry
	// docFreq counts how many entries contain a term, so a term appearing on every page of a
	// datasheet ("voltage") contributes far less than one that appears twice.
	docFreq map[string]int
}

// Build indexes every text block and table cell of a document. Blocks with no text are skipped
// rather than indexed empty, and the result is deterministic: the same document yields the same
// index, so a hit list is stable between runs.
func Build(d *docpb.Document) *Index {
	ix := &Index{docFreq: map[string]int{}}
	for _, pg := range d.GetPages() {
		for _, tb := range pg.GetTextBlocks() {
			ix.add(Hit{Page: pg.GetNumber(), RegionID: tb.GetId(), Row: -1, Col: -1, Text: tb.GetText()})
		}
		for _, t := range pg.GetTables() {
			for _, c := range t.GetCells() {
				if strings.TrimSpace(c.GetText()) == "" {
					continue
				}
				ix.add(Hit{
					Page: pg.GetNumber(), RegionID: t.GetId(),
					Row: c.GetRow(), Col: c.GetCol(),
					Text:    c.GetText(),
					Context: cellContext(t, c),
				})
			}
		}
	}
	return ix
}

func (ix *Index) add(h Hit) {
	if strings.TrimSpace(h.Text) == "" {
		return
	}
	// The context is indexed with the cell, so searching "VCCA max" finds the value cell rather than
	// only the label. A value cell alone carries none of the words a person searches by.
	toks := tokenize(h.Text + " " + h.Context)
	if len(toks) == 0 {
		return
	}
	tf := map[string]int{}
	for _, t := range toks {
		tf[t]++
	}
	for t := range tf {
		ix.docFreq[t]++
	}
	ix.entries = append(ix.entries, entry{hit: h, tokens: tf, length: len(toks)})
}

// Search returns the best matches for a query, highest score first, capped at limit (<=0 means 10).
//
// Scoring is deliberately lexical and explainable rather than learned. Datasheet vocabulary is small
// and highly conventional, and a baseline that can be reasoned about is worth having before anything
// is embedded: a hit here can be justified to a person, which matters when the person's job is to
// decide whether to trust it.
func (ix *Index) Search(q string, limit int) []Hit {
	if limit <= 0 {
		limit = 10
	}
	qt := tokenize(q)
	if len(qt) == 0 || len(ix.entries) == 0 {
		return nil
	}
	n := float64(len(ix.entries))
	var out []Hit
	for _, e := range ix.entries {
		var score float64
		matched := 0
		for _, t := range qt {
			c, ok := e.tokens[t]
			if !ok {
				continue
			}
			matched++
			// Rarity matters more than repetition: a term in few entries is what makes a hit
			// specific, while a long block repeating a common word is not a better answer.
			idf := n / float64(1+ix.docFreq[t])
			score += idf * (1 + float64(c-1)*0.2)
		}
		if matched == 0 {
			continue
		}
		// Prefer entries that matched MORE of the query, and shorter ones among equals: a cell
		// stating the fact beats a paragraph mentioning it.
		score *= float64(matched) / float64(len(qt))
		score /= 1 + float64(e.length)/40
		h := e.hit
		h.Score = score
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Page != out[j].Page {
			return out[i].Page < out[j].Page
		}
		if out[i].RegionID != out[j].RegionID {
			return out[i].RegionID < out[j].RegionID
		}
		if out[i].Row != out[j].Row {
			return out[i].Row < out[j].Row
		}
		return out[i].Col < out[j].Col
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// cellContext builds the row label and column header that make a value cell mean something. Both are
// taken from the table's own first column and header row, which is where every datasheet puts them.
func cellContext(t *docpb.Table, c *docpb.Cell) string {
	var row, col string
	for _, o := range t.GetCells() {
		if o == c {
			continue
		}
		if o.GetRow() == c.GetRow() && o.GetCol() == 0 {
			row = strings.TrimSpace(o.GetText())
		}
		if o.GetCol() == c.GetCol() && o.GetRow() == 0 {
			col = strings.TrimSpace(o.GetText())
		}
	}
	switch {
	case row != "" && col != "":
		return row + " " + col
	case row != "":
		return row
	default:
		return col
	}
}

// tokenize lowercases and splits on anything that is not a letter or digit, then adds a REJOINED
// form for runs of short adjacent tokens.
//
// That last part is not a nicety. Producers flatten a subscript with an injected space, so a
// datasheet printing "VCCA" reaches the doc-IR as "V CCA" and a search for the symbol as printed
// finds nothing. The same document also carries "V CCB", "I CC" and every other subscripted symbol
// the part defines, so this is the common case rather than an edge one. Runs are joined only while
// the pieces are short and alphanumeric, which is the same test that repairs a flattened pin name in
// the derive stage; a real multi-word phrase is left alone.
func tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	out = append(out, fields...)
	for i := range fields {
		if len(fields[i]) > 4 {
			continue
		}
		joined := fields[i]
		for j := i + 1; j < len(fields) && j <= i+2; j++ {
			if len(fields[j]) > 4 {
				break
			}
			joined += fields[j]
			out = append(out, joined)
		}
	}
	return out
}
