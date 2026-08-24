// Command catalogdocs generates the docsite's browsable rule and relation catalog (issue 14)
// from the composed engine catalog and the embedded per-rule/per-relation Detail markdown. It
// is the single generator behind docsite/content/reference/{rules,relations}/ and the SVG cards
// under docsite/static/images/catalog/. The stdlib docs stay the one source of truth; this tool
// projects them into docsite pages so the site reflects the shipped catalog instead of drifting
// hand-copied prose.
//
// Run it from the repo root via `make catalog-docs`. `make catalog-docs-check` regenerates and
// fails when the committed output drifts, the same freshness discipline as the generated roadmap
// index. Output is deterministic (sorted) so a clean regen produces no diff.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/stdlib/profiles"
	"github.com/panyam/agni/stdlib/rules/datalog"
	"github.com/panyam/agni/stdlib/rules/intent"

	// Blank imports register the built-in rule catalog and the query relations (plus their embedded
	// Detail docs) into the process-global registries DefaultCatalog and query.Catalog read. The
	// non-builtin rule sources (intent/datalog/profile) are pulled by their own DocRules() accessors
	// below, not via DefaultCatalog, because intent/profile rules are generated per-declaration and
	// have no static catalog entry.
	_ "github.com/panyam/agni/stdlib/relations"
	_ "github.com/panyam/agni/stdlib/rules/builtin"
)

var (
	contentDir = flag.String("content", "docsite/content/reference", "docsite reference content dir")
	staticDir  = flag.String("static", "docsite/static/images/catalog", "docsite static dir for catalog images")
	ruleImgSrc   = flag.String("rule-images", "stdlib/rules/builtin/docs/images", "source dir for built-in rule doc images")
	intentImgSrc = flag.String("intent-images", "stdlib/rules/intent/docs/images", "source dir for intent rule doc images")
	relImgSrc    = flag.String("relation-images", "stdlib/relations/facts/docs/images", "source dir for relation doc images")
)

// imageRef matches a markdown image whose target is a doc-relative images/<file> path, the form
// the embedded Detail uses. Only this form is rewritten; absolute or templated refs are left alone.
var imageRef = regexp.MustCompile(`\]\(images/([^)]+)\)`)

// leadingHeading matches a leading "## <title>" line (plus its trailing blank line) at the very
// start of a Detail body. The generated page's front-matter supplies the H1, so the duplicate
// heading is stripped.
var leadingHeading = regexp.MustCompile(`\A## [^\n]*\n+`)

// ruleCategoryOrder is the fixed display order for the rules index; a category not listed here
// sorts last, alphabetically, so a new category still renders rather than vanishing.
var ruleCategoryOrder = []string{
	check.CategoryConnectivity,
	check.CategoryPower,
	check.CategoryNaming,
	check.CategoryBoard,
	check.CategoryDatasheet,
	check.CategoryIntegrity,
}

// relationKindOrder is the fixed display order for the relations index, matching the picker order.
var relationKindOrder = []string{
	query.KindNetlist,
	query.KindBoard,
	query.KindDatasheet,
	query.KindPredicate,
	query.KindOverlay,
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatalf("catalogdocs: %v", err)
	}
}

func run() error {
	if err := genRules(); err != nil {
		return fmt.Errorf("rules: %w", err)
	}
	if err := genRelations(); err != nil {
		return fmt.Errorf("relations: %w", err)
	}
	return nil
}

// ruleSource is one origin of documented rules the catalog projects. label is shown in the index's
// Source column so a reader can tell a built-in from an intent/datalog/profile rule; prefix namespaces
// the page slug and link so a non-built-in name never collides with a built-in of the same name (and
// carries provenance in the URL). imgSrc is the dir holding this source's docs/images, empty when the
// source ships no cards.
type ruleSource struct {
	rules  []*check.Rule
	label  string
	prefix string
	imgSrc string
}

// catalogRow is one rendered index entry: enough to place a rule in its category table with its
// source, severity, and a link to its page.
type catalogRow struct {
	category, label, slug, source, severity, summary string
}

