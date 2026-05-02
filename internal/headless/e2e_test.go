package headless_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/headless"

	// Side-effect import to register the Gemini provider with the resolver.
	_ "github.com/go-steer/cogo/internal/models/gemini"
)

// TestE2E_VertexPing exercises the full path against real Gemini via Vertex.
//
// Skipped unless COGO_E2E=1 is set, so the default `go test ./...` never
// hits the network. Requires Application Default Credentials and the
// dev Vertex env vars (see .env.example).
func TestE2E_VertexPing(t *testing.T) {
	if os.Getenv("COGO_E2E") != "1" {
		t.Skip("set COGO_E2E=1 to run end-to-end tests against real Vertex")
	}
	if os.Getenv("GOOGLE_GENAI_USE_VERTEXAI") != "true" || os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		t.Skip("e2e requires GOOGLE_GENAI_USE_VERTEXAI=true and GOOGLE_CLOUD_PROJECT")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := config.DefaultConfig()
	// Auto-detect handles vertex routing via env vars; nothing to override.

	var stdout, stderr bytes.Buffer
	code, err := headless.RunFromConfig(ctx, cfg, "Reply with exactly the word PONG and nothing else.", &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunFromConfig: %v (stderr=%s)", err, stderr.String())
	}
	if code != headless.ExitOK {
		t.Fatalf("exit code = %d, want %d", code, headless.ExitOK)
	}
	got := strings.TrimSpace(strings.ToUpper(stdout.String()))
	if !strings.Contains(got, "PONG") {
		t.Fatalf("expected response to contain PONG, got %q", got)
	}
}
