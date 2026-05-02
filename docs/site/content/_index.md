---
title: Cogo
toc: false
---

{{< hextra/hero-badge link="https://github.com/go-steer/cogo/releases" >}}
  <span>Latest release</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  An agentic CLI&nbsp;<br class="hx:sm:block hx:hidden" />for Go developers
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  Like Claude Code, but built in Go on the Google ADK and&nbsp;<br class="hx:sm:block hx:hidden" />Gemini 3.x. One static binary, MCP-native, project-scoped.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="Get Started" link="docs/getting-started/" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="Two modes, one binary"
    subtitle="Pipeable `cogo -p` for shell + CI; full Bubble Tea TUI on a TTY with streaming markdown, slash + @-file palettes, and prompt history."
    class="hx:aspect-auto hx:md:aspect-[1.1/1] hx:max-md:min-h-[340px]"
  >}}
  {{< hextra/feature-card
    title="Project-scoped"
    subtitle="A `.agents/` directory holds model, permissions, MCP servers, skills, and project memory. Auto-discovered like `.git`."
    class="hx:aspect-auto hx:md:aspect-[1.1/1] hx:max-md:min-h-[340px]"
  >}}
  {{< hextra/feature-card
    title="MCP-native"
    subtitle="Drop a `.agents/mcp.json` describing stdio or HTTP MCP servers and their tools become callable. Schema-driven elicitation modal for servers that prompt the user."
    class="hx:aspect-auto hx:md:aspect-[1.1/1] hx:max-md:min-h-[340px]"
  >}}
  {{< hextra/feature-card
    title="Claude-compatible skills"
    subtitle="`SKILL.md` bundles under `.agents/skills/<name>/` are loaded lazily and invoked as agent tools. Same format as Claude Code."
    class="hx:aspect-auto hx:md:aspect-[1.1/1] hx:max-md:min-h-[340px]"
  >}}
  {{< hextra/feature-card
    title="Permissions you can trust"
    subtitle="Three modes (ask / allow / yolo), in-TUI approval modal, non-overridable bash denylist, path-scope confinement for file tools."
    class="hx:aspect-auto hx:md:aspect-[1.1/1] hx:max-md:min-h-[340px]"
  >}}
  {{< hextra/feature-card
    title="Cost & telemetry"
    subtitle="Per-turn `↑in · ↓out · $cost`, session totals in the header, OpenTelemetry support, and JSON transcripts persisted on exit."
    class="hx:aspect-auto hx:md:aspect-[1.1/1] hx:max-md:min-h-[340px]"
  >}}
{{< /hextra/feature-grid >}}

<div class="hx:mt-12 hx:mb-6">

## Install

{{< tabs items="Homebrew,Docker,go install,Source" >}}

  {{< tab >}}
  ```bash
  brew install go-steer/cogo/cogo
  cogo --version
  ```
  {{< /tab >}}

  {{< tab >}}
  ```bash
  docker pull ghcr.io/go-steer/cogo:latest
  docker run --rm -it -v "$PWD:/work" -w /work \
    ghcr.io/go-steer/cogo:latest -p "hello"
  ```
  {{< /tab >}}

  {{< tab >}}
  ```bash
  go install github.com/go-steer/cogo/cmd/cogo@latest
  ```
  {{< /tab >}}

  {{< tab >}}
  ```bash
  git clone https://github.com/go-steer/cogo
  cd cogo
  go run ./cmd/cogo -p "hello"
  ```
  {{< /tab >}}

{{< /tabs >}}

</div>

<div class="hx:mt-6 hx:mb-6">

## Try it in 60 seconds

```bash
# Auth (pick one)
export GOOGLE_API_KEY=...                            # public Gemini API
# or
gcloud auth application-default login                # Vertex AI
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=your-project

# Single turn, pipeable
cogo -p "Summarize the files in this directory."

# Or interactive
cogo
```

For a fresh project: `cogo init` writes a sensible `.agents/` skeleton; `cogo init --interactive` walks you through provider, model, and permission mode.

</div>
