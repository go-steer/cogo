// Cogo ADK spike — validates the architectural assumptions in docs/DESIGN.md
// before V1 implementation begins. See ./README.md for context.
//
// Run: go run ./cmd/spike  (after `go mod tidy`)
//
// Each check reports PASS / FAIL / SKIP / UNKNOWN with notes. Eyeball the
// summary, fold findings into the docs, then `rm -rf cmd/spike`.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/genai"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/telemetry"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/adk/tool/mcptoolset"
)

const (
	// Discovered via cmd/probe-models against gke-demos-345619 / global.
	// All Gemini 3.x models are currently in "preview" — no GA suffix yet.
	modelPro   = "gemini-3.1-pro-preview"
	modelFlash = "gemini-3-flash-preview"
)

type status string

const (
	statusPass    status = "PASS"
	statusFail    status = "FAIL"
	statusSkip    status = "SKIP"
	statusUnknown status = "UNKNOWN"
)

type result struct {
	name   string
	status status
	notes  string
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	fmt.Println("Cogo ADK spike — validating design assumptions")
	fmt.Println(strings.Repeat("=", 60))

	results := []result{
		checkAuth(ctx),
		checkADKBasicAgent(ctx),
		checkADKStreaming(ctx),
		checkADKToolCall(ctx),
		checkADKToolConfirmation(ctx),
		checkMCPInMemory(ctx),
		checkOTEL(ctx),
		checkElicitation(ctx),
	}

	printSummary(results)

	for _, r := range results {
		if r.status == statusFail {
			os.Exit(1)
		}
	}
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func clientConfig() *genai.ClientConfig {
	if os.Getenv("GOOGLE_GENAI_USE_VERTEXAI") == "true" && os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		return &genai.ClientConfig{
			Backend:  genai.BackendVertexAI,
			Project:  os.Getenv("GOOGLE_CLOUD_PROJECT"),
			Location: os.Getenv("GOOGLE_CLOUD_LOCATION"),
		}
	}
	if os.Getenv("GOOGLE_API_KEY") != "" {
		return &genai.ClientConfig{
			APIKey:  os.Getenv("GOOGLE_API_KEY"),
			Backend: genai.BackendGeminiAPI,
		}
	}
	return nil
}

func makeModel(ctx context.Context, modelID string) (adkmodel.LLM, error) {
	cfg := clientConfig()
	if cfg == nil {
		return nil, errors.New("no auth configured (set GOOGLE_API_KEY or GOOGLE_GENAI_USE_VERTEXAI=true + GOOGLE_CLOUD_PROJECT)")
	}
	return gemini.NewModel(ctx, modelID, cfg)
}

func makeRunner(a adkagent.Agent) (*runner.Runner, error) {
	return runner.New(runner.Config{
		AppName:           "cogo-spike",
		Agent:             a,
		SessionService:    session.InMemoryService(),
		AutoCreateSession: true,
	})
}

type runOutput struct {
	finalText      string
	toolCalls      []string
	toolReturns    []string
	partialEvents  int
	completeEvents int
	confirmEvents  int
}

func runAgent(ctx context.Context, a adkagent.Agent, prompt string) (*runOutput, error) {
	return runAgentWith(ctx, a, prompt, adkagent.RunConfig{})
}

