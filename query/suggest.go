package query

import "fmt"

// didYouMean returns a `; did you mean "X"?` hint for a relation name the engine does not know,
// naming the closest catalog relation when one is a plausible typo, else "" (WS14-003). It teaches
// the vocabulary at the exact moment a user gets it wrong — the most common newcomer error is a
// mistyped or half-remembered relation name.
func didYouMean(name string) string {
	if s := suggestRelation(name); s != "" {
		return fmt.Sprintf(`; did you mean %q?`, s)
	}
	return ""
}

// suggestRelation returns the catalog relation whose name is closest to `name` when it is within a
// plausible-typo distance, else "" — so a genuinely-unrelated token gets no misleading suggestion.
// The threshold scales with the name length (a longer name tolerates more slips) with a floor of 2.
func suggestRelation(name string) string {
	best, bestDist := "", 0
	for _, r := range Catalog() {
		d := levenshtein(name, r.Name)
		if best == "" || d < bestDist {
			best, bestDist = r.Name, d
		}
	}
	threshold := len(name)/4 + 1
	if threshold < 2 {
		threshold = 2
	}
	if best != "" && bestDist <= threshold {
		return best
	}
	return ""
}

// levenshtein is the edit distance (insert/delete/substitute) between two strings — the closeness
// measure suggestRelation ranks candidates by.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, min(cur[j-1]+1, prev[j-1]+cost))
		}
		prev = cur
	}
	return prev[len(rb)]
}
