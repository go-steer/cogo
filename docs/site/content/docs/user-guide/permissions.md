---
title: Permissions
weight: 4
---

Cogo's agent loop calls tools — file edits, shell commands, MCP servers, skills. The permission system controls what runs without asking and what prompts you first.

## Modes

Set in `.agents/config.json` under `permissions.mode`:

| Mode    | Behavior                                                                            |
|---------|-------------------------------------------------------------------------------------|
| `ask`   | Prompt before any mutating tool call. **Default.**                                  |
| `allow` | Pre-approve everything in `permissions.allow`. Prompt for everything else.          |
| `yolo`  | No prompts. The non-overridable bash denylist still blocks the truly dangerous bits.|

The current mode shows in the header (the `yolo` badge is **bold red** so you can never miss it).

## The approval modal

When the gate decides a tool needs approval, the input area is replaced by a modal:

```
File write: write_file(path="internal/agent/agent.go", bytes=4231)
[y] allow once   [s] allow session   [a] always allow   [n/esc] deny
```

| Key       | Decision                                                                          |
|-----------|-----------------------------------------------------------------------------------|
| `y`       | Allow this single call.                                                           |
| `s`       | Allow this exact pattern for the rest of the session.                             |
| `a`       | Always allow — persisted to `.agents/config.json` under `permissions.allow`.      |
| `n` / Esc | Deny. The agent gets a "permission denied" tool result.                           |

A system message echoes your decision into chat history so there's a paper trail.

## Bash denylist

Some bash patterns are refused unconditionally — they don't even reach the modal. The denylist covers things like:

- `rm -rf /`, `rm -rf $HOME`, etc.
- `dd if=/dev/zero of=/dev/sd*`
- `mkfs.*`, `:(){ :|:& };:` (fork bomb), `chmod -R 777 /`
- Pipe-to-shell (`curl ... | sh`), unredirected `> /dev/sd*`

This list is **not configurable**. If you genuinely need to format a disk, do it in your own shell — Cogo will not.

## Path scope

File-touching tools (`read_file`, `write_file`, `edit_file`, `list_dir`) are confined by default to:

- The project root (the directory containing `.agents/`).
- `~/.cogo/` (your user-global config + memory).
- Any glob patterns in `path_scope.allow`.

A read or write outside scope triggers an approval prompt; if you pick `[a] always allow`, the path's parent directory is added to `path_scope.allow` and persisted.

Example `config.json` snippet:

```json
{
  "permissions": {
    "mode": "ask",
    "allow": [
      "bash:git status*",
      "bash:go test*",
      "path_scope:/Users/me/notes"
    ]
  },
  "path_scope": {
    "allow": ["/Users/me/notes/**"]
  }
}
```

## Allowlist patterns

Entries in `permissions.allow` use the form `<tool>:<key>`:

- **Bash**: `bash:git status*` — prefix match with `*` wildcard. `bash:git push origin main` is exact.
- **Path scope**: `path_scope:/Users/me/notes` — directory or file path prefix.
- **MCP tools**: `mcp:filesystem_read_file` — namespaced by server.
- **Skills**: `skills:my-skill`.

The `[a] always allow` button picks the right pattern for you based on the tool that triggered the prompt.

## Headless mode

`cogo -p` runs without a TTY, so there's no modal to show. The gate runs in **strict** mode: anything that would prompt fails fast with a clear "add an explicit allowlist entry" error. Pre-approve the tools you need before scripting Cogo into CI.
