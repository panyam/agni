// Package ident compares identifiers that name the same thing in two documents that spell it
// differently: a pin map against a netlist, a selected function against a vendor's pin table.
//
// It exists because that comparison is where the false warnings come from. We reproduced a shipped
// in-house checker against a large production board, and EVERY warning that run produced was a
// string-comparison artifact rather than a design defect. The pin was legal in all of them. The two
// strings naming it disagreed. A checker whose warnings are all noise is a checker people turn off,
// so the comparison is the load-bearing part rather than a detail of the rule that calls it.
//
// ONE canonical form, defined once, applied to BOTH sides. That tool carried two normalizers with
// opposite conventions (one stripped leading zeros, PTA00 to PTA0; the other padded to two digits,
// PTE7 to PTE07), on the same field in the same codebase. It worked only because each was used on its
// own side of a comparison. Everything here goes through Canonical, and a caller that reaches past it
// to compare raw strings is the bug this package exists to prevent.
package ident

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Match is how closely two identifiers agree. The three positive answers are kept apart because a
// reviewer does something different about each, and collapsing them to a bool is what turns a
// checker into one that is either noisy or silently permissive.
type Match string

const (
	// Exact: the two strings are byte-identical. Nothing was done to reach this.
	Exact Match = "exact"
	// Normalized: equal once both sides were canonicalized. A real match, and the Note says what
	// differed, because "these differ only by a character you cannot see" is the sentence that gets
	// a map fixed where a bare mismatch gets the tool switched off.
	Normalized Match = "normalized"
	// Fuzzy: equal only after factoring out tokens one side carries and the other does not. It must
	// be REPORTED rather than passed silently. Vendor tables are internally inconsistent (one IOMUX
	// sheet listed GMAC0_MII_RMII_RGMII_TXD[0] and GMAC0_MII_RGMII_TXD2 on the same pin), so an
	// author cannot be asked to guess which spelling a checker prefers, and a tool cannot claim the
	// two are the same thing without saying it inferred that.
	Fuzzy Match = "fuzzy"
	// None: no reading of either string makes them agree.
	None Match = "none"
)

// Result is one comparison and why it came out that way.
type Result struct {
	// Match is how closely the two agree.
	Match Match
	// Note says what had to be done to reach a Normalized or Fuzzy match, phrased for the person
	// reading the finding. Empty for Exact and None.
	Note string
	// A and B are the alternatives that actually matched, which is not always the strings passed in:
	// either side may be a multi-valued cell, and naming the half that matched is the difference
	// between a reviewer confirming the answer and re-deriving it.
	A, B string
}

// formatRunes are the invisible characters that survive every defence people write for them.
//
// The trap is worth stating because the obvious guard does not work. That in-house tool normalized
// with `re.sub(r'\s+', ”, s.upper())`, which looks like it handles this. U+200B ZERO WIDTH SPACE is
// a FORMAT character (category Cf), not whitespace, so `\s` does not match it in Python, in
// JavaScript, or in Go, where `\s` is ASCII-only and narrower still. The strip written to make the
// comparison tolerant walks straight past the one character that breaks it.
//
// The whole Cf category is stripped rather than a named list. The four observed in real documents
// are U+200B, U+200C, U+200D and U+FEFF, and the bidi marks alongside them in that category are the
// same hazard for the same reason. U+00A0 NBSP is NOT here: it is a space separator, so NFKC turns
// it into an ordinary space and the whitespace stage removes it.
func hasFormatRunes(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return unicode.Is(unicode.Cf, r) }) >= 0
}

func stripFormatRunes(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
}

