package render

import (
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// textRun is one piece of a label that draws in one style.
type textRun struct {
	text string
	kind geom.TextRun_Kind
}

// kicadEscapes are the brace escapes KiCad writes for a character a name cannot hold, as its
// UnescapeString reads them.
var kicadEscapes = map[string]string{
	"slash": "/", "backslash": `\`, "dblquote": `"`, "quote": "'", "lt": "<", "gt": ">",
	"bar": "|", "comma": ",", "colon": ":", "space": " ", "dollar": "$", "tab": "\t",
	"return": "\n",
}

// markupRuns splits text written in the named markup into its runs, joined they read as the text
// would be printed. It returns nil when the markup is unknown or the text uses none of it, and the
// text then draws as written.
//
// KiCad's markup is `~{X}` for an overbar, `_{X}` for a subscript and `^{X}` for a superscript, plus
// the brace escapes. A marker with no closing brace prints as written, as KiCad prints it. Markup
// does not nest here. A marker inside a marked run prints as its text.
func markupRuns(markup, text string) []textRun {
	if markup != "kicad" || !strings.Contains(text, "{") {
		return nil
	}
	var runs []textRun
	var plain strings.Builder
	used := false
	flush := func() {
		if plain.Len() > 0 {
			runs = append(runs, textRun{text: plain.String()})
			plain.Reset()
		}
	}
	for i := 0; i < len(text); {
		if text[i] == '{' {
			if end := strings.IndexByte(text[i:], '}'); end > 0 {
				if ch, ok := kicadEscapes[text[i+1:i+end]]; ok {
					plain.WriteString(ch)
					i += end + 1
					used = true
					continue
				}
			}
		}
		if i+1 < len(text) && text[i+1] == '{' {
			kind := geom.TextRun_KIND_PLAIN
			switch text[i] {
			case '~':
				kind = geom.TextRun_KIND_OVERBAR
			case '_':
				kind = geom.TextRun_KIND_SUBSCRIPT
			case '^':
				kind = geom.TextRun_KIND_SUPERSCRIPT
			}
			if kind != geom.TextRun_KIND_PLAIN {
				if end := closingBrace(text[i+2:]); end >= 0 {
					flush()
					inner := text[i+2 : i+2+end]
					if sub := markupRuns(markup, inner); sub != nil {
						var b strings.Builder
						for _, r := range sub {
							b.WriteString(r.text)
						}
						inner = b.String()
					}
					runs = append(runs, textRun{text: inner, kind: kind})
					i += end + 3
					used = true
					continue
				}
			}
		}
		plain.WriteByte(text[i])
		i++
	}
	flush()
	if !used {
		return nil
	}
	return runs
}

// runsText joins runs into the text they print, or returns text unchanged when there are none.
func runsText(runs []textRun, text string) string {
	if runs == nil {
		return text
	}
	var b strings.Builder
	for _, r := range runs {
		b.WriteString(r.text)
	}
	return b.String()
}

// closingBrace returns the index in s of the brace that closes a run opened just before s, counting
// any brace pairs inside it, or -1 when the run never closes.
func closingBrace(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}
