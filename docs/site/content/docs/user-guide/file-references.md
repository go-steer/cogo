---
title: "@-File References"
weight: 3
---

The `@` token is Cogo's shorthand for "include this file's contents in my prompt". Useful when you want to direct the agent's attention at a specific file without making it call `read_file` first.

## How it works

Type `@` anywhere in your prompt. A file picker opens immediately, filtered as you keep typing:

```
look at @int‸
```

Selecting a file inserts the path (e.g. `@internal/agent/agent.go`) and a trailing space. Selecting a directory drills in — the picker stays open with the new prefix.

When you submit:

1. Each `@<path>` token in the prompt is expanded.
2. The file's contents are inlined as fenced code blocks beneath your prompt.
3. The user-facing message in chat history shows your original prompt with `@` tokens preserved (so you can see what you asked).
4. The agent receives the expanded version with the file contents inline.

A system message confirms what was inlined:

```
Inlined file references: internal/agent/agent.go, internal/tui/program.go
```

## Path scope warnings

If an `@`-reference points outside the project's [path scope](../permissions/#path-scope), Cogo still inlines the file (your keystroke is consent) but emits a one-line warning so you don't accidentally paste private files into the model context:

```
⚠ Inlined out-of-scope file(s): /Users/me/.ssh/id_rsa — these were sent to the model.
```

To allow without the warning, add the path to `path_scope.allow` in `.agents/config.json`.

## Size limits

Each `@`-reference is capped at **64 KiB** to keep prompts manageable. Files larger than that are truncated with a notice; for those, ask the agent to use `read_file` directly so it can stream chunks.

## Limitations

- The picker lists files relative to the project root, not the cwd. If you `cd` into a subdirectory, paths still anchor at the root.
- Symlinks are followed only when they resolve inside the path scope.
- Binary files are excluded from the picker.
