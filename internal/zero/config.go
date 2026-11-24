package zero

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Workspaces (docs/spec/workspaces.md): a workspace is one sandbox with its
// folders. The config holds names and folder paths only, never secrets.

type Workspace struct {
	Folders []string `yaml:"folders"`
	// Tools are the forge MCP tools of the workspace: "default" or tool
	// names. Empty means none (ADR 0043).
	Tools []string `yaml:"tools,omitempty"`
}

type Config struct {
	Current    string                `yaml:"current,omitempty"`
	Workspaces map[string]*Workspace `yaml:"workspaces"`
}

var wsNameRe = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)

func configPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "zero", "config.yaml")
}

func loadConfig() (*Config, error) {
	c := &Config{}
	b, err := os.ReadFile(configPath())
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(bytes.TrimSpace(b)) > 0 {
		dec := yaml.NewDecoder(bytes.NewReader(b))
		dec.KnownFields(true)
		if err := dec.Decode(c); err != nil {
			return nil, fmt.Errorf("%s: %w", configPath(), err)
		}
	}
	if c.Workspaces == nil {
		c.Workspaces = map[string]*Workspace{}
	}
	for name, w := range c.Workspaces {
		if !wsNameRe.MatchString(name) || w == nil || len(w.Folders) == 0 {
			return nil, fmt.Errorf("%s: workspace %q needs a name of [a-z0-9-]{1,40} and at least one folder", configPath(), name)
		}
		if _, err := mcpTools(w.Tools); err != nil {
			return nil, fmt.Errorf("%s: workspace %q: %w", configPath(), name, err)
		}
	}
	return c, nil
}