func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// canonIndex rewrites an identifier's trailing index into one spelling: brackets removed and leading
// zeros dropped, so PTE07, PTE7 and TXD[1] against TXD1 all settle.
//
// Leading zeros are DROPPED rather than padded to a fixed width, and that is the deliberate half. A
// width is a guess about the largest index a part family will ever reach, and the two normalizers we
// found disagreed precisely because one of them had guessed. Dropping is the unique minimal form and
// guesses nothing.
//
// A trailing group that is not all digits is left alone. `DATA[1:0]` is a bus range rather than an
// index, and xschem and gEDA both write that form, so rewriting it would collapse two different
// things into one.
func canonIndex(s string) string {
	if i := strings.LastIndexByte(s, '['); i >= 0 && strings.HasSuffix(s, "]") {
		if inner := s[i+1 : len(s)-1]; inner != "" && allDigits(inner) {
			s = s[:i] + inner
		}
	}
	j := len(s)
	for j > 0 && s[j-1] >= '0' && s[j-1] <= '9' {
		j--
	}
	if j == len(s) || j == 0 {
		return s // no trailing digits, or the whole string is digits (a bare number keeps its form)
	}
	n, err := strconv.Atoi(s[j:])
	if err != nil {
		return s // longer than an int; leave it rather than guess
	}
	return s[:j] + strconv.Itoa(n)
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Canonical is the one canonical form of an identifier, and the only spelling anything in this
// package compares.
//
// The stages run in this order and each one has a reason to be where it is. NFKC first, so a
// fullwidth digit lifted out of a PDF becomes the ASCII one and a non-breaking space becomes an
// ordinary space that the whitespace stage can then remove. Format characters next, since they
// survive NFKC untouched. Upper case after that, because vendor tables mix camel-case and upper-case
// spellings of one peripheral freely. Whitespace removed rather than collapsed, because a space
// inside an identifier is noise: a multi-valued cell is split by Alternatives BEFORE this runs, so
// no separator depends on the whitespace surviving. The index last, once the digits are ASCII.
func Canonical(s string) string {
	s = norm.NFKC.String(s)
	s = stripFormatRunes(s)
	s = strings.ToUpper(s)
	s = stripSpace(s)
	return canonIndex(s)
}

// altSeparators are the ways a document writes several names in one cell. All four were observed in
// real pin maps and vendor tables, the newline inside a single spreadsheet cell included.
var altSeparators = []string{"\n", "/", ","}

// Alternatives splits a cell that may name several functions into the ones it names, always keeping
// the WHOLE cell as the first alternative.
//
// Keeping the whole string is what makes this safe on a name that legitimately contains a separator.
// `R/W` is an ordinary pin name (read/write), and so are `CS/` and `WR/`; a splitter that only
// returned the halves would compare `R` against a table and match the wrong thing while losing the
// name that was actually written. Returning both readings and letting the caller report WHICH one
// matched keeps that decidable.
//
// Both sides of a comparison are split by this same function, which is the fix for the largest class
// of false warning we measured. That tool's three match branches all looked for a separator in the
// AVAILABLE function name and never in the SELECTED one, so a map cell reading `GPIO[34] / WKPU[8]`
// could not match a table listing `GPIO[34]` and `WKPU[8]` on separate rows, even though both halves
// are legal on that pin.
func Alternatives(s string) []string {
	out := []string{strings.TrimSpace(s)}
	parts := []string{s}
	for _, sep := range altSeparators {
		var next []string
		for _, p := range parts {
			next = append(next, strings.Split(p, sep)...)
		}
		parts = next
	}
	seen := map[string]bool{out[0]: true}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// Compare reports whether two identifiers name the same thing, and what had to be done to say so.
//
// It tries the readings in order of how much they assume: byte equality, then canonical equality,
// then a token-subsequence match. The first that succeeds wins, so a pair that agrees exactly is
// never reported as having needed normalizing.
//
// Every alternative on the left is tried against every alternative on the right, which is the whole
// point of splitting both sides. The result names the pair that matched.
func Compare(a, b string) Result {
	if a == b {
		return Result{Match: Exact, A: a, B: b}
	}
	altsA, altsB := Alternatives(a), Alternatives(b)
	for _, x := range altsA {
		for _, y := range altsB {
			if x == y {
				return Result{Match: Exact, A: x, B: y, Note: alternativeNote(a, b, x, y)}
			}
		}
	}
	for _, x := range altsA {
		for _, y := range altsB {
			if Canonical(x) == Canonical(y) {
				return Result{Match: Normalized, A: x, B: y, Note: joinNotes(differences(x, y), alternativeNote(a, b, x, y))}
			}
		}
	}
	for _, x := range altsA {
		for _, y := range altsB {
			if skipped, ok := tokenSubsequence(Canonical(x), Canonical(y)); ok {
				return Result{Match: Fuzzy, A: x, B: y,
					Note: joinNotes("one side carries "+strings.Join(skipped, ", ")+" and the other does not",
						differences(x, y), alternativeNote(a, b, x, y))}
			}
		}
	}
	return Result{Match: None, A: a, B: b}
}

// alternativeNote says which half of a multi-valued cell matched, and only when a cell really had
// several halves. A note claiming an alternative was chosen where none existed is noise.
func alternativeNote(a, b, x, y string) string {
	var parts []string
	if x != strings.TrimSpace(a) {
		parts = append(parts, strconv.Quote(x)+" of "+strconv.Quote(a))
	}
	if y != strings.TrimSpace(b) {
		parts = append(parts, strconv.Quote(y)+" of "+strconv.Quote(b))
	}
	if len(parts) == 0 {
		return ""
	}
	return "matched " + strings.Join(parts, " against ")
}

// differences names every normalization that actually altered one of the two strings, so the note
// says what the reader cannot see rather than only that something was done.
//
// It reports what was APPLIED rather than trying to single out one decisive step, because two
// spellings usually differ in more than one way at once and naming only the last stage would leave a
// reader hunting for a case difference that was never the problem.
func differences(x, y string) string {
	var why []string
	if hasFormatRunes(x) || hasFormatRunes(y) {
		why = append(why, "an invisible character")
	}
	nx, ny := stripFormatRunes(norm.NFKC.String(x)), stripFormatRunes(norm.NFKC.String(y))
	if norm.NFKC.String(x) != x || norm.NFKC.String(y) != y {
		why = append(why, "a compatibility character")
	}
	if strings.ToUpper(nx) != nx || strings.ToUpper(ny) != ny {
		why = append(why, "letter case")
	}
	ux, uy := strings.ToUpper(nx), strings.ToUpper(ny)
	if stripSpace(ux) != ux || stripSpace(uy) != uy {
		why = append(why, "whitespace")
	}
	sx, sy := stripSpace(ux), stripSpace(uy)
	if canonIndex(sx) != sx || canonIndex(sy) != sy {
		why = append(why, "a zero-padded or bracketed index")
	}
	if len(why) == 0 {
		return ""
	}
	return "differ by " + strings.Join(why, " and ")
}

func joinNotes(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "; ")
}

// tokenSubsequence reports whether the shorter identifier's underscore-separated tokens appear in
// order within the longer's, naming the tokens the longer one carries extra.
//
// This is the structural answer to a vendor table that is inconsistent with itself. One IOMUX sheet
// listed GMAC0_MII_RMII_RGMII_TXD[0] and GMAC0_MII_RGMII_TXD2 on the same pin, two spellings of one
// signal family differing by an inserted interface-mode token. The map author followed the second
// pattern and got flagged.
//
// It is deliberately STRUCTURAL and names no vendor vocabulary. Hard-coding MII, RMII and RGMII as
// optional would put one vendor's interface tokens in a shared library and would miss the next
// vendor's equivalent.
//
// Both ends must match, which is what stops the rule widening into nonsense. Without that anchor
// TXD1 is a subsequence of GMAC0_MII_RGMII_TXD1 and every signal on the pin would match every other.
// A single token is never fuzzy-matched for the same reason: there is nothing to anchor.
func tokenSubsequence(x, y string) ([]string, bool) {
	a, b := strings.Split(x, "_"), strings.Split(y, "_")
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(a) < 2 || len(a) == len(b) {
		return nil, false
	}
	if a[0] != b[0] || a[len(a)-1] != b[len(b)-1] {
		return nil, false
	}
	var skipped []string
	i := 0
	for _, t := range b {
		if i < len(a) && a[i] == t {
			i++
			continue
		}
		skipped = append(skipped, t)
	}
	if i != len(a) {
		return nil, false
	}
	return skipped, true
}
