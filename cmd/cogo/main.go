// Command cogo is the entry point for the Cogo agentic CLI.
//
// Two modes:
//   - Headless (`cogo -p "prompt"`): run one agent turn, stream the
//     reply to stdout, exit.
//   - Interactive TUI (`cogo` on a TTY): launch the Bubble Tea chat.
//
// When invoked with no -p and no TTY (piped input or CI), prints a hint
// pointing at -p and exits non-zero so callers don't hang waiting for
// a TUI that can't run.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"

	"github.com/go-steer/cogo/internal/config"
	"github.com/go-steer/cogo/internal/headless"
	"github.com/go-steer/cogo/internal/tui"

	// Register the Gemini provider with models.Resolve.
	_ "github.com/go-steer/cogo/internal/models/gemini"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable main entry. Keeping it separate from main() lets
// future tests drive flag parsing without forking a subprocess.
func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("cogo", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		prompt string
		debug  bool
		help   bool
	)
	fs.StringVar(&prompt, "p", "", "Shorthand for -prompt.")
	fs.StringVar(&prompt, "prompt", "", "Run a single prompt non-interactively and stream the reply to stdout, then exit.")
	fs.BoolVar(&debug, "debug", false, "Enable verbose logging to stderr.")
	fs.BoolVar(&help, "h", false, "Show help and exit.")
	fs.BoolVar(&help, "help", false, "Show help and exit.")
	fs.Usage = func() { printUsage(stderr) }

	if err := fs.Parse(args); err != nil {
		// flag.ContinueOnError prints its own message to fs.Output() (stderr).
		return headless.ExitConfigError
	}
	if help {
		printUsage(stdout)
		return headless.ExitOK
	}

	setupLogging(debug, stderr)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "cogo: get cwd: %v\n", err)
		return headless.ExitConfigError
	}
	cfg, agentsDir, err := config.LoadOrDefault(cwd)
	if err != nil {
		fmt.Fprintf(stderr, "cogo: %v\n", err)
		return headless.ExitConfigError
	}
	if agentsDir != "" {
		slog.Debug("loaded project config", "agentsDir", agentsDir)
	} else {
		slog.Debug("no .agents/ found; using built-in defaults")
	}

	if prompt != "" {
		code, err := headless.RunFromConfig(ctx, cfg, prompt, stdout, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "cogo: %v\n", err)
		}
		return code
	}

	// No -p supplied. Launch interactive TUI when attached to a terminal.
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintln(stderr, "cogo: interactive TUI requires a terminal. Use -p \"your prompt\" for headless mode.")
		return headless.ExitConfigError
	}
	code, err := tui.Run(ctx, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "cogo: %v\n", err)
	}
	return code
}

func setupLogging(debug bool, w *os.File) {
	level := slog.LevelWarn
	if debug {
		level = slog.LevelDebug
	}
	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, `Cogo — a terminal-native agentic CLI for Go developers.

Usage:
  cogo                 Open the interactive TUI (requires a terminal).
  cogo -p "<prompt>"   Run a single prompt non-interactively and exit.
  cogo -h              Show this help.

Flags:
  -p, -prompt <text>   Run a single prompt non-interactively and stream the
                       assistant reply to stdout, then exit.
  -debug               Enable verbose logging to stderr.
  -h, -help            Show this help.

Authentication:
  Set GOOGLE_API_KEY for the public Gemini API, or
  GOOGLE_GENAI_USE_VERTEXAI=true with GOOGLE_CLOUD_PROJECT (and Application
  Default Credentials) for Vertex AI.

See docs/REQUIREMENTS.md, docs/DESIGN.md, and docs/SLICES.md for the spec.`)
}
