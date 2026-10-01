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

//go:embed *.dl
var modules embed.FS

// docs holds one reference page per public member, docs/<path>.md, served by Registry.Doc and
// generated into the docsite's relation reference by `make catalog-docs`.
//
//go:embed docs/*.md
var docs embed.FS

func init() {
	var ms []ns.Module
	for _, m := range Modules() {
		ms = append(ms, ns.Module{Path: m.Path, Language: datalog.LanguageName, Text: m.Text})
	}
	facts.RegisterModules(ms...)
	facts.RegisterDocs(Docs())
}

// Docs returns each member's reference markdown keyed by its path. A file whose name starts with "_"
// is a template and is skipped.
func Docs() map[string]string {
	names, err := fs.Glob(docs, "docs/*.md")
	if err != nil {
		panic("stdlib/lib: " + err.Error())
	}
	out := map[string]string{}
	for _, n := range names {
		base := path.Base(n)
		if strings.HasPrefix(base, "_") {
			continue
		}
		text, err := docs.ReadFile(n)
		if err != nil {
			panic("stdlib/lib: " + err.Error())
		}
		out[strings.TrimSuffix(base, ".md")] = string(text)
	}
	return out
}

// Module is one library file: the module path its rules register under, and its Datalog text.
type Module struct {
	Path string
	Text string
}

// Modules returns the library's modules in file-name order. A file's name without its extension is
// its module path, so net.dl defines the members under `net`.
func Modules() []Module {
	names, err := fs.Glob(modules, "*.dl")
	if err != nil {
		panic("stdlib/lib: " + err.Error()) // the pattern is a constant; Glob fails only on a malformed one
	}
	out := make([]Module, 0, len(names))
	for _, n := range names {
		text, err := modules.ReadFile(n)
		if err != nil {
			panic("stdlib/lib: " + err.Error())
		}
		out = append(out, Module{Path: strings.TrimSuffix(path.Base(n), ".dl"), Text: string(text)})
	}
	return out
}
