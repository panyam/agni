package odb

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// lines yields a file's records with comments and blank lines dropped. ODB++ comments start with
// `#` at the head of a line, and a record never does.
func lines(b []byte, each func(n int, line string) error) error {
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(strings.TrimSuffix(sc.Text(), "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := each(n, line); err != nil {
			return err
		}
	}
	return sc.Err()
}

// fields splits a record into whitespace-separated words, keeping a single-quoted string whole
// (with its quotes removed), since a property value or a text record's string can hold spaces.
// The attribute tail after `;` is returned apart, unsplit.
func fields(line string) (words []string, attrs string) {
	var cur strings.Builder
	inQuote, have := false, false
	flush := func() {
		if have {
			words = append(words, cur.String())
			cur.Reset()
			have = false
		}
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case inQuote:
			if c == '\'' {
				inQuote = false
				continue
			}
			cur.WriteByte(c)
		case c == '\'':
			inQuote, have = true, true
		case c == ';':
			flush()
			return words, strings.TrimSpace(line[i+1:])
		case c == ' ' || c == '\t':
			flush()
		default:
			cur.WriteByte(c)
			have = true
		}
	}
	flush()
	return words, ""
}

// structured is one block of a structured-text file (`LAYER { KEY=VALUE ... }`), or the top level
// of a file of bare KEY=VALUE lines such as misc/info.
type structured struct {
	kind string
	kv   map[string]string
}

// parseStructured reads a structured-text file into its blocks. Keys are upper-cased, since
// producers disagree on case and the spec does not.
func parseStructured(b []byte) ([]structured, error) {
	top := structured{kv: map[string]string{}}
	var out []structured
	var cur *structured
	err := lines(b, func(n int, line string) error {
		switch {
		case strings.HasSuffix(line, "{"):
			if cur != nil {
				return fmt.Errorf("line %d: a block opens inside another", n)
			}
			cur = &structured{kind: strings.ToUpper(strings.TrimSpace(strings.TrimSuffix(line, "{"))), kv: map[string]string{}}
		case line == "}":
			if cur == nil {
				return fmt.Errorf("line %d: a block closes that never opened", n)
			}
			out = append(out, *cur)
			cur = nil
		default:
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				return nil
			}
			dst := &top
			if cur != nil {
				dst = cur
			}
			dst.kv[strings.ToUpper(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if cur != nil {
		return nil, fmt.Errorf("a %s block never closes", cur.kind)
	}
	if len(top.kv) > 0 {
		out = append([]structured{top}, out...)
	}
	return out, nil
}

// matrixLayer is one LAYER block of the matrix.
type matrixLayer struct {
	row       int
	context   string // BOARD or MISC
	typ       string // SIGNAL, POWER_GROUND, MIXED, SOLDER_MASK, SILK_SCREEN, SOLDER_PASTE, DRILL, ROUT, DOCUMENT, COMPONENT, DIELECTRIC
	name      string
	polarity  string
	startName string // a drill or rout layer's span
	endName   string
}

// matrix is the parsed matrix/matrix file, its layers in row order.
type matrix struct {
	steps  []string
	layers []matrixLayer
}

func parseMatrix(b []byte) (*matrix, error) {
	blocks, err := parseStructured(b)
	if err != nil {
		return nil, fmt.Errorf("odb: matrix: %w", err)
	}
	m := &matrix{}
	for _, bl := range blocks {
		switch bl.kind {
		case "STEP":
			if name := bl.kv["NAME"]; name != "" {
				m.steps = append(m.steps, name)
			}
		case "LAYER":
			row, _ := strconv.Atoi(bl.kv["ROW"])
			m.layers = append(m.layers, matrixLayer{
				row:       row,
				context:   strings.ToUpper(bl.kv["CONTEXT"]),
				typ:       strings.ToUpper(bl.kv["TYPE"]),
				name:      bl.kv["NAME"],
				polarity:  strings.ToUpper(bl.kv["POLARITY"]),
				startName: bl.kv["START_NAME"],
				endName:   bl.kv["END_NAME"],
			})
		}
	}
	sort.SliceStable(m.layers, func(a, b int) bool { return m.layers[a].row < m.layers[b].row })
	return m, nil
}

// isCopper reports whether a matrix layer carries copper the board is routed on.
func (l matrixLayer) isCopper() bool {
	if l.context != "BOARD" {
		return false
	}
	switch l.typ {
	case "SIGNAL", "POWER_GROUND", "MIXED":
		return true
	}
	return false
}

// units is a file's length unit: the factor from a coordinate to nanometres, and from a symbol
// dimension to nanometres. ODB++ writes coordinates in millimetres or inches and symbol sizes in
// microns or mils to match.
type units struct {
	coord, symbol float64
}

var (
	unitsMM   = units{coord: 1e6, symbol: 1e3}
	unitsInch = units{coord: 25.4e6, symbol: 25.4e3}
)

// unitsOf reads a file's `UNITS=` line (or the older `U MM` / `U INCH` record), defaulting to
// inches, the spec's default when a file states none.
func unitsOf(b []byte) units {
	u := unitsInch
	_ = lines(b, func(_ int, line string) error {
		up := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(up, "UNITS="):
			if strings.TrimSpace(up[len("UNITS="):]) == "MM" {
				u = unitsMM
			} else {
				u = unitsInch
			}
			return errStop
		case up == "U MM":
			u = unitsMM
			return errStop
		case up == "U INCH":
			u = unitsInch
			return errStop
		}
		return nil
	})
	return u
}

var errStop = fmt.Errorf("stop")

func atof(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// atoi parses a record's integer field.
func atoi(s string) (int, error) {
	return strconv.Atoi(s)
}
