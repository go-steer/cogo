---
title: Slash Commands
weight: 2
---

All slash commands work both ways: type `/<name>` and submit, or type `/` to open the palette and pick from the list.

## Reference

| Command             | What it does                                                                          |
|---------------------|---------------------------------------------------------------------------------------|
| `/help`             | Show the full keymap and command list.                                                |
| `/memory`           | List which memory files (`AGENTS.md`, `CLAUDE.md`, `GEMINI.md`) loaded and from where.|
| `/stats`            | Per-session token + cost breakdown: turns, input/output tokens, cost, duration, model.|
| `/model`            | Open the model picker. Append an ID (e.g. `/model gemini-3.5-flash-preview`) to switch directly. |
| `/mcp`              | Show configured MCP servers, their status, and the tool names they expose.            |
| `/skills`           | List discovered skills under `.agents/skills/` with their descriptions.               |
| `/reload`           | Re-read the entire `.agents/` directory and rebuild the agent in place. Chat history and usage totals are preserved. |
| `/clear`            | Clear chat history (asks for `y` / `yes` confirmation).                               |
| `/quit`             | Exit cleanly. A session transcript is written to `.agents/sessions/`.                 |

## The slash palette

Type `/` at the start of an empty prompt to open a filterable palette of every command:

```
  /help    Show the help text
▸ /memory  Inspect loaded project memory
  /stats   Per-session token + cost summary
  /model   Open the model picker
  /mcp     Show MCP server status
  /skills  Show discovered skills
  /reload  Re-read .agents/ from disk
  /clear   Clear the chat history
  /quit    Exit
```

- ↑/↓ navigate.
- **Enter** runs the highlighted command.
- **Tab** fills the input with `<command> ` (trailing space) so you can add args without submitting yet.
- **Esc** dismisses the palette.

## `/reload` in detail

`/reload` re-reads the entire `.agents/` directory:

- New / changed `mcp.json` — old MCP servers are torn down (stdio children get SIGTERM → 3s grace → SIGKILL), new ones are spawned.
- New / changed `skills/` — the skill toolset is rebuilt.
- New / changed `AGENTS.md` (or fallbacks) — memory is reloaded into the system prompt.
- New / changed `config.json` — picked up for the next turn.

Chat history and the running session-cost totals stay intact. The reload happens atomically — partial failure rolls back to the previous generation rather than leaving you in a half-rebuilt state.

## `/quit` and Ctrl+C

Both exit cleanly:

- A session transcript is written to `.agents/sessions/<rfc3339>.json` (when a project root exists).
- Stdio MCP children are reaped before exit.
- The OpenTelemetry shutdown hook flushes any pending spans.
