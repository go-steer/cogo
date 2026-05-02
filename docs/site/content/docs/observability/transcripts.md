---
title: Session Transcripts
weight: 2
---

Every interactive session writes a JSON transcript to `.agents/sessions/<rfc3339>.json` on exit (`/quit` or Ctrl+C). Headless `cogo -p` invocations also write a one-turn transcript when there's a project root.

## Format

```json
{
  "version": 1,
  "started_at": "2026-05-02T19:00:00Z",
  "ended_at":   "2026-05-02T19:14:23Z",
  "model": "gemini-3.1-pro-preview",
  "messages": [
    { "role": "user",       "text": "Refactor internal/agent to ..." },
    { "role": "assistant",  "text": "Sure, here's the plan ..." },
    { "role": "system",     "text": "Permission allow-once: bash — go test ./..." },
    { "role": "user",       "text": "Looks good." },
    { "role": "assistant",  "text": "Done." }
  ],
  "usage": {
    "turns": 5,
    "input_tokens": 12348,
    "output_tokens": 3902,
    "cost_usd": 0.041
  }
}
```

## Why

Two main use cases:

1. **Reproducibility** — replay or audit what the agent saw + said. Tool-call summaries land in `system` role messages.
2. **Cost reporting** — the `usage` block has the totals matching the `/stats` and exit-summary numbers.

## Atomic writes

Transcripts are written via temp + rename. Crashes mid-write don't leave a half-file behind.

## Where

```
.agents/sessions/
├── 2026-05-02T19-00-00Z.json
├── 2026-05-02T20-15-42Z.json
└── 2026-05-02T22-03-11Z.json
```

The `:` in RFC 3339 is replaced with `-` for filename portability (Windows-friendly).

## Gitignore

`cogo init` adds `sessions/` to `.agents/.gitignore` by default — transcripts are local artifacts, not source. If you want to keep them in version control (some teams do, for audit), remove that line.

## Inspecting

Any JSON tool works:

```bash
# Latest transcript
ls -t .agents/sessions/*.json | head -1 | xargs cat | jq .usage

# Total cost across the last 7 days
find .agents/sessions -name "*.json" -mtime -7 \
  | xargs -I{} jq '.usage.cost_usd' {} \
  | awk '{ s+=$1 } END { print s }'
```

## Disable

Transcripts only write when there's an `.agents/` project root. To disable, `cogo` from outside any project (no transcript is written) — but you lose every other `.agents/`-anchored feature too. There's no per-feature opt-out yet.
