// Package odb reads an ODB++ job into the neutral IR (agni.v1.ir) and the board-geometry sidecar
// (agni.v1.geom). It is core, runtime-agnostic Go (CONSTRAINTS C1): a job is a directory tree,
// so the reader takes an fs.FS rooted at the job and opens nothing itself. FromArchive gives a
// .tgz or .zip the same shape.
//
// ODB++ is a manufacturing hand-off, so like IPC-2581 it carries the netlist (components, their
// pins and the net each pin is on) and the physical board (layers, copper, outline, placements),
// and no schematic. The grammar is line records in plain text files:
//
//	matrix/matrix                         the layer stack and the steps
//	steps/<step>/eda/data                 nets, packages and which copper feature is on which net
//	steps/<step>/layers/comp_+_top/components   placed parts and their pins (comp_+_bot for the back)
//	steps/<step>/layers/<layer>/features  each layer's lines, arcs, pads and surfaces
//	steps/<step>/profile                  the board outline
//
// Coordinates are Y-up, as the geom contract is, so they map with no flip. Angles are clockwise,
// so they are negated into the contract's counter-clockwise degrees.
package odb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// job is one opened ODB++ job: the file system it lives in, the step being read, and the parsed
// matrix. Every file is opened through open, which tries the spelling the matrix gives and then
// the lower-case one the spec requires on disk.
type job struct {
	fsys   fs.FS
	step   string
	matrix *matrix
}

// openJob reads the matrix and picks the step to read: the one named "pcb" when the job has it,
// as KiCad, Altium and Xpedition all name the board step, and otherwise the first the matrix lists.
func openJob(fsys fs.FS) (*job, error) {
	j := &job{fsys: fsys}
	b, err := j.read("matrix/matrix")
	if err != nil {
		return nil, fmt.Errorf("odb: not an ODB++ job (%w)", err)
	}
	m, err := parseMatrix(b)
	if err != nil {
		return nil, err
	}
	j.matrix = m
	if len(m.steps) == 0 {
		return nil, errors.New("odb: the matrix lists no step")
	}
	j.step = m.steps[0]
	for _, s := range m.steps {
		if strings.EqualFold(s, "pcb") {
			j.step = s
			break
		}
	}
	return j, nil
}

// stepFile is the path of a file inside the step being read.
func (j *job) stepFile(rel string) string {
	return path.Join("steps", strings.ToLower(j.step), rel)
}

// layerFile is the path of a file inside one layer's directory.
func (j *job) layerFile(layer, file string) string {
	return j.stepFile(path.Join("layers", strings.ToLower(layer), file))
}

// read returns a file's bytes. A member the producer compressed (`features.z`, the Unix compress
// format Valor-lineage tools write) is decompressed, and a gzipped one (`.gz`) too.
func (j *job) read(name string) ([]byte, error) {
	for _, cand := range []string{name, strings.ToLower(name)} {
		b, err := fs.ReadFile(j.fsys, cand)
		if err == nil {
			return b, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		for _, ext := range []string{".z", ".Z"} {
			if zb, zerr := fs.ReadFile(j.fsys, cand+ext); zerr == nil {
				out, derr := uncompress(zb)
				if derr != nil {
					return nil, fmt.Errorf("odb: %s%s: %w", cand, ext, derr)
				}
				return out, nil
			}
		}
		if gb, gerr := fs.ReadFile(j.fsys, cand+".gz"); gerr == nil {
			out, derr := gunzip(gb)
			if derr != nil {
				return nil, fmt.Errorf("odb: %s.gz: %w", cand, derr)
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("%s: %w", name, fs.ErrNotExist)
}

// readOptional is read for a file a job may leave out, which answers nil rather than an error.
func (j *job) readOptional(name string) ([]byte, error) {
	b, err := j.read(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

// FindRoot returns the directory inside fsys that holds an ODB++ job, the one with a
// matrix/matrix file, searching the top level and then each directory below it. An archive
// usually wraps the job in a folder named for it, and sometimes in two.
func FindRoot(fsys fs.FS) (string, error) {
	var found []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.Count(p, "/") > 3 {
				return fs.SkipDir
			}
			return nil
		}
		if strings.EqualFold(path.Base(p), "matrix") && strings.EqualFold(path.Base(path.Dir(p)), "matrix") {
			found = append(found, path.Dir(path.Dir(p)))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(found) == 0 {
		return "", errors.New("odb: no matrix/matrix file, so this is not an ODB++ job")
	}
	sort.Slice(found, func(a, b int) bool { return len(found[a]) < len(found[b]) })
	return found[0], nil
}

// IsMatrixPath reports whether a path names an ODB++ job's matrix file, `<job>/matrix/matrix`,
// which is the file a job is entered by since a job is a directory.
func IsMatrixPath(p string) bool {
	p = strings.ReplaceAll(p, "\\", "/")
	return strings.EqualFold(path.Base(p), "matrix") && strings.EqualFold(path.Base(path.Dir(p)), "matrix")
}

// JobDir returns the job directory a matrix path belongs to.
func JobDir(matrixPath string) string {
	p := strings.ReplaceAll(matrixPath, "\\", "/")
	return path.Dir(path.Dir(p))
}

// readAll is io.ReadAll, named for the archive readers.
func readAll(r io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	return buf.Bytes(), err
}
