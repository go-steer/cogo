// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0

package permissions

import "testing"

func TestPolicy_Match(t *testing.T) {
	t.Parallel()
	p, err := NewPolicy(
		[]string{"bash:git status", "bash:git diff*", "bash:ls *"},
		[]string{"bash:rm -rf*", "bash:sudo *"},
	)
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}

	cases := []struct {
		name string
		tool string
		key  string
		want Outcome
	}{
		{"exact allow", "bash", "git status", OutcomeAllow},
		{"prefix allow", "bash", "git diff main..HEAD", OutcomeAllow},
		{"unrelated bash", "bash", "git push", OutcomeUnmatched},
		{"deny wins over allow", "bash", "rm -rf /tmp/x", OutcomeDeny},
		{"sudo deny", "bash", "sudo apt-get update", OutcomeDeny},
		{"different tool not matched", "read_file", "git status", OutcomeUnmatched},
		{"plain ls glob", "bash", "ls -la /tmp", OutcomeAllow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := p.Match(tc.tool, tc.key)
			if got != tc.want {
				t.Errorf("Match(%q,%q) = %v, want %v", tc.tool, tc.key, got, tc.want)
			}
		})
	}
}

func TestPolicy_AnyToolPattern(t *testing.T) {
	t.Parallel()
	p, _ := NewPolicy([]string{"*foo*"}, nil)
	// Bare patterns use filepath.Match semantics, so they're best for
	// non-path keys (commands). Slash-containing keys typically use the
	// tool: prefix form.
	if p.Match("bash", "echo foobar") != OutcomeAllow {
		t.Errorf("any-tool wildcard did not match bash command")
	}
}
