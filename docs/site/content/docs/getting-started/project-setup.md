---
title: Project Setup
weight: 4
---

Cogo can run with built-in defaults in any directory, but a real project gets the most value from an `.agents/` directory.

## Scaffold with `cogo init`

From the root of your project:

```bash
cogo init
```

This creates:

```
.agents/
├── config.json      # model, provider, permission mode, path scope
├── .gitignore       # ignores sessions/ and logs/ inside .agents/
└── AGENTS.md        # project memory loaded into the system prompt
```

If `.agents/` already exists, `cogo init` refuses to overwrite. Pass `--force` to clobber.

### Interactive mode

```bash
cogo init --interactive
```

Walks a Bubble Tea wizard with three steps:

1. **Provider** — `gemini` (public API) or `vertex` (Vertex AI).
2. **Model** — pre-filled with the current default; edit if you want a different one.
3. **Permission mode** — `ask` (default), `allow`, or `yolo`.

A confirmation screen shows the resulting `config.json` before writing.

## What gets auto-discovered

Cogo walks up from the current working directory looking for `.agents/`, the same way Git finds `.git`. So you can `cd` into any subdirectory and Cogo still finds the project root's config.

When no `.agents/` exists, Cogo runs with built-in defaults — useful for quick one-offs in arbitrary directories.

## Optional pieces

You can layer these in over time:

- **`.agents/mcp.json`** — declare MCP servers to extend the agent's tool set. See [MCP Servers](../../configuration/mcp-servers/).
- **`.agents/skills/<name>/SKILL.md`** — drop in Claude-compatible skill bundles. See [Skills](../../configuration/skills/).
- **`.agents/AGENTS.md`** (or `CLAUDE.md` / `GEMINI.md`) — project memory loaded into the system prompt. Plus `~/.cogo/AGENTS.md` for personal preferences across all projects. See [Memory](../../configuration/memory/).
- **`.agents/sessions/<timestamp>.json`** — automatically written on exit.
- **`.agents/logs/<timestamp>.jsonl`** — written when `--debug` is passed.

## Next

→ [User Guide](../../user-guide/) — explore the interactive TUI and slash commands in depth.
