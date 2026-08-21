package intent

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/panyam/agni/core/check"
)

// ruleDocs embeds the per-rule documentation for the intent rules under docs/, the single source of
// Rule.Detail. The convention is docsite/content/build/check-rule.md. Two deviations here: the whole
// docs/ directory is embedded rather than a *.md glob, so a doc can add an image with no build change,
// and the files are keyed by rule KIND (see intentDoc). The 1:1 between kinds and doc files, and that
// every emitted rule's Detail comes from its doc, is harness-enforced (docs_test.go).
//
//go:embed docs
var ruleDocs embed.FS

// intentDoc returns the embedded markdown for an intent rule doc KEY (docKey, not the composed
// "intent/" catalog name and not the per-instance rule name). A missing file is a programmer error
// caught at package init, when Compile loads each rule's Detail. Keys are rule KINDS (module-missing,
// protection-ovp, subsystem, ...) rather than rule NAMES, which is what lets the dynamically-named
// subsystem-<slug> family and the per-kind protection rules share one doc each.
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

// docKeySequence is the family doc for intent/sequence-<slug>, docKeySubsystem's shape. The card
// explains the power-good/enable check every sequence rule runs.
const docKeySequence = "sequence"

// docKeyStrapGroup is the family doc for intent/strap-group-<slug>, docKeySubsystem's shape. Every
// group runs the identical decode.
const docKeyStrapGroup = "strap-group"

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
	RuleRailCurrentCapacity,             // rail-current-capacity
	RuleRailCurrentMargin,               // rail-current-margin
	RuleLoadSwitchTripBelowBudget,       // load-switch-trip-below-budget
	docKeySequence,                      // sequence (family doc for intent/sequence-<slug>)
	docKeyStrapGroup,                    // strap-group (family doc for intent/strap-group-<slug>)
	RuleStrapAddressCollision,           // strap-address-collision (cross-group, one for all)
}

// docKey maps a Rule.Name to its doc key: identity for the fixed-name rules (module-missing,
// module-count, voltage-domain-mismatch, protection-<kind>), and each dynamically-named family
// collapses to its family key. It is the inverse of the rule builders' wiring (each sets Detail:
// intentDoc(<its key>)), and the harness uses it to tie an emitted rule back to its doc.
func docKey(ruleName string) string {
	if strings.HasPrefix(ruleName, "subsystem-") {
		return docKeySubsystem
	}
	if strings.HasPrefix(ruleName, "sequence-") {
		return docKeySequence
	}
	if strings.HasPrefix(ruleName, "strap-group-") {
		return docKeyStrapGroup
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
	RuleRailCurrentCapacity:             "The part supplying a rail is rated below the peak current the design intent declares for that rail.",
	RuleRailCurrentMargin:               "A rail's supply meets its declared peak current budget but not the declared margin factor over it.",
	RuleLoadSwitchTripBelowBudget:       "A load switch's current limit is set below the peak current the design intent declares for the rail it feeds.",
	docKeySequence:                      "A declared power-up order is not enforced by the design's power-good/enable chain, or the chain runs the other way round.",
	docKeyStrapGroup:                    "a group of strap nets does not encode the value the design intent declares",
	RuleStrapAddressCollision:           "two devices on one bus strap to the same address",
}

// docRemedies is what to DO about each intent rule KIND, in the imperative (check.Rule.Remedy).
//
// It is keyed by docKey rather than written at each rule builder because an intent rule is generated
// per-declaration while its remedy is not: the fix for a missing OV clamp is the same sentence on
// every rail that declares one. Keying it here gives the runtime rule and the docsite exemplar one
// source instead of two copies to drift apart. Every docKey has an entry, held 1:1 by TestDocRules.
var docRemedies = map[string]string{
	RuleModuleMissing:                   "Add the missing block to the schematic, or amend the intent declaration if the architecture has moved on. One of the two is out of date, and only the author knows which.",
	RuleModuleCount:                     "Add or remove instances until the count matches the declaration, or amend the declaration. A dropped channel and a stale declaration look identical from here.",
	RuleVoltageDomain:                   "Add the declared rail, or reconcile its name with the voltage the domain declares. A rail named for one voltage and declared at another will mislead every reader after you.",
	"protection-" + ProtectionOVP:       "Fit the over-voltage clamp the intent declares for this rail, chosen to conduct below the lowest absolute maximum the rail feeds.",
	"protection-" + ProtectionDischarge: "Fit a bleeder resistor across the rail, sized so the rail discharges within the time the design assumes when it powers down.",
	docKeySubsystem:                     "Add the missing part or net to the subsystem, or amend the declaration if the architecture changed and the intent document did not.",
	"property-" + PropResetPolarity:     "Bias the reset net to its DE-asserted level. As drawn, the part is held in reset from power-up, which reads at bring-up as a device that never starts.",
	"property-" + PropACCoupled:         "Put a series capacitor in the net, sized for the lowest frequency the link has to pass.",
	"property-" + PropStrap:             "Move the strap resistor to the rail that latches the declared level. The board boots either way, configured as something other than what was intended.",
	RuleRailCurrentCapacity:             "Fit a supply rated above the rail's declared peak current, or reduce the load the rail carries. As drawn, the rail is specified beyond what its source can deliver.",
	RuleRailCurrentMargin:               "Fit a supply carrying the declared margin over the rail's peak, or lower the margin factor if the declaration is stricter than this design needs.",
	RuleLoadSwitchTripBelowBudget:       "Raise the switch's current limit above the rail's declared peak, or lower the peak the rail is budgeted for. As set, the switch trips in normal operation.",
	docKeySequence:                      "Wire the power-good or enable chain so each rail's release depends on the one before it, in the declared order. Check the direction as well as the presence, since a chain wired backwards satisfies neither.",
	docKeyStrapGroup:                    "Re-bias the straps in the group until they encode the declared value, working the value out from the datasheet's strap table rather than from the resistors one at a time.",
	RuleStrapAddressCollision:           "Re-strap one of the two devices to a free address, taking the address map from each part's datasheet rather than from the schematic.",
}

// intentRemedy is the Remedy a rule of this kind carries, for the rule builders and DocRules alike.
func intentRemedy(docKey string) string { return docRemedies[docKey] }

// DocRules returns one representative rule per intent rule KIND (docKey) for the docsite catalog
// generator. Intent rules are generated per-declaration, so there is no static catalog to enumerate.
// Each documented kind becomes one page-worthy rule carrying its Detail card, its classification
// (intentTags), and a generic caption (docSummaries) in place of the design-specific runtime Summary.
// Each dynamically-named family is one page, named for its family key. Callers must not mutate the
// returned rules.
func DocRules() []*check.Rule {
	out := make([]*check.Rule, 0, len(docKeys))
	for _, k := range docKeys {
		out = append(out, &check.Rule{
			Name:     k,
			Severity: "warning",
			Summary:  docSummaries[k],
			Remedy:   intentRemedy(k),
			Detail:   intentDoc(k),
			Tags:     intentTags(),
		})
	}
	return out
}

// RuleDocImageHandler serves the intent rule docs' embedded schematic-card images (the diagram a
// docs/<kind>.md references) as a read-only static route, so the web rules/checks panels resolve the
// relative image refs in an intent rule's Detail. Same shape as builtin.RuleDocImageHandler. Only .svg
// and .png are served; any other path (the markdown itself, a directory, a traversal attempt) is 404.
// Images come from the embed FS alone, no filesystem access, which keeps the core free of file I/O.
// Mount it under a prefix (the handler sees the prefix-stripped, relative path, e.g.
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
