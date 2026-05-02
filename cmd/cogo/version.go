package main

import "fmt"

// Build-time metadata. Populated via -ldflags by goreleaser:
//
//	-X main.version={{.Version}}
//	-X main.commit={{.Commit}}
//	-X main.date={{.Date}}
//
// Defaults below apply when building with plain `go build` (e.g. local
// dev) so `cogo --version` always returns something useful.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// versionString renders the build identity for the --version flag.
//
// Format: "cogo <semver> (commit <8-char-sha>, built <RFC3339-date>)".
// Stable enough that scripts can grep the leading word, the second
// token is always the version.
func versionString() string {
	short := commit
	if len(short) > 8 {
		short = short[:8]
	}
	return fmt.Sprintf("cogo %s (commit %s, built %s)", version, short, date)
}
