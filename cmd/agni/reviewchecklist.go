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
// It lives at the CLI edge and not in the service, and that is a contract rather than a preference.
// CreateReview takes the manifest as a VALUE (C22), which is what lets it score a design with no
// filesystem at all; a service that resolved a project's checklist_uri behind the caller's back would
// give the run a file dependency the whole seam exists to remove. Reading the file the project named
// is the CLI's job, exactly as reading the file `--conventions` names is.

// reviewManifestFor returns the manifest `agni review` should run: the file the operator named, or
// the one the designs' project declares when they named none.
//
// An explicit --checklist WINS outright and resolves nothing, which is what keeps the flag meaningful
// on a design that does belong to a project. It also means the loose-file case — a design on a mounted
// folder that belongs to no project — behaves exactly as it always has, flag and all.
func reviewManifestFor(ctx context.Context, checklist string, designs []string) (review.Manifest, string, error) {
	if checklist != "" {
		man, err := loadManifest(checklist)
		return man, "", err
	}
	return resolveChecklist(ctx, designs)
}

// resolveChecklist returns the manifest to run and a note describing where it came from, for the
// designs named on the command line.
//
// It resolves PER DESIGN and then insists the answers agree. A rollup scored against two different
// manifests would render as though it were one: RenderAggregateMarkdown builds its traceability
// matrix from Reports[0].Areas on the documented assumption that "all reports share the manifest
// structure", so the rows would be labelled from the first design's checklist and the cells filled
// from the second's. Every row would look answered and the labels would be wrong, which is worse than
// refusing.
func resolveChecklist(ctx context.Context, designs []string) (review.Manifest, string, error) {
	// byURI keeps the designs behind each distinct checklist so a disagreement can name both sides
	// rather than just asserting that one exists.
	byURI := map[string][]string{}
	var unowned, noChecklist []string
	for _, d := range designs {
		uri, project := cliProjectChecklist(ctx, d)
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
		// No designs at all. cobra's MinimumNArgs(1) makes this unreachable from the CLI, and it is
		// handled rather than left to return a zero Manifest that would score every item not-automated.
		return review.Manifest{}, "", fmt.Errorf("review needs --checklist <manifest.yaml>")
	}
	man, err := loadManifest(localOf(uri))
	if err != nil {
		// The project named this file, so the operator did not type it and cannot see it in their
		// command. Naming it is the difference between an actionable error and a confusing one.
		return review.Manifest{}, "", fmt.Errorf("the checklist %s declares (%s): %w", projectOf(byURI, uri, ctx), uri, err)
	}
	return man, fmt.Sprintf("note: running the checklist %s declares (%s); pass --checklist to run a different one.\n",
		projectOf(byURI, uri, ctx), uri), nil
}

// projectOf names the project that declared uri, for a message. Every design behind one URI resolved
// to the same project in practice, so the first is representative.
func projectOf(byURI map[string][]string, uri string, ctx context.Context) string {
	for _, d := range byURI[uri] {
		if _, project := cliProjectChecklist(ctx, d); project != "" {
			return project
		}
	}
	return "the project"
}

// describeSplit renders "uri (design, design), uri (design)" with a stable order, so the same
// disagreement produces the same message rather than one that varies by map iteration.
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

// noteChecklist writes the resolution note, if there is one. Notes go to stderr so a redirect never
// contaminates a `--format json` report on stdout, matching noteSource.
func noteChecklist(w io.Writer, note string) {
	if note != "" {
		fmt.Fprint(w, note)
	}
}
