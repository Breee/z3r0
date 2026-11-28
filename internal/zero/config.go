package zero

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/Breee/z3r0/internal/core"
	"github.com/Breee/z3r0/internal/sandbox/openshell"
	"github.com/Breee/z3r0/internal/workspace"
)

// Workspaces (docs/spec/workspaces.md): a workspace is one sandbox with its
// directories. The config holds names and directory paths only, never secrets.

// loadConfig reads the config and checks what only zero's parts can check.
func loadConfig() (*workspace.Config, error) {
	c, err := workspace.Load()
	if err != nil {
		return nil, err
	}
	for name, w := range c.Workspaces {
		for _, f := range w.Folders {
			if filepath.Base(f) == guidanceDir {
				return nil, fmt.Errorf("%s: workspace %q: a directory cannot be named %s", workspace.Path(), name, guidanceDir)
			}
		}
		if _, err := mcpTools(w.Tools); err != nil {
			return nil, fmt.Errorf("%s: workspace %q: %w", workspace.Path(), name, err)
		}
		if _, _, err := guidance(w); err != nil {
			return nil, fmt.Errorf("%s: workspace %q: %w", workspace.Path(), name, err)
		}
	}
	return c, nil
}

func gitTop() string {
	top, err := core.Sh("git", "rev-parse", "--show-toplevel")
	if err != nil || top == "" {
		return ""
	}
	if p, err := workspace.RealPath(top); err == nil {
		return p
	}
	return top
}

// Open returns the sandbox of the selected workspace and prints which one it
// is. With create, a missing workspace is made from the git repo around cwd.
func Open(flag string, create bool) (*Sandbox, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if p, err := workspace.RealPath(cwd); err == nil {
		cwd = p
	}
	top := gitTop()
	changed := c.Adopt(top)
	name, source, err := c.Resolve(flag, os.Getenv("ZERO_CONTEXT"), cwd)
	if err != nil {
		return nil, err
	}
	// zero up in a repo that is in no workspace starts a new one; it does
	// not act on the current workspace somewhere else.
	if create && source == "current" && top != "" && len(c.At(top)) == 0 {
		name = ""
	}
	if name == "" {
		if !create {
			return nil, fmt.Errorf("no workspace here; run zero up in a git repo, or use -c NAME (zero ctx lists them)")
		}
		if top == "" {
			return nil, fmt.Errorf("not inside a git repository (run zero up from a repo, or zero ctx new NAME PATH)")
		}
		name = workspace.Name(filepath.Base(top))
		if name == "" {
			return nil, fmt.Errorf("cannot derive a workspace name from %s; use zero ctx new NAME", top)
		}
		if c.Workspaces[name] != nil {
			return nil, fmt.Errorf("workspace %s exists with other directories; use zero ctx new NAME here", name)
		}
		c.Workspaces[name] = &workspace.Workspace{Folders: []string{top}}
		c.Current, source, changed = name, "new", true
	}
	if changed {
		if err := c.Save(); err != nil {
			return nil, err
		}
	}
	fmt.Fprintf(os.Stderr, "workspace: %s (%s)\n", name, source)
	s := sandboxOf(c, name)
	seen := map[string]string{}
	for _, f := range s.Folders {
		if _, err := os.Stat(f); err != nil {
			return nil, fmt.Errorf("directory %s is missing; restore it, or take it out of the workspace: zero -c %s ctx remove %s", f, name, f)
		}
		if other, ok := seen[filepath.Base(f)]; ok {
			return nil, fmt.Errorf("directories %s and %s have the same name; a workspace needs distinct directory names: rename one or put it in its own workspace (zero ctx new NAME PATH)", other, f)
		}
		seen[filepath.Base(f)] = f
	}
	return s, nil
}

func sandboxOf(c *workspace.Config, name string) *Sandbox {
	top := c.Workspaces[name].Folders[0]
	repo := filepath.Base(top)
	// The agent works in the first repo; a workspace of input directories only
	// has no checkout.
	dir := "/tmp/zero"
	for _, f := range c.Workspaces[name].Folders {
		if !isInput(f) {
			dir = sandboxDir(f)
			break
		}
	}
	tools, _ := mcpTools(c.Workspaces[name].Tools) // checked by loadConfig
	instr, skills, _ := guidance(c.Workspaces[name])
	s := &Sandbox{
		Name:         "zero-" + name,
		Folders:      c.Workspaces[name].Folders,
		Tools:        tools,
		Instructions: instr,
		Skills:       skills,
		Models:       c.Workspaces[name].Models,
		Model:        c.Workspaces[name].Model,
		Providers:    c.Providers,
		ProviderKeys: c.ProviderKeys,
		Repo:         repo,
		Top:          top,
		Dir:          dir,
		State:        filepath.Join(core.StateRoot(), "zero-"+name),
		Host:         rt.HostAlias("zero-" + name),
	}
	s.setGateway(s.read("gateway"))
	return s
}

// sandboxDir is where a host directory lives inside the sandbox.
func sandboxDir(top string) string {
	if isInput(top) {
		return inputMount + "/" + filepath.Base(top)
	}
	return "/tmp/zero/" + filepath.Base(top)
}

// paths resolves the given directories, or cwd when none are given.
func paths(args []string) ([]string, error) {
	if len(args) == 0 {
		args = []string{"."}
	}
	var out []string
	for _, a := range args {
		p, err := workspace.RealPath(a)
		if err != nil {
			return nil, err
		}
		if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", a)
		}
		out = append(out, p)
	}
	return out, nil
}

