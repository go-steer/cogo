---
title: MCP Servers
weight: 3
---

[MCP](https://modelcontextprotocol.io) (Model Context Protocol) lets Cogo borrow tools from external servers — a filesystem mount, a GitHub-API server, an internal company API, anything that speaks the protocol. Drop a `.agents/mcp.json` and the servers come up at startup.

## Schema

```json
{
  "version": 1,
  "servers": {
    "<name>": {
      "transport": "stdio" | "http",
      // stdio fields:
      "command": "...",
      "args": ["..."],
      "env": { "KEY": "value or ${env:OTHER}" },
      // http fields:
      "url": "https://...",
      "headers": { "Authorization": "Bearer ${env:TOKEN}" }
    }
  }
}
```

The server name is the namespace prefix for its tools — `filesystem` exposes its `read_file` as `filesystem_read_file`, so MCP tools never collide with Cogo's built-ins.

## Stdio transport

Spawns the configured command as a child process and speaks MCP over its stdin/stdout.

```json
{
  "version": 1,
  "servers": {
    "filesystem": {
      "transport": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/Users/me/work"]
    },
    "git": {
      "transport": "stdio",
      "command": "uvx",
      "args": ["mcp-server-git", "--repository", "."]
    }
  }
}
```

Lifecycle:

- Spawned eagerly when Cogo starts (and on `/reload`).
- On `/reload` and on TUI exit, children get **SIGTERM → 3-second grace → SIGKILL**, then `Wait` is called to reap.
- If a server fails to start, it appears in `/mcp` with `status: error` and the message — the rest of the agent keeps running.

## Streamable HTTP transport

Talks to a long-lived HTTP server using MCP's POST + SSE transport.

```json
{
  "version": 1,
  "servers": {
    "github": {
      "transport": "http",
      "url": "https://mcp.example.com/v1",
      "headers": {
        "Authorization": "Bearer ${env:GITHUB_TOKEN}"
      }
    }
  }
}
```

WebSocket is intentionally not supported — it's not in the current MCP spec.

## Env-var interpolation

Both `env` (stdio) and `headers` (http) values support `${env:NAME}` interpolation:

```json
"headers": {
  "Authorization": "Bearer ${env:GH_TOKEN}",
  "X-Org": "${env:ORG_ID}"
}
```

The substitution happens once at server startup; secrets are not echoed in `/mcp` output.

## `/mcp` output

```
MCP servers:
  filesystem — ok
      • filesystem_list_directory
      • filesystem_read_file
      • filesystem_write_file
  github — error (dial tcp: lookup mcp.example.com: no such host)
```

Run `/reload` after editing `mcp.json` to apply changes without restarting.

## Permissions

MCP tools go through the same permission gate as built-ins. Patterns use the `mcp:` namespace:

```json
{
  "permissions": {
    "allow": [
      "mcp:filesystem_read_file",
      "mcp:github_list_pull_requests"
    ]
  }
}
```

`[a] always allow` in the modal will pick the right pattern for you.

## Elicitation

If an MCP server requests user input mid-call (the protocol's `elicit` method), Cogo opens a modal:

- **Form mode** — for primitive fields (string, number, integer, boolean, enum). Tab to navigate, Enter to submit, Esc to cancel.
- **URL mode** — shows a URL with `o` to open in your browser, `a` to confirm completion, `n` to decline, Esc to cancel.

Schemas with nested objects or arrays (which the spec disallows for elicitation) are auto-declined with a system-message notice rather than rendering an unsafe form.

In headless mode (`cogo -p`) there's no UI, so elicit requests are auto-declined with a one-line stderr notice.
