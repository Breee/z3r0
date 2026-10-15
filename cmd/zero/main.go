// zero: per-repo OpenShell sandbox with `opencode serve` inside; the host
// opencode attaches over an SSH tunnel (F17).
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/Breee/z3r0/internal/zero"
)

const usageText = `zero: per-repo OpenShell sandbox with opencode serve inside; host opencode attaches.

Usage (inside a git repo): zero <command>
  up            create-or-reuse sandbox, clone repo in (first time only), start serve + forward
  attach        up + opencode TUI attached to the sandbox
  run <prompt>  up + one-shot opencode run against the sandbox
  code          up + VS Code Remote-SSH on the same sandbox (Copilot runs inside)
  status        show sandbox / forward / serve state
  stop          end the host port forward (sandbox keeps running)
  logs [-f]     serve.log from inside the sandbox
  rm            stop + delete sandbox (work not pushed is lost)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "-h", "--help", "help":
		fmt.Print(usageText)
		return
	}

	z, err := zero.New()
	if err != nil {
		fatal(err)
	}

	switch cmd {
	case "up":
		_, err = z.Up(os.Stdout)
	case "attach":
		err = z.Attach(os.Stdout)
	case "run":
		err = z.Run(os.Stdout, args)
	case "code":
		err = z.Code(os.Stdout)
	case "status":
		err = z.Status(os.Stdout)
	case "stop":
		err = z.Stop(os.Stdout)
	case "logs":
		follow := len(args) > 0 && (args[0] == "-f" || args[0] == "--follow")
		err = z.Logs(os.Stdout, follow)
	case "rm":
		err = z.Rm(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "zero: unknown command %q\n\n", cmd)
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(2)
	}
	if err != nil {
		// Propagate the exit code of interactive children (attach/run/code).
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.ExitCode())
		}
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "zero:", err)
	os.Exit(1)
}
