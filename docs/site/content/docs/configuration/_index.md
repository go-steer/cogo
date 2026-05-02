---
title: Configuration
weight: 3
sidebar:
  open: true
---

Everything Cogo reads from disk lives under `.agents/`. Each file is optional — Cogo runs with built-in defaults when nothing's there.

{{< cards >}}
  {{< card link="agents-directory/" title=".agents/ Directory" subtitle="The full layout: what each file is for and when it's read." icon="folder" >}}
  {{< card link="config-json/" title="config.json" subtitle="Model, provider, permission mode, path scope, OTEL settings." icon="adjustments" >}}
  {{< card link="mcp-servers/" title="MCP Servers" subtitle="mcp.json schema, stdio + Streamable HTTP transports, env interpolation." icon="server" >}}
  {{< card link="skills/" title="Skills" subtitle="Claude-compatible SKILL.md bundles. Frontmatter, body, discovery." icon="puzzle" >}}
  {{< card link="memory/" title="Project Memory" subtitle="AGENTS.md / CLAUDE.md / GEMINI.md fallback chain + ~/.cogo/AGENTS.md." icon="document-text" >}}
{{< /cards >}}