// genRules writes one page per documented rule across every source (built-in plus the non-built-in
// intent/datalog/profile catalogs), the grouped index with a Source column, and copies the images
// those pages reference. Built-in pages keep flat slugs (URLs unchanged); non-built-in pages are
// namespaced by source so a name shared with a built-in (or a future collision) cannot overwrite.
func genRules() error {
	sources := []ruleSource{
		{rules: check.BuiltinRules(), label: "built-in", prefix: "", imgSrc: *ruleImgSrc},
		{rules: intent.DocRules(), label: "intent", prefix: "intent", imgSrc: *intentImgSrc},
		{rules: datalog.DocRules(), label: "datalog", prefix: "dl", imgSrc: ""},
		{rules: profiles.DocRules(), label: "profile", prefix: "profile", imgSrc: ""},
	}

	outDir := filepath.Join(*contentDir, "rules")
	imgOut := filepath.Join(*staticDir, "rules")
	if err := resetDir(outDir, ".md"); err != nil {
		return err
	}
	if err := resetDir(imgOut, ".svg"); err != nil {
		return err
	}

	var rows []catalogRow
	for _, s := range sources {
		images := map[string]bool{}
		for _, r := range s.rules {
			if strings.TrimSpace(r.Detail) == "" {
				continue
			}
			slug := pageSlug(s.prefix, r.Name)
			label := linkLabel(s.prefix, r.Name)
			body := prepareDetail(r.Detail, "rules", images)
			page := frontMatter(label, r.Summary) + remedySection(r.Remedy) + body
			if err := os.WriteFile(filepath.Join(outDir, slug+".md"), []byte(page), 0o644); err != nil {
				return err
			}
			rows = append(rows, catalogRow{
				category: r.Tags[check.KeyCategory],
				label:    label,
				slug:     slug,
				source:   s.label,
				severity: r.Severity,
				summary:  r.Summary,
			})
		}
		if s.imgSrc != "" {
			if err := copyImages(s.imgSrc, imgOut, images); err != nil {
				return err
			}
		}
	}
	return os.WriteFile(filepath.Join(outDir, "index.md"), []byte(rulesIndex(rows)), 0o644)
}

// pageSlug is a rule's page filename stem: flat for the built-ins (prefix ""), else "<prefix>-<name>"
// so a non-built-in page never overwrites a built-in of the same bare name.
func pageSlug(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "-" + name
}

// linkLabel is the display/link text for a rule: the bare name for built-ins, else "<prefix>/<name>"
// (the composed catalog name a review manifest binds to), so provenance shows inline.
func linkLabel(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}

// genRelations writes one page per documented relation plus the grouped index, and copies the
// images those pages reference. A relation with no Detail yet (the staged backfill) gets an index
// row but no page and no link, so the index never points at a missing file.
func genRelations() error {
	rels := query.Catalog()
	sort.Slice(rels, func(i, j int) bool { return rels[i].Name < rels[j].Name })

	outDir := filepath.Join(*contentDir, "relations")
	imgOut := filepath.Join(*staticDir, "relations")
	if err := resetDir(outDir, ".md"); err != nil {
		return err
	}
	if err := resetDir(imgOut, ".svg"); err != nil {
		return err
	}

	images := map[string]bool{}
	for _, rel := range rels {
		if strings.TrimSpace(rel.Detail) == "" {
			continue
		}
		body := prepareDetail(rel.Detail, "relations", images)
		page := frontMatter(rel.Name, rel.Summary) + body
		if err := os.WriteFile(filepath.Join(outDir, rel.Name+".md"), []byte(page), 0o644); err != nil {
			return err
		}
	}
	if err := copyImages(*relImgSrc, imgOut, images); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "index.md"), []byte(relationsIndex(rels)), 0o644)
}

// prepareDetail strips the leading duplicate heading and rewrites doc-relative image refs to the
// docsite static path, recording each referenced image basename in seen so only used images copy.
func prepareDetail(detail, kind string, seen map[string]bool) string {
	body := leadingHeading.ReplaceAllString(strings.TrimSpace(detail), "")
	body = imageRef.ReplaceAllStringFunc(body, func(m string) string {
		name := imageRef.FindStringSubmatch(m)[1]
		seen[name] = true
		return fmt.Sprintf("](%s/static/images/catalog/%s/%s)", pathPrefixExpr, kind, name)
	})
	return body + "\n"
}

// pathPrefixExpr is the s3gen template expression for the site's URL prefix; content markdown is
// templated, so an absolute static link stays correct if the prefix ever changes.
const pathPrefixExpr = "{{.Site.PathPrefix}}"

// remedySection renders a rule's Remedy as the page's first "### " section, or "" for a rule that
// states none.
//
// It LEADS the page rather than trailing it because of who arrives here: a reader follows this link
// from a finding that just fired, so "what do I do" is the question they came with rather than the one
// they work up to. Everything below it explains why the rule fired.
//
// A heading rather than a blockquote, because the docsite's stylesheet has no blockquote rule at all,
// so a "> " callout renders as unstyled indented text and reads as an aside. A "### " section is
// styled like every other section on the page, and it matches the docs' own convention of proper
// sections over bold run-ins (build/check-rule.md).
//
// ONLY Remedy is projected, deliberately, though the Impact FIELD is equally absent from these pages.
// Most rule docs already write their own "### Impact" section in prose tuned to the page, so injecting
// the field would print the same point twice in slightly different words across the majority of the
// catalog. Remedy has no such section anywhere, by rule, so it is the half genuinely missing. Wanting
// the Impact field here too means first taking that section out of the doc bodies.
func remedySection(remedy string) string {
	remedy = strings.TrimSpace(remedy)
	if remedy == "" {
		return ""
	}
	return "### Remedy\n\n" + remedy + "\n\n"
}

