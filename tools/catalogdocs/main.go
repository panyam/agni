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

	// Blank imports register the built-in rule catalog and the query relations (plus their
	// embedded Detail docs) into the process-global registries DefaultCatalog and query.Catalog
	// read. Only the built-in sources are pulled: this generator scopes to the two surfaces with
	// the richest reference docs (issue 14), leaving datalog/intent/profile catalogs for later.
	_ "github.com/panyam/agni/stdlib/relations"
	_ "github.com/panyam/agni/stdlib/rules/builtin"
)

var (
	contentDir = flag.String("content", "docsite/content/reference", "docsite reference content dir")
	staticDir  = flag.String("static", "docsite/static/images/catalog", "docsite static dir for catalog images")
	ruleImgSrc = flag.String("rule-images", "stdlib/rules/builtin/docs/images", "source dir for rule doc images")
	relImgSrc  = flag.String("relation-images", "stdlib/relations/facts/docs/images", "source dir for relation doc images")
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

// genRules writes one page per built-in rule plus the grouped index, and copies the images those
// pages reference into the catalog's rules image dir.
func genRules() error {
	rules := check.DefaultCatalog().Rules()
	sort.Slice(rules, func(i, j int) bool { return rules[i].Name < rules[j].Name })

	outDir := filepath.Join(*contentDir, "rules")
	imgOut := filepath.Join(*staticDir, "rules")
	if err := resetDir(outDir, ".md"); err != nil {
		return err
	}
	if err := resetDir(imgOut, ".svg"); err != nil {
		return err
	}

	images := map[string]bool{}
	for _, r := range rules {
		if strings.TrimSpace(r.Detail) == "" {
			continue
		}
		body := prepareDetail(r.Detail, "rules", images)
		page := frontMatter(r.Name, r.Summary) + body
		if err := os.WriteFile(filepath.Join(outDir, r.Name+".md"), []byte(page), 0o644); err != nil {
			return err
		}
	}
	if err := copyImages(*ruleImgSrc, imgOut, images); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "index.md"), []byte(rulesIndex(rules)), 0o644)
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

// rulesIndex renders the rules catalog landing page: a table per category, rows sorted by name,
// each linking to its page with its severity.
func rulesIndex(rules []*check.Rule) string {
	var b strings.Builder
	b.WriteString(frontMatter("Rules catalog", "Every built-in check rule, grouped by category."))
	b.WriteString("The built-in EE rule catalog. Each rule links to its full reference: what it means, why it matters, the guards it applies, and a fires-versus-fine diagram. This page is generated from the shipped catalog, so it always matches the engine.\n\n")

	byCat := map[string][]*check.Rule{}
	for _, r := range rules {
		byCat[r.Tags[check.KeyCategory]] = append(byCat[r.Tags[check.KeyCategory]], r)
	}
	for _, cat := range orderedKeys(byCat, ruleCategoryOrder) {
		title := cat
		if title == "" {
			title = "other"
		}
		b.WriteString("## " + title + "\n\n")
		b.WriteString("| Rule | Severity | What it checks |\n|---|---|---|\n")
		for _, r := range byCat[cat] {
			b.WriteString(fmt.Sprintf("| [%s](%s/) | %s | %s |\n", r.Name, r.Name, r.Severity, cell(r.Summary)))
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
