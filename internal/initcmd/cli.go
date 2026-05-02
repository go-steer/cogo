package initcmd

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// RunCLI implements the `cogo init` subcommand. argv excludes the
// "init" token (so RunCLI sees only the post-subcommand args).
//
// Returns an exit code suitable for os.Exit. 0 = success, 1 = config
// or wizard error, 2 = usage error.
func RunCLI(argv []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cogo init", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		interactive bool
		force       bool
		dir         string
	)
	fs.BoolVar(&interactive, "interactive", false, "Walk through a wizard for provider/model/mode.")
	fs.BoolVar(&interactive, "i", false, "Shorthand for -interactive.")
	fs.BoolVar(&force, "force", false, "Overwrite an existing .agents/config.json.")
	fs.StringVar(&dir, "dir", "", "Target directory (defaults to the current directory).")

	if err := fs.Parse(argv); err != nil {
		return 2
	}
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "cogo init: %v\n", err)
			return 1
		}
		dir = cwd
	}

	opts := Options{Force: force}

	if interactive {
		res, err := RunWizard()
		if err != nil {
			fmt.Fprintf(stderr, "cogo init: %v\n", err)
			return 1
		}
		if res.Cancelled {
			fmt.Fprintln(stderr, "cogo init: cancelled.")
			return 0
		}
		opts.Cfg = res.Cfg
	}

	if err := WriteAgentsDir(dir, opts); err != nil {
		fmt.Fprintf(stderr, "cogo init: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Initialized %s/.agents/\n", dir)
	return 0
}
