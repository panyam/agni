// Command agni-extension is the extension template's composing binary. Copy this whole module,
// rename it, and fill in the myfmt/ reader and myrules/ rules to extend the public agni engine
// without forking it.
//
// The composition is two blank imports for your own packages, whose init registers them
// (formats.Register, check.RegisterSource), plus agni.New over the engine's four. agni.New refuses
// a binary that misses the built-in rules or the fact base, since either one makes every design
// report clean. See docsite/content/build/extending.md.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/panyam/agni"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/readers/formats"

	// TODO: point these at your renamed packages.
	_ "github.com/panyam/agni/examples/extension-template/myfmt"   // registers the .myfmt reader
	_ "github.com/panyam/agni/examples/extension-template/myrules" // registers the myco/ rules

	_ "github.com/panyam/agni/stdlib/relations"     // the fact base every datalog rule reads
	_ "github.com/panyam/agni/stdlib/reviewquery"   // compiles a review manifest's inline queries
	_ "github.com/panyam/agni/stdlib/rules/builtin" // the shipped EE rule catalog
	_ "github.com/panyam/agni/stdlib/rules/datalog" // the datalog-authored rule suite
)

func main() {
	engine, err := agni.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "agni-extension:", err)
		os.Exit(1)
	}
	for _, w := range engine.Warnings() {
		fmt.Fprintln(os.Stderr, "note:", w)
	}
	if len(os.Args) < 2 {
		fmt.Println("usage: agni-extension <design-file>")
		fmt.Printf("registered custom format: %v\n", formats.NameForExt("x.myfmt") != "")
		fmt.Printf("registered custom rule: %v\n", engine.Catalog().Lookup("myco/example-rule") != nil)
		return
	}
	if err := run(engine, os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "agni-extension:", err)
		os.Exit(1)
	}
}

func run(engine *agni.Engine, path string) error {
	// The Loader reads the file through your registered reader and runs the format-neutral passes.
	d, err := (&formats.Loader{}).ReadDesign(path)
	if err != nil {
		return err
	}
	// Run takes a context so a long check stops when its caller does; a command line has none to
	// cancel. It returns an error only when that context is done.
	findings, err := check.Run(context.Background(), check.NewModel(d), engine.Catalog().Rules())
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d components, %d finding(s)\n", path, len(d.Components), len(findings))
	for _, f := range findings {
		fmt.Printf("  [%s] %s: %s (%s)\n", f.Severity, f.Rule, check.EntityRef(f.Subject), f.Message)
	}
	return nil
}
