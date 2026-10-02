package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/panyam/agni/core/review"
)

// This file resolves `agni review`'s checklist when the operator did not name one.
//
// It lives at the CLI edge because CreateReview takes the manifest as a VALUE (C22) and has no
// filesystem. Reading the file the project named is the CLI's job, as reading the file
// `--conventions` names is (docsite/content/architecture/web-services.md).

// reviewManifestFor returns the manifest `agni review` should run, which is the file the operator
// named or, when they named none, the one the designs' project declares.
//
// An explicit --checklist WINS outright and resolves nothing, so the flag still overrides a project's
// checklist, and a loose file (a design on a mounted folder that belongs to no project) needs it.
func reviewManifestFor(ctx context.Context, checklist string, stdin io.Reader, designs []string) (review.Manifest, string, error) {
	// `-` reads the manifest from stdin, which is how the Python client's CLI transport sends the
	// one a CreateReview request carries (agni issue 734), as `query --set -` takes a set.
	if checklist == "-" {
		man, err := review.Load(stdin)
		return man, "", err
	}
	if checklist != "" {
		man, err := loadManifest(checklist)
		return man, "", err
	}
	return resolveChecklist(ctx, designs)
}

// resolveChecklist returns the manifest to run and a note describing where it came from, for the
// designs named on the command line.
//
// It resolves PER DESIGN and then insists the answers agree. RenderAggregateMarkdown labels its
// traceability matrix from Reports[0].Areas, assuming "all reports share the manifest structure", so
// a rollup over two manifests would label rows from the first checklist and fill cells from the
// second, with every row looking answered.
func resolveChecklist(ctx context.Context, designs []string) (review.Manifest, string, error) {
	// byURI keeps the designs behind each distinct checklist so a disagreement can name both sides.
	byURI := map[string][]string{}
	var unowned, noChecklist []string
	for _, d := range designs {
		uri, project, err := cliProjectChecklist(ctx, d)
		if err != nil {
			return review.Manifest{}, "", err
		}
		switch {
		case project == "":
			unowned = append(unowned, d)
		case uri == "":
			noChecklist = append(noChecklist, fmt.Sprintf("%s (%s)", d, project))
		default:
			byURI[uri] = append(byURI[uri], d)
		}
	}
	if len(unowned) > 0 {
		return review.Manifest{}, "", fmt.Errorf(
			"review needs --checklist <manifest.yaml>: %s %s no project, so there is no declared checklist to fall back on",
			strings.Join(unowned, ", "), plural(len(unowned), "belongs to", "belong to"))
	}
	if len(noChecklist) > 0 {
		return review.Manifest{}, "", fmt.Errorf(
			"review needs --checklist <manifest.yaml>: %s %s no checklist. Add a `checklist:` line to project.yaml, or put a review.yaml beside it",
			strings.Join(noChecklist, ", "), plural(len(noChecklist), "declares", "declare"))
	}
	if len(byURI) > 1 {
		return review.Manifest{}, "", fmt.Errorf(
			"the named designs resolve to different checklists (%s); pass --checklist to score them all against one",
			describeSplit(byURI))
	}
	var uri string
	for u := range byURI {
		uri = u
	}
	if uri == "" {
		// No designs at all. cobra's MinimumNArgs(1) makes this unreachable from the CLI, but a zero
		// Manifest would score every item not-automated.
		return review.Manifest{}, "", fmt.Errorf("review needs --checklist <manifest.yaml>")
	}
	man, err := loadManifest(localOf(uri))
	if err != nil {
		// The project named this file, so the operator never typed it. The error has to name it.
		return review.Manifest{}, "", fmt.Errorf("the checklist %s declares (%s): %w", projectOf(byURI, uri, ctx), uri, err)
	}
	return man, fmt.Sprintf("note: running the checklist %s declares (%s); pass --checklist to run a different one.\n",
		projectOf(byURI, uri, ctx), uri), nil
}

// projectOf names the project that declared uri, for a message. Every design behind one URI resolved
// to the same project in practice, so the first is representative.
func projectOf(byURI map[string][]string, uri string, ctx context.Context) string {
	for _, d := range byURI[uri] {
		// The error is dropped on purpose. Every design here already resolved cleanly in
		// resolveChecklist, and this lookup is only for a MESSAGE, so a failure (the descriptor changed
		// mid-command) falls back to "the project".
		if _, project, _ := cliProjectChecklist(ctx, d); project != "" {
			return project
		}
	}
	return "the project"
}

// describeSplit renders "uri for design, design; uri for design" in sorted order, so the same
// disagreement always produces the same message.
func describeSplit(byURI map[string][]string) string {
	uris := make([]string, 0, len(byURI))
	for u := range byURI {
		uris = append(uris, u)
	}
	sort.Strings(uris)
	parts := make([]string, 0, len(uris))
	for _, u := range uris {
		ds := append([]string{}, byURI[u]...)
		sort.Strings(ds)
		parts = append(parts, fmt.Sprintf("%s for %s", u, strings.Join(ds, ", ")))
	}
	return strings.Join(parts, "; ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// noteChecklist writes the resolution note, if there is one. w is stderr, as for noteSource.
func noteChecklist(w io.Writer, note string) {
	if note != "" {
		fmt.Fprint(w, note)
	}
}
