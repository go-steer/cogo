package tui

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// fileEntryLimit caps how many files we surface in the @-palette so a
// huge repo doesn't make discovery sluggish.
const fileEntryLimit = 200

// excludedDirs are skipped during the file walk. Heavy build / cache /
// VCS dirs that are almost never the right thing to reference.
var excludedDirs = map[string]bool{
	".git":            true,
	"node_modules":    true,
	"vendor":          true,
	"dist":            true,
	"build":           true,
	".next":           true,
	".cache":          true,
	"target":          true,
	".venv":           true,
	"__pycache__":     true,
	".idea":           true,
	".vscode":         true,
	".terraform":      true,
}

// listProjectFiles walks root and returns up to fileEntryLimit files
// (not directories) whose path matches filter (case-insensitive
// substring). Excluded dirs are pruned. The returned paths are
// relative to root and use forward slashes for cross-platform
// consistency.
func listProjectFiles(root, filter string) []paletteItem {
	if root == "" {
		root = "."
	}
	low := strings.ToLower(filter)
	var matches []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if excludedDirs[name] {
				return fs.SkipDir
			}
			// Skip the .agents/sessions and .agents/logs subtrees; the
			// .agents dir itself stays visible because its config and
			// memory files are useful references.
			if rel, _ := filepath.Rel(root, path); rel == ".agents/sessions" || rel == ".agents/logs" {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if filter != "" && !strings.Contains(strings.ToLower(rel), low) {
			return nil
		}
		matches = append(matches, rel)
		if len(matches) >= fileEntryLimit*4 {
			// over-collect so the rank+truncate below picks the best.
			return fs.SkipDir
		}
		return nil
	})

	// Rank: prefix match on the basename first, then substring.
	type ranked struct {
		path string
		rank int
	}
	var rs []ranked
	for _, p := range matches {
		base := strings.ToLower(filepath.Base(p))
		switch {
		case low == "" || strings.HasPrefix(base, low):
			rs = append(rs, ranked{p, 0})
		case strings.Contains(strings.ToLower(p), low):
			rs = append(rs, ranked{p, 1})
		}
	}
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].rank != rs[j].rank {
			return rs[i].rank < rs[j].rank
		}
		return rs[i].path < rs[j].path
	})
	if len(rs) > fileEntryLimit {
		rs = rs[:fileEntryLimit]
	}

	out := make([]paletteItem, 0, len(rs))
	for _, r := range rs {
		out = append(out, paletteItem{
			Display: r.path,
			Value:   "@" + r.path,
		})
	}
	return out
}

// expandAtRefs scans prompt for `@<path>` tokens (where the token is
// preceded by whitespace or at start of string), reads each referenced
// file via fileReader (typically os.ReadFile), and returns the
// original prompt followed by an appended "Referenced files" section.
//
// Files that fail to read are noted in the returned diagnostics slice
// so the caller can show them to the user; the expansion still
// succeeds so the prompt round-trips even when some refs are bad.
func expandAtRefs(prompt string, fileReader func(string) ([]byte, error)) (expanded string, refs []string, diagnostics []string) {
	tokens := tokenizeAtRefs(prompt)
	if len(tokens) == 0 {
		return prompt, nil, nil
	}
	var b strings.Builder
	b.WriteString(prompt)
	b.WriteString("\n\nReferenced files:\n")
	seen := map[string]bool{}
	for _, t := range tokens {
		if seen[t] {
			continue
		}
		seen[t] = true
		data, err := fileReader(t)
		if err != nil {
			diagnostics = append(diagnostics, "could not read "+t+": "+err.Error())
			continue
		}
		refs = append(refs, t)
		b.WriteString("\n--- ")
		b.WriteString(t)
		b.WriteString(" ---\n")
		b.Write(data)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	return b.String(), refs, diagnostics
}

// tokenizeAtRefs returns the file references found in s. A reference
// is an `@<path>` token where `<path>` is a non-empty run of
// non-whitespace characters and the `@` is at the start of the string
// or preceded by whitespace.
func tokenizeAtRefs(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		c := s[i]
		if c != '@' || (i > 0 && !isWhitespaceByte(s[i-1])) {
			i++
			continue
		}
		j := i + 1
		for j < len(s) && !isWhitespaceByte(s[j]) {
			j++
		}
		path := s[i+1 : j]
		if path != "" {
			out = append(out, path)
		}
		i = j
	}
	return out
}

func isWhitespaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// readFileSafe is the default reader for expandAtRefs in the TUI:
// reads via os.ReadFile and tail-truncates if the file exceeds
// maxBytes (so a 10 MB log doesn't blow up the prompt).
func readFileSafe(maxBytes int) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if maxBytes > 0 && len(data) > maxBytes {
			data = data[:maxBytes]
			data = append(data, []byte("\n... [truncated]\n")...)
		}
		return data, nil
	}
}
