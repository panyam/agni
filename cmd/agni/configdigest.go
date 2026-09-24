package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// A config digest identifies the BYTES a config resolution read, which is what lets an overlay
// composed from it say whether two runs were configured alike (agni issue 390, service.Overlay's
// Identity).
//
// It hashes CONTENT rather than stat metadata. The directories involved hold a handful of small YAML
// files, so reading them costs a fraction of the analysis the digest exists to let a caller skip, and
// content is the thing a reader actually means by "the same config". mtime and size agree for a file
// restored from a copy, which is a state a developer reaches by checking out a branch.
//
// The walk is SORTED and each entry folds in its relative path with an explicit length, so two trees
// cannot digest alike by holding the same bytes under different names, and one cannot be confused
// with another by running paths and contents together.

// digestConfig returns one digest over the bytes under paths, plus the literal names.
//
// The split is not cosmetic. paths are host paths this resolution OPENED, and their contents decide
// the run. names are things it only referred to, symbol library URIs being the case: they are handed
// to the reader rather than read here, they are not host paths at all, and statting one is an error
// rather than a digest. A run searching a different library is configured differently, so they are
// folded in by name.
func digestConfig(paths, names []string) (string, error) {
	h := sha256.New()
	for _, root := range paths {
		if err := foldPath(h, root); err != nil {
			return "", err
		}
	}
	for _, n := range names {
		foldField(h, []byte(n))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func foldPath(h io.Writer, root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("digest %s: %w", root, err)
	}
	// A root's own NAME is not folded in. The same profile set mounted at two paths is the same
	// config, and its bytes are what decide the run. What IS folded is a boundary marker and the
	// entry count, because without them two roots holding one file each produce the same byte stream
	// as one root holding both.
	foldField(h, []byte("root"))
	if !info.IsDir() {
		foldCount(h, 1)
		return foldFile(h, root, ".")
	}
	var rels []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rels = append(rels, rel)
		return nil
	})
	if err != nil {
		return fmt.Errorf("digest %s: %w", root, err)
	}
	// WalkDir is already lexical, but the order is what the digest MEANS, so it is sorted here rather
	// than inherited from a traversal whose guarantees could change.
	sort.Strings(rels)
	foldCount(h, len(rels))
	for _, rel := range rels {
		if err := foldFile(h, filepath.Join(root, rel), rel); err != nil {
			return err
		}
	}
	return nil
}

func foldFile(h io.Writer, path, rel string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("digest %s: %w", path, err)
	}
	defer f.Close()
	foldField(h, []byte(filepath.ToSlash(rel)))
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return fmt.Errorf("digest %s: %w", path, err)
	}
	foldField(h, sum.Sum(nil))
	return nil
}

// foldCount writes a count, so a tree's entries cannot be confused with another tree's.
func foldCount(h io.Writer, n int) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(n))
	h.Write(b[:])
}

// foldField writes a length-prefixed field, so adjacent fields cannot run together into one value
// that a different pair of fields could also produce.
func foldField(h io.Writer, b []byte) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(b)))
	h.Write(n[:])
	h.Write(b)
}
