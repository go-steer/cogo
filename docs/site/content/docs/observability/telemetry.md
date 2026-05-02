---
title: OpenTelemetry
weight: 1
---

Cogo emits OpenTelemetry traces (and, in the future, metrics) covering the agent loop, individual tool calls, and provider requests. Off by default — opt in via `config.json` or env vars.

## Exporters

Set `otel.exporter` in `.agents/config.json`:

| Value       | Behavior                                                                      |
|-------------|-------------------------------------------------------------------------------|
| `"none"`    | No telemetry. **Default.**                                                    |
| `"console"` | Pretty-printed spans to stderr. Best for local debugging.                     |
| `"otlp"`    | OTLP gRPC exporter; uses `OTEL_EXPORTER_OTLP_ENDPOINT` (and friends).         |

```json
{
  "otel": { "exporter": "otlp" }
}
```

Then point at your collector:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
export OTEL_SERVICE_NAME=cogo
```

Standard OTEL env vars work — `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_EXPORTER_OTLP_HEADERS`, etc.

## What's traced

- **`cogo.session`** — root span for the whole interactive or headless session.
- **`cogo.turn`** — one span per agent turn (user prompt → final assistant response).
- **`cogo.tool.<name>`** — one span per tool invocation, with attributes for tool name, args summary, success/failure, output bytes.
- **`cogo.provider.<provider>.generate`** — one span per provider call, with attributes for model, input/output tokens, finish reason, latency.

The agent SDK (Google ADK) emits its own internal spans too — those nest under the `cogo.turn` span and give you visibility into the multi-step tool-use loop.

## Console exporter (quick local debug)

```bash
# .agents/config.json: { "otel": { "exporter": "console" } }
cogo -p "list the test files"
```

Spans land on stderr alongside any other output. Useful for "why did this turn take 6 seconds" questions without standing up a collector.

## OTLP setup

Any OTLP-compatible collector works. The most common local setup:

```bash
# Run an OTEL collector via Docker
docker run --rm -p 4317:4317 -p 4318:4318 \
  -v "$PWD/otel-config.yaml:/etc/otel-config.yaml" \
  otel/opentelemetry-collector-contrib:latest \
  --config=/etc/otel-config.yaml
```

Then:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
cogo
```

Spans flow into the collector's configured exporters (Jaeger, Tempo, Honeycomb, Datadog, etc.).

## Production tip

Don't enable `console` in production — span output mixes with the user-facing stderr stream and confuses anyone reading logs. Use `otlp` to a real collector or leave it `none`.

## Future

Metrics (turn count, tool error rate, token throughput) are tracked as future work — for now it's traces only.
