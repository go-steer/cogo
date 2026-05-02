---
title: "config.json"
weight: 2
---

`.agents/config.json` is the central configuration file. Every field has a sensible default — you only need to set what you want to override.

## Full schema

```json
{
  "version": 1,
  "model": {
    "provider": "vertex",
    "name": "gemini-3.1-pro-preview"
  },
  "permissions": {
    "mode": "ask",
    "allow": [
      "bash:git status*",
      "bash:go test*",
      "path_scope:/Users/me/notes"
    ]
  },
  "path_scope": {
    "allow": [
      "/Users/me/notes/**"
    ]
  },
  "tool_output": {
    "default_max_bytes": 65536,
    "default_max_lines": 2000,
    "per_tool": {
      "bash":     { "max_bytes": 131072, "max_lines": 5000 },
      "read_file":{ "max_bytes": 524288, "max_lines": 0 }
    }
  },
  "otel": {
    "exporter": "none"
  },
  "pricing": {
    "gemini-3.1-pro-preview": {
      "input_per_million_usd": 1.25,
      "output_per_million_usd": 5.00
    }
  }
}
```

## Top-level fields

### `version` *(int)*

Schema version. Currently `1`. Set by `cogo init`; you don't need to bump it.

### `model` *(object)*

| Field      | Default                       | Notes                                                    |
|------------|-------------------------------|----------------------------------------------------------|
| `provider` | auto-detect from env vars     | `"gemini"` (public API) or `"vertex"` (Vertex AI).       |
| `name`     | `"gemini-3.1-pro-preview"`    | Any model ID the chosen provider exposes.                |

When `provider` is unset, Cogo picks one based on env vars: if `GOOGLE_GENAI_USE_VERTEXAI=true`, Vertex; if `GOOGLE_API_KEY` is set, Gemini API; otherwise an error at startup.

### `permissions` *(object)*

| Field   | Default | Notes                                                                    |
|---------|---------|--------------------------------------------------------------------------|
| `mode`  | `"ask"` | `"ask"` / `"allow"` / `"yolo"`. See [Permissions](../../user-guide/permissions/). |
| `allow` | `[]`    | Pre-approved patterns of the form `<tool>:<key>`.                        |

`allow` entries are appended to whenever the user picks `[a] always allow` in the modal.

### `path_scope` *(object)*

| Field    | Default | Notes                                                  |
|----------|---------|--------------------------------------------------------|
| `allow`  | `[]`    | Glob patterns of additional paths file tools may touch.|

The project root and `~/.cogo/` are always in scope; `path_scope.allow` extends that. Patterns use Go's `filepath.Match` glob syntax with `**` for recursive matches.

### `tool_output` *(object)*

Caps on how much output a single tool call can return — protects the model context window from runaway file reads or chatty bash commands.

| Field                 | Default | Notes                                              |
|-----------------------|---------|----------------------------------------------------|
| `default_max_bytes`   | `65536` | 64 KiB.                                            |
| `default_max_lines`   | `2000`  | Set to `0` to disable line cap.                    |
| `per_tool.<name>`     | —       | Override the defaults per tool name.               |

When output is truncated, the agent sees a notice (`… [truncated, 5 KiB more]`) so it can decide to chunk-read.

### `otel` *(object)*

| Field      | Default  | Notes                                                              |
|------------|----------|--------------------------------------------------------------------|
| `exporter` | `"none"` | `"none"`, `"console"` (stderr), or `"otlp"` (uses `OTEL_EXPORTER_OTLP_ENDPOINT`). |

See [Observability](../../observability/telemetry/) for full setup.

### `pricing` *(map)*

Optional per-model pricing for the cost surfacing. Defaults are baked in for the common Gemini SKUs; override here if your contract has different rates (or when a new model lands before Cogo has its pricing built in).

```json
"pricing": {
  "my-custom-model": {
    "input_per_million_usd": 0.50,
    "output_per_million_usd": 1.50
  }
}
```

## Editing live

`/reload` re-reads `config.json` and rebuilds the agent. Changes to `model.name` / `permissions.allow` / `path_scope.allow` take effect immediately. Some fields (e.g. `otel.exporter`) require a process restart because the OpenTelemetry providers can't be safely swapped at runtime.