func (c *Config) save() error {
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// realPath returns p absolute with symlinks resolved.
func realPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// wsName turns a folder name into a workspace name.
func wsName(base string) string {
	n := strings.Trim(strings.TrimPrefix(SandboxName(base), "zero-"), "-")
	if len(n) > 40 {
		n = strings.Trim(n[:40], "-")
	}
	return n
}

// at lists the workspaces that have a folder containing dir.
func (c *Config) at(dir string) []string {
	var names []string
	for name, w := range c.Workspaces {
		for _, f := range w.Folders {
			if dir == f || strings.HasPrefix(dir, f+string(filepath.Separator)) {
				names = append(names, name)
				break
			}
		}
	}
	sort.Strings(names)
	return names
}

// resolve picks the workspace: -c, ZERO_CONTEXT, cwd, current. An empty name
// without error means there is none.
func (c *Config) resolve(flag, env, cwd string) (name, source string, err error) {
	for _, pick := range []struct{ name, source string }{{flag, "flag"}, {env, "env"}} {
		if pick.name == "" {
			continue
		}
		if c.Workspaces[pick.name] == nil {
			return "", "", fmt.Errorf("no workspace %q (zero ctx lists them)", pick.name)
		}
		return pick.name, pick.source, nil
	}
	switch here := c.at(cwd); {
	case len(here) == 1:
		return here[0], "cwd", nil
	case slices.Contains(here, c.Current):
		return c.Current, "current", nil
	case len(here) > 1:
		return "", "", fmt.Errorf("%s is in workspaces %s; use -c or zero ctx use", cwd, strings.Join(here, ", "))
	}
	if c.Workspaces[c.Current] != nil {
		return c.Current, "current", nil
	}
	return "", "", nil
}

// adopt registers a sandbox made before workspaces existed: state for the
// repo around cwd, but no config entry.
func (c *Config) adopt(top string) bool {
	name := wsName(filepath.Base(top))
	if top == "" || name == "" || c.Workspaces[name] != nil || len(c.at(top)) > 0 {
		return false
	}
	if _, err := os.Stat(filepath.Join(stateRoot(), "zero-"+name)); err != nil {
		return false
	}
	c.Workspaces[name] = &Workspace{Folders: []string{top}}
	return true
}

func gitTop() string {
	top, err := sh("git", "rev-parse", "--show-toplevel")
	if err != nil || top == "" {
		return ""
	}
	if p, err := realPath(top); err == nil {
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
	if p, err := realPath(cwd); err == nil {
		cwd = p
	}
	top := gitTop()
	changed := c.adopt(top)
	name, source, err := c.resolve(flag, os.Getenv("ZERO_CONTEXT"), cwd)
	if err != nil {
		return nil, err
	}
	// zero up in a repo that is in no workspace starts a new one; it does
	// not act on the current workspace somewhere else.
	if create && source == "current" && top != "" && len(c.at(top)) == 0 {
		name = ""
	}
	if name == "" {
		if !create {
			return nil, fmt.Errorf("no workspace here; run zero up in a git repo, or use -c NAME (zero ctx lists them)")
		}
		if top == "" {
			return nil, fmt.Errorf("not inside a git repository (run zero up from a repo, or zero ctx new NAME PATH)")
		}
		name = wsName(filepath.Base(top))
		if name == "" {
			return nil, fmt.Errorf("cannot derive a workspace name from %s; use zero ctx new NAME", top)
		}
		if c.Workspaces[name] != nil {
			return nil, fmt.Errorf("workspace %s exists with other folders; use zero ctx new NAME here", name)
		}
		c.Workspaces[name] = &Workspace{Folders: []string{top}}
		c.Current, source, changed = name, "new", true
	}
	if changed {
		if err := c.save(); err != nil {
			return nil, err
		}
	}
	fmt.Fprintf(os.Stderr, "workspace: %s (%s)\n", name, source)
	s := c.sandbox(name)
	seen := map[string]string{}
	for _, f := range s.Folders {
		if _, err := os.Stat(f); err != nil {
			return nil, fmt.Errorf("folder %s is missing; restore it, or take it out of the workspace: zero -c %s ctx remove %s", f, name, f)
		}
		if other, ok := seen[filepath.Base(f)]; ok {
			return nil, fmt.Errorf("folders %s and %s have the same name; a workspace needs distinct folder names: rename one or put it in its own workspace (zero ctx new NAME PATH)", other, f)
		}
		seen[filepath.Base(f)] = f
	}
	return s, nil
}

func (c *Config) sandbox(name string) *Sandbox {
	top := c.Workspaces[name].Folders[0]
	repo := filepath.Base(top)
	// The agent works in the first repo; a workspace of input folders only
	// has no checkout.
	dir := "/tmp/zero"
	for _, f := range c.Workspaces[name].Folders {
		if !isInput(f) {
			dir = sandboxDir(f)
			break
		}
	}
	tools, _ := mcpTools(c.Workspaces[name].Tools) // checked by loadConfig
	s := &Sandbox{
		Name:    "zero-" + name,
		Folders: c.Workspaces[name].Folders,
		Tools:   tools,
		Repo:    repo,
		Top:     top,
		Dir:     dir,
		State:   filepath.Join(stateRoot(), "zero-"+name),
		Host:    rt.HostAlias("zero-" + name),
	}
	s.setGateway(s.read("gateway"))
	return s
}

// sandboxDir is where a host folder lives inside the sandbox.
func sandboxDir(top string) string {
	if isInput(top) {
		return inputMount + "/" + filepath.Base(top)
	}
	return "/tmp/zero/" + filepath.Base(top)
}

// paths resolves the given folders, or cwd when none are given.
func paths(args []string) ([]string, error) {
	if len(args) == 0 {
		args = []string{"."}
	}
	var out []string
	for _, a := range args {
		p, err := realPath(a)
		if err != nil {
			return nil, err
		}
		if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
			return nil, fmt.Errorf("%s is not a folder", a)
		}
		out = append(out, p)
	}
	return out, nil
}

// listed resolves the folders to take out of a workspace. A path that is in
// the workspace as written wins over what it links to now; a folder that no
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
			if p, err := realPath(a); err == nil {
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
	if p, err := realPath(cwd); err == nil {
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
		here := c.at(cwd)
		for _, n := range names {
			mark := " "
			if n == c.Current {
				mark = "*"
			} else if slices.Contains(here, n) {
				mark = "+"
			}
			fmt.Fprintf(out, "%s %-20s %s\n", mark, n, strings.Join(c.Workspaces[n].Folders, " "))
		}
		fmt.Fprintln(os.Stderr, "legend: * current workspace, + workspace of this folder")
		return nil
	case "--prompt":
		if name, _, err := c.resolve(flag, os.Getenv("ZERO_CONTEXT"), cwd); err == nil {
			fmt.Fprintln(out, name)
		}
		return nil
	case "use":
		if len(args) != 1 || c.Workspaces[args[0]] == nil {
			return fmt.Errorf("usage: zero ctx use NAME (zero ctx lists them)")
		}
		c.Current = args[0]
	case "new":
		if len(args) == 0 || !wsNameRe.MatchString(args[0]) {
			return fmt.Errorf("usage: zero ctx new NAME [PATH...]; NAME is [a-z0-9-]{1,40}")
		}
		if c.Workspaces[args[0]] != nil {
			return fmt.Errorf("workspace %s exists", args[0])
		}
		ps, err := paths(args[1:])
		if err != nil {
			return err
		}
		c.Workspaces[args[0]] = &Workspace{Folders: ps}
		c.Current = args[0]
	case "add", "remove":
		name, _, err := c.resolve(flag, os.Getenv("ZERO_CONTEXT"), cwd)
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
				return fmt.Errorf("%s is not in workspace %s (folders: %s); name one of them: zero -c %s ctx remove PATH", p, name, strings.Join(w.Folders, " "), name)
			case sub == "remove":
				if !isInput(p) || !fileExists(p) {
					if s := c.sandbox(name); s.exists() {
						fmt.Fprintf(out, "%s: unpushed: %s -> kept in sandbox, not reachable via token\n", sandboxDir(p), s.unpushedIn(p))
					}
				}
				w.Folders = slices.Delete(w.Folders, i, i+1)
			}
		}
		if len(w.Folders) == 0 {
			return fmt.Errorf("a workspace needs at least one folder; to delete the workspace: zero ctx rm %s", name)
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
		s := c.sandbox(args[0])
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
	return c.save()
}

// rmLeftover removes what a workspace that is no longer in the config left
// behind (zero ls: leftover): sandbox, token, input copy, state.
func rmLeftover(out io.Writer, name string, yes bool) error {
	sb := "zero-" + name
	s := &Sandbox{Name: sb, State: filepath.Join(stateRoot(), sb), Host: rt.HostAlias(sb)}
	if !wsNameRe.MatchString(name) {
		return fmt.Errorf("no workspace %q (zero ls lists them)", name)
	}
	s.setGateway(s.read("gateway"))
	if !fileExists(s.State) && !s.exists() {
		s.setGateway(vmGateway)
	}
	if !fileExists(s.State) && !fileExists(s.stage()) && !s.exists() {
		return fmt.Errorf("no workspace %q and nothing left of one (zero ls lists them)", name)
	}
	return s.Rm(out, yes)
}
