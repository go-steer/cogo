package initcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-steer/cogo/internal/config"
)

func TestRunCLI_SilentDefaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stdout, err := os.CreateTemp(dir, "out")
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	stderr, err := os.CreateTemp(dir, "err")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()

	code := RunCLI([]string{"--dir", dir}, stdout, stderr)
	if code != 0 {
		body, _ := os.ReadFile(stderr.Name())
		t.Fatalf("RunCLI returned %d (stderr=%s)", code, body)
	}
	// .agents/ should now exist with config.json.
	if _, err := os.Stat(filepath.Join(dir, AgentsDirName, config.ConfigFileName)); err != nil {
		t.Fatalf("config not written: %v", err)
	}
}

func TestRunCLI_RefusesReinit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stdout, stderr := &captureFile{name: "stdout"}, &captureFile{name: "stderr"}
	if code := RunCLI([]string{"--dir", dir}, stdout.f(t), stderr.f(t)); code != 0 {
		t.Fatalf("first init returned %d", code)
	}
	// Second init without --force should fail with exit 1.
	stderr2, err := os.CreateTemp(dir, "err2")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr2.Close()
	if code := RunCLI([]string{"--dir", dir}, stdout.f(t), stderr2); code != 1 {
		t.Errorf("second init returned %d, want 1", code)
	}
	body, _ := os.ReadFile(stderr2.Name())
	if !strings.Contains(string(body), "already exists") {
		t.Errorf("stderr missing already-exists hint: %s", body)
	}
}

func TestRunCLI_ForceOverwrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stdout, stderr := &captureFile{name: "stdout"}, &captureFile{name: "stderr"}
	if code := RunCLI([]string{"--dir", dir}, stdout.f(t), stderr.f(t)); code != 0 {
		t.Fatalf("first init returned %d", code)
	}
	if code := RunCLI([]string{"--dir", dir, "--force"}, stdout.f(t), stderr.f(t)); code != 0 {
		t.Fatalf("--force init returned %d", code)
	}
}

func TestRunCLI_BadFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stderr, _ := os.CreateTemp(dir, "err")
	defer stderr.Close()
	stdout, _ := os.CreateTemp(dir, "out")
	defer stdout.Close()
	if code := RunCLI([]string{"--nope"}, stdout, stderr); code != 2 {
		t.Errorf("bad flag returned %d, want 2 (usage error)", code)
	}
}

// captureFile is a tiny helper that lets the same test re-allocate
// its temp file lazily without the caller juggling defers.
type captureFile struct {
	name string
	buf  *bytes.Buffer
}

func (c *captureFile) f(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), c.name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}
