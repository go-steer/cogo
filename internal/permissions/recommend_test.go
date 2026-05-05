// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package permissions

import (
	"strings"
	"testing"
	"time"
)

func mkApprovals(rows ...[2]string) []ApprovalLog {
	out := make([]ApprovalLog, 0, len(rows))
	now := time.Now()
	for _, r := range rows {
		out = append(out, ApprovalLog{Tool: r[0], Key: r[1], Decision: DecisionAllowOnce, At: now})
	}
	return out
}

func patterns(recs []Recommendation) []string {
	out := make([]string, len(recs))
	for i, r := range recs {
		out[i] = r.Pattern
	}
	return out
}

// TestRecommend covers each classification branch the picker relies
// on. The recommendation list IS the user-facing surface of the
// /permissions feature, so wrong patterns here mean wrong allowlist
// entries get persisted to .agents/config.json.
func TestRecommend(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		input  []ApprovalLog
		expect []string
	}{
		{
			"empty log -> nothing",
			nil,
			nil,
		},
		{
			"single bash command -> exact pattern",
			mkApprovals([2]string{"bash", "git status"}),
			[]string{"bash:git status"},
		},
		{
			"multiple bash sharing a verb -> verb glob",
			mkApprovals(
				[2]string{"bash", "git status"},
				[2]string{"bash", "git log -p"},
				[2]string{"bash", "git diff HEAD"},
			),
			[]string{"bash:git *"},
		},
		{
			"multiple bash with no shared verb -> tool-wide",
			mkApprovals(
				[2]string{"bash", "ls -la"},
				[2]string{"bash", "pwd"},
				[2]string{"bash", "echo hi"},
			),
			[]string{"bash:*"},
		},
		{
			"file reads with shared dir prefix -> path glob",
			mkApprovals(
				[2]string{"read_file", "internal/tui/model.go"},
				[2]string{"read_file", "internal/tui/view.go"},
				[2]string{"read_file", "internal/tui/update.go"},
			),
			[]string{"read_file:internal/tui/**"},
		},
		{
			"file reads with no shared dir -> tool-wide",
			mkApprovals(
				[2]string{"read_file", "go.mod"},
				[2]string{"read_file", "README.md"},
				[2]string{"read_file", "Makefile"},
			),
			[]string{"read_file:*"},
		},
		{
			"two tools each get their own recommendation",
			mkApprovals(
				[2]string{"bash", "git status"},
				[2]string{"bash", "git log"},
				[2]string{"read_file", "go.mod"},
			),
			[]string{"bash:git *", "read_file:go.mod"},
		},
		{
			"duplicate keys collapse to one",
			mkApprovals(
				[2]string{"bash", "git status"},
				[2]string{"bash", "git status"},
				[2]string{"bash", "git status"},
			),
			[]string{"bash:git status"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := patterns(Recommend(tc.input))
			if !sliceEq(got, tc.expect) {
				t.Errorf("Recommend produced %v; want %v", got, tc.expect)
			}
		})
	}
}

// TestRecommend_FileToolPrefixDropsBasename guards a subtle case:
// when EVERY approved path is the same file (so the longest common
// prefix is the file itself), we must back off to the parent
// directory rather than emitting a useless "<file>/**" pattern.
func TestRecommend_FileToolPrefixDropsBasename(t *testing.T) {
	t.Parallel()
	// All paths share `internal/tui/model.go` exactly; the prefix
	// algorithm should fall back to the parent directory.
	got := patterns(Recommend(mkApprovals(
		[2]string{"read_file", "internal/tui/model.go"},
		[2]string{"read_file", "internal/tui/model.go"},
	)))
	for _, p := range got {
		if strings.HasSuffix(p, ".go/**") {
			t.Errorf("recommendation %q glob-pasted onto a filename, not a directory", p)
		}
	}
}

// TestSortRecommendations pins the picker ordering: specific patterns
// (no `*`) above wildcard patterns. Otherwise the safer recommendation
// hides below the broad one and a hurried user might pick the wrong one.
func TestSortRecommendations(t *testing.T) {
	t.Parallel()
	recs := []Recommendation{
		{Pattern: "bash:*"},
		{Pattern: "read_file:go.mod"},
		{Pattern: "bash:git *"},
		{Pattern: "read_file:internal/tui/**"},
	}
	SortRecommendations(recs)
	got := patterns(recs)
	want := []string{"read_file:go.mod", "bash:*", "bash:git *", "read_file:internal/tui/**"}
	if !sliceEq(got, want) {
		t.Errorf("SortRecommendations -> %v; want %v", got, want)
	}
}

func sliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