// frontMatter builds a page's YAML header. The title is the entity name; the description is its
// one-line summary with quotes escaped so the YAML stays valid.
func frontMatter(name, summary string) string {
	desc := strings.ReplaceAll(summary, `"`, `\"`)
	return fmt.Sprintf("---\ntitle: \"%s\"\ndescription: \"%s\"\n---\n\n", name, desc)
}

// resetDir ensures dir exists and removes every file in it with the given extension, so the
// generated output is a pure function of the catalog: a removed rule or relation drops its page,
// and a removed image ref drops its card, rather than lingering as an orphan the freshness check
// cannot see. Only the one extension is touched, so a sibling hand-authored file would survive
// (there are none today; the reference/{rules,relations} dirs are fully generated).
func resetDir(dir, ext string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ext) {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyImages copies the named SVGs from src into dst, replacing dst's prior contents for the ones
// listed so a removed reference stops shipping its image. Missing source images are reported, not
// skipped silently, since a referenced-but-absent card is a real doc bug.
func copyImages(src, dst string, names map[string]bool) error {
	if len(names) == 0 {
		return nil
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for name := range names {
		b, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			return fmt.Errorf("image %q referenced but not found in %s: %w", name, src, err)
		}
		if err := os.WriteFile(filepath.Join(dst, name), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// rulesIndex renders the rules catalog landing page: a table per category, rows sorted by label, each
// linking to its page with its source and severity. The Source column flags where a rule comes from —
// a built-in, a design-intent check, a datalog-authored rule, or an interface profile — so a reader
// can tell a new category from a new "shape" of rule within an existing one.
func rulesIndex(rows []catalogRow) string {
	var b strings.Builder
	b.WriteString(frontMatter("Rules catalog", "Every check rule the catalog ships, grouped by category, with its source."))
	b.WriteString("The EE rule catalog. Each rule links to its full reference: what it means, why it matters, the guards it applies, and a fires-versus-fine diagram. The Source column flags where a rule comes from: a built-in, a design-intent check (`intent/`), a datalog-authored rule (`dl/`), or an interface profile (`profile/`). This page is generated from the shipped catalog, so it always matches the engine.\n\n")

	byCat := map[string][]catalogRow{}
	for _, r := range rows {
		byCat[r.category] = append(byCat[r.category], r)
	}
	for _, cat := range orderedKeys(byCat, ruleCategoryOrder) {
		title := cat
		if title == "" {
			title = "other"
		}
		catRows := byCat[cat]
		sort.Slice(catRows, func(i, j int) bool { return catRows[i].label < catRows[j].label })
		b.WriteString("## " + title + "\n\n")
		b.WriteString("| Rule | Source | Severity | What it checks |\n|---|---|---|---|\n")
		for _, r := range catRows {
			b.WriteString(fmt.Sprintf("| [%s](%s/) | %s | %s | %s |\n", r.label, r.slug, r.source, r.severity, cell(r.summary)))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// relationsIndex renders the relations catalog landing page: a table per relation kind. A relation
// with a reference page links to it; one still awaiting a doc shows its summary without a link.
func relationsIndex(rels []query.RelationInfo) string {
	var b strings.Builder
	b.WriteString(frontMatter("Relations catalog", "Every query relation the fact base exposes, grouped by kind."))
	b.WriteString("The relations a datalog query joins over. Each documented relation links to its full reference: the hardware it describes, its Go projector, and example queries. See the [querying guide](../../guide/querying/) for how to compose them. This page is generated from the shipped fact base.\n\n")

	byKind := map[string][]query.RelationInfo{}
	for _, r := range rels {
		byKind[r.Kind] = append(byKind[r.Kind], r)
	}
	for _, kind := range orderedKeys(byKind, relationKindOrder) {
		title := kind
		if title == "" {
			title = "other"
		}
		b.WriteString("## " + title + "\n\n")
		b.WriteString("| Relation | Summary |\n|---|---|\n")
		for _, r := range byKind[kind] {
			sig := r.Name
			if len(r.Args) > 0 {
				sig = fmt.Sprintf("%s(%s)", r.Name, strings.Join(r.Args, ", "))
			}
			name := "`" + sig + "`"
			if strings.TrimSpace(r.Detail) != "" {
				name = fmt.Sprintf("[`%s`](%s/)", sig, r.Name)
			}
			b.WriteString(fmt.Sprintf("| %s | %s |\n", name, cell(r.Summary)))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// orderedKeys returns the keys of m in preferred order first, then any remaining keys sorted, so
// the output is deterministic and a newly added group still appears.
func orderedKeys[V any](m map[string][]V, preferred []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, k := range preferred {
		if _, ok := m[k]; ok {
			out = append(out, k)
			seen[k] = true
		}
	}
	var rest []string
	for k := range m {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// cell escapes a summary for a markdown table cell (pipes would split the row).
func cell(s string) string { return strings.ReplaceAll(s, "|", "\\|") }
