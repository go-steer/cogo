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

// requireE2E gates each test on COGO_E2E=1 and the Vertex env vars.
// Returns once it's safe to proceed; otherwise calls t.Skip.
func requireE2E(t *testing.T) {
	t.Helper()
	if os.Getenv("COGO_E2E") != "1" {
		t.Skip("set COGO_E2E=1 to run end-to-end tests against real Vertex")
	}
	if os.Getenv("GOOGLE_GENAI_USE_VERTEXAI") != "true" || os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		t.Skip("e2e requires GOOGLE_GENAI_USE_VERTEXAI=true and GOOGLE_CLOUD_PROJECT")
	}
}

// TestE2E_VertexPing exercises the full path against real Gemini via Vertex.
//
// Skipped unless COGO_E2E=1 is set, so the default `go test ./...` never
// hits the network. Requires Application Default Credentials and the
// dev Vertex env vars (see .env.example).
func TestE2E_VertexPing(t *testing.T) {
	requireE2E(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := config.DefaultConfig()
	cfg.Permissions.Mode = "yolo" // headless: no prompter; need non-ask mode

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

// TestE2E_VertexTools exercises a tool round-trip against real Gemini:
// the model is asked to read a file we wrote, and we assert the
// content shows up in its reply.
func TestE2E_VertexTools(t *testing.T) {
	requireE2E(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	dir := t.TempDir()
	target := dir + "/secret.txt"
	if err := os.WriteFile(target, []byte("THE_MAGIC_WORD_IS_GROBBLE"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Place .agents/ in dir so the path is in scope without prompting.
	if err := os.MkdirAll(dir+"/.agents", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.Permissions.Mode = "yolo"

	prompt := "Use the read_file tool to read " + target + " and tell me the file's contents verbatim."

	var stdout, stderr bytes.Buffer
	code, err := headless.RunFromConfig(ctx, cfg, prompt, &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunFromConfig: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if code != headless.ExitOK {
		t.Fatalf("exit code = %d, want %d", code, headless.ExitOK)
	}
	if !strings.Contains(stdout.String(), "GROBBLE") {
		t.Fatalf("expected magic word in output; got: %q (stderr=%q)", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "read_file") {
		t.Fatalf("expected stderr to surface the read_file tool call; got %q", stderr.String())
	}
}
