---
title: Built-in Tools
weight: 4
sidebar:
  open: true
---

Cogo ships with a small, opinionated tool set. Every tool is gated by the [permission system](../user-guide/permissions/) and capped by the `tool_output` config so a runaway call can't blow out your context window.

## Reference

| Tool          | Purpose                                                              | Permission category |
|---------------|----------------------------------------------------------------------|---------------------|
| `read_file`   | Read a file's contents (or a byte range).                            | `path_scope`        |
| `write_file`  | Create or overwrite a file. Atomic temp + rename.                    | `path_scope`        |
| `edit_file`   | Apply a string replacement, asserting the original text is present.  | `path_scope`        |
| `list_dir`    | List a directory non-recursively, with type + size per entry.        | `path_scope`        |
| `bash`        | Run a shell command via `/bin/sh -c`. Output captured + capped.      | `bash`              |
| `todo`        | Maintain an in-session task list the agent uses to plan + track.     | n/a (no I/O)        |

Plus any [MCP tools](../configuration/mcp-servers/) and [skills](../configuration/skills/) you've configured — those use the same permission gate and output capping.

## File tools

`read_file` / `write_file` / `edit_file` / `list_dir` are confined to the [path scope](../user-guide/permissions/#path-scope) — by default, the project root and `~/.cogo/`. Out-of-scope reads + writes prompt the user.

`write_file` and `edit_file` use **atomic temp + rename**: the new content is written to a sibling tempfile, fsync'd, then renamed over the target. Power loss mid-write can't leave the file half-written.

`edit_file` requires the `old_string` to be present and unique. If it appears multiple times the call fails with a clear error so the agent can re-narrow the search.

## bash

Runs the command via `/bin/sh -c "<command>"`. Designed for short utility commands, not long-running daemons.

- **Timeout**: default 60s, override with the `timeout_seconds` argument.
- **Output cap**: 64 KiB stdout + 64 KiB stderr by default. Override per-tool in `config.json`.
- **Non-zero exit**: surfaced to the agent with the exit code; not a hard failure.
- **Denylist**: a non-overridable list of dangerous patterns is refused before the modal even appears (see [Permissions](../user-guide/permissions/#bash-denylist)).

The agent gets a structured response:

```json
{
  "exit_code": 0,
  "stdout": "...",
  "stderr": "...",
  "timed_out": false,
  "truncated": false
}
```

## todo

A purely in-session task tracker — no disk I/O, no permission gating. The agent uses it to plan multi-step work and check off progress as it goes.

State persists for the session and is wiped on `/clear` or exit. The full list shows in any `todo` call's response so the model can see everything at a glance without scrolling chat history.

This is one of the highest-leverage tools — coding agents that don't track state tend to lose track of multi-step plans. Cogo's todo is modeled on Claude Code's.

## Output capping

Every tool's output is capped per `config.json → tool_output`. Defaults:

- 64 KiB max bytes
- 2000 max lines
- Override per tool with `tool_output.per_tool.<name>`.

When output is truncated, the agent gets a notice (`… [truncated, X bytes more]`) so it can decide to chunk-read or narrow the query.

Set the cap to `0` to disable a particular limit (useful for `read_file` if you trust the model to handle large files).
