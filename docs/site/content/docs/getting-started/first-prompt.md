---
title: First Prompt
weight: 3
---

Cogo runs in two modes — pick the one that matches what you're doing.

## One-shot (headless)

Single turn, streamed to stdout, exit. Designed for shell pipelines and CI:

```bash
cogo -p "What does this directory do? Read AGENTS.md if present."
```

- Tool calls are echoed to stderr (one line per call) so the stdout stays clean for piping.
- A one-line cost summary lands on stderr at exit: `cogo: 1 turn(s) · ↑1234 ↓456 tokens · $0.0021 (gemini-3.1-pro-preview)`.

Pipe it to anything that takes stdin:

```bash
cogo -p "List the test files." | xargs ls -la
```

## Interactive (TUI)

No `-p` and on a TTY → opens an interactive Bubble Tea chat:

```bash
cogo
```

Inside the chat:

- **Type** to compose. **Enter** sends. **Shift+Enter** for newlines.
- **`/`** at the start of an empty prompt opens the slash-command palette.
- **`@`** anywhere opens a file picker — selecting inserts `@<path>` and the file contents are inlined when you submit.
- **Ctrl+C** while streaming cancels the current turn. A second Ctrl+C while idle exits.
- **Ctrl+L** clears the viewport (history is preserved).
- **Up arrow** on an empty input recalls previous prompts (shell-style history).

The header shows the current model and a running session total (`σ ↑in ↓out · $cost`); each completed assistant turn carries a per-turn footer below it.

## Slash commands at a glance

| Command   | Effect                                                     |
|-----------|------------------------------------------------------------|
| `/help`   | Full keymap + command list.                                |
| `/memory` | Show which memory files were loaded.                       |
| `/stats`  | Per-session token + cost breakdown.                        |
| `/model`  | Open the model picker (or `/model <id>` to switch).        |
| `/mcp`    | List MCP servers + their tools.                            |
| `/skills` | List discovered skills.                                    |
| `/reload` | Re-read `.agents/` from disk and rebuild in place.         |
| `/clear`  | Clear chat history (with confirmation).                    |
| `/quit`   | Exit, persisting a session transcript.                     |

See the full [Slash Commands](../../user-guide/slash-commands/) reference for details.

## Next

→ [Project Setup](../project-setup/) — scaffold an `.agents/` directory for a real project.
