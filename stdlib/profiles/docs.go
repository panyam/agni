package profiles

import (
	"embed"
	"fmt"

	"github.com/panyam/agni/core/check"
)

// requirementDocs is the set of requirement TYPES the compiler documents (one docs/<type>.md each),
// and the canonical key list DocRules and the docs harness both iterate. It lives here, not in the
// test, so DocRules and TestRuleDocsOneToOne share one source of truth.
var requirementDocs = []string{"signal-missing", "missing-pullup", "signal-dangling", "termination", "esd"}

// requirementCaption is the docsite caption and severity for each requirement TYPE, shown in the
// docsite reference index (tools/catalogdocs). The caption is interface-agnostic on purpose: the
// runtime Rule.Summary a finding carries embeds the profile name (e.g. "A required CAN signal is
// absent"), so a generic per-requirement page needs a name-free caption. Severity matches the
// generated rule's.
// remedy is what to DO about a violation, in the imperative (check.Rule.Remedy). It sits here rather
// than at each rule builder for the reason the caption does: the rule is generated per-profile, but
// the fix is per-REQUIREMENT and identical across every interface that declares one. Both the runtime
// rule and the docsite exemplar read it from here, so there is one copy rather than two.
var requirementCaption = map[string]struct {
	summary  string
	severity string
	remedy   string
}{
	"signal-missing": {"A signal a required interface declares is absent from the design.", "error",
		"Add the missing signal to the design, or drop the interface requirement if this board genuinely does not implement it."},
	"missing-pullup": {"An interface signal that needs a pull-up reaches no rail.", "warning",
		"Fit a pull-up from the signal to its rail, sized for the speed the bus runs at and the capacitance it carries."},
	"signal-dangling": {"An interface signal net has fewer than two connections (a dangling stub).", "warning",
		"Connect the far end of the signal. The net exists, so presence checks pass, but only one end of it is actually wired."},
	"termination": {"A bus that requires termination has no terminating device across its pair.", "warning",
		"Fit the termination the bus calls for across the pair, at the physical end of the bus rather than partway along it."},
	"esd": {"An interface signal leaves the board through a connector with no ESD clamp.", "warning",
		"Fit an ESD clamp on the signal at the connector, ahead of the transceiver it protects."},
}

// requirementRemedy is the Remedy a rule for this requirement type carries, for the profile compiler
// and DocRules alike.
func requirementRemedy(req string) string { return requirementCaption[req].remedy }

// DocRules returns one representative rule per requirement TYPE for the docsite catalog generator
// (tools/catalogdocs). Profile rules are generated per-profile, so there is no static catalog to
// enumerate; this projects each requirement's Detail into a single page-worthy rule with a generic
// caption (requirementDoc) in place of the profile-specific runtime Summary. Callers must not mutate
// the returned rules.
func DocRules() []*check.Rule {
	out := make([]*check.Rule, 0, len(requirementDocs))
	for _, req := range requirementDocs {
		d := requirementCaption[req]
		out = append(out, &check.Rule{
			Name:     req,
			Severity: d.severity,
			Summary:  d.summary,
			Remedy:   d.remedy,
			Detail:   ruleDoc(req),
			Tags:     map[string]string{check.KeyCategory: check.CategoryConnectivity, check.KeyDistribution: check.DistOpen},
		})
	}
	return out
}

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
