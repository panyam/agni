package review

import (
	"fmt"
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// The seeding work list, meaning what a run asked for and could not get.
//
// This is the demand side of corpus seeding, so bulk extraction is not speculative. Extracting every
// fact a rule MIGHT want fills a store with unverified rows that look like data. What runs asked for
// and failed to find is bounded, and prioritised by how many items each fact blocks.

// WorkItem is the wire type agni.v1.checks.WorkItem, computed directly rather than through a Go twin
// because the portal consuming it reads the proto.
type WorkItem = checkspb.WorkItem

// WorkList collapses every unmet dependency in a report into the set of facts to find.
//
// Ordering is by how many items a fact blocks, descending, then by part and symbol so the list is
// stable between runs. A report with nothing blocked returns nothing, which is the ordinary state of
// a fully seeded design and not an error.
func WorkList(r Report) []*WorkItem {
	return WorkListAcross([]Report{r})
}

// WorkListAcross merges the work lists of several reports into one, so a fact shared by several
// designs appears once, since seeding its part once unblocks every design that places it.
func WorkListAcross(rs []Report) []*WorkItem {
	type agg struct {
		dep     check.UnmetDependency
		blocked []string
		seen    map[string]bool
	}
	byKey := map[string]*agg{}
	var order []string
	for _, r := range rs {
		for _, ar := range r.Areas {
			for _, it := range ar.Items {
				for _, d := range it.Unmet {
					key := strings.ToUpper(d.MPN) + "\x00" + strings.ToUpper(d.Symbol)
					a, ok := byKey[key]
					if !ok {
						a = &agg{dep: d, seen: map[string]bool{}}
						byKey[key] = a
						order = append(order, key)
					}
					// SpecAbsent from any design wins, so the list does not understate the work.
					if d.SpecAbsent {
						a.dep.SpecAbsent = true
					}
					if a.dep.Manufacturer == "" {
						a.dep.Manufacturer = d.Manufacturer
					}
					label := it.Item.ID
					if label == "" {
						label = it.Item.Title
					}
					if label != "" && !a.seen[label] {
						a.seen[label] = true
						a.blocked = append(a.blocked, label)
					}
				}
			}
		}
	}
	out := make([]*WorkItem, 0, len(order))
	for _, k := range order {
		a := byKey[k]
		sort.Strings(a.blocked)
		out = append(out, &checkspb.WorkItem{
			Dependency: &checkspb.UnmetDependency{
				Mpn: a.dep.MPN, Manufacturer: a.dep.Manufacturer,
				Symbol: a.dep.Symbol, SpecAbsent: a.dep.SpecAbsent,
			},
			Blocked: a.blocked,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].GetBlocked()) != len(out[j].GetBlocked()) {
			return len(out[i].GetBlocked()) > len(out[j].GetBlocked())
		}
		if out[i].GetDependency().GetMpn() != out[j].GetDependency().GetMpn() {
			return out[i].GetDependency().GetMpn() < out[j].GetDependency().GetMpn()
		}
		return out[i].GetDependency().GetSymbol() < out[j].GetDependency().GetSymbol()
	})
	return out
}

// RenderWorkListMarkdown writes the work list as a table, or a single line when there is nothing to
// do, because a table with no rows looks like a bug.
func RenderWorkListMarkdown(items []*WorkItem) string {
	if len(items) == 0 {
		return "No unmet datasheet facts: every check that needed one found it.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "This run needs %d fact(s):\n\n", len(items))
	b.WriteString("| Part | Manufacturer | Symbol | Blocks | Items |\n")
	b.WriteString("|---|---|---|---:|---|\n")
	for _, w := range items {
		part := w.GetDependency().GetMpn()
		if w.GetDependency().GetSpecAbsent() {
			// No document to search means a different next step, so say so.
			part += " (no spec)"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %s |\n",
			part, w.GetDependency().GetManufacturer(), w.GetDependency().GetSymbol(),
			len(w.GetBlocked()), strings.Join(w.GetBlocked(), ", "))
	}
	return b.String()
}