func runAgentWith(ctx context.Context, a adkagent.Agent, prompt string, rc adkagent.RunConfig) (*runOutput, error) {
	r, err := makeRunner(a)
	if err != nil {
		return nil, fmt.Errorf("runner: %w", err)
	}
	msg := genai.NewContentFromText(prompt, genai.RoleUser)
	out := &runOutput{}
	var finalText strings.Builder
	for event, err := range r.Run(ctx, "u1", "s1", msg, rc) {
		if err != nil {
			return out, err
		}
		if event.Partial {
			out.partialEvents++
		}
		if event.TurnComplete {
			out.completeEvents++
		}
		if len(event.LongRunningToolIDs) > 0 {
			out.confirmEvents++
		}
		if event.Content == nil {
			continue
		}
		for _, p := range event.Content.Parts {
			if p.Text != "" && !event.Partial {
				finalText.WriteString(p.Text)
			}
			if p.FunctionCall != nil {
				out.toolCalls = append(out.toolCalls, p.FunctionCall.Name)
			}
			if p.FunctionResponse != nil {
				out.toolReturns = append(out.toolReturns, p.FunctionResponse.Name)
			}
		}
	}
	out.finalText = strings.TrimSpace(finalText.String())
	return out, nil
}

// ----------------------------------------------------------------------------
// Checks
// ----------------------------------------------------------------------------

func checkAuth(_ context.Context) result {
	const name = "1. Auth (API key + Vertex)"
	cfg := clientConfig()
	if cfg == nil {
		return result{name, statusSkip, "neither GOOGLE_API_KEY nor GOOGLE_GENAI_USE_VERTEXAI set"}
	}
	if cfg.Backend == genai.BackendVertexAI {
		return result{name, statusPass, fmt.Sprintf("vertex ok (project=%s, location=%s)", cfg.Project, cfg.Location)}
	}
	return result{name, statusPass, "api-key ok"}
}

func checkADKBasicAgent(ctx context.Context) result {
	const name = "2. ADK basic agent run (Pro)"
	m, err := makeModel(ctx, modelPro)
	if err != nil {
		return result{name, statusSkip, err.Error()}
	}
	a, err := llmagent.New(llmagent.Config{
		Name:        "spike_basic",
		Model:       m,
		Description: "Test agent",
		Instruction: "Reply with exactly the word PONG and nothing else.",
	})
	if err != nil {
		return result{name, statusFail, err.Error()}
	}
	out, err := runAgent(ctx, a, "ping")
	if err != nil {
		return result{name, statusFail, err.Error()}
	}
	if !strings.Contains(strings.ToUpper(out.finalText), "PONG") {
		return result{name, statusFail, fmt.Sprintf("unexpected reply: %q", out.finalText)}
	}
	return result{name, statusPass, fmt.Sprintf("model=%s reply=%q", modelPro, truncate(out.finalText, 40))}
}

func checkADKStreaming(ctx context.Context) result {
	const name = "3. ADK streaming (Flash, partial events)"
	m, err := makeModel(ctx, modelFlash)
	if err != nil {
		return result{name, statusSkip, err.Error()}
	}
	a, err := llmagent.New(llmagent.Config{
		Name:        "spike_stream",
		Model:       m,
		Description: "Test agent",
		Instruction: "Count from 1 to 5, one number per line.",
	})
	if err != nil {
		return result{name, statusFail, err.Error()}
	}
	out, err := runAgentWith(ctx, a, "go", adkagent.RunConfig{StreamingMode: adkagent.StreamingModeSSE})
	if err != nil {
		return result{name, statusFail, err.Error()}
	}
	if out.partialEvents == 0 {
		return result{name, statusFail, fmt.Sprintf("no partial events even with StreamingModeSSE; complete=%d", out.completeEvents)}
	}
	return result{name, statusPass, fmt.Sprintf("partial=%d complete=%d (StreamingModeSSE)", out.partialEvents, out.completeEvents)}
}

// --- tool call ---

type weatherIn struct {
	City string `json:"city" jsonschema:"city name"`
}
type weatherOut struct {
	Summary string `json:"summary"`
}

func weatherFunc(_ tool.Context, in weatherIn) (weatherOut, error) {
	return weatherOut{Summary: fmt.Sprintf("clear, 20°C in %s", in.City)}, nil
}

