package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_MissingIsEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Servers) != 0 {
		t.Errorf("expected no servers, got %+v", got)
	}
}

func TestLoad_StdioParse(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := `{"version":1,"servers":{"fs":{"transport":"stdio","command":"mcp-fs","args":["--root","/tmp"],"env":{"X":"y"}}}}`
	if err := os.WriteFile(filepath.Join(dir, MCPFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := got.Servers["fs"]
	if !ok {
		t.Fatalf("missing fs server: %+v", got)
	}
	if spec.Transport != "stdio" || spec.Command != "mcp-fs" {
		t.Errorf("wrong fields: %+v", spec)
	}
	if len(spec.Args) != 2 || spec.Env["X"] != "y" {
		t.Errorf("args/env not parsed: %+v", spec)
	}
}

func TestLoad_RejectsBadTransport(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := `{"version":1,"servers":{"x":{"transport":"smoke-signals"}}}`
	if err := os.WriteFile(filepath.Join(dir, MCPFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), "unknown transport") {
		t.Fatalf("expected unknown-transport error, got %v", err)
	}
}

func TestLoad_RejectsStdioWithURL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := `{"version":1,"servers":{"x":{"transport":"stdio","command":"a","url":"https://b"}}}`
	if err := os.WriteFile(filepath.Join(dir, MCPFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), "must not set url") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestInterpolateEnv(t *testing.T) {
	t.Setenv("FOO", "bar")
	if got := InterpolateEnv("Bearer ${env:FOO}"); got != "Bearer bar" {
		t.Errorf("got %q", got)
	}
	if got := InterpolateEnv("${env:NOT_SET}"); got != "" {
		t.Errorf("unset env should be empty: %q", got)
	}
	if got := InterpolateEnv("plain text"); got != "plain text" {
		t.Errorf("non-template should pass through: %q", got)
	}
}

func TestInterpolateMap(t *testing.T) {
	t.Setenv("TOKEN", "secret")
	got := InterpolateMap(map[string]string{
		"Authorization": "Bearer ${env:TOKEN}",
		"X-Static":      "value",
	})
	if got["Authorization"] != "Bearer secret" || got["X-Static"] != "value" {
		t.Errorf("map interpolation wrong: %+v", got)
	}
}
