// Command agni-overlay is the overlay template's composing binary. Copy this whole module,
// rename it, fill in the myfmt/ reader and myrules/ rules, and you have a private overlay that
// extends the public agni engine without forking it.
//
// The composition is two blank imports: the reader and rule packages register themselves in
// their init (formats.Register / check.RegisterSource), so from here the engine's library
// resolves your format and runs your rules alongside the built-ins. See
// docs/OVERLAY_AUTHORING.md.
package main

import (
	"fmt"
	"os"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/readers/formats"

	// TODO: point these at your renamed packages.
	_ "github.com/panyam/agni/examples/overlay-template/myfmt"  // registers the .myfmt reader
	_ "github.com/panyam/agni/examples/overlay-template/myrules" // registers the myco/ rules
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: agni-overlay <design-file>")
		fmt.Printf("registered custom format: %v\n", formats.NameForExt("x.myfmt") != "")
		fmt.Printf("registered custom rules: %d in the catalog\n", len(check.DefaultCatalog().Rules()))
		return
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "agni-overlay:", err)
		os.Exit(1)
	}
}

func run(path string) error {
	d, err := (&formats.Loader{}).ReadDesign(path)
	if err != nil {
		return err
	}
	findings := check.Run(check.NewModel(d), check.DefaultCatalog().Rules())
	fmt.Printf("%s: %d components, %d finding(s)\n", path, len(d.Components), len(findings))
	for _, f := range findings {
		fmt.Printf("  [%s] %s: %s (%s)\n", f.Severity, f.Rule, check.EntityRef(f.Subject), f.Message)
	}
	return nil
}
