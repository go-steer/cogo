---
title: ".agents/ Directory"
weight: 1
---

Cogo discovers `.agents/` by walking up from the current working directory, the same way Git finds `.git`. The first directory matched becomes the project root.

## Full layout

```
.agents/
├── config.json          # core config: model, permissions, path scope, OTEL
├── mcp.json             # MCP server definitions (stdio + Streamable HTTP)
├── skills/              # one subdirectory per skill
│   ├── refactor/
│   │   └── SKILL.md
│   └── git-flow/
│       └── SKILL.md
├── AGENTS.md            # project memory (preferred name)
├── CLAUDE.md            #   …or fall back to this
├── GEMINI.md            #   …or this
├── sessions/            # transcripts (auto-written on /quit + Ctrl+C)
│   └── 2026-05-02T19-00-00Z.json
├── logs/                # JSONL debug logs (only when --debug is set)
│   └── 2026-05-02T19-00-00Z.jsonl
└── .gitignore           # ignores sessions/ + logs/
```

Every file is optional. Missing files are treated as "use built-in defaults", not "fail to start".

## What's loaded when

| File / dir       | When                  | Reload trigger                              |
|------------------|-----------------------|---------------------------------------------|
| `config.json`    | Process start, `/reload` | Edit + `/reload` (or restart).            |
| `mcp.json`       | Process start, `/reload` | Edit + `/reload`. Stdio children are reaped before respawn. |
| `skills/*`       | Process start, `/reload` | Edit + `/reload`. SKILL.md body is loaded lazily on invoke. |
| `AGENTS.md` etc. | Process start, `/reload` | Edit + `/reload`.                         |
| `sessions/`      | Written on exit       | n/a (output only).                          |
| `logs/`          | Written during `--debug` runs | n/a (output only).                  |

## User-global counterpart

In addition to the per-project `.agents/`, Cogo reads `~/.cogo/`:

```
~/.cogo/
├── AGENTS.md           # personal preferences across all projects
└── (future: skills/, mcp.json)
```

Currently only `~/.cogo/AGENTS.md` is consumed; user-global skills + MCP are tracked as future work.

## Running without `.agents/`

`cogo` works fine in any directory. Without an `.agents/`:

- Built-in defaults for everything in `config.json`.
- No MCP servers, no skills.
- No project memory (only `~/.cogo/AGENTS.md` if present).
- `/reload` is a no-op (nothing to reload from).
- `/quit` exits without writing a transcript (no `.agents/sessions/` to write to).

Useful for one-off `cogo -p` invocations from arbitrary shells.

## Bootstrap

`cogo init` writes a minimal `.agents/{config.json, .gitignore, AGENTS.md}` skeleton. Pass `--interactive` to walk a wizard. See [Project Setup](../../getting-started/project-setup/).
