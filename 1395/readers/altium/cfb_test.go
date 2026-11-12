package altium

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"
)

// buildCFB writes a version 3 compound file holding a 5,000-byte stream "Big" at the root and a
// 100-byte stream "Data" inside a storage "S". "Big" is over the 4,096-byte cutoff, so it lives in
// regular sectors, and "Data" lives in the mini stream.
func buildCFB(big, small []byte) []byte {
	const sec = 512
	put32 := func(b []byte, off int, v uint32) { binary.LittleEndian.PutUint32(b[off:], v) }
	hdr := make([]byte, sec)
	copy(hdr, cfbSignature)
	binary.LittleEndian.PutUint16(hdr[0x18:], 0x3E)
	binary.LittleEndian.PutUint16(hdr[0x1A:], 3)
	binary.LittleEndian.PutUint16(hdr[0x1C:], 0xFFFE)
	binary.LittleEndian.PutUint16(hdr[0x1E:], 9)
	binary.LittleEndian.PutUint16(hdr[0x20:], 6)
	put32(hdr, 0x2C, 1)      // FAT sectors
	put32(hdr, 0x30, 1)      // first directory sector
	put32(hdr, 0x38, 4096)   // mini stream cutoff
	put32(hdr, 0x3C, 2)      // first mini FAT sector
	put32(hdr, 0x40, 1)      // mini FAT sectors
	put32(hdr, 0x44, cfbEnd) // no DIFAT sectors
	for i := 0; i < 109; i++ {
		put32(hdr, 0x4C+4*i, cfbFree)
	}
	put32(hdr, 0x4C, 0) // the FAT is sector 0

	bigSectors := (len(big) + sec - 1) / sec
	fat := make([]byte, sec)
	for i := 0; i < sec/4; i++ {
		put32(fat, 4*i, cfbFree)
	}
	put32(fat, 0, 0xFFFFFFFD) // sector 0 holds the FAT
	put32(fat, 4, cfbEnd)     // sector 1, the directory
	put32(fat, 8, cfbEnd)     // sector 2, the mini FAT
	put32(fat, 12, cfbEnd)    // sector 3, the mini stream
	for i := 0; i < bigSectors; i++ {
		next := uint32(4 + i + 1)
		if i == bigSectors-1 {
			next = cfbEnd
		}
		put32(fat, 4*(4+i), next)
	}

	entry := func(name string, kind byte, left, right, child, start uint32, size uint64) []byte {
		e := make([]byte, 128)
		u := utf16.Encode([]rune(name))
		for i, r := range u {
			binary.LittleEndian.PutUint16(e[2*i:], r)
		}
		binary.LittleEndian.PutUint16(e[64:], uint16(2*len(u)+2))
		e[66] = kind
		put32(e, 68, left)
		put32(e, 72, right)
		put32(e, 76, child)
		put32(e, 116, start)
		binary.LittleEndian.PutUint64(e[120:], size)
		return e
	}
	miniSectors := (len(small) + 63) / 64
	var dir []byte
	dir = append(dir, entry("Root Entry", 5, cfbNone, cfbNone, 1, 3, uint64(miniSectors*64))...)
	dir = append(dir, entry("Big", 2, cfbNone, 2, cfbNone, 4, uint64(len(big)))...)
	dir = append(dir, entry("S", 1, cfbNone, cfbNone, 3, 0, 0)...)
	dir = append(dir, entry("Data", 2, cfbNone, cfbNone, cfbNone, 0, uint64(len(small)))...)

	minifat := make([]byte, sec)
	for i := 0; i < sec/4; i++ {
		put32(minifat, 4*i, cfbFree)
	}
	for i := 0; i < miniSectors; i++ {
		next := uint32(i + 1)
		if i == miniSectors-1 {
			next = cfbEnd
		}
		put32(minifat, 4*i, next)
	}
	ministream := make([]byte, sec)
	copy(ministream, small)

	out := append([]byte{}, hdr...)
	out = append(out, fat...)
	out = append(out, dir...)
	out = append(out, minifat...)
	out = append(out, ministream...)
	body := make([]byte, bigSectors*sec)
	copy(body, big)
	return append(out, body...)
}

func TestACompoundFileReadsBothKindsOfStream(t *testing.T) {
	big := bytes.Repeat([]byte("0123456789"), 500)
	small := []byte(strings.Repeat("ab", 50))
	c, err := openCFB(buildCFB(big, small))
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := c.stream("Big")
	if err != nil || !ok || !bytes.Equal(got, big) {
		t.Errorf("Big: %d bytes, found %v, err %v; want the 5,000 bytes written", len(got), ok, err)
	}
	got, ok, err = c.stream("s/data") // names compare without case
	if err != nil || !ok || !bytes.Equal(got, small) {
		t.Errorf("S/Data: %q, found %v, err %v; want the 100 bytes written", got, ok, err)
	}
	if _, ok, _ := c.stream("S/Missing"); ok {
		t.Error("a missing stream was found")
	}
	if _, ok, _ := c.stream("Big/Data"); ok {
		t.Error("a stream was opened as a storage")
	}
}

func TestADamagedCompoundFileIsAnError(t *testing.T) {
	good := buildCFB(bytes.Repeat([]byte("x"), 5000), []byte("small"))
	if _, err := openCFB([]byte("not a compound file at all, and long enough")); err == nil {
		t.Error("a non-CFB file opened")
	}
	cut := good[:len(good)-1024] // the big stream's last sectors are gone
	c, err := openCFB(cut)
	if err != nil {
		t.Fatalf("the header and directory are intact, so it should open: %v", err)
	}
	if _, _, err := c.stream("Big"); err == nil {
		t.Error("a stream whose sectors are past the end of the file read without an error")
	}
	loop := append([]byte{}, good...)
	binary.LittleEndian.PutUint32(loop[512+4*8:], 4) // the big stream's last sector points back at its first
	c, err = openCFB(loop)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.stream("Big"); err == nil {
		t.Error("a sector chain that loops read without an error")
	}
}
