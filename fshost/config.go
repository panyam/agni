package fshost

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"github.com/panyam/agni/stdlib/lib"
	"github.com/panyam/agni/stdlib/profiles"
	"github.com/panyam/jaala/datalog"
)

// ConfigResolver is the service.ConfigResolver over a mount table. It reads the interface profiles,
// seeded parameters, symbol libraries and derived-relation libraries a project names. A design's
// intent is not among them, since it arrives as a value the service compiles. The server and the CLI
// resolve through it over os.DirFS mounts, and the wasm engine over in-memory ones, so a project's
// config composes one way everywhere.
//
// It holds NO CACHE. An operator edits a profile or seeds a part while the server runs, and an index
// answering with the previous version would produce a confident wrong verdict.
type ConfigResolver struct {
	Mounts []Mount
	// SymbolPath turns a symbol-library directory into the name the host's formats.Loader opens. A
	// symbol library is handed to the reader rather than read here, so it has to be spelled in the
	// reader's name space: a host path for a loader reading the host filesystem. Nil spells it as its
	// path in Root, which is what a Host's loader opens.
	SymbolPath func(uri artifact.URI) (string, error)
}

// ResolveConfig loads what an AnalysisConfig's URIs point at, whether it came from a project
// descriptor or from a request.
//
// A tier that fails to load is an ERROR, not a skip, since an operator who wrote a profiles
// directory and silently got the built-ins would read the clean report as a clean design (C24). A
// tier the config does not name is not an error, because the loops below run zero times.
func (c *ConfigResolver) ResolveConfig(_ context.Context, cfg *webapi.AnalysisConfig, namespace string) (service.ResolvedConfig, error) {
	var out service.ResolvedConfig
	// read records every directory this resolution opened, so the digest covers what was read rather
	// than what the config named. One URI can resolve to different bytes on two servers.
	var read []DigestRoot
	for _, uri := range cfg.GetProfileUris() {
		d, err := c.dir(uri)
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		read = append(read, d)
		ps, err := profiles.LoadFS(d.FS, d.Name)
		if err != nil {
			return service.ResolvedConfig{}, fmt.Errorf("%s profiles %s: %w", namespace, uri, err)
		}
		out.Sources = append(out.Sources, profiles.Source(ProfileSourceName(namespace), ps))
		out.InterfaceProfiles = append(out.InterfaceProfiles, ps...)
		out.Profiles = true
	}
	for _, uri := range cfg.GetParamUris() {
		d, err := c.dir(uri)
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		read = append(read, d)
		sub, err := fs.Sub(d.FS, d.Name)
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		set, err := param.LoadSet(sub)
		if err != nil {
			return service.ResolvedConfig{}, fmt.Errorf("%s params %s: %w", namespace, uri, err)
		}
		out.Specs = set
	}
	for _, uri := range cfg.GetSymbolPathUris() {
		u, err := artifact.Parse(uri)
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		var p string
		if c.SymbolPath != nil {
			p, err = c.SymbolPath(u)
		} else {
			var d DigestRoot
			d, err = c.dir(uri)
			p = path.Join(u.Mount, d.Name)
		}
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		out.SymbolPaths = append(out.SymbolPaths, p)
	}
	// A project's own library is read here and composed by the service (Overlay.Registry), so a module
	// that does not parse or collides is reported when a query first runs over it, naming the file.
	for _, uri := range cfg.GetLibraryUris() {
		d, err := c.dir(uri)
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		read = append(read, d)
		sub, err := fs.Sub(d.FS, d.Name)
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		mods, docs, err := lib.Read(sub)
		if err != nil {
			return service.ResolvedConfig{}, fmt.Errorf("%s library %s: %w", namespace, uri, err)
		}
		for _, m := range mods {
			out.Library = append(out.Library, service.LibraryModule{
				Path: m.Path, Language: datalog.LanguageName, Text: m.Text, Source: strings.TrimSuffix(uri, "/") + "/" + m.File,
			})
		}
		for p, doc := range docs {
			if out.LibraryDocs == nil {
				out.LibraryDocs = map[string]string{}
			}
			out.LibraryDocs[p] = doc
		}
	}
	// A resolution that read nothing still gets a digest identifying it as such. Symbol paths go in
	// as NAMES, since this call never opens them.
	digest, err := DigestConfig(read, cfg.GetSymbolPathUris())
	if err != nil {
		return service.ResolvedConfig{}, fmt.Errorf("%s config digest: %w", namespace, err)
	}
	out.Digest = digest
	return out, nil
}

