package profiles

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/core/query"
)

// This file is the ONE place a net name is matched against a profile signal. There are two callers'
// worlds — the generated datalog (netMatch, compiled into every requirement's rules) and plain Go
// (netMatchesSignal, behind InUse / Nets / Coverage) — and they must agree, or a rule that will not
// fire gets scored as a clean pass (the WS3-090 false-pass shape) or a panel binds a net no finding
// ever names. Both live here, side by side, and the glob/regex forms share one compiler with the
// query predicates (query.CompileGlob / query.CompilePattern) rather than twinning the translation.
//
// A signal declares exactly one matcher FORM (WS3-057):
//
//	affix  prefix and/or suffix, conjunctive. The readable default: the role is the tail of the net
//	       name, optionally discriminated by a bus prefix (PCIE_ + _TXP).
//	glob   whole-name shell-style glob (ETH_SW*_A_H). For naming where the identity is the prefix and
//	       the suffix is shared with a foreign bus, which affix matching cannot tell apart.
//	regex  an unanchored RE2 escape hatch, for multi-instance naming a glob cannot express
//	       (^ETH_SW\d+_P\d+_.*_H$).
//
// validateSignalMatcher is the load-time gate; Parse runs it for YAML profiles and Compile runs it
// for Go-literal ones, so an unsound matcher can never reach rule generation.

// netMatch is the datalog literal(s) binding net-var v to signal s's naming convention. Every
// generated rule that selects a signal's net goes through here — presence, the completeness anchor,
// host-present, pull-up, and dangling — so a discriminating matcher is applied uniformly and no rule
// can latch onto a foreign net that merely shares a suffix.
func netMatch(v query.Term, s Signal) []query.Literal {
	switch {
	case s.Glob != "":
		return []query.Literal{query.Pos(query.Rel("glob", v, query.Str(s.Glob)))}
	case s.Regex != "":
		return []query.Literal{query.Pos(query.Rel("match", v, query.Str(s.Regex)))}
	}
	var lits []query.Literal
	if s.Suffix != "" {
		lits = append(lits, query.Pos(query.Rel("suffix", v, query.Str(s.Suffix))))
	}
	if s.Prefix != "" {
		lits = append(lits, query.Pos(query.Rel("prefix", v, query.Str(s.Prefix))))
	}
	return lits
}

// netMatchesSignal is the Go twin of netMatch: a net satisfies a signal under the signal's declared
// form. A signal with NO matcher matches nothing — the safe direction for the presence gates, which
// take a Profile directly and so are not covered by Compile's validation.
func netMatchesSignal(name string, s Signal) bool {
	switch {
	case s.Glob != "":
		re, err := query.CompileGlob(s.Glob)
		return err == nil && re.MatchString(name)
	case s.Regex != "":
		re, err := query.CompilePattern(s.Regex)
		return err == nil && re.MatchString(name)
	}
	if s.Suffix == "" && s.Prefix == "" {
		return false
	}
	if s.Suffix != "" && !strings.HasSuffix(name, s.Suffix) {
		return false
	}
	if s.Prefix != "" && !strings.HasPrefix(name, s.Prefix) {
		return false
	}
	return true
}

// matchesAnySignal reports whether name satisfies ANY of p's signals — the profile-level convention
// match behind the review scope fallback.
func matchesAnySignal(p Profile, name string) bool {
	for _, s := range p.Signals {
		if netMatchesSignal(name, s) {
			return true
		}
	}
	return false
}

// validateSignalMatcher rejects a signal whose matcher cannot discriminate: no form declared, more
// than one form declared, a pattern that does not compile, or a pattern that matches the EMPTY net
// name. The last is the over-broad guard: `*`, `.*`, and an alternation with an empty branch all
// match every net on the design, so a completeness check built on one anchors anywhere and reports
// noise. It is a static rule on purpose — "matches an implausible COUNT of nets" needs the design,
// which neither Parse nor Compile has.
func validateSignalMatcher(s Signal) error {
	forms := 0
	if s.Prefix != "" || s.Suffix != "" {
		forms++
	}
	if s.Glob != "" {
		forms++
	}
	if s.Regex != "" {
		forms++
	}
	switch {
	case forms == 0:
		return fmt.Errorf("signal %q declares no matcher: set one of suffix (optionally with prefix), glob, or regex", s.Name)
	case forms > 1:
		return fmt.Errorf("signal %q declares %d matcher forms: set exactly one of suffix/prefix, glob, or regex", s.Name, forms)
	}
	switch {
	case s.Glob != "":
		re, err := query.CompileGlob(s.Glob)
		if err != nil {
			return fmt.Errorf("signal %q: %w", s.Name, err)
		}
		if re.MatchString("") {
			return overBroad(s.Name, "glob", s.Glob)
		}
	case s.Regex != "":
		re, err := query.CompilePattern(s.Regex)
		if err != nil {
			return fmt.Errorf("signal %q: %w", s.Name, err)
		}
		if re.MatchString("") {
			return overBroad(s.Name, "regex", s.Regex)
		}
	}
	return nil
}

func overBroad(signal, form, pattern string) error {
	return fmt.Errorf("signal %q: %s %q matches every net name (it matches the empty string); narrow it so it can discriminate this interface's nets",
		signal, form, pattern)
}
