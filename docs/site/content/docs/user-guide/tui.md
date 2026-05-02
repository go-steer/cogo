---
title: Interactive TUI
weight: 1
---

The interactive mode (`cogo` on a TTY) is a Bubble Tea program with streaming responses, syntax-highlighted markdown, and several overlays.

## Layout

```
┌─ Cogo · gemini-3.1-pro-preview      ~/projects/cogo · vertex · ask · σ ↑1234/↓456/$0.0021 ─┐
│                                                                                            │
│  ❯ Summarize the agent loop in three bullets.                                              │
│                                                                                            │
│  Cogo's agent loop:                                                                        │
│  • Sends the user's prompt + project memory to Gemini …                                    │
│  • Streams partial events back to the TUI as they arrive.                                  │
│  • Tool calls are gated by the permission system before exec.                              │
│  ↑1234 in · ↓456 out · $0.0021                                                             │
│                                                                                            │
└─ Type a message, or /help…                                                                 ┘
  /help · /quit · Ctrl+C to exit
```

- **Header** — model name, working directory, provider, permission mode badge, running session totals.
- **History viewport** — scrollable; every message is rendered with a per-role style.
- **Input area** — multi-line textarea with rounded border.
- **Footer** — context-sensitive hints.

## Keymap

| Key                    | Action                                                         |
|------------------------|----------------------------------------------------------------|
| Enter                  | Send the current prompt.                                       |
| Shift+Enter / Ctrl+J   | Insert a newline.                                              |
| Ctrl+C                 | Cancel current turn (while streaming) / exit (second press).   |
| Ctrl+L                 | Clear the viewport (history is preserved).                     |
| Ctrl+U                 | Clear the textarea.                                            |
| PgUp / PgDn            | Scroll the viewport.                                           |
| ↑ / ↓                  | Recall previous prompts (when input is empty).                 |
| `/` (start of input)   | Open slash-command palette.                                    |
| `@` (anywhere)         | Open file picker.                                              |
| Esc                    | Close any open overlay.                                        |
| Tab                    | Fill the highlighted palette item without submitting.          |

## Streaming

Assistant responses stream in token by token. While streaming:

- The textarea stays usable — you can compose your next prompt in the background.
- Submit (Enter) is disabled until the current turn finishes, so you don't accidentally enqueue mid-thought.
- Markdown is rendered through Glamour once the turn completes (during stream, raw text is shown for low latency).

## Modal overlays

Three modal flows take over key handling when active:

- **Permission modal** — appears when the agent wants to do something the permission gate flagged. Reply with `y` / `s` / `a` / `n` (or Esc).
- **Model picker** — `/model` opens it; ↑/↓ navigate, Enter selects, Esc cancels.
- **MCP elicitation modal** — appears when an MCP server requests user input. Form mode lets you fill fields with Tab navigation; URL mode shows a URL with `o` to open in a browser.

## Theming

Cogo's markdown renderer (Glamour) auto-detects light vs dark terminal background once at startup. The palette uses adaptive colors so it works under both.

The permission-mode badge in the header is colored: `ask` is accent-colored, `yolo` is **bold red** so you can never miss it.
