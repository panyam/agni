package main

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every link the BUILT site makes to itself names a page or file that exists (agni issue 1169).
//
// Built rather than source, because the builder rewrites links: a relation card's
// `../../../../docsite/content/reference/analogy.md` comes out as `/agni/reference/analogy/`, and the
// nav templates and HeaderNavLinks.json only become links in the build. A source scan would need a
// second copy of that rewrite and still miss the template links. A link one level short builds,
// renders and 404s, which is how web-client.md's link to the embedding guide shipped in #1158.
func TestBuiltSiteLinksResolve(t *testing.T) {
	out := t.TempDir()
	saved := Site.OutputDir
	Site.OutputDir = out
	t.Cleanup(func() { Site.OutputDir = saved })
	Site.Rebuild(nil)

	broken, checked, err := brokenLinks(out, "static")
	if err != nil {
		t.Fatal(err)
	}
	// A walk that read no links passes over nothing, so it has to have read the site's.
	if checked < 1000 {
		t.Fatalf("checked %d links across the built site, which is too few to be the site", checked)
	}
	for _, b := range broken {
		t.Error(b)
	}
}

// The checker reports the one link that names nothing, and none of the ones that resolve: a page
// directory, a file, a static asset, a fragment, an external link and a link outside the site.
func TestBrokenLinksFindsOnlyTheBrokenOne(t *testing.T) {
	out, static := t.TempDir(), t.TempDir()
	write := func(root, rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(out, "guide/index.html", "")
	write(out, "img/a.svg", "")
	write(static, "app.css", "")
	write(out, "page/index.html", `
		<a href="/agni/guide/">ok</a> <a href="/agni/guide/#part">ok</a> <img src="/agni/img/a.svg">
		<link href="/agni/static/app.css"> <a href="https://example.com/x">ok</a> <a href="/agni/demo/">ok</a>
		<a href="/agni/pagefind/pagefind.js">ok</a> <a href="#top">ok</a> <a href="mailto:x@example.com">ok</a>
		<a href="../guide/">ok</a> <a href="../../agni/img/a.svg">ok</a>
		<a href="../architecture/build/embedding-the-viewer/">broken</a>`)

	broken, checked, err := brokenLinks(out, static)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"page/index.html links to /agni/architecture/build/embedding-the-viewer/, which the build has no page or file for"}
	// The broken link is relative, as the one that shipped in #1158 was, so this also proves a relative
	// link is resolved against the page rather than skipped.
	if fmt.Sprint(broken) != fmt.Sprint(want) {
		t.Errorf("broken = %q, want %q", broken, want)
	}
	if checked != 7 {
		t.Errorf("checked %d links on the page, want the 7 inside the site", checked)
	}
}

var linkAttr = regexp.MustCompile(`(?:href|src)="([^"]*)"`)

// brokenLinks reads every HTML page under out and returns, sorted, each link under PathPrefix that
// names nothing in out, and how many such links it checked. A link into PathPrefix/static/ resolves
// against staticDir, since the build copies that folder in after it renders. Two prefixes are not
// the build's to answer for: demo/ is the static demo docs.yml deploys beside the site, and
// pagefind/ is the search index `make build` writes over the finished tree.
func brokenLinks(out, staticDir string) ([]string, int, error) {
	prefix := PathPrefix + "/"
	elsewhere := []string{prefix + "demo/", prefix + "pagefind/"}
	seen := map[string]bool{}
	var broken []string
	checked := 0
	err := filepath.WalkDir(out, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		page, _ := filepath.Rel(out, p)
		// The page's own URL, which a relative link resolves against: dir/index.html serves at dir/.
		base, err := url.Parse(prefix + strings.TrimSuffix(filepath.ToSlash(page), "index.html"))
		if err != nil {
			return err
		}
		for _, m := range linkAttr.FindAllStringSubmatch(string(body), -1) {
			ref, err := url.Parse(strings.ReplaceAll(m[1], "&amp;", "&"))
			if err != nil || ref.Scheme != "" || ref.Host != "" || ref.Path == "" {
				continue // another site, a mailto:, or a fragment on this page
			}
			// The builder leaves a page-relative link as written, so it is checked where a browser
			// would send it.
			u := base.ResolveReference(ref)
			if !strings.HasPrefix(u.Path, prefix) || hasAnyPrefix(u.Path, elsewhere) {
				continue
			}
			checked++
			if !exists(u.Path, prefix, out, staticDir) {
				msg := fmt.Sprintf("%s links to %s, which the build has no page or file for", filepath.ToSlash(page), u.Path)
				if !seen[msg] {
					seen[msg] = true
					broken = append(broken, msg)
				}
			}
		}
		return nil
	})
	sort.Strings(broken)
	return broken, checked, err
}

// exists says whether a site path names a page (a directory with an index.html) or a file.
func exists(sitePath, prefix, out, staticDir string) bool {
	rel := strings.TrimPrefix(sitePath, prefix)
	root := out
	if rest, ok := strings.CutPrefix(rel, "static/"); ok {
		root, rel = staticDir, rest
	}
	p := filepath.Join(root, filepath.FromSlash(path.Clean("/"+rel)))
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	if info.IsDir() {
		_, err = os.Stat(filepath.Join(p, "index.html"))
		return err == nil
	}
	return true
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
