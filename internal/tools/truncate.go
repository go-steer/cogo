// Package tools implements Cogo's built-in tool suite for the agent
// loop: file I/O, shell, todo tracking, and (later) web access.
//
// All tools share a common shape via google.golang.org/adk/tool/functiontool
// and a few cross-cutting helpers in this package: output truncation
// (per REQUIREMENTS FR-14) and gate consultation (via internal/permissions).
//
// Tool authors define a typed Args + Result pair and a handler closing
// over any dependencies (the gate, the config, etc.); register.go
// assembles them into the slice consumed by internal/agent.
package tools

import (
	"fmt"
	"strings"
)

// Truncate caps s at the lower of maxBytes / maxLines. When truncation
// occurs, a marker line is appended so the model knows it received an
// abridged output and can ask for more (e.g. via offset/limit on a
// re-read or a narrower grep).
//
// maxBytes <= 0 means "no byte limit"; same for maxLines.
func Truncate(s string, maxBytes, maxLines int) string {
	if s == "" {
		return s
	}
	truncated := false
	originalBytes := len(s)
	originalLines := -1

	if maxBytes > 0 && len(s) > maxBytes {
		s = s[:maxBytes]
		truncated = true
	}
	if maxLines > 0 {
		lines := strings.SplitAfter(s, "\n")
		if len(lines) > maxLines {
			originalLines = countLines(s) // lower bound; we already byte-truncated
			s = strings.Join(lines[:maxLines], "")
			truncated = true
		}
	}
	if !truncated {
		return s
	}
	marker := fmt.Sprintf("\n... [truncated by cogo: original size %d bytes", originalBytes)
	if originalLines > 0 {
		marker += fmt.Sprintf(", %d lines", originalLines)
	}
	marker += "; ask with a narrower scope or read in chunks]"
	return s + marker
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
