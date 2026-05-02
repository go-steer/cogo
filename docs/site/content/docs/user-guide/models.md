---
title: Switching Models
weight: 5
---

Mid-session model switching lets you pop between a fast/cheap model and a stronger/slower one without restarting Cogo.

## Picker

Type `/model` with no args:

```
Model picker (↑/↓ select · enter switch · esc cancel)
▸ gemini-3.1-pro-preview  (current)
  gemini-3.5-flash-preview
  gemini-3.0-pro
  ...
```

↑/↓ navigate, Enter switches, Esc cancels.

## Direct switch

Skip the picker by passing the model ID:

```
/model gemini-3.5-flash-preview
```

## What changes when you switch

- The agent is rebuilt with the new model bound, but **everything else carries over**: project memory, MCP servers, skills, permission state.
- The conversation context **resets** — the new model gets a fresh system prompt with the same memory, but no prior turns. (Different models have different context windows and prompt formats; mixing histories tends to confuse them.)
- A system message confirms the switch:
  ```
  Switched to gemini-3.5-flash-preview. Conversation context resets for the new model.
  ```

## Persistence

If your project has an `.agents/config.json`, the new model ID is **persisted** to that file under `model.name`. Subsequent runs will use the new default.

To switch in-session only (don't persist), edit `config.json` afterward to revert.

If you're running without an `.agents/` (built-in defaults), the switch is in-session only — nothing is written to disk.

## Why switch?

Common patterns:

- **`pro` for planning, `flash` for execution** — start a session in `gemini-3.1-pro-preview` to scope a refactor, then `/model gemini-3.5-flash-preview` for the long tail of mechanical edits.
- **Cost-sensitive headless runs** — set `model.name` to `flash` in `config.json` for CI scripts; flip back to `pro` interactively.
- **Trying a new release** — `/model gemini-X-preview` to A/B against the current default without committing.
