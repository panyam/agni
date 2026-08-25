package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/core/check"
	rpt "github.com/panyam/agni/core/report"
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/internal/mounts"
)

// buildChecklist maps one review run onto the shared report model.
//
// The mapping lives here rather than in core/report for the reason buildVerdictReport does: the
// report package owns the page and the vocabulary, and the CLI owns the adapter from whatever shape
// this surface happens to hold. That keeps core/report from importing core/review, so a report is a
// projection over a run rather than a package that knows every run type.
//
// Area and item ORDER is the manifest's, untouched. The check report sorts rules worst-first because
// nobody authored that order; a checklist's order is the team's, and an item that has been P3 in
// their process for years has to stay where they expect it.
func buildChecklist(r review.Report, meta rpt.Checklist) rpt.Checklist {
	out := meta
	out.Name = r.Manifest
	if out.Name == "" {
		out.Name = "Review"
	}
	t := r.Tally()
	out.Summary = t.String()
	out.Covered, out.Answered, out.Total = t.Covered(), t.Answered(), t.Total
	for _, a := range r.Areas {
		at := a.Tally()
		area := rpt.ChecklistArea{Name: a.Area.Name, Summary: at.String()}
		for _, it := range a.Items {
			area.Items = append(area.Items, rpt.ChecklistItem{
				ID:       it.Item.ID,
				Title:    it.Item.Title,
				Outcome:  string(it.Outcome),
				Note:     it.Note,
				Evidence: evidenceFor(it, meta),
			})
		}
		out.Areas = append(out.Areas, area)
	}
	return out
}

// evidenceFor turns an item's findings into linked rows.
//
// THE LINK IS SYNTHESIZED, and that is worth stating plainly. A review item carries findings, not
// verdicts, because core/review evaluates rules and keeps what fired rather than what was considered.
// VerdictID is DERIVED rather than assigned, so the id a verdict for this (rule, subject) would carry
// can be computed here without one in hand, which is the property its own doc comment calls being a
// question you can pose.
//
// KNOWN LIMIT, which is agni issue 349. A rule whose verdict names a TUPLE (a clearance between two
// nets) files its finding under one subject, so the id built from that finding names one entity and
// the real verdict names two. The viewer treats an id it cannot find as a stale link: it opens the
// design, runs the checks, highlights nothing, and leaves the canvas alone. So the reader still lands
// on the right board with the answers computed, and does not get a broken page. Closing 349 or giving
// review a verdict of its own removes the gap; neither is needed for this to be worth shipping.
func evidenceFor(it review.ItemResult, meta rpt.Checklist) []rpt.ChecklistEvidence {
	if len(it.Findings) == 0 {
		return nil
	}
	out := make([]rpt.ChecklistEvidence, 0, len(it.Findings))
	for _, f := range it.Findings {
		id := check.VerdictID(check.Verdict{Rule: f.Rule, Subjects: []check.Entity{f.Subject}})
		out = append(out, rpt.ChecklistEvidence{
			Rule:    f.Rule,
			Subject: check.EntityRef(f.Subject),
			Message: f.Message,
			URL: rpt.VerdictURL(rpt.Report{
				URLBase: meta.URLBase, MountPath: meta.MountPath, ContentHash: meta.ContentHash,
			}, id),
		})
	}
	return out
}

// checklistMeta builds the page header and settles whether the run may promise links.
//
// It applies exactly the rule `check --url-base` applies, by calling the same two functions rather
// than restating them: the mount has to be one the operator DECLARED, and the server has to agree it
// serves that name from the same root. Restating it here is how the two surfaces would drift into
// disagreeing about what a link means, which is the whole reason the report model is shared.
func checklistMeta(cmd *cobra.Command, ll *localLoader, designArg, urlBase string) (rpt.Checklist, error) {
	designURI, err := cliArgURI(designArg)
	if err != nil {
		return rpt.Checklist{}, err
	}
	ws, _ := workspace()
	mountPath, contentHash, why := verdictLinkTarget(cmd.Context(), ws, ll, designURI)
	if urlBase != "" && why != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: --url-base is set but no findings were linked: %s\n", why)
	}
	if urlBase != "" && mountPath != "" {
		if m, ok := mounts.Find(ws.Mounts(), mountURIAuthority(designURI)); ok {
			keep, note := verifyServerMount(cmd.Context(), urlBase, m)
			if note != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s\n", note)
			}
			if !keep {
				mountPath = ""
			}
		}
	}
	return rpt.Checklist{
		Design:      designURI,
		Generated:   time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		ContentHash: contentHash,
		URLBase:     urlBase,
		MountPath:   mountPath,
	}, nil
}
