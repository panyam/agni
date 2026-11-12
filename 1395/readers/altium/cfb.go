package altium

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

// cfb is a Microsoft Compound File Binary container read whole from memory, after Microsoft's
// published [MS-CFB] specification. A .PcbDoc is one: a small file system of storages (folders)
// and streams (files). The reader only reads streams; it never writes.
type cfb struct {
	b          []byte
	sector     int // sector size: 512 (version 3) or 4096 (version 4)
	mini       int // mini sector size, 64
	cutoff     uint32
	fat        []uint32
	minifat    []uint32
	ministream []byte
	entries    []cfbEntry
}

type cfbEntry struct {
	name               string
	kind               byte // 1 storage, 2 stream, 5 root
	left, right, child uint32
	start              uint32
	size               uint64
}

// Special sector numbers.
const (
	cfbEnd  = 0xFFFFFFFE // end of a chain
	cfbFree = 0xFFFFFFFF
	cfbNone = 0xFFFFFFFF // no sibling or child
)

var cfbSignature = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// openCFB reads a compound file's header, sector allocation tables and directory. Every chain is
// bounded by the number of sectors the file holds, so a damaged file stops with an error rather
// than looping.
func openCFB(b []byte) (*cfb, error) {
	if len(b) < 512 || !bytes.Equal(b[:8], cfbSignature) {
		return nil, fmt.Errorf("not a compound file (no CFB signature)")
	}
	shift, miniShift := binary.LittleEndian.Uint16(b[0x1E:]), binary.LittleEndian.Uint16(b[0x20:])
	if shift != 9 && shift != 12 {
		return nil, fmt.Errorf("compound file sector shift %d, want 9 or 12", shift)
	}
	if miniShift != 6 {
		return nil, fmt.Errorf("compound file mini sector shift %d, want 6", miniShift)
	}
	c := &cfb{b: b, sector: 1 << shift, mini: 1 << miniShift, cutoff: binary.LittleEndian.Uint32(b[0x38:])}

	// The FAT's own sectors are listed by the DIFAT: 109 entries in the header, then a chain of
	// DIFAT sectors whose last word points at the next.
	var fatSectors []uint32
	for i := 0; i < 109; i++ {
		if s := binary.LittleEndian.Uint32(b[0x4C+4*i:]); s != cfbFree {
			fatSectors = append(fatSectors, s)
		}
	}
	per := c.sector / 4
	next := binary.LittleEndian.Uint32(b[0x44:])
	for n := 0; next != cfbEnd && next != cfbFree; n++ {
		if n > c.sectors() {
			return nil, fmt.Errorf("compound file DIFAT chain does not end")
		}
		sec, err := c.sectorBytes(next)
		if err != nil {
			return nil, err
		}
		for i := 0; i < per-1; i++ {
			if s := binary.LittleEndian.Uint32(sec[4*i:]); s != cfbFree {
				fatSectors = append(fatSectors, s)
			}
		}
		next = binary.LittleEndian.Uint32(sec[4*(per-1):])
	}
	for _, s := range fatSectors {
		sec, err := c.sectorBytes(s)
		if err != nil {
			return nil, err
		}
		for i := 0; i < per; i++ {
			c.fat = append(c.fat, binary.LittleEndian.Uint32(sec[4*i:]))
		}
	}

	dir, err := c.chain(binary.LittleEndian.Uint32(b[0x30:]), -1)
	if err != nil {
		return nil, fmt.Errorf("compound file directory: %w", err)
	}
	for off := 0; off+128 <= len(dir); off += 128 {
		e := dir[off : off+128]
		n := int(binary.LittleEndian.Uint16(e[64:]))
		if n > 64 {
			n = 64
		}
		u := make([]uint16, 0, n/2)
		for i := 0; i+1 < n; i += 2 {
			if r := binary.LittleEndian.Uint16(e[i:]); r != 0 {
				u = append(u, r)
			}
		}
		size := binary.LittleEndian.Uint64(e[120:])
		if shift == 9 {
			size &= 0xFFFFFFFF // version 3 files may leave the high word uninitialised
		}
		c.entries = append(c.entries, cfbEntry{
			name:  string(utf16.Decode(u)),
			kind:  e[66],
			left:  binary.LittleEndian.Uint32(e[68:]),
			right: binary.LittleEndian.Uint32(e[72:]),
			child: binary.LittleEndian.Uint32(e[76:]),
			start: binary.LittleEndian.Uint32(e[116:]),
			size:  size,
		})
	}
	if len(c.entries) == 0 || c.entries[0].kind != 5 {
		return nil, fmt.Errorf("compound file has no root entry")
	}

	// Streams under the cutoff live in the mini stream, the root entry's own data, allocated in
	// 64-byte mini sectors by the mini FAT.
	root := c.entries[0]
	rootSize, err := c.size(root)
	if err != nil {
		return nil, err
	}
	if c.ministream, err = c.chain(root.start, rootSize); err != nil {
		return nil, fmt.Errorf("compound file mini stream: %w", err)
	}
	mf, err := c.chain(binary.LittleEndian.Uint32(b[0x3C:]), -1)
	if err != nil {
		return nil, fmt.Errorf("compound file mini FAT: %w", err)
	}
	for i := 0; i+4 <= len(mf); i += 4 {
		c.minifat = append(c.minifat, binary.LittleEndian.Uint32(mf[i:]))
	}
	return c, nil
}

