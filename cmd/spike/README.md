# Cogo ADK Spike

A throwaway program whose only job is to **validate the architectural assumptions in `docs/DESIGN.md` and `docs/REQUIREMENTS.md`** before we commit to building V1.

If everything here passes, the design is buildable as written. If something fails or surprises us, we update the docs *before* writing real code.

---

## What we're validating — all PASSING as of 2026-05-02

| # | Assumption | Status | Notes |
|---|-----------|--------|-------|
| 1 | Auth: API key + Vertex (ADC) | PASS | Vertex via `genai.BackendVertexAI` + project + location |
| 2 | ADK basic agent run on Gemini 3.1 Pro | PASS | Real model ID is `gemini-3.1-pro-preview` |
| 3 | Streaming partial events | PASS | Requires `agent.RunConfig{StreamingMode: agent.StreamingModeSSE}` — default is no streaming |
| 4 | Tool call round-trip via `functiontool` | PASS | Generic `functiontool.New[In,Out]` infers JSON Schema from struct tags |
| 5 | Tool confirmation / HITL | PASS | `RequireConfirmation: true` → runner emits `event.LongRunningToolIDs`; this is the permission-gate hook |
| 6 | MCP integration via `mcptoolset` | PASS | Uses official `github.com/modelcontextprotocol/go-sdk/mcp` v1.2+ |
| 7 | OTEL span capture | PASS | Must call `providers.SetGlobalOtelProviders()` after `telemetry.New` — globals are not auto-installed |
| 8 | MCP elicitation surface (FR-6.7) | PASS | `mcp.ClientOptions{ElicitationHandler}` works; pass custom client via `mcptoolset.Config{Client}` |

**Conclusion:** the architecture in `docs/DESIGN.md` is buildable as written. No design changes needed; only specific gotchas to remember (streaming opt-in, OTEL global install, HITL via LongRunningToolIDs).

---

## How to run

```bash
# 1. Pick an auth path (do at least one)
export GOOGLE_API_KEY=...           # public Gemini API
# OR
gcloud auth application-default login
export GOOGLE_CLOUD_PROJECT=...
export GOOGLE_CLOUD_LOCATION=us-central1
export GOOGLE_GENAI_USE_VERTEXAI=true

# 2. Pull deps and run
go mod tidy
go run ./cmd/spike
```

The spike prints a per-check status table at the end. Each check is one of:
- **PASS** — assumption confirmed.
- **FAIL** — assumption broken; details in the notes.
- **SKIP** — required setup absent (e.g. no API key); not a failure of the design.
- **UNKNOWN** — needs manual investigation (e.g. "look for ADK module").

---

## Resolved: where the ADK lives

`google.golang.org/adk` (canonical) — the GitHub mirror `github.com/google/adk-go` resolves to the same module but `go get` rejects it with "module declares its path as: google.golang.org/adk". Use the vanity import.

Latest version at validation time: **v1.2.0**.

---

## Cleanup

When V1 development starts, delete `cmd/spike/` entirely:
```bash
rm -rf cmd/spike
```

The findings should be folded into the docs, not preserved as code.
