// Command catalogdocs generates the docsite's rule and relation catalog (issue 14) from the composed
// engine catalog and the embedded per-rule and per-relation Detail markdown. It writes
// docsite/content/reference/{rules,relations}/ and the SVG cards under
// docsite/static/images/catalog/, so the stdlib docs stay the one source of truth.
//
// Run it from the repo root via `make catalog-docs`. `make catalog-docs-check` regenerates and fails
// when the committed output drifts. Output is sorted, so a clean regen produces no diff.
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

	// Blank imports register the built-in rules and the query relations (with their Detail docs)
	// into the process-global registries. The intent, datalog and profile rules come from their own
	// DocRules() accessors below, because intent and profile rules are generated per declaration and
	// have no static catalog entry.
	_ "github.com/panyam/agni/stdlib/lib"
	_ "github.com/panyam/agni/stdlib/relations"
	_ "github.com/panyam/agni/stdlib/rules/builtin"
)

var (
	contentDir   = flag.String("content", "docsite/content/reference", "docsite reference content dir")
	staticDir    = flag.String("static", "docsite/static/images/catalog", "docsite static dir for catalog images")
	ruleImgSrc   = flag.String("rule-images", "stdlib/rules/builtin/docs/images", "source dir for built-in rule doc images")
	intentImgSrc = flag.String("intent-images", "stdlib/rules/intent/docs/images", "source dir for intent rule doc images")
	relImgSrc    = flag.String("relation-images", "stdlib/relations/facts/docs/images", "source dir for relation doc images")
)

// imageRef matches a markdown image whose target is a doc-relative images/<file> path, the form the
// embedded Detail uses. Absolute or templated refs are left alone.
var imageRef = regexp.MustCompile(`\]\(images/([^)]+)\)`)

// leadingHeading matches the "## <title>" line (and trailing blank lines) opening a Detail body. It
// is stripped because the page's front-matter supplies the H1.
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
	query.KindDerived,
	query.KindPredicate,
	query.KindExtension,
}

// libSource is where a library module's file is read on GitHub, for the link a derived member's page
// carries to its definition.
const libSource = "https://github.com/panyam/agni/blob/main/stdlib/lib/"

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

// ruleSource is one origin of documented rules. label fills the index's Source column. prefix
// namespaces the page slug and link so a non-built-in name never collides with a built-in of the
// same name. imgSrc is the dir holding this source's docs/images, empty when it ships no cards.
type ruleSource struct {
	rules  []*check.Rule
	label  string
	prefix string
	imgSrc string
}

// catalogRow is one rendered rules-index entry.
type catalogRow struct {
	category, label, slug, source, severity, summary string
}

// genRules writes one page per documented rule across every source, the grouped index, and the
// images those pages reference. Built-in pages keep flat slugs; the others are namespaced by source
// so a name shared with a built-in cannot overwrite its page.
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

// pageSlug is a rule's page filename stem, the bare name for built-ins (prefix "") and
// "<prefix>-<name>" otherwise.
func pageSlug(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "-" + name
}

// linkLabel is a rule's display text, the bare name for built-ins and otherwise "<prefix>/<name>",
// the composed catalog name a review manifest binds to.
func linkLabel(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}

// genRelations writes one page per documented relation, the grouped index, and the images those
// pages reference. A relation with no Detail (a computed predicate may have none) gets an index row
// with no page and no link.
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
		if rel.Kind == query.KindDerived {
			def, err := definitionSection(rel.Name)
			if err != nil {
				return err
			}
			body += def
		}
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

// definitionSection renders a derived member's definition as the engine registered it: the typed
// signature, which argument types were inferred rather than declared, every clause, and a link to the
// module file. It is generated rather than written in the member's doc so the page cannot drift from
// the rules.
func definitionSection(path string) (string, error) {
	e, err := query.Describe(path)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("\n### How it is defined\n\n")
	fmt.Fprintf(&b, "`%s`, a derived relation in the `%s` module, defined in Datalog in [`stdlib/lib/%s.dl`](%s%s.dl). ", e.Signature(), e.Module, e.Module, libSource, e.Module)
	var inferred []string
	for _, a := range e.Args {
		if a.Inferred {
			inferred = append(inferred, "`"+a.Name+"`")
		}
	}
	if len(inferred) > 0 {
		fmt.Fprintf(&b, "The types of %s are inferred from the rules rather than declared. ", strings.Join(inferred, ", "))
	}
	b.WriteString("In a clause, a bare name is another member of the same module and a dotted name is a full path.\n\n")
	b.WriteString("```\n")
	for _, clause := range e.Definition {
		b.WriteString(clause + ";\n")
	}
	b.WriteString("```\n\n")
	b.WriteString("`agni query --relations " + path + "` prints the same definition. [Adding a library member](../../../build/library-member/) explains how the library is built.\n")
	return b.String(), nil
}

// prepareDetail strips the leading heading and rewrites doc-relative image refs to the docsite
// static path, recording each image basename in seen so only referenced images are copied.
func prepareDetail(detail, kind string, seen map[string]bool) string {
	body := leadingHeading.ReplaceAllString(strings.TrimSpace(detail), "")
	body = imageRef.ReplaceAllStringFunc(body, func(m string) string {
		name := imageRef.FindStringSubmatch(m)[1]
		seen[name] = true
		return fmt.Sprintf("](%s/static/images/catalog/%s/%s)", pathPrefixExpr, kind, name)
	})
	return body + "\n"
}

// pathPrefixExpr is the s3gen template expression for the site's URL prefix. Content markdown is
// templated, so an absolute static link follows a prefix change.
const pathPrefixExpr = "{{.Site.PathPrefix}}"

// remedySection renders a rule's Remedy as the page's first "### " section, or "" for a rule that
// states none. It leads the page because a reader arrives from a finding that just fired and wants
// to know what to do.
//
// A heading rather than a blockquote, because the docsite's stylesheet has no blockquote rule and a
// "> " callout renders as unstyled indented text (build/check-rule.md prefers sections too).
//
// ONLY Remedy is projected, though the Impact FIELD is equally absent here. Most rule docs already
// write their own "### Impact" section, so injecting the field would print it twice. Adding Impact
// means first taking that section out of the doc bodies.
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

// resetDir ensures dir exists and removes every file in it with the given extension, so a removed
// rule, relation or image ref drops its file rather than lingering as an orphan the freshness check
// cannot see. Files with other extensions survive.
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

// copyImages copies the named SVGs from src into dst. A referenced image missing from src is an
// error, not a silent skip.
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

// rulesIndex renders the rules catalog landing page, a table per category with rows sorted by label,
// each linking to its page with its source and severity.
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

// relationsIndex renders the relations catalog landing page, a table per relation kind. A relation
// with a reference page links to it, and one without shows its summary unlinked.
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
		if kind == query.KindDerived {
			b.WriteString("Defined in Datalog over the relations above rather than projected from the design, in the shipped library under `stdlib/lib`. A query calls them the same way. [Adding a library member](../../build/library-member/) explains how they are built.\n\n")
		}
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
