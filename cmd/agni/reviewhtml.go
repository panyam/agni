package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/core/check"
	rpt "github.com/panyam/agni/core/report"
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/mounts"
)

// buildChecklist maps one review run onto the shared report model.
//
// The mapping lives in the CLI, as service.VerdictReport's does, so core/report never imports
// core/review.
//
// Area and item ORDER is the manifest's, untouched. The check report sorts rules worst-first because
// nobody authored that order, while a checklist's order is the team's.
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
// THE LINK IS SYNTHESIZED. A review item carries findings, not verdicts, because core/review keeps
// what fired rather than what was considered. VerdictID is derived from (rule, subjects), so the id a
// verdict for this finding would carry can be computed without one in hand.
//
// KNOWN LIMIT (agni issue 349): a rule whose verdict names a TUPLE (a clearance between two nets)
// files its finding under one subject, so the id built here names one entity and the real verdict
// names two. The viewer treats that id as a stale link, opening the design with the checks run and
// nothing highlighted.
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
			}, id, f.Rule),
		})
	}
	return out
}

// checklistMeta builds the page header and settles whether the run may promise links.
//
// It applies the rule `check --server` applies by calling the same two functions, verdictLinkTarget
// and verifyServerMount, so the two surfaces cannot disagree about what a link means. The mount has to
// be one the operator DECLARED, and the server has to agree it serves that name from the same root.
func checklistMeta(cmd *cobra.Command, ll *localLoader, designArg string, spec serverSpec) (rpt.Checklist, error) {
	designURI, err := cliArgURI(designArg)
	if err != nil {
		return rpt.Checklist{}, err
	}
	ws, _ := workspace()
	urlBase := spec.base()
	mountPath, contentHash, why := verdictLinkTarget(cmd.Context(), ws, ll, designURI, spec.self)
	withheld := ""
	if urlBase != "" && why != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: --server is set but no findings were linked: %s\n", why)
		withheld = why
	}
	if urlBase != "" && mountPath != "" {
		if m, ok := mounts.Find(ws.Mounts(), mountURIAuthority(designURI)); ok {
			keep, note := verifyServerMount(cmd.Context(), urlBase, m)
			if note != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s\n", note)
			}
			if !keep {
				mountPath = ""
				withheld = note
			}
		}
	}
	return rpt.Checklist{
		Design:        designURI,
		Generated:     time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		ContentHash:   contentHash,
		URLBase:       urlBase,
		MountPath:     mountPath,
		LinksWithheld: withheld,
	}, nil
}
