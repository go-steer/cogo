---
title: Observability
weight: 5
sidebar:
  open: true
---

Cogo has three observability surfaces — pick whichever matches what you're investigating.

{{< cards >}}
  {{< card link="telemetry/" title="OpenTelemetry" subtitle="Console + OTLP exporters covering the agent loop, tools, and provider calls." icon="chart-bar" >}}
  {{< card link="transcripts/" title="Session Transcripts" subtitle="JSON dump of every interactive session, written on exit." icon="document" >}}
  {{< card link="debug-logging/" title="Debug Logging" subtitle="--debug swaps slog to a JSONL handler under .agents/logs/." icon="bug-ant" >}}
{{< /cards >}}