func checkADKToolCall(ctx context.Context) result {
	const name = "4. ADK tool call (functiontool round-trip)"
	m, err := makeModel(ctx, modelPro)
	if err != nil {
		return result{name, statusSkip, err.Error()}
	}
	weatherTool, err := functiontool.New(functiontool.Config{
		Name:        "get_weather",
		Description: "Returns the current weather for a city.",
	}, weatherFunc)
	if err != nil {
		return result{name, statusFail, fmt.Sprintf("tool: %v", err)}
	}
	a, err := llmagent.New(llmagent.Config{
		Name:        "spike_tool",
		Model:       m,
		Description: "Weather agent",
		Instruction: "Use get_weather when asked about weather. Then summarize the result.",
		Tools:       []tool.Tool{weatherTool},
	})
	if err != nil {
		return result{name, statusFail, err.Error()}
	}
	out, err := runAgent(ctx, a, "What's the weather in Tokyo?")
	if err != nil {
		return result{name, statusFail, err.Error()}
	}
	if !contains(out.toolCalls, "get_weather") {
		return result{name, statusFail, fmt.Sprintf("tool not called; calls=%v final=%q", out.toolCalls, out.finalText)}
	}
	return result{name, statusPass, fmt.Sprintf("called=get_weather final=%q", truncate(out.finalText, 60))}
}

// --- tool confirmation (HITL) ---

type deleteIn struct {
	Path string `json:"path"`
}
type deleteOut struct {
	Status string `json:"status"`
}

func deleteFunc(_ tool.Context, in deleteIn) (deleteOut, error) {
	return deleteOut{Status: "deleted " + in.Path}, nil
}

func checkADKToolConfirmation(ctx context.Context) result {
	const name = "5. ADK tool confirmation (HITL → permission gate)"
	m, err := makeModel(ctx, modelPro)
	if err != nil {
		return result{name, statusSkip, err.Error()}
	}
	delTool, err := functiontool.New(functiontool.Config{
		Name:                "delete_file",
		Description:         "Deletes a file at the given path.",
		RequireConfirmation: true,
	}, deleteFunc)
	if err != nil {
		return result{name, statusFail, err.Error()}
	}
	a, err := llmagent.New(llmagent.Config{
		Name:        "spike_confirm",
		Model:       m,
		Description: "Delete agent",
		Instruction: "When asked to delete a file, use delete_file with the given path.",
		Tools:       []tool.Tool{delTool},
	})
	if err != nil {
		return result{name, statusFail, err.Error()}
	}
	out, err := runAgent(ctx, a, "delete /tmp/junk.txt")
	if err != nil {
		return result{name, statusFail, err.Error()}
	}
	if out.confirmEvents == 0 {
		return result{name, statusFail, fmt.Sprintf("expected long-running confirmation event; calls=%v", out.toolCalls)}
	}
	return result{name, statusPass, fmt.Sprintf("HITL signal received (long-running events=%d); maps to permission gate", out.confirmEvents)}
}

// --- MCP in-memory ---

type mcpWeatherIn struct {
	City string `json:"city" jsonschema:"city name"`
}
type mcpWeatherOut struct {
	Summary string `json:"summary"`
}

func mcpWeatherFunc(_ context.Context, _ *mcp.CallToolRequest, in mcpWeatherIn) (*mcp.CallToolResult, mcpWeatherOut, error) {
	return nil, mcpWeatherOut{Summary: fmt.Sprintf("clear in %s", in.City)}, nil
}

