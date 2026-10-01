// Package lib is the shipped library of derived relations: Datalog modules over the relations
// stdlib/relations projects, registered in the fact layer's vocabulary at the paths their file names
// give (agni issue 751). component.dl defines component.two_terminal, component.probed_both and
// component.probed_one; net.dl defines net.has_test_point. A query calls them exactly as it calls a
// base relation, and `agni query --relations <path>` prints a member's signature and definition.
// Each member has a reference page in docs/, and docsite/content/build/library-member.md is the
// how-to for adding one.
//
// A member belongs here when it is a general hardware question many queries ask. One that only a
// single report needs stays in that report's query, and one that a Go rule or a Spec must read is
// promoted to a base relation in stdlib/relations instead (DECISIONS, "The Datalog engine lives in
// jaala").
//
// The package imports core/query and stdlib/relations so the Datalog language and the relations these
// modules read are registered before the modules are. The fact layer composes and checks the
// vocabulary at every registration, so a module arriving first would be refused. A binary that wants
// the library blank-imports this package.
package lib

import (
	"embed"
	"io/fs"
	"path"
	"strings"

	"github.com/panyam/agni/core/facts"
	_ "github.com/panyam/agni/core/query"       // registers the datalog language these modules are written in
	_ "github.com/panyam/agni/stdlib/relations" // registers the relations these modules read
	"github.com/panyam/jaala/datalog"
	"github.com/panyam/jaala/ns"
)

// shipped is the library this package registers: one module per `<path>.dl` file, and one reference
// page per public member in `docs/<path>.md`, served by Registry.Doc and generated into the docsite's
// relation reference by `make catalog-docs`.
//
//go:embed *.dl docs/*.md
var shipped embed.FS

func init() {
	mods, docs, err := Read(shipped)
	if err != nil {
		panic("stdlib/lib: " + err.Error()) // the files are embedded, so this is a broken build
	}
	var ms []ns.Module
	for _, m := range mods {
		ms = append(ms, ns.Module{Path: m.Path, Language: datalog.LanguageName, Text: m.Text, Origin: "stdlib/lib/" + m.File})
	}
	facts.RegisterModules(ms...)
	facts.RegisterDocs(docs)
}

// Module is one library file: the module path its rules register under, its Datalog text, and the
// file it was read from.
type Module struct {
	Path string
	Text string
	File string
}

// Read loads a library laid out as this package's is: each `<module.path>.dl` file at the root of fsys
// is one module, registered at the path its name gives, and each `docs/<member.path>.md` is a member's
// reference page. A file whose name starts with "_" is skipped, so a template can sit beside the
// pages. A missing docs directory is an empty set of pages, not an error.
//
// A project's own `lib/` directory is read with the same function, so the two layouts cannot drift.
// Read only reads files; whether the modules parse and what they may read is decided when they are
// composed into a vocabulary.
func Read(fsys fs.FS) ([]Module, map[string]string, error) {
	names, err := fs.Glob(fsys, "*.dl")
	if err != nil {
		return nil, nil, err
	}
	var mods []Module
	for _, n := range names {
		if strings.HasPrefix(n, "_") {
			continue
		}
		text, err := fs.ReadFile(fsys, n)
		if err != nil {
			return nil, nil, err
		}
		mods = append(mods, Module{Path: strings.TrimSuffix(n, ".dl"), Text: string(text), File: n})
	}
	pages, err := fs.Glob(fsys, "docs/*.md")
	if err != nil {
		return nil, nil, err
	}
	docs := map[string]string{}
	for _, n := range pages {
		base := path.Base(n)
		if strings.HasPrefix(base, "_") {
			continue
		}
		text, err := fs.ReadFile(fsys, n)
		if err != nil {
			return nil, nil, err
		}
		docs[strings.TrimSuffix(base, ".md")] = string(text)
	}
	return mods, docs, nil
}

// Modules returns the shipped library's modules in file-name order.
func Modules() []Module {
	mods, _, err := Read(shipped)
	if err != nil {
		panic("stdlib/lib: " + err.Error())
	}
	return mods
}

// Docs returns each shipped member's reference markdown keyed by its path.
func Docs() map[string]string {
	_, docs, err := Read(shipped)
	if err != nil {
		panic("stdlib/lib: " + err.Error())
	}
	return docs
}
