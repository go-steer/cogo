---
title: Debug Logging
weight: 3
---

For after-the-fact investigation, `--debug` swaps Cogo's slog handler to a JSONL writer that captures every log event to disk.

## Usage

```bash
cogo --debug -p "what does internal/agent do?"
# logs land in .agents/logs/2026-05-02T19-00-00Z.jsonl
```

Or interactively:

```bash
cogo --debug
```

The flag is process-wide — every package's `slog` calls go into the same file for the duration of the run.

## What's logged

- Tool dispatch (which tool, what args summary)
- Permission decisions (allow / deny / always-allow)
- MCP server lifecycle (start, stop, tool list)
- Agent loop state transitions
- Provider request/response metadata (no full prompts)
- Error stacks for non-fatal failures

Sensitive data — full prompts, response bodies, env-interpolated MCP secrets — is **not** logged. The goal is "what happened" not "what was said".

## Format

JSON-lines, one event per line:

```json
{"time":"2026-05-02T19:00:01.234Z","level":"INFO","msg":"agent.turn.start","model":"gemini-3.1-pro-preview","prompt_len":127}
{"time":"2026-05-02T19:00:02.456Z","level":"INFO","msg":"tool.dispatch","tool":"read_file","path":"internal/agent/agent.go"}
{"time":"2026-05-02T19:00:02.789Z","level":"INFO","msg":"tool.complete","tool":"read_file","bytes":4231,"truncated":false}
{"time":"2026-05-02T19:00:08.012Z","level":"INFO","msg":"agent.turn.complete","input_tokens":1248,"output_tokens":423}
```

Pipe through `jq` for ad-hoc analysis:

```bash
# Tool call counts for the latest run
ls -t .agents/logs/*.jsonl | head -1 \
  | xargs jq -r 'select(.msg=="tool.dispatch") | .tool' \
  | sort | uniq -c | sort -rn
```

## Without `--debug`

Without the flag, slog uses Go's text handler at INFO level → stderr. Less detail, easier on the eyes for normal use.

## Rotation

No automatic rotation today. Long sessions can produce large files; rotate with `logrotate` or just clear `.agents/logs/` periodically. `.agents/.gitignore` (written by `cogo init`) ignores the directory by default.
