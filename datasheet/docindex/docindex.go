// Package docindex answers questions INSIDE one datasheet. Given a phrase, it finds which passages or
// table cells of this document are about it, and exactly where they are.
//
// The doc-IR is addressable but not searchable. A page, block or cell can be located once you know
// which one you want, and "where does this document state the VCC range" has no form at all.
//
// # This index must never be reachable from a check
//
// Two lookups get conflated once a retrieval index exists, and conflating them puts a confident
// wrong answer in front of a design review:
//
//   - A FACT lookup is exact, keyed by part and symbol. param.LoadSet refuses a near-miss MPN,
//     because a near-miss is a different part until a human says otherwise.
//   - PASSAGE retrieval is fuzzy by construction, and puts a person or an extractor in front of
//     the right paragraph.
//
// The engine reads the first, and people and extractors read the second. So this package lives
// under datasheet/ beside the authoring surfaces and is imported by none of core/check, core/review
// or core/query. A check that needs to consult it is a design error.
//
// # Derived, never a second source of truth
//
// An Index is built from a doc-IR and holds no state the doc-IR does not. It can be rebuilt at any
// time, so a stale index is a performance problem rather than a correctness one.
package docindex

import (
	"sort"
	"strings"
	"unicode"

	docpb "github.com/panyam/agni/datasheet/gen/go/agni/v1/doc"
)

// Hit is one passage the index matched, located precisely enough to highlight. Every hit names the
// doc-IR region it came from, and a table hit names the cell, because verification has to be possible
// at a glance or a reviewer waves through whatever they are shown.
type Hit struct {
	Page int32
	// RegionID is the doc-IR text block or table id, which is what a viewer highlights.
	RegionID string
	// Row and Col locate a table cell, and are -1 for a text block.
	Row, Col int32
	// Text is the matched content verbatim, so a caller quotes the document rather than paraphrasing.
	Text string
	// Context is a cell's row label and column header, so "3.6" reads as "VCCA MAX 3.6". Empty for a
	// text block, which carries its own context.
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
	// docFreq counts how many entries contain a term, so a term on every page ("voltage")
	// contributes far less than one that appears twice.
	docFreq map[string]int
}

// Build indexes every text block and table cell of a document. Blocks with no text are skipped, and
// the same document always yields the same index, so a hit list is stable between runs.
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
	// only the label.
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
// Scoring is lexical rather than learned. Datasheet vocabulary is small and conventional, and a
// lexical hit can be justified to the person deciding whether to trust it.
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
			// Rarity outweighs repetition, so a long block repeating a common word does not
			// outrank a specific hit.
			idf := n / float64(1+ix.docFreq[t])
			score += idf * (1 + float64(c-1)*0.2)
		}
		if matched == 0 {
			continue
		}
		// Prefer entries that matched MORE of the query, and shorter ones among equals, so a
		// cell stating the fact beats a paragraph mentioning it.
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

// cellContext returns a cell's row label and column header, taken from the table's first column and
// header row.
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
// Producers flatten a subscript with an injected space, so a datasheet printing "VCCA" reaches the
// doc-IR as "V CCA" and a search for the symbol as printed would find nothing. Every subscripted
// symbol in a datasheet does this. Runs are joined only while the pieces are short (up to four
// characters, three pieces at most), the same test that repairs a flattened pin name in the derive
// stage, so a real multi-word phrase is left alone.
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
