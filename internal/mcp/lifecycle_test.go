package mcp

import (
	"os/exec"
	"testing"
	"time"
)

func TestServer_Close_NilSafe(t *testing.T) {
	t.Parallel()
	// Nil receiver, no cmd, no process — none of these should panic.
	(*Server)(nil).Close()
	(&Server{}).Close()
	(&Server{cmd: exec.Command("/bin/true")}).Close() // not Started
}

func TestServer_Close_ReapsStartedProcess(t *testing.T) {
	t.Parallel()
	// Spawn a child that sleeps long enough to verify the SIGTERM
	// path actually drives the wait — without Close, the test would
	// hang on the deferred cmd.Wait at the end (or leak the process).
	cmd := exec.Command("/bin/sh", "-c", "sleep 60")
	if err := cmd.Start(); err != nil {
		t.Skipf("can't spawn child: %v", err)
	}
	srv := &Server{cmd: cmd}

	done := make(chan struct{})
	go func() {
		srv.Close()
		close(done)
	}()
	select {
	case <-done:
		// good
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill() // safety
		t.Fatal("Server.Close did not return within 5s")
	}
}