// dir resolves a project-config URI to a directory inside its mount's own FS.
func (c *ConfigResolver) dir(uri string) (DigestRoot, error) {
	u, err := artifact.Parse(uri)
	if err != nil {
		return DigestRoot{}, err
	}
	for _, m := range c.Mounts {
		if m.Name == u.Mount {
			name := path.Clean(u.Path)
			if name == "" {
				name = "."
			}
			if !fs.ValidPath(name) {
				return DigestRoot{}, fmt.Errorf("%w: %q", service.ErrInvalidPath, u.Path)
			}
			return DigestRoot{FS: m.FS, Name: name}, nil
		}
	}
	return DigestRoot{}, fmt.Errorf("no such mount %q: %w", u.Mount, service.ErrNotFound)
}

// ProfileSourceName is the catalog namespace a project's interface profiles appear under, so a
// finding reads `gateway-profiles/can-esd-missing` and says which project asked for it.
func ProfileSourceName(namespace string) string {
	if id, ok := service.ProjectID(namespace); ok {
		return id + "-profiles"
	}
	// A namespace that is not a project resource name is a request's. It keeps the `-profiles` suffix
	// and cannot collide with a project's, because a project id can never be the literal "request".
	return namespace + "-profiles"
}

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

// DigestRoot is one file or directory a resolution read, as a name inside an FS.
type DigestRoot struct {
	FS   fs.FS
	Name string
}

// DigestConfig returns one digest over the bytes under roots, plus the literal names.
//
// The split is not cosmetic. roots are what this resolution OPENED, and their contents decide the
// run. names are things it only referred to, symbol library URIs being the case: they are handed to
// the reader rather than read here, so they are folded in by name. A run searching a different
// library is configured differently.
func DigestConfig(roots []DigestRoot, names []string) (string, error) {
	h := sha256.New()
	for _, r := range roots {
		if err := foldRoot(h, r); err != nil {
			return "", err
		}
	}
	for _, n := range names {
		foldField(h, []byte(n))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func foldRoot(h io.Writer, r DigestRoot) error {
	info, err := fs.Stat(r.FS, r.Name)
	if err != nil {
		return fmt.Errorf("digest %s: %w", r.Name, err)
	}
	// A root's own NAME is not folded in. The same profile set mounted at two paths is the same
	// config, and its bytes are what decide the run. What IS folded is a boundary marker and the
	// entry count, because without them two roots holding one file each produce the same byte stream
	// as one root holding both.
	foldField(h, []byte("root"))
	if !info.IsDir() {
		foldCount(h, 1)
		return foldFile(h, r.FS, r.Name, ".")
	}
	var rels []string
	err = fs.WalkDir(r.FS, r.Name, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := p
		if r.Name != "." {
			rel = strings.TrimPrefix(p, r.Name+"/")
		}
		rels = append(rels, rel)
		return nil
	})
	if err != nil {
		return fmt.Errorf("digest %s: %w", r.Name, err)
	}
	// WalkDir is already lexical, but the order is what the digest MEANS, so it is sorted here rather
	// than inherited from a traversal whose guarantees could change.
	sort.Strings(rels)
	foldCount(h, len(rels))
	for _, rel := range rels {
		if err := foldFile(h, r.FS, path.Join(r.Name, rel), rel); err != nil {
			return err
		}
	}
	return nil
}

func foldFile(h io.Writer, fsys fs.FS, name, rel string) error {
	f, err := fsys.Open(name)
	if err != nil {
		return fmt.Errorf("digest %s: %w", name, err)
	}
	defer f.Close()
	foldField(h, []byte(rel))
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return fmt.Errorf("digest %s: %w", name, err)
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
