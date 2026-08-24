// Command agni-overlay is the reference open-core overlay skeleton (WS12-001): a binary in a
// SEPARATE Go module that depends on the public engine and adds a private format reader and a
// private rule suite through the engine's public extension points, without forking it.
//
// The composition is two blank imports — the overlay's reader and rule packages register
// themselves via formats.Register (WS12-003) and check.RegisterSource (WS12-004) in their init.
// From there the engine's own library resolves the .acme format and runs the acme/ rule
// alongside the built-ins. A production overlay would instead reuse the engine's whole CLI
// (see the follow-up ticket to export a reusable command root); this skeleton drives the
// library directly so the composition is visible in one file.
package main

import (
	"fmt"
	"os"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/readers/formats"

	_ "github.com/panyam/agni/examples/overlay/acmeformat" // registers the .acme reader
	_ "github.com/panyam/agni/examples/overlay/acmerules"  // registers the acme/ rule suite

	// The engine's fact base, needed by any DATALOG rule (the suite above has one). Omitting this
	// import does not fail to build and does not error at runtime: the fact base is simply empty,
	// so every datalog rule matches nothing and reports clean. A quiet pass on a design that may
	// be violating the rule is the worst failure shape there is, so the import is deliberate and
	// commented rather than left to be discovered.
	_ "github.com/panyam/agni/stdlib/relations"
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
	// The overlay's reader was registered by the blank import above, so the engine's Loader
	// resolves .acme with no special-casing.
	d, err := (&formats.Loader{}).ReadDesign(path)
	if err != nil {
		return err
	}
	fmt.Printf("loaded %s: %d components, %d nets (via the overlay's .acme reader)\n\n", path, len(d.Components), len(d.Nets))

	// DefaultCatalog is the built-ins PLUS every registered source, so the acme/ rule runs
	// alongside the engine's own checks.
	catalog := check.DefaultCatalog()
	findings := check.Run(check.NewModel(d), catalog.Rules())
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
