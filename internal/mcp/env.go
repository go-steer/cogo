package mcp

import "os"

// osEnviron is a thin indirection over os.Environ used by lifecycle.go
// so tests can stub the parent environment if they ever need to. Today
// it just delegates.
func osEnviron() []string { return os.Environ() }
