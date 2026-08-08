package intent

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/panyam/agni/core/check"
)

// ruleDocs embeds the per-rule documentation for the intent rules: one markdown file (plus its
// referenced schematic-card image) per rule KIND under docs/, the single source of Rule.Detail — the
// same convention as stdlib/rules/builtin/docs and stdlib/profiles/docs, so prose and diagrams are
// authored as markdown beside the rule, not as Go strings inline. The whole docs/ directory is
// embedded (not a *.md glob) so a doc can add an image with no build change and a zero-match glob never
// breaks the build. The 1:1 between the intent rule KINDS and the doc files, and that every emitted
// rule's Detail comes from its doc, is harness-enforced (docs_test.go).
//
//go:embed docs
var ruleDocs embed.FS

// intentDoc returns the embedded markdown for an intent rule doc KEY (docKey, not the composed
// "intent/" catalog name and not the per-instance rule name). A missing file is a programmer error
// caught at package init, when Compile loads each rule's Detail. Keying by rule KIND (module-missing,
// protection-ovp, subsystem, ...) rather than rule NAME is what lets the dynamically-named
// subsystem-<slug> family and the per-kind protection rules share one doc each — the same
// "keyed by requirement type, not rule name" shape stdlib/profiles uses.
func intentDoc(key string) string {
	b, err := ruleDocs.ReadFile("docs/" + key + ".md")
	if err != nil {
		panic(fmt.Sprintf("intent: no doc for rule kind %q (want docs/%s.md): %v", key, key, err))
	}
	return string(b)
}

// docKeySubsystem is the single doc key shared by every intent/subsystem-<slug> rule: subsystem rule
// names are per-design (derived from the declared subsystem name), so they cannot each own a doc file;
// the family doc explains the shared source-and-nets check they all run.
const docKeySubsystem = "subsystem"

// docKeys is the canonical set of intent rule-doc keys: every kind Compile can emit maps to exactly one
// entry here, and each has a docs/<key>.md. It is the harness's expectation set (docs_test.go holds
// docKeys, the emitted rules, and the docs/ directory to each other), so a new intent rule kind added
// to Compile without its doc key + doc file fails CI.
var docKeys = []string{
	RuleModuleMissing,                   // module-missing
	RuleModuleCount,                     // module-count
	RuleVoltageDomain,                   // voltage-domain-mismatch
	"protection-" + ProtectionOVP,       // protection-ovp
	"protection-" + ProtectionDischarge, // protection-discharge
	docKeySubsystem,                     // subsystem (family doc for intent/subsystem-<slug>)
	"property-" + PropResetPolarity,     // property-reset-polarity
	"property-" + PropACCoupled,         // property-ac-coupled
	"property-" + PropStrap,             // property-strap
}

// docKey maps a Rule.Name to its doc key: identity for the fixed-name rules (module-missing,
// module-count, voltage-domain-mismatch, protection-<kind>), and the dynamically-named
// subsystem-<slug> family collapses to the single "subsystem" key. It is the inverse of the wiring in
// the rule builders (each sets Detail: intentDoc(<its key>)) and the join the harness uses to tie an
// emitted rule back to the doc its Detail must come from.
func docKey(ruleName string) string {
	if strings.HasPrefix(ruleName, "subsystem-") {
		return docKeySubsystem
	}
	return ruleName
}

// docSummaries is the one-line catalog caption for each intent rule KIND, shown in the docsite
// reference index (tools/catalogdocs). It is a doc caption, distinct from the runtime Rule.Summary a
// finding carries: the runtime summaries for the per-kind protection and per-instance subsystem rules
// embed a design-specific kind/name, so a generic page needs a name-free caption. Every docKey has an
// entry (DocRules would emit an empty caption otherwise; TestDocRules holds them 1:1).
var docSummaries = map[string]string{
	RuleModuleMissing:                   "A functional block the design intent declares required is absent from the design.",
	RuleModuleCount:                     "The number of components for a declared module does not match the design intent.",
	RuleVoltageDomain:                   "A declared voltage domain's rail is absent or named for a different nominal voltage.",
	"protection-" + ProtectionOVP:       "A rail the design intent declares needs OV protection has no TVS/zener clamp.",
	"protection-" + ProtectionDischarge: "A rail the design intent declares needs a discharge path has no bleeder resistor.",
	docKeySubsystem:                     "An architectural subsystem the design intent declares is missing a required part or net.",
	"property-" + PropResetPolarity:     "A net the design intent declares as a reset is biased to its ASSERTED level, holding the part in reset.",
	"property-" + PropACCoupled:         "A net the design intent declares AC-coupled is carried by no series capacitor.",
	"property-" + PropStrap:             "A boot/config strap net is biased to the OPPOSITE level from the one the design intent declares it should latch.",
}

// DocRules returns one representative rule per intent rule KIND (docKey) for the docsite catalog
// generator. Intent rules are generated per-declaration, so there is no static catalog to enumerate;
// this projects each documented kind into a single page-worthy rule carrying its Detail card, its
// classification (intentTags), and a generic caption (docSummaries) in place of the design-specific
// runtime Summary. The subsystem family (intent/subsystem-<slug> at runtime) is one page named
// "subsystem". Callers must not mutate the returned rules.
func DocRules() []*check.Rule {
	out := make([]*check.Rule, 0, len(docKeys))
	for _, k := range docKeys {
		out = append(out, &check.Rule{
			Name:     k,
			Severity: "warning",
			Summary:  docSummaries[k],
			Detail:   intentDoc(k),
			Tags:     intentTags(),
		})
	}
	return out
}

// RuleDocImageHandler serves the intent rule docs' embedded schematic-card images (the diagram a
// docs/<kind>.md references) as a read-only static route, so the web rules/checks panels resolve the
// relative image refs in an intent rule's Detail the same way builtin.RuleDocImageHandler does for the
// built-ins. Only image files under docs/images/ are served (.svg and .png); any other path (the
// markdown itself, a directory, a traversal attempt) is 404, so the handler never exposes anything but
// the diagrams. The images come from the embed FS alone — no filesystem access, keeping the core free
// of file I/O. Mount it under a prefix (the handler sees the prefix-stripped, relative path, e.g.
// "images/protection-ovp.svg"). SVG's content-type is set explicitly because Go's mime table resolves
// .svg only from the host's mime files, which CI/WASM may lack.
func RuleDocImageHandler() http.Handler {
	sub, err := fs.Sub(ruleDocs, "docs")
	if err != nil {
		panic(fmt.Sprintf("intent: rule docs sub-FS: %v", err)) // embed path is a constant; unreachable
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".svg"):
			w.Header().Set("Content-Type", "image/svg+xml")
		case strings.HasSuffix(r.URL.Path, ".png"):
		default:
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}
