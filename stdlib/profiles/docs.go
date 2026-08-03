package profiles

import (
	"embed"
	"fmt"
)

// ruleDocs embeds the per-REQUIREMENT documentation (signal-missing / missing-pullup /
// signal-dangling), shared across every profile the compiler generates — the rule is per-profile
// (spi_nor-signal-missing), but the contract it enforces is per-requirement, so the doc is too. The
// whole docs/ directory is embedded so a doc can add images (mirrors check/docs, datalogrules/docs).
//
//go:embed docs
var ruleDocs embed.FS

// ruleDoc returns the embedded markdown for a requirement type (signal-missing, missing-pullup,
// signal-dangling). A missing file is a programmer error caught at init, when Compile loads Detail.
func ruleDoc(requirement string) string {
	b, err := ruleDocs.ReadFile("docs/" + requirement + ".md")
	if err != nil {
		panic(fmt.Sprintf("profiles: no doc for requirement %q (want docs/%s.md): %v", requirement, requirement, err))
	}
	return string(b)
}