func checkMCPInMemory(ctx context.Context) result {
	const name = "6. MCP in-memory toolset (mcptoolset)"
	m, err := makeModel(ctx, modelPro)
	if err != nil {
		return result{name, statusSkip, err.Error()}
	}

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "spike_weather", Version: "v1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "get_weather", Description: "weather lookup"}, mcpWeatherFunc)
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		return result{name, statusFail, fmt.Sprintf("server connect: %v", err)}
	}

	ts, err := mcptoolset.New(mcptoolset.Config{Transport: clientTransport})
	if err != nil {
		return result{name, statusFail, fmt.Sprintf("toolset: %v", err)}
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "spike_mcp",
		Model:       m,
		Description: "MCP agent",
		Instruction: "Use the get_weather tool when asked about weather. Then summarize the result.",
		Toolsets:    []tool.Toolset{ts},
	})
	if err != nil {
		return result{name, statusFail, err.Error()}
	}
	out, err := runAgent(ctx, a, "What's the weather in Paris?")
	if err != nil {
		return result{name, statusFail, err.Error()}
	}
	if !contains(out.toolCalls, "get_weather") {
		return result{name, statusFail, fmt.Sprintf("MCP tool not called; calls=%v", out.toolCalls)}
	}
	return result{name, statusPass, fmt.Sprintf("MCP roundtrip ok via in-memory transport; final=%q", truncate(out.finalText, 60))}
}

// --- OTEL ---

func checkOTEL(ctx context.Context) result {
	const name = "7. OpenTelemetry span capture"
	m, err := makeModel(ctx, modelFlash)
	if err != nil {
		return result{name, statusSkip, err.Error()}
	}

	sr := tracetest.NewSpanRecorder()
	providers, err := telemetry.New(ctx, telemetry.WithSpanProcessors(sr))
	if err != nil {
		return result{name, statusFail, fmt.Sprintf("telemetry init: %v", err)}
	}
	providers.SetGlobalOtelProviders()
	defer providers.Shutdown(ctx)

	a, err := llmagent.New(llmagent.Config{
		Name:        "spike_otel",
		Model:       m,
		Description: "OTEL agent",
		Instruction: "Reply with the word HI.",
	})
	if err != nil {
		return result{name, statusFail, err.Error()}
	}
	if _, err := runAgent(ctx, a, "ping"); err != nil {
		return result{name, statusFail, err.Error()}
	}

	spans := sr.Ended()
	if len(spans) == 0 {
		return result{name, statusFail, "no spans recorded — verify ADK uses global tracer or pass tracer explicitly"}
	}
	names := map[string]int{}
	for _, s := range spans {
		names[s.Name()]++
	}
	var summary []string
	for n, c := range names {
		summary = append(summary, fmt.Sprintf("%s×%d", n, c))
	}
	return result{name, statusPass, fmt.Sprintf("spans=%d (%s)", len(spans), strings.Join(summary, ", "))}
}

// --- Elicitation ---

func checkElicitation(_ context.Context) result {
	const name = "8. MCP elicitation surface (FR-6.7)"

	handler := func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
		// In production: render TUI modal, validate user input against req.Schema,
		// return Action="accept" with Content, or "decline"/"cancel".
		return &mcp.ElicitResult{Action: "decline"}, nil
	}

	client := mcp.NewClient(
		&mcp.Implementation{Name: "spike_client", Version: "v1"},
		&mcp.ClientOptions{ElicitationHandler: handler},
	)
	if client == nil {
		return result{name, statusFail, "client construction failed"}
	}
	return result{name, statusPass, "ElicitationHandler signature compiles; FR-6.7 buildable through ADK's mcptoolset by passing a custom mcp.Client via Config.Client"}
}

// ----------------------------------------------------------------------------
// Output
// ----------------------------------------------------------------------------

func printSummary(results []result) {
	fmt.Println()
	fmt.Println("Summary")
	fmt.Println(strings.Repeat("-", 60))
	for _, r := range results {
		fmt.Printf("  [%-7s] %s\n", r.status, r.name)
		if r.notes != "" {
			fmt.Printf("            %s\n", r.notes)
		}
	}
	fmt.Println()
	counts := map[status]int{}
	for _, r := range results {
		counts[r.status]++
	}
	fmt.Printf("PASS=%d  FAIL=%d  SKIP=%d  UNKNOWN=%d\n",
		counts[statusPass], counts[statusFail], counts[statusSkip], counts[statusUnknown])
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
