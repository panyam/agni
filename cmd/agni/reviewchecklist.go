package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// This file resolves which checklist `agni review` runs.
//
// It lives at the CLI edge because CreateReview takes the manifest as a VALUE (C22) and has no
// filesystem. A project's checklists already arrive as values in its config (agni issue 828), so
// only a checklist FILE is read here, as the file `--conventions` names is
// (docsite/content/architecture/web-services.md).

// reviewManifestFor returns the manifest `agni review` should run and a note saying where it came
// from.
//
// --checklist takes a NAME or a FILE. A value ending in .yaml or .yml, or "-" for stdin, is a file,
// read outright, so a manifest from outside the project still runs and a loose design (one on a
// mounted folder that belongs to no project) needs it. Any other value names one of the designs'
// project checklists. With no --checklist the project's first checklist runs, which is its default.
func reviewManifestFor(ctx context.Context, checklist string, stdin io.Reader, designs []string) (review.Manifest, string, error) {
	// `-` reads the manifest from stdin, which is how the Python client's CLI transport sends the
	// one a CreateReview request carries (agni issue 734), as `query --set -` takes a set.
	if checklist == "-" {
		man, err := review.Load(stdin)
		return man, "", err
	}
	if isChecklistFile(checklist) {
		man, err := loadManifest(checklist)
		return man, "", err
	}
	return resolveChecklist(ctx, checklist, designs)
}

// isChecklistFile reports whether a --checklist value names a file rather than a project checklist.
// A checklist name is a plain word, so anything spelled as a YAML file is one.
func isChecklistFile(v string) bool {
	return strings.HasSuffix(v, ".yaml") || strings.HasSuffix(v, ".yml")
}

// resolveChecklist returns the project checklist called name (the project's first when name is
// empty) and a note naming it, for the designs on the command line.
//
// It resolves PER DESIGN and then insists the answers agree. RenderAggregateMarkdown labels its
// traceability matrix from Reports[0].Areas, assuming "all reports share the manifest structure", so
// a rollup over two manifests would label rows from the first checklist and fill cells from the
// second, with every row looking answered.
func resolveChecklist(ctx context.Context, name string, designs []string) (review.Manifest, string, error) {
	// byKey keeps the designs behind each distinct project checklist, keyed "project:name", so a
	// disagreement can name both sides.
	byKey := map[string][]string{}
	found := map[string]*webapi.NamedChecklist{}
	var unowned, missing []string
	for _, d := range designs {
		lists, project, err := cliProjectChecklists(ctx, d)
		if err != nil {
			return review.Manifest{}, "", err
		}
		if project == "" {
			unowned = append(unowned, d)
			continue
		}
		c := pickChecklist(lists, name)
		if c == nil {
			missing = append(missing, fmt.Sprintf("%s (%s, %s)", d, project, declaredChecklists(lists)))
			continue
		}
		key := project + ":" + c.GetName()
		byKey[key] = append(byKey[key], d)
		found[key] = c
	}
	if len(unowned) > 0 {
		return review.Manifest{}, "", fmt.Errorf(
			"review needs --checklist <manifest.yaml>: %s %s no project, so there is no declared checklist to fall back on",
			strings.Join(unowned, ", "), plural(len(unowned), "belongs to", "belong to"))
	}
	if len(missing) > 0 {
		want := "no checklist. Add one under checklists: in project.yaml, or pass --checklist <manifest.yaml>"
		if name != "" {
			want = fmt.Sprintf("no checklist named %q", name)
		}
		return review.Manifest{}, "", fmt.Errorf("review: %s %s %s",
			strings.Join(missing, ", "), plural(len(missing), "declares", "declare"), want)
	}
	if len(byKey) > 1 {
		return review.Manifest{}, "", fmt.Errorf(
			"the named designs resolve to different checklists (%s); pass --checklist to score them all against one",
			describeSplit(byKey))
	}
	var key string
	for k := range byKey {
		key = k
	}
	if key == "" {
		// No designs at all. cobra's MinimumNArgs(1) makes this unreachable from the CLI, but a zero
		// Manifest would score every item not-automated.
		return review.Manifest{}, "", fmt.Errorf("review needs --checklist <manifest.yaml>")
	}
	project, checklist, _ := strings.Cut(key, ":")
	man := service.ManifestFromProto(found[key].GetManifest())
	// The project validated it when its descriptor loaded, so a failure here means the descriptor
	// changed underneath the command. Say whose checklist it was, since the operator never typed it.
	if err := review.ValidateStructure(man); err != nil {
		return review.Manifest{}, "", fmt.Errorf("the checklist %q %s declares: %w", checklist, project, err)
	}
	return man, fmt.Sprintf("note: running the checklist %q %s declares; pass --checklist to run a different one.\n",
		checklist, project), nil
}

// pickChecklist returns the checklist called name, or the first when name is empty, nil when there
// is none.
func pickChecklist(lists []*webapi.NamedChecklist, name string) *webapi.NamedChecklist {
	for _, c := range lists {
		if name == "" || c.GetName() == name {
			return c
		}
	}
	return nil
}

// declaredChecklists names what a project does declare, for the message that says it lacks the one
// asked for.
func declaredChecklists(lists []*webapi.NamedChecklist) string {
	if len(lists) == 0 {
		return "which declares none"
	}
	names := make([]string, 0, len(lists))
	for _, c := range lists {
		names = append(names, c.GetName())
	}
	return "which declares " + strings.Join(names, ", ")
}

// describeSplit renders "key for design, design; key for design" in sorted order, so the same
// disagreement always produces the same message.
func describeSplit(byKey map[string][]string) string {
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		ds := append([]string{}, byKey[k]...)
		sort.Strings(ds)
		parts = append(parts, fmt.Sprintf("%s for %s", k, strings.Join(ds, ", ")))
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
