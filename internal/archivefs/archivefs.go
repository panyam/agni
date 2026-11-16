// Package archivefs presents an archive as the folder it was made from (agni issue 1591). A zip, a
// tar or a 7z holds many named files, so it reads as a directory of the same name. A tar is usually
// wrapped in a stream compressor, gzip, bzip2, xz or zstd, which is a content encoding of one stream,
// so it is peeled off first and the tar inside chosen by its bytes. No reader knows an archive was
// involved. A reader that spans files, such as an ODB++ job or a KiCad hierarchy, reads the folder as
// it reads any other.
//
// An archive is hostile input (C1). Every container kind is held to the same bounds while it decodes:
// MaxMembers files, MaxMemberBytes for any one file, and MaxExpandedBytes in all. A member whose path
// climbs out of the archive is read from inside it, and an encrypted archive fails with ErrEncrypted.
package archivefs

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// The bounds every archive is held to. An ODB++ job is text, and a large board's job runs to a few
// hundred megabytes, so a gigabyte in all leaves room and still refuses a decompression bomb.
const (
	MaxExpandedBytes = 1 << 30
	MaxMemberBytes   = 1 << 29
	MaxMembers       = 100_000
)

// The bounds as the decoders read them. A test lowers them, so a bomb can be small.
var (
	maxExpanded int64 = MaxExpandedBytes
	maxMember   int64 = MaxMemberBytes
	maxMembers        = MaxMembers
)

var (
	// ErrNotArchive says the bytes are no container this package reads. A compressed stream that
	// holds one file rather than a tar is not an archive either.
	ErrNotArchive = errors.New("archivefs: not a zip, tar or 7z archive")
	// ErrEncrypted says the archive needs a password, which agni never has.
	ErrEncrypted = errors.New("archivefs: the archive is encrypted")
	// ErrTooLarge says the archive expands past MaxExpandedBytes, or a member past MaxMemberBytes.
	ErrTooLarge = errors.New("archivefs: the archive expands past its bound")
	// ErrTooMany says the archive holds more than MaxMembers files.
	ErrTooMany = errors.New("archivefs: the archive holds too many files")
)

// suffixes are the names a host treats as an archive to try. The bytes still choose the decoder, so a
// .zip that is really a .tgz opens, and a .zip that is not an archive stays a file.
var suffixes = []string{".zip", ".7z", ".tar", ".tgz", ".tar.gz", ".tbz2", ".tar.bz2", ".txz", ".tar.xz", ".tzst", ".tar.zst"}

// Candidate reports whether a name is one a host should try to open as an archive.
func Candidate(name string) bool {
	lower := strings.ToLower(name)
	for _, s := range suffixes {
		if strings.HasSuffix(lower, s) && len(lower) > len(s) {
			return true
		}
	}
	return false
}

// Open presents an archive's bytes as a file system holding its members. The archive's kind comes
// from its bytes. A zip reads its members on demand. Every other kind decodes whole into memory,
// since a tar and a solid 7z can only be read from the front.
func Open(b []byte) (fs.FS, error) {
	switch {
	case bytes.HasPrefix(b, []byte("PK\x03\x04")) || bytes.HasPrefix(b, []byte("PK\x05\x06")):
		return openZip(b)
	case bytes.HasPrefix(b, []byte("7z\xbc\xaf\x27\x1c")):
		return open7z(b)
	case isTar(b):
		return untar(bytes.NewReader(b))
	}
	r, err := decompressor(b)
	if err != nil {
		return nil, err
	}
	// The peeled stream must be a tar. Its first block says so before the rest is decoded.
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("archivefs: %w", err)
	}
	if !isTar(head[:n]) {
		return nil, ErrNotArchive
	}
	return untar(io.MultiReader(bytes.NewReader(head[:n]), r))
}

// Sniff reports whether bytes start as an archive or a compressed stream does, from their first
// block alone. A compressed stream may still hold one file rather than a tar, which Open refuses.
func Sniff(head []byte) bool {
	for _, sig := range [][]byte{[]byte("PK\x03\x04"), []byte("PK\x05\x06"), []byte("7z\xbc\xaf\x27\x1c"), {0x1f, 0x8b}, []byte("BZh"), {0xfd, '7', 'z', 'X', 'Z', 0x00}, {0x28, 0xb5, 0x2f, 0xfd}} {
		if bytes.HasPrefix(head, sig) {
			return true
		}
	}
	return isTar(head)
}

// isTar reports a POSIX or GNU tar header, whose magic sits at byte 257.
func isTar(b []byte) bool {
	return len(b) >= 262 && string(b[257:262]) == "ustar"
}

