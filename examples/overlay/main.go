// Command agni-overlay is the reference open-core overlay skeleton (WS12-001): a binary in a
// SEPARATE Go module that depends on the public engine and adds a private format reader and a
// private rule suite through the engine's public extension points, without forking it.
//
// The composition is two blank imports for the overlay's own extensions, plus agni.New for the
// engine's. The overlay's reader and rule packages register themselves via formats.Register
// (WS12-003) and check.RegisterSource (WS12-004) in their init; from there the engine's own library
// resolves the .acme format and runs the acme/ rule alongside the built-ins.
//
// The engine's four registration seams arrive as the blank imports below and are CHECKED by
// agni.New, which is the point of composing through it rather than by hand. Three of the four fail
// silently when a binary forgets one: no built-in rules, or an empty fact base, and every design
// reports clean. This file used to carry a comment warning about exactly that, because nothing
// enforced it.
package main

import (
	"fmt"
	"os"

	"github.com/panyam/agni"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/readers/formats"

	_ "github.com/panyam/agni/examples/overlay/acmeformat" // registers the .acme reader
	_ "github.com/panyam/agni/examples/overlay/acmerules"  // registers the acme/ rule suite

	_ "github.com/panyam/agni/stdlib/relations"     // the fact base every datalog rule reads
	_ "github.com/panyam/agni/stdlib/reviewquery"   // compiles a review manifest's inline queries
	_ "github.com/panyam/agni/stdlib/rules/builtin" // the shipped EE rule catalog
	_ "github.com/panyam/agni/stdlib/rules/datalog" // the datalog-authored rule suite
)

func main() {
	path := "testdata/example.acme"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	if err := run(path); err != nil {
		fmt.Fprintln(os.Stderr, "agni-overlay:", err)
		os.Exit(1)
	}
}

func run(path string) error {
	// agni.New composes the catalog and the fact base together and refuses a composition that would
	// run clean for a reason nobody could see. A warning is a legitimate composition worth saying out
	// loud, so it goes to stderr rather than stopping the run.
	engine, err := agni.New()
	if err != nil {
		return err
	}
	for _, w := range engine.Warnings() {
		fmt.Fprintln(os.Stderr, "note:", w)
	}

	// The overlay's reader was registered by the blank import above, so the engine's Loader
	// resolves .acme with no special-casing.
	d, err := (&formats.Loader{}).ReadDesign(path)
	if err != nil {
		return err
	}
	fmt.Printf("loaded %s: %d components, %d nets (via the overlay's .acme reader)\n\n", path, len(d.Components), len(d.Nets))

	// The engine's catalog is the built-ins PLUS every registered source, so the acme/ rule runs
	// alongside the engine's own checks.
	findings := check.Run(check.NewModel(d), engine.Catalog().Rules())
	if len(findings) == 0 {
		fmt.Println("no findings")
		return nil
	}
	fmt.Printf("%d finding(s):\n", len(findings))
	for _, f := range findings {
		fmt.Printf("  [%s] %s: %s (%s)\n", f.Severity, f.Rule, check.EntityRef(f.Subject), f.Message)
	}
	return nil
}
