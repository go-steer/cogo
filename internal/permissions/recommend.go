// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package permissions

import (
	"path/filepath"
	"sort"
	"strings"
)

// Recommendation describes one suggested allowlist entry the user
// could persist into .agents/config.json's `permissions.allow` block.
// Pattern is in the existing tool:glob form — see internal/permissions
// policy.go for the grammar — so a recommendation can be appended
// verbatim. Reason and Evidence let the picker explain WHY before the
// user agrees to broaden their permanent allowlist.
type Recommendation struct {
	Pattern  string   // e.g. "bash:git *", "read_file:internal/tui/**"
	Reason   string   // one-line human explanation of what this covers
	Evidence []string // sample keys that motivated the recommendation
}

// Recommend turns a session's interactive approval log into a small
// list of suggested permanent allowlist entries. Heuristics, not
// magic — the rules below were chosen to surface the patterns that
// dogfood actually produces (lots of `git`, lots of reads under the
// project tree, the same `ls -la` over and over). Anything that
// can't be classified is skipped; an empty result is normal for a
// session that did one-off things.
//
// The classifier is deterministic so the picker can show stable
// recommendations across rerenders.
func Recommend(approvals []ApprovalLog) []Recommendation {
	if len(approvals) == 0 {
		return nil
	}
	// Group keys by tool, deduping while preserving first-seen order
	// so the evidence list reads chronologically.
	byTool := map[string][]string{}
	seen := map[string]bool{}
	tools := []string{}
	for _, a := range approvals {
		if !seen[a.Tool] {
			tools = append(tools, a.Tool)
		}
		k := a.Tool + "|" + a.Key
		if seen[k] {
			continue
		}
		seen[k] = true
		seen[a.Tool] = true
		byTool[a.Tool] = append(byTool[a.Tool], a.Key)
	}

	var out []Recommendation
	for _, tool := range tools {
		out = append(out, classify(tool, byTool[tool])...)
	}
	return out
}

// classify produces 0-2 recommendations for one tool's approved keys.
func classify(tool string, keys []string) []Recommendation {
	if len(keys) == 0 {
		return nil
	}

	// Single key approved at least once -> recommend the exact pattern
	// only if it was approved 3+ times (signal that it's frequent).
	// Approval count is reflected in len(keys) only when keys are
	// distinct; for repeats we'd see one entry. So this branch covers
	// "the user approved the same one-off twice" which is rarely
	// worth persisting; we let it fall through.
	if len(keys) == 1 {
		// Single distinct key. Recommend the exact pattern verbatim;
		// the picker will show the count of approvals separately if
		// needed. Tool author can decide if this is worth pinning.
		return []Recommendation{{
			Pattern:  tool + ":" + keys[0],
			Reason:   "approved once this session — pin if you'll keep doing it",
			Evidence: keys,
		}}
	}

	// Bash-specific: detect a shared leading verb (git, npm, go, etc.)
	// because that's the highest-signal pattern in dogfood.
	if tool == "bash" {
		if verb, all := bashCommonVerb(keys); verb != "" && all {
			return []Recommendation{
				{
					Pattern:  "bash:" + verb + " *",
					Reason:   "all " + plural(len(keys), "command") + " start with `" + verb + "` — persist a verb-wide allow",
					Evidence: keys,
				},
			}
		}
	}

	// File-tool path-prefix detection: if all keys share a common
	// directory prefix, recommend that directory glob.
	if isFileTool(tool) {
		if pref := commonDirPrefix(keys); pref != "" && pref != "." && pref != "/" {
			return []Recommendation{{
				Pattern:  tool + ":" + pref + "/**",
				Reason:   plural(len(keys), "path") + " under `" + pref + "/` approved",
				Evidence: keys,
			}}
		}
	}

	// Final fallback: tool-wide. Broader than the user might want,
	// but we always show it as a separate recommendation so they can
	// opt out by leaving it unchecked.
	return []Recommendation{{
		Pattern:  tool + ":*",
		Reason:   plural(len(keys), "distinct call") + " — broaden to all " + tool + " calls",
		Evidence: keys,
	}}
}

// bashCommonVerb returns the leading whitespace-separated token if
// every key in keys starts with that same token (e.g. "git status",
// "git log -p" → "git"). Returns ("", false) if there's no match.
func bashCommonVerb(keys []string) (string, bool) {
	if len(keys) == 0 {
		return "", false
	}
	first := strings.Fields(keys[0])
	if len(first) == 0 {
		return "", false
	}
	verb := first[0]
	for _, k := range keys[1:] {
		toks := strings.Fields(k)
		if len(toks) == 0 || toks[0] != verb {
			return "", false
		}
	}
	return verb, true
}

// isFileTool reports whether the given tool's keys are filesystem
// paths (vs. command strings). Conservative — only the cogo built-ins
// that we know take a path argument.
func isFileTool(tool string) bool {
	switch tool {
	case "read_file", "write_file", "edit_file", "list_dir":
		return true
	}
	return false
}

// commonDirPrefix returns the longest leading directory shared by every
// path in paths. Returns "" if there's no useful (non-root) prefix.
func commonDirPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	// Split each path into its directory components and find the
	// longest shared prefix.
	cleaned := make([][]string, len(paths))
	for i, p := range paths {
		cleaned[i] = strings.Split(filepath.ToSlash(filepath.Clean(p)), "/")
	}
	prefix := append([]string(nil), cleaned[0]...)
	for _, parts := range cleaned[1:] {
		// Trim prefix to the matching head with parts.
		max := len(prefix)
		if len(parts) < max {
			max = len(parts)
		}
		i := 0
		for i < max && prefix[i] == parts[i] {
			i++
		}
		prefix = prefix[:i]
		if len(prefix) == 0 {
			return ""
		}
	}
	// We want the directory prefix, not a complete-match path. If the
	// full path matched, drop the basename so we recommend the parent.
	for _, parts := range cleaned {
		if len(parts) == len(prefix) {
			prefix = prefix[:len(prefix)-1]
			break
		}
	}
	if len(prefix) == 0 {
		return ""
	}
	return strings.Join(prefix, "/")
}

// plural returns "<n> <word>" or "<n> <word>s" depending on n. No
// fancy English handling beyond the trailing s — the recommendations
// only use simple nouns.
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return itoa(n) + " " + word + "s"
}

// itoa avoids pulling in strconv just for one call site.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// SortRecommendations returns a stable ordering: more specific
// patterns (without `*`) first, then by Pattern lex order. The
// picker shows them in this order so the safer recommendations
// surface above the broad tool-wide ones.
func SortRecommendations(recs []Recommendation) {
	sort.SliceStable(recs, func(i, j int) bool {
		ai := strings.Contains(recs[i].Pattern, "*")
		aj := strings.Contains(recs[j].Pattern, "*")
		if ai != aj {
			return !ai // non-wildcard first
		}
		return recs[i].Pattern < recs[j].Pattern
	})
}
