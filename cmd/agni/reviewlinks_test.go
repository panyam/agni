package main

import (
	"html"
	"net/url"
	"regexp"
	"sort"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
)

var verdictParam = regexp.MustCompile(`verdict=([^&"]+)`)

// TestEveryChecklistLinkResolves is agni issue 485's acceptance, measured the way the ticket
// measured it. The tutorial project's two checklists are rendered as pages with links, every verdict
// id a page links is collected, and each must be among the verdicts `check --verdicts` reports for
// the same design. A link synthesized from a finding's subject misses whenever the verdict is keyed
// on a tuple, on a context entity, or on a profile's signal, and the viewer then opens the design with
// nothing highlighted.
func TestEveryChecklistLinkResolves(t *testing.T) {
	freshWorkspace(t)
	cliMountSpecs = []string{"tp=../../examples/tutorial-project"}
	const design = "mount://tp/designs/gateway"

	var run webapi.CheckDesignResponse
	if err := protojson.Unmarshal([]byte(runCheck(t, "--verdicts", "--format", "json", design)), &run); err != nil {
		t.Fatal(err)
	}
	verdicts := map[string]bool{}
	for _, v := range run.GetVerdicts() {
		verdicts[v.GetId()] = true
	}
	if len(verdicts) == 0 {
		t.Fatal("the check reported no verdicts, so every link would fail for the wrong reason")
	}

	links, missing := 0, []string{}
	for _, checklist := range []string{"review", "house"} {
		page, errOut, err := runReviewCapturing(t, "--checklist", checklist, "--format", "html", "--server", urlBaseFlag, design)
		if err != nil {
			t.Fatalf("review --checklist %s: %v\n%s", checklist, err, errOut)
		}
		for _, m := range verdictParam.FindAllStringSubmatch(page, -1) {
			id, err := url.QueryUnescape(html.UnescapeString(m[1]))
			if err != nil {
				t.Fatalf("link %q does not unescape: %v", m[1], err)
			}
			links++
			if !verdicts[id] {
				missing = append(missing, checklist+": "+id)
			}
		}
	}
	// The count guard. Fewer links than the tutorial board's checklists carry means the pages lost
	// their links, and an empty loop resolves every link it saw.
	if links < 10 {
		t.Fatalf("the two checklists linked %d verdicts, want at least 10", links)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d of %d checklist links resolve; these name no verdict the run reported:\n%v", links-len(missing), links, missing)
	}
	t.Logf("%d of %d checklist links resolve", links-len(missing), links)
}
