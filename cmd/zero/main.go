// zero: per-repo OpenShell sandbox with `opencode serve` inside; the host
// opencode attaches over an SSH tunnel (F17).
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/Breee/z3r0/internal/zero"
)

const usageText = `zero: per-repo OpenShell sandbox with opencode serve inside; host opencode attaches.

Usage: zero [-c WORKSPACE] <command>
  up            create-or-reuse sandbox, clone repo in (first time only), start serve + forward
  up --vm       same, new sandbox as MicroVM (gateway openshell-vm; or Z3R0_DRIVER=vm)
  up --approve  accept a wider .zero/policy.yaml without the y/N question
  up --max-input MiB  size limit for input folders (default 100)
  attach        up + opencode TUI attached to the sandbox
  run <prompt>  up + one-shot opencode run against the sandbox
  code          up + VS Code Remote-SSH on the same sandbox (Copilot runs inside)
  web           up + open the opencode web UI in the browser
  status        show sandbox / forward / serve state
  stop          end the host port forward (sandbox keeps running)
  logs [-f]     opencode service log from inside the sandbox
  pause         stop the sandbox and revoke its token; files stay, zero up continues
  rm [-y]       stop + delete sandbox; lists work not pushed, asks for the workspace name
  ls            all workspaces with status, token, policy; leftovers without a workspace
  ctx           list workspaces (* current, + here); ctx use NAME | new NAME [PATH...] |
                add [PATH...] | remove [PATH...] | rm [-y] NAME | --prompt
  install       (sudo) gateway + rootless podman under user openshell, pinned OpenShell
  doctor        check host setup and, inside a repo, forge guardrails

The workspace is picked by -c, then ZERO_CONTEXT, then the current folder, then
zero ctx use. zero up in a git repo without a workspace creates one.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(2)
	}
	ctx, rest := contextFlag(os.Args[1:])
	if len(rest) == 0 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(2)
	}
	cmd, args := rest[0], rest[1:]
	switch cmd {
	case "-h", "--help", "help":
		fmt.Print(usageText)
		return
	case "install":
		if err := zero.Install(); err != nil {
			fatal(err)
		}
		return
	case "doctor":
		if err := zero.Doctor(os.Stdout); err != nil {
			fatal(err)
		}
		return
	case "ls":
		if err := zero.Ls(os.Stdout); err != nil {
			fatal(err)
		}
		return
	case "ctx":
		if err := zero.Ctx(os.Stdout, ctx, args); err != nil {
			fatal(err)
		}
		return
	}

	if !slices.Contains([]string{"up", "attach", "run", "code", "web", "status", "stop", "logs", "pause", "rm"}, cmd) {
		fmt.Fprintf(os.Stderr, "zero: unknown command %q\n\n", cmd)
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(2)
	}
	create := slices.Contains([]string{"up", "attach", "run", "code", "web"}, cmd)
	z, err := zero.Open(ctx, create)
	if err != nil {
		fatal(err)
	}

	switch cmd {
	case "up":
		z.Approve = slices.Contains(args, "--approve")
		if i := slices.Index(args, "--max-input"); i >= 0 {
			mib := 0
			if i+1 < len(args) {
				mib, _ = strconv.Atoi(args[i+1])
			}
			if mib <= 0 {
				fatal(fmt.Errorf("--max-input needs a size in MiB"))
			}
			z.MaxInput = int64(mib) << 20
		}
		if slices.Contains(args, "--vm") || os.Getenv("Z3R0_DRIVER") == "vm" {
			if err = z.UseVM(); err != nil {
				break
			}
		}
		_, err = z.Up(os.Stdout)
	case "attach":
		err = z.Attach(os.Stdout)
	case "run":
		err = z.Run(os.Stdout, args)
	case "code":
		err = z.Code(os.Stdout)
	case "web":
		err = z.Web(os.Stdout)
	case "status":
		err = z.Status(os.Stdout)
	case "stop":
		err = z.Stop(os.Stdout)
	case "logs":
		follow := len(args) > 0 && (args[0] == "-f" || args[0] == "--follow")
		err = z.Logs(os.Stdout, follow)
	case "pause":
		err = z.Pause(os.Stdout)
	case "rm":
		err = z.Rm(os.Stdout, slices.Contains(args, "-y"))
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

// contextFlag takes -c/--context NAME out of the arguments. Arguments after
// `run` belong to the prompt and stay untouched.
func contextFlag(in []string) (ctx string, rest []string) {
	for i := 0; i < len(in); i++ {
		a := in[i]
		switch {
		case len(rest) > 0 && rest[0] == "run":
			rest = append(rest, a)
		case (a == "-c" || a == "--context") && i+1 < len(in):
			i++
			ctx = in[i]
		case strings.HasPrefix(a, "--context="):
			ctx = strings.TrimPrefix(a, "--context=")
		default:
			rest = append(rest, a)
		}
	}
	return ctx, rest
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "zero:", err)
	os.Exit(1)
}