// decompressor peels a stream compressor off b, or answers ErrNotArchive.
func decompressor(b []byte) (io.Reader, error) {
	br := bytes.NewReader(b)
	switch {
	case bytes.HasPrefix(b, []byte{0x1f, 0x8b}):
		r, err := gzip.NewReader(br)
		if err != nil {
			return nil, fmt.Errorf("archivefs: gzip: %w", err)
		}
		return r, nil
	case bytes.HasPrefix(b, []byte("BZh")):
		return bzip2.NewReader(br), nil
	case bytes.HasPrefix(b, []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}):
		r, err := xz.NewReader(br)
		if err != nil {
			return nil, fmt.Errorf("archivefs: xz: %w", err)
		}
		return r, nil
	case bytes.HasPrefix(b, []byte{0x28, 0xb5, 0x2f, 0xfd}):
		r, err := zstd.NewReader(br, zstd.WithDecoderConcurrency(1))
		if err != nil {
			return nil, fmt.Errorf("archivefs: zstd: %w", err)
		}
		return r.IOReadCloser(), nil
	}
	return nil, ErrNotArchive
}

// unsigned is a bound as a zip or 7z directory states sizes.
func unsigned(bound int64) uint64 {
	return uint64(bound) // #nosec G115 -- the bounds are positive constants, or a test's smaller ones
}

// budget counts what an archive has expanded to so far.
type budget struct {
	members int
	bytes   int64
}

// take reads one member through the bounds, so a member that lies about its size still stops at
// MaxMemberBytes.
func (b *budget) take(r io.Reader) ([]byte, error) {
	if b.members++; b.members > maxMembers {
		return nil, ErrTooMany
	}
	out, err := io.ReadAll(io.LimitReader(r, maxMember+1))
	if err != nil {
		return nil, err
	}
	if int64(len(out)) > maxMember {
		return nil, ErrTooLarge
	}
	if b.bytes += int64(len(out)); b.bytes > maxExpanded {
		return nil, ErrTooLarge
	}
	return out, nil
}

func untar(r io.Reader) (fs.FS, error) {
	files := map[string][]byte{}
	var b budget
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return newMemFS(files), nil
		}
		if err != nil {
			return nil, fmt.Errorf("archivefs: tar: %w", err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		name, keep := member(h.Name)
		data, err := b.take(tr)
		if err != nil {
			return nil, err
		}
		if keep {
			files[name] = data
		}
	}
}

// openZip checks a zip's directory against the bounds and serves its members on demand. archive/zip
// refuses to read past a member's declared size, so declared sizes within the bounds hold the reads
// within them too.
func openZip(b []byte) (fs.FS, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, fmt.Errorf("archivefs: zip: %w", err)
	}
	if len(zr.File) > maxMembers {
		return nil, ErrTooMany
	}
	var total uint64
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		if f.Flags&0x1 != 0 {
			return nil, ErrEncrypted
		}
		if f.FileInfo().IsDir() {
			continue
		}
		name, keep := member(f.Name)
		if f.UncompressedSize64 > unsigned(maxMember) {
			return nil, ErrTooLarge
		}
		if total += f.UncompressedSize64; total > unsigned(maxExpanded) {
			return nil, ErrTooLarge
		}
		if keep {
			files[name] = f
		}
	}
	return newZipFS(files), nil
}

func open7z(b []byte) (fs.FS, error) {
	zr, err := sevenzip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, sevenErr(err)
	}
	if len(zr.File) > maxMembers {
		return nil, ErrTooMany
	}
	var declared uint64
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() {
			if declared += f.UncompressedSize; declared > unsigned(maxExpanded) {
				return nil, ErrTooLarge
			}
		}
	}
	files := map[string][]byte{}
	var bud budget
	// Members are read in the archive's order, so a solid block decodes once from its front.
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name, keep := member(f.Name)
		rc, err := f.Open()
		if err != nil {
			return nil, sevenErr(err)
		}
		data, err := bud.take(rc)
		rc.Close()
		if err != nil {
			return nil, sevenErr(err)
		}
		if keep {
			files[name] = data
		}
	}
	return newMemFS(files), nil
}

// sevenErr names an encrypted 7z as such. The library hints at encryption on the error it wraps.
func sevenErr(err error) error {
	var re sevenzip.ReadError
	if errors.As(err, &re) && re.Encrypted {
		return ErrEncrypted
	}
	var rp *sevenzip.ReadError
	if errors.As(err, &rp) && rp.Encrypted {
		return ErrEncrypted
	}
	if errors.Is(err, ErrTooLarge) || errors.Is(err, ErrTooMany) {
		return err
	}
	return fmt.Errorf("archivefs: 7z: %w", err)
}

// member turns an archive member's name into an fs.ValidPath, kept inside the archive: a name that
// climbs out or starts at the root is read from the archive's own top. keep is false for a name with
// nothing left, and for macOS's AppleDouble shadows, which sort ahead of the files they shadow.
func member(name string) (clean string, keep bool) {
	clean = strings.TrimPrefix(path.Clean("/"+strings.ReplaceAll(name, "\\", "/")), "/")
	if clean == "" || !fs.ValidPath(clean) {
		return "", false
	}
	for _, el := range strings.Split(clean, "/") {
		if strings.HasPrefix(el, "._") || el == "__MACOSX" {
			return "", false
		}
	}
	return clean, true
}

// Gunzip decodes one gzip stream, held to MaxMemberBytes. A format that gzips single members, as an
// ODB++ job may, decodes them through it.
func Gunzip(b []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	var bud budget
	return bud.take(zr)
}
