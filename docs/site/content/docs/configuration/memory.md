---
title: Project Memory
weight: 5
---

Project memory is a Markdown file that gets prepended to the agent's system prompt at startup. It's how you teach the agent about your project's conventions, gotchas, and house style without re-explaining them every turn.

## Fallback chain

Cogo looks for the first file that exists, in this order:

1. **`.agents/AGENTS.md`** — preferred name, lives next to other Cogo config.
2. **`AGENTS.md`** at the project root — for projects already using the [AGENTS.md convention](https://agents.md).
3. **`CLAUDE.md`** at the project root — for projects already using Claude Code.
4. **`GEMINI.md`** at the project root — for projects already using Gemini CLI tools.

Plus, **always** loaded if present:

5. **`~/.cogo/AGENTS.md`** — your personal preferences across every Cogo project.

The user-global file is concatenated after the project file (project-specific instructions outrank personal style).

## Inspect what loaded

`/memory` shows the resolved chain:

```
Memory loaded:
  project: /Users/me/work/api/.agents/AGENTS.md — 4 KiB
  user:    /Users/me/.cogo/AGENTS.md — 1 KiB
```

Truncated files are flagged with `(truncated)` so you know the model isn't seeing the full content.

## What to put in it

Cogo's own `AGENTS.md` (loaded when running inside this repo) is a useful template. Common sections:

- **Project structure** — high-level layout, where things live.
- **Build / test commands** — `dev/tools/ci`, `go test ./...`, etc.
- **Conventions** — commit style, branch model, formatting rules, comment policy.
- **Pitfalls** — non-obvious things that bite first-time contributors.
- **External resources** — links to internal dashboards, runbooks, design docs.

Keep it under ~10 KB. Larger memory files are loaded but cost a meaningful chunk of context window every turn.

## Truncation

Memory files larger than 10 KiB are truncated; the truncation point is logged in `/memory` output. To work around it, split into multiple files (`.agents/AGENTS.md` + a separate doc the agent fetches with `read_file` on demand) or trim the less-relevant sections.

## When does memory reload?

Memory is read at process start and again on `/reload`. Edit `AGENTS.md`, run `/reload`, and the next turn picks up the new content.

## Why three filename fallbacks?

Many teams already have a `CLAUDE.md` or `GEMINI.md` for their existing tooling. Honoring those means Cogo Just Works in mixed-tool repos without forcing rename churn. New projects should prefer `AGENTS.md` — it's the cross-tool convention and signals the file is for any agentic CLI, not one specific vendor.
