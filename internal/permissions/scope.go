package permissions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathScope implements REQUIREMENTS FR-5.5: file tools may only touch
// paths inside the project root, ~/.cogo/, or any explicit pattern in
// path_scope.allow. Out-of-scope access escalates to a prompt (in the
// TUI) or fails immediately (in headless).
//
// Patterns supported in the allow list:
//   - Exact absolute paths.
//   - Directory trees ending with `/...` (e.g. "/etc/myapp/...").
//   - Standard filepath glob patterns (passed through path/filepath.Match).
type PathScope struct {
	roots []string // absolute, cleaned
	allow []string // absolute, cleaned; "/..." subtrees expanded as prefixes
}

// NewPathScope constructs a scope from the project root, the user-global
// home (typically ~/.cogo/), and an extra allowlist of patterns.
//
// projectRoot may be empty, in which case only the user root and
// allowlist apply. userRoot may be empty for tests.
func NewPathScope(projectRoot, userRoot string, allow []string) (*PathScope, error) {
	s := &PathScope{}
	for _, r := range []string{projectRoot, userRoot} {
		if r == "" {
			continue
		}
		abs, err := filepath.Abs(r)
		if err != nil {
			return nil, fmt.Errorf("path scope: %w", err)
		}
		s.roots = append(s.roots, filepath.Clean(abs))
	}
	for _, p := range allow {
		s.allow = append(s.allow, expandUser(p))
	}
	return s, nil
}

// Roots returns a copy of the configured scope roots.
func (s *PathScope) Roots() []string {
	out := make([]string, len(s.roots))
	copy(out, s.roots)
	return out
}

// AllowList returns a copy of the configured allowlist patterns.
func (s *PathScope) AllowList() []string {
	out := make([]string, len(s.allow))
	copy(out, s.allow)
	return out
}

// Contains reports whether path is in scope. The path is resolved to
// an absolute, cleaned form before comparison; symlinks are not
// followed (we trust the input).
func (s *PathScope) Contains(path string) (bool, error) {
	abs, err := filepath.Abs(expandUser(path))
	if err != nil {
		return false, fmt.Errorf("path scope: %w", err)
	}
	abs = filepath.Clean(abs)

	for _, root := range s.roots {
		if isInside(abs, root) {
			return true, nil
		}
	}
	for _, pat := range s.allow {
		if matchesPattern(abs, pat) {
			return true, nil
		}
	}
	return false, nil
}

// AddAlwaysAllow appends a pattern to the in-memory allowlist. The
// caller is responsible for persisting the change to config.json (see
// internal/permissions/persist.go).
func (s *PathScope) AddAlwaysAllow(pattern string) {
	s.allow = append(s.allow, expandUser(pattern))
}

// expandUser turns a leading "~" into the user's home directory. We
// match shell behavior: only "~" or "~/" at the start expands; "~user"
// is left alone (rare and ambiguous).
func expandUser(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
		}
	}
	return p
}

// isInside reports whether path is the same as or nested under root.
// Both arguments must be absolute and cleaned.
func isInside(path, root string) bool {
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != ".."
}

// matchesPattern checks an absolute, cleaned path against an allowlist
// pattern. Patterns ending with "/..." treat everything beneath as
// in-scope; otherwise filepath.Match is used.
func matchesPattern(path, pattern string) bool {
	if strings.HasSuffix(pattern, "/...") {
		root := strings.TrimSuffix(pattern, "/...")
		return path == root || isInside(path, root)
	}
	if ok, _ := filepath.Match(pattern, path); ok {
		return true
	}
	return path == pattern
}
