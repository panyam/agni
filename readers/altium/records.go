package altium

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
)

// props is one property-list record, its keys upper-cased. A key Altium repeats keeps its first
// value, since Board6 repeats keys and the first occurrence is the one that describes the board.
type props map[string]string

// parseProps reads a stream of property lists, each a u32 length then `|KEY=VALUE|...` text
// ending in a NUL. A length whose top byte is set marks a binary payload, which these streams do
// not hold.
func parseProps(b []byte) ([]props, error) {
	var out []props
	for off := 0; off < len(b); {
		if off+4 > len(b) {
			return nil, fmt.Errorf("property list at %d is cut short", off)
		}
		raw := binary.LittleEndian.Uint32(b[off:])
		if raw>>24 != 0 {
			return nil, fmt.Errorf("property list at %d holds a binary payload", off)
		}
		n := int(raw & 0xFFFFFF)
		off += 4
		if off+n > len(b) {
			return nil, fmt.Errorf("property list at %d runs past the end of its stream", off)
		}
		out = append(out, propsOf(b[off:off+n]))
		off += n
	}
	return out, nil
}

func propsOf(b []byte) props {
	p := props{}
	for _, tok := range strings.Split(strings.TrimRight(latin1(b), "\x00"), "|") {
		k, v, ok := strings.Cut(tok, "=")
		if !ok {
			continue
		}
		k = strings.ToUpper(strings.TrimSpace(k))
		if _, seen := p[k]; !seen {
			p[k] = strings.TrimSpace(v)
		}
	}
	return p
}

// length reads a length property such as "4149.6063mil" as internal units (0.1 µinch).
func (p props) length(k string) int64 {
	s := strings.TrimSuffix(strings.ToLower(p[k]), "mil")
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(v * 10000))
}

func (p props) float(k string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(p[k]), 64)
	if err != nil {
		return 0
	}
	return v
}

func (p props) int(k string) (int, bool) {
	v, err := strconv.Atoi(strings.TrimSpace(p[k]))
	return v, err == nil
}

// record is one binary primitive: its type byte and its subrecords' bytes.
type record struct {
	typ  byte
	subs [][]byte
}

// parseRecords reads a stream of binary primitives, each a type byte and then n subrecords of a
// u32 length and that many bytes. A record of another type, or a stream that does not end on a
// record boundary, is an error.
func parseRecords(b []byte, typ byte, n int) ([]record, error) {
	var out []record
	for off := 0; off < len(b); {
		if b[off] != typ {
			return nil, fmt.Errorf("record at %d has type 0x%02X, want 0x%02X", off, b[off], typ)
		}
		off++
		r := record{typ: typ}
		for i := 0; i < n; i++ {
			if off+4 > len(b) {
				return nil, fmt.Errorf("subrecord %d at %d is cut short", i+1, off)
			}
			l := int(binary.LittleEndian.Uint32(b[off:]))
			off += 4
			if l < 0 || off+l > len(b) {
				return nil, fmt.Errorf("subrecord %d at %d runs past the end of its stream", i+1, off)
			}
			r.subs = append(r.subs, b[off:off+l])
			off += l
		}
		out = append(out, r)
	}
	return out, nil
}

// field reads little-endian values from a subrecord, answering zero past its end. Altium appends
// fields in newer versions, so a short subrecord is an older one and its missing fields take their
// defaults.
type field []byte

func (f field) u8(off int) byte {
	if off < 0 || off >= len(f) {
		return 0
	}
	return f[off]
}

func (f field) u16(off int) uint16 {
	if off < 0 || off+2 > len(f) {
		return 0
	}
	return binary.LittleEndian.Uint16(f[off:])
}

func (f field) u32(off int) uint32 {
	if off < 0 || off+4 > len(f) {
		return 0
	}
	return binary.LittleEndian.Uint32(f[off:])
}

func (f field) i32(off int) int32 { return int32(f.u32(off)) } // #nosec G115 -- a coordinate is a signed word on disk

func (f field) f64(off int) float64 {
	if off < 0 || off+8 > len(f) {
		return 0
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(f[off:]))
}

// parseWideStrings reads WideStrings6: entries of a u32 index, a u32 byte length and that many
// bytes of UTF-16LE ending in a NUL. A length of 2 or less is an empty string with no bytes after
// it.
func parseWideStrings(b []byte) (map[uint32]string, error) {
	out := map[uint32]string{}
	for off := 0; off+8 <= len(b); {
		idx := binary.LittleEndian.Uint32(b[off:])
		n := int(binary.LittleEndian.Uint32(b[off+4:]))
		off += 8
		if n <= 2 {
			out[idx] = ""
			continue
		}
		if off+n > len(b) {
			return nil, fmt.Errorf("wide string %d runs past the end of its stream", idx)
		}
		u := make([]uint16, 0, n/2)
		for i := 0; i+1 < n; i += 2 {
			if r := binary.LittleEndian.Uint16(b[off+i:]); r != 0 {
				u = append(u, r)
			}
		}
		out[idx] = string(utf16.Decode(u))
		off += n
	}
	return out, nil
}

func latin1(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}
