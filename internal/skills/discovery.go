// Package skills loads Claude-compatible SKILL.md bundles from
// .agents/skills/<name>/ and exposes them as an ADK Toolset the agent
// can invoke.
//
// The schema mirrors Claude Code's SKILL.md frontmatter so users can
// drop existing skill bundles directly into a Cogo project (resolved
// decision in REQUIREMENTS FR-7.1).
//
// Bodies load lazily on invocation — we keep cold-start fast by
// skipping skill.WithCompletePreloadSource.
package skills

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/skilltoolset"
	"google.golang.org/adk/tool/skilltoolset/skill"
)

// SkillDirName is the project-local directory holding skill bundles.
const SkillDirName = "skills"

// Info is the per-skill metadata surfaced via /skills.
type Info struct {
	Name        string
	Description string
}

// Skills bundles the discovered skills' toolset (for agent.WithToolsets)
// alongside the metadata list (for /skills display).
type Skills struct {
	Toolset adktool.Toolset
	Infos   []Info
}

// Empty reports whether no skills were discovered.
func (s Skills) Empty() bool { return s.Toolset == nil }

// Load discovers skills under agentsDir/skills/. A missing directory
// (or empty agentsDir) yields a zero Skills with no error — most
// projects don't use skills.
func Load(ctx context.Context, agentsDir string) (Skills, error) {
	if agentsDir == "" {
		return Skills{}, nil
	}
	skillsDir := filepath.Join(agentsDir, SkillDirName)
	info, err := os.Stat(skillsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Skills{}, nil
		}
		return Skills{}, fmt.Errorf("skills: stat %s: %w", skillsDir, err)
	}
	if !info.IsDir() {
		return Skills{}, fmt.Errorf("skills: %s is not a directory", skillsDir)
	}

	source := skill.NewFileSystemSource(os.DirFS(skillsDir))
	frontmatters, err := source.ListFrontmatters(ctx)
	if err != nil {
		return Skills{}, fmt.Errorf("skills: list: %w", err)
	}
	if len(frontmatters) == 0 {
		// Directory exists but holds no SKILL.md bundles.
		return Skills{}, nil
	}

	ts, err := skilltoolset.New(ctx, skilltoolset.Config{Source: source})
	if err != nil {
		return Skills{}, fmt.Errorf("skills: build toolset: %w", err)
	}

	infos := make([]Info, 0, len(frontmatters))
	for _, fm := range frontmatters {
		infos = append(infos, Info{Name: fm.Name, Description: fm.Description})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })

	return Skills{Toolset: ts, Infos: infos}, nil
}
