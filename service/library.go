package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/jaala/ns"
)

// LibraryModule is one module of a project's own derived relations, as a ConfigResolver read it: the
// module path its members register under, the language and text it is written in, and Source, where
// it was read from, which an error names so a reader can find the file.
type LibraryModule struct {
	Path     string
	Language string
	Text     string
	Source   string
}

// Registry is the query vocabulary this overlay's queries run over: the process default, plus the
// project's own library when it has one (agni issue 773). An overlay with no library gets
// facts.DefaultRegistry itself, so nothing changes for a project that declares none.
//
// A library is composed once per distinct content and cached, because a server answers many queries
// over one project and composing checks every module. An error names the module's file. A member
// whose path the shipped vocabulary already defines is refused by name before composition, since the
// vocabulary's own message for that names the path and not the file that caused it.
func (o Overlay) Registry() (*facts.Registry, error) {
	if len(o.Library) == 0 && len(o.LibraryDocs) == 0 {
		return facts.DefaultRegistry(), nil
	}
	key := libraryKey(o.Library, o.LibraryDocs)
	if v, ok := libraryCache.Load(key); ok {
		e := v.(libraryEntry)
		return e.reg, e.err
	}
	reg, err := composeLibrary(o.Library, o.LibraryDocs)
	libraryCache.Store(key, libraryEntry{reg, err})
	return reg, err
}

// libraryCache holds every library composition this process has made, keyed on the library's content
// (libraryKey). A project edited on disk has new content and so a new key; the number of distinct
// libraries a server sees is small, so entries are not evicted.
var libraryCache sync.Map

type libraryEntry struct {
	reg *facts.Registry
	err error
}

func libraryKey(mods []LibraryModule, docs map[string]string) string {
	h := sha256.New()
	for _, m := range mods {
		fmt.Fprintf(h, "%q %q %q\n", m.Path, m.Language, m.Text)
	}
	for _, p := range slices.Sorted(maps.Keys(docs)) {
		fmt.Fprintf(h, "doc %q %q\n", p, docs[p])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func composeLibrary(mods []LibraryModule, docs map[string]string) (*facts.Registry, error) {
	base := facts.DefaultRegistry().Vocabulary()
	ms := make([]ns.Module, 0, len(mods))
	for _, m := range mods {
		names, err := query.ModuleMembers(m.Text)
		if err != nil {
			return nil, fmt.Errorf("project library %s: %w", m.Source, err)
		}
		for _, n := range names {
			p := n
			if m.Path != "" {
				p = m.Path + "." + n
			}
			if base.Has(p) {
				return nil, fmt.Errorf("project library %s defines %s, which agni already defines; a project may add members to a module but not replace one", m.Source, p)
			}
		}
		ms = append(ms, ns.Module{Path: m.Path, Language: m.Language, Text: m.Text})
	}
	opts := append(facts.Registered(), facts.WithModules(ms...))
	if len(docs) > 0 {
		opts = append(opts, facts.WithDocs(docs))
	}
	reg, err := facts.NewRegistry(opts...)
	if err != nil {
		// The vocabulary's message names a module by its path, which is its file's name, so naming the
		// directories it was read from locates the file. panyam/jaala#30 asks for an error carrying the
		// module itself, which would let this name the file outright.
		return nil, fmt.Errorf("project library %s: %w", strings.Join(libraryDirs(mods), ", "), err)
	}
	return reg, nil
}

// libraryDirs is the distinct directories a library's modules were read from, in order.
func libraryDirs(mods []LibraryModule) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range mods {
		d := m.Source
		if i := strings.LastIndex(d, "/"); i >= 0 {
			d = d[:i]
		}
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out
}

// mergeDocs layers b's pages over a's without changing either.
func mergeDocs(a, b map[string]string) map[string]string {
	if len(b) == 0 {
		return a
	}
	out := maps.Clone(a)
	if out == nil {
		out = map[string]string{}
	}
	maps.Copy(out, b)
	return out
}
