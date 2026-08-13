package review

import (
	"fmt"
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// The seeding work list: what a run asked for and could not get.
//
// This is the demand side of corpus seeding, and it exists so that bulk extraction is not
// speculative. Extracting every fact a rule MIGHT one day want fills a store with unverified rows
// nobody consults, and each one is a liability because it looks like data. Extracting what runs
// actually asked for and failed to find is bounded by definition and prioritised for free: a fact
// blocking six items is worth more than one blocking one, and only the demand side knows that.

// WorkItem is the wire type: see agni.v1.checks.WorkItem. This layer computes it directly rather
// than through a Go twin, because the portal is its consumer and a parallel type on each side of
// that boundary is what a shared schema exists to avoid.
type WorkItem = checkspb.WorkItem

// WorkList collapses every unmet dependency in a report into the set of facts to find.
//
// Ordering is by how many items a fact blocks, descending, then by part and symbol so a list is
// stable between runs on an unchanged design. A caller working top-down therefore clears the most
// blockage first without having to sort it themselves.
//
// A report with nothing blocked returns nothing, which is the ordinary state of a fully seeded
// design and must not read as an error.
func WorkList(r Report) []*WorkItem {
	return WorkListAcross([]Report{r})
}

// WorkListAcross merges the work lists of several reports, which is what a project spanning designs
// needs: one part seeded once unblocks every design that places it, and asking a person to find the
// same fact per design is how a work list becomes ignored.
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
					// A part with no spec anywhere is the stronger claim, and one design seeing a
					// spec does not make the others' absence go away; keep the harder state so the
					// list does not understate the work.
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
// do. The empty case is stated rather than rendered as an empty table, because "this run needs
// nothing" is a result worth reading and a table with no rows looks like a bug.
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
			// The next step differs, so the list says which it is rather than leaving a reader to
			// discover on arrival that there is no document to search.
			part += " (no spec)"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %s |\n",
			part, w.GetDependency().GetManufacturer(), w.GetDependency().GetSymbol(),
			len(w.GetBlocked()), strings.Join(w.GetBlocked(), ", "))
	}
	return b.String()
}