// listed resolves the directories to take out of a workspace. A path that is in
// the workspace as written wins over what it links to now; a directory that no
// longer exists is matched the same way.
func listed(folders, args []string) ([]string, error) {
	if len(args) == 0 {
		args = []string{"."}
	}
	var out []string
	for _, a := range args {
		abs, err := filepath.Abs(a)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(folders, abs) {
			if p, err := workspace.RealPath(a); err == nil {
				abs = p
			}
		}
		out = append(out, abs)
	}
	return out, nil
}

// Ctx implements `zero ctx ...`.
func Ctx(out io.Writer, flag string, args []string) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	if p, err := workspace.RealPath(cwd); err == nil {
		cwd = p
	}
	sub := ""
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	switch sub {
	case "", "list":
		names := make([]string, 0, len(c.Workspaces))
		for n := range c.Workspaces {
			names = append(names, n)
		}
		sort.Strings(names)
		here := c.At(cwd)
		for _, n := range names {
			mark := " "
			if n == c.Current {
				mark = "*"
			} else if slices.Contains(here, n) {
				mark = "+"
			}
			fmt.Fprintf(out, "%s %-20s %s\n", mark, n, strings.Join(c.Workspaces[n].Folders, " "))
		}
		fmt.Fprintln(os.Stderr, "legend: * current workspace, + workspace of this directory")
		return nil
	case "--prompt":
		if name, _, err := c.Resolve(flag, os.Getenv("ZERO_CONTEXT"), cwd); err == nil {
			fmt.Fprintln(out, name)
		}
		return nil
	case "use":
		if len(args) != 1 || c.Workspaces[args[0]] == nil {
			return fmt.Errorf("usage: zero ctx use NAME (zero ctx lists them)")
		}
		c.Current = args[0]
	case "new":
		if len(args) == 0 || !workspace.NameRe.MatchString(args[0]) {
			return fmt.Errorf("usage: zero ctx new NAME [PATH...]; NAME is [a-z0-9-]{1,40}")
		}
		if c.Workspaces[args[0]] != nil {
			return fmt.Errorf("workspace %s exists", args[0])
		}
		ps, err := paths(args[1:])
		if err != nil {
			return err
		}
		c.Workspaces[args[0]] = &workspace.Workspace{Folders: ps}
		c.Current = args[0]
	case "add", "remove":
		name, _, err := c.Resolve(flag, os.Getenv("ZERO_CONTEXT"), cwd)
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("no workspace selected; name one: zero -c NAME ctx %s [PATH...] (zero ls lists them)", sub)
		}
		w := c.Workspaces[name]
		ps, err := paths(args)
		if sub == "remove" {
			ps, err = listed(w.Folders, args)
		}
		if err != nil {
			return err
		}
		for _, p := range ps {
			i := slices.Index(w.Folders, p)
			switch {
			case sub == "add" && i < 0:
				w.Folders = append(w.Folders, p)
			case sub == "remove" && i < 0:
				return fmt.Errorf("%s is not in workspace %s (directories: %s); name one of them: zero -c %s ctx remove PATH", p, name, strings.Join(w.Folders, " "), name)
			case sub == "remove":
				if !isInput(p) || !core.FileExists(p) {
					if s := sandboxOf(c, name); s.exists() {
						fmt.Fprintf(out, "%s: unpushed: %s -> kept in sandbox, not reachable via token\n", sandboxDir(p), s.unpushedIn(p))
					}
				}
				w.Folders = slices.Delete(w.Folders, i, i+1)
			}
		}
		if len(w.Folders) == 0 {
			return fmt.Errorf("a workspace needs at least one directory; to delete the workspace: zero ctx rm %s", name)
		}
		fmt.Fprintf(out, "%s: %s\n", name, strings.Join(w.Folders, " "))
	case "rm":
		yes := slices.Contains(args, "-y")
		args = slices.DeleteFunc(args, func(a string) bool { return a == "-y" })
		if len(args) != 1 {
			return fmt.Errorf("usage: zero ctx rm [-y] NAME (zero ctx lists them)")
		}
		if c.Workspaces[args[0]] == nil {
			return rmLeftover(out, args[0], yes)
		}
		s := sandboxOf(c, args[0])
		if err := s.Rm(out, yes); err != nil {
			return err
		}
		delete(c.Workspaces, args[0])
		fmt.Fprintf(out, "deleted workspace %s\n", args[0])
		if c.Current == args[0] {
			c.Current = ""
		}
	default:
		return fmt.Errorf("usage: zero ctx [use NAME | new NAME [PATH...] | add [PATH...] | remove [PATH...] | rm [-y] NAME | --prompt]")
	}
	return c.Save()
}

// rmLeftover removes what a workspace that is no longer in the config left
// behind (zero ls: leftover): sandbox, token, input copy, state.
func rmLeftover(out io.Writer, name string, yes bool) error {
	sb := "zero-" + name
	s := &Sandbox{Name: sb, State: filepath.Join(core.StateRoot(), sb), Host: rt.HostAlias(sb)}
	if !workspace.NameRe.MatchString(name) {
		return fmt.Errorf("no workspace %q (zero ls lists them)", name)
	}
	s.setGateway(s.read("gateway"))
	if !core.FileExists(s.State) && !s.exists() {
		s.setGateway(openshell.VMGateway)
	}
	if !core.FileExists(s.State) && !core.FileExists(s.stage()) && !s.exists() {
		return fmt.Errorf("no workspace %q and nothing left of one (zero ls lists them)", name)
	}
	return s.Rm(out, yes)
}
