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

const usageText = `zero: an opencode agent in a sandbox per workspace; it pushes only to its own branch.

Usage: zero [-c WORKSPACE] COMMAND
Work in the workspace
  up [--approve] [--vm] [--max-input MiB]  create or continue the sandbox
  opencode      up, then opencode in this terminal
  web           up, then opencode in the browser
  vscode        up, then VS Code on the sandbox
  run PROMPT    up, then one prompt
  status        sandbox, token, policy and forward
  logs [-f]     opencode log of the sandbox
  stop          end the port forward; the sandbox keeps running
  pause         stop the sandbox and revoke its token; files stay, zero up continues
  rm [-y]       delete the sandbox, after listing work not pushed; the workspace is kept
Workspaces
  ls            all workspaces with status, token, policy; leftovers
  switch [NAME] make NAME the current workspace; without NAME pick from the list
  ctx           list workspaces with folders; ctx new NAME [PATH...] | add [PATH...] |
                remove [PATH...] | rm [-y] NAME | use NAME | --prompt
This machine
  install       (sudo) set up the host
  doctor        check the host and, inside a repository, its forge
  version       version of this build
  help          this text
  completion bash  bash completion: source <(zero completion bash)
Examples
  cd ~/repos/app && zero opencode    zero -c app status      zero up --approve
  zero ctx new both ~/repos/api ~/repos/web      zero switch both
The workspace is -c, else ZERO_CONTEXT, else the one of the current folder, else the current one.
Exit codes: 0 done, 1 failed, 2 wrong command or option (nothing ran).
`

const bashCompletion = `_zero() {
  local cur=${COMP_WORDS[COMP_CWORD]}
  case ${COMP_WORDS[COMP_CWORD-1]} in
    switch|use|-c|--context)
      COMPREPLY=($(compgen -W "$(zero ctx 2>/dev/null | cut -c3- | cut -d' ' -f1)" -- "$cur")) ;;
    ctx)
      COMPREPLY=($(compgen -W "use new add remove rm" -- "$cur")) ;;
    zero)
      COMPREPLY=($(compgen -W "up opencode vscode web run status stop logs pause rm ls switch ctx install doctor version completion" -- "$cur")) ;;
  esac
}
complete -F _zero zero
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(2)
	}
	ctx, rest, err := contextFlag(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if len(rest) == 0 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(2)
	}
	cmd, args := rest[0], rest[1:]
	switch cmd {
	case "-h", "--help":
		cmd = "help"
	case "--version":
		cmd = "version"
	case "attach": // name before #148
		cmd = "opencode"
	case "code":
		cmd = "vscode"
	}
	if _, ok := argSpecs[cmd]; !ok && cmd != "run" {
		fmt.Fprintf(os.Stderr, "zero: unknown command %q\n\n", cmd)
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(2)
	}
	if err := checkArgs(cmd, args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	switch cmd {
	case "help":
		fmt.Print(usageText)
		return
	case "version":
		fmt.Println("zero", zero.Version())
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
	case "switch":
		if err := zero.Switch(os.Stdin, os.Stdout, args); err != nil {
			fatal(err)
		}
		return
	case "completion":
		if args[0] != "bash" {
			fmt.Fprintln(os.Stderr, "usage: zero completion bash")
			os.Exit(2)
		}
		fmt.Print(bashCompletion)
		return
	case "ctx":
		if err := zero.Ctx(os.Stdout, ctx, args); err != nil {
			fatal(err)
		}
		return
	}

	create := slices.Contains([]string{"up", "opencode", "run", "vscode", "web"}, cmd)
	z, err := zero.Open(ctx, create)
	if err != nil {
		fatal(err)
	}

	switch cmd {
	case "up":
		z.Approve = slices.Contains(args, "--approve")
		if i := slices.Index(args, "--max-input"); i >= 0 {
			mib, _ := strconv.Atoi(args[i+1])
			z.MaxInput = int64(mib) << 20
		}
		if slices.Contains(args, "--vm") || os.Getenv("Z3R0_DRIVER") == "vm" {
			if err = z.UseVM(); err != nil {
				break
			}
		}
		if _, err = z.Up(os.Stdout); err == nil {
			fmt.Println("next: zero opencode")
		}
	case "opencode":
		err = z.Attach(os.Stdout)
	case "run":
		err = z.Run(os.Stdout, args)
	case "vscode":
		err = z.Code(os.Stdout)
	case "web":
		err = z.Web(os.Stdout)
	case "status":
		err = z.Status(os.Stdout)
	case "stop":
		err = z.Stop(os.Stdout)
	case "logs":
		err = z.Logs(os.Stdout, len(args) > 0)
	case "pause":
		err = z.Pause(os.Stdout)
	case "rm":
		if err = z.Rm(os.Stdout, slices.Contains(args, "-y")); err == nil {
			ws := strings.TrimPrefix(z.Name, "zero-")
			fmt.Printf("workspace %s is kept; to delete it: zero ctx rm %s\n", ws, ws)
		}
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
func contextFlag(in []string) (ctx string, rest []string, err error) {
	named := func(v string) error {
		if v == "" || strings.HasPrefix(v, "-") {
			return usageError(fmt.Sprintf("zero: -c needs a workspace name, got %q", v))
		}
		ctx = v
		return nil
	}
	for i := 0; i < len(in); i++ {
		a := in[i]
		switch {
		case len(rest) > 0 && rest[0] == "run":
			rest = append(rest, a)
		case a == "-c" || a == "--context":
			i++
			if i == len(in) {
				return "", nil, usageError("zero: -c needs a workspace name")
			}
			if err := named(in[i]); err != nil {
				return "", nil, err
			}
		case strings.HasPrefix(a, "--context="):
			if err := named(strings.TrimPrefix(a, "--context=")); err != nil {
				return "", nil, err
			}
		default:
			rest = append(rest, a)
		}
	}
	return ctx, rest, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "zero:", err)
	os.Exit(1)
}