// size is an entry's byte length. No stream can be longer than the file holding it, so a larger
// declared size marks a damaged file.
func (c *cfb) size(e cfbEntry) (int64, error) {
	if e.size > uint64(len(c.b)) {
		return 0, fmt.Errorf("compound file entry %q declares %d bytes in a %d-byte file", e.name, e.size, len(c.b))
	}
	return int64(e.size), nil // #nosec G115 -- bounded by the file's length just above
}

func (c *cfb) sectors() int { return len(c.b)/c.sector - 1 }

func (c *cfb) sectorBytes(s uint32) ([]byte, error) {
	off := (int64(s) + 1) * int64(c.sector)
	if s >= cfbFree-4 || off+int64(c.sector) > int64(len(c.b)) {
		return nil, fmt.Errorf("compound file sector %d is outside the file", s)
	}
	return c.b[off : off+int64(c.sector)], nil
}

// chain concatenates the sectors of a FAT chain from start, cut to size when size is not negative.
func (c *cfb) chain(start uint32, size int64) ([]byte, error) {
	var out []byte
	for s, n := start, 0; s != cfbEnd && s != cfbFree; n++ {
		if n > c.sectors() {
			return nil, fmt.Errorf("sector chain does not end")
		}
		sec, err := c.sectorBytes(s)
		if err != nil {
			return nil, err
		}
		out = append(out, sec...)
		if int(s) >= len(c.fat) {
			return nil, fmt.Errorf("sector %d has no FAT entry", s)
		}
		s = c.fat[s]
	}
	if size >= 0 {
		if int64(len(out)) < size {
			return nil, fmt.Errorf("stream is %d bytes long and its chain holds %d", size, len(out))
		}
		out = out[:size]
	}
	return out, nil
}

func (c *cfb) miniChain(start uint32, size int64) ([]byte, error) {
	out := make([]byte, 0, size)
	for s, n := start, 0; s != cfbEnd && s != cfbFree && int64(len(out)) < size; n++ {
		if n > len(c.minifat) {
			return nil, fmt.Errorf("mini sector chain does not end")
		}
		off := int(s) * c.mini
		if off+c.mini > len(c.ministream) || int(s) >= len(c.minifat) {
			return nil, fmt.Errorf("mini sector %d is outside the mini stream", s)
		}
		out = append(out, c.ministream[off:off+c.mini]...)
		s = c.minifat[s]
	}
	if int64(len(out)) < size {
		return nil, fmt.Errorf("stream is %d bytes long and its mini chain holds %d", size, len(out))
	}
	return out[:size], nil
}

// stream returns the bytes of the stream at a slash-separated path, such as "Board6/Data". Names
// compare without regard to case, as the format does. ok is false when no stream has that path.
func (c *cfb) stream(path string) (data []byte, ok bool, err error) {
	dir := uint32(0)
	parts := strings.Split(path, "/")
	for i, part := range parts {
		idx, found := c.find(c.entries[dir].child, part, 0)
		if !found {
			return nil, false, nil
		}
		e := c.entries[idx]
		if i < len(parts)-1 {
			if e.kind != 1 {
				return nil, false, nil
			}
			dir = idx
			continue
		}
		if e.kind != 2 {
			return nil, false, nil
		}
		n, err := c.size(e)
		if err != nil {
			return nil, true, err
		}
		if e.size < uint64(c.cutoff) {
			data, err = c.miniChain(e.start, n)
		} else {
			data, err = c.chain(e.start, n)
		}
		if err != nil {
			return nil, true, fmt.Errorf("stream %s: %w", path, err)
		}
		return data, true, nil
	}
	return nil, false, nil
}

// find searches the sibling tree rooted at idx for name. The tree is red-black ordered by a key
// that is not plain string order, so every node is visited; a depth bound guards a damaged file.
func (c *cfb) find(idx uint32, name string, depth int) (uint32, bool) {
	if idx == cfbNone || int(idx) >= len(c.entries) || depth > len(c.entries) {
		return 0, false
	}
	e := c.entries[idx]
	if strings.EqualFold(e.name, name) {
		return idx, true
	}
	if i, ok := c.find(e.left, name, depth+1); ok {
		return i, true
	}
	return c.find(e.right, name, depth+1)
}
