package workspace

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Breee/z3r0/internal/core"

	"go.yaml.in/yaml/v3"
)

// Workspaces (docs/spec/workspaces.md): a workspace is one sandbox with its
// directories. The config holds names and directory paths only, never secrets.

type Workspace struct {
	Folders []string `yaml:"folders"`
	// Tools are the forge MCP tools of the workspace: "default" or tool
	// names. Empty means none (ADR 0043).
	Tools []string `yaml:"tools,omitempty"`
	// Instructions are files and Skills are directories holding a SKILL.md; both
	// are copied read-only into the sandbox (ADR 0049).
	Instructions []string `yaml:"instructions,omitempty"`
	Skills       []string `yaml:"skills,omitempty"`
	// Models are the models the workspace's agents may use through the
	// model proxy: "provider/model" or "provider/*" (ADR 0049).
	Models []string `yaml:"models,omitempty"`
	// Model is the default model of the workspace: one of Models (ADR 0054).
	Model string `yaml:"model,omitempty"`
}

// Parts are the opt-in services next to the sandbox (ADR 0049).
type Parts struct {
	Models string `yaml:"models,omitempty"`
}

type Config struct {
	Current string `yaml:"current,omitempty"`
	Parts   Parts  `yaml:"parts,omitempty"`
	// Providers are the model providers behind the model proxy.
	Providers []string `yaml:"providers,omitempty"`
	// ProviderKeys names, per provider that takes a key, the command that
	// prints it. The key itself is in no file of zero (ADR 0056).
	ProviderKeys map[string]string     `yaml:"provider_keys,omitempty"`
	Workspaces   map[string]*Workspace `yaml:"workspaces"`
}

var NameRe = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)

func Path() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "zero", "config.yaml")
}

func Load() (*Config, error) {
	c := &Config{}
	b, err := os.ReadFile(Path())
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(bytes.TrimSpace(b)) > 0 {
		dec := yaml.NewDecoder(bytes.NewReader(b))
		dec.KnownFields(true)
		if err := dec.Decode(c); err != nil {
			return nil, fmt.Errorf("%s: %w", Path(), err)
		}
	}
	if c.Workspaces == nil {
		c.Workspaces = map[string]*Workspace{}
	}
	if err := c.CheckModels(); err != nil {
		return nil, fmt.Errorf("%s: %w", Path(), err)
	}
	for name, w := range c.Workspaces {
		if !NameRe.MatchString(name) || w == nil || len(w.Folders) == 0 {
			return nil, fmt.Errorf("%s: workspace %q needs a name of [a-z0-9-]{1,40} and At least one directory", Path(), name)
		}
	}
	return c, nil
}

func (c *Config) Save() error {
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// RealPath returns p absolute with symlinks resolved.
func RealPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// Name turns a directory name into a workspace name.
func Name(base string) string {
	n := strings.Trim(strings.TrimPrefix(SandboxName(base), "zero-"), "-")
	if len(n) > 40 {
		n = strings.Trim(n[:40], "-")
	}
	return n
}

// At lists the workspaces that have a directory containing dir.
func (c *Config) At(dir string) []string {
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

// Resolve picks the workspace: -c, ZERO_CONTEXT, cwd, current. An empty name
// without error means there is none.
func (c *Config) Resolve(flag, env, cwd string) (name, source string, err error) {
	for _, pick := range []struct{ name, source string }{{flag, "flag"}, {env, "env"}} {
		if pick.name == "" {
			continue
		}
		if c.Workspaces[pick.name] == nil {
			return "", "", fmt.Errorf("no workspace %q (zero ctx lists them)", pick.name)
		}
		return pick.name, pick.source, nil
	}
	switch here := c.At(cwd); {
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

// Adopt registers a sandbox made before workspaces existed: state for the
// repo around cwd, but no config entry.
func (c *Config) Adopt(top string) bool {
	name := Name(filepath.Base(top))
	if top == "" || name == "" || c.Workspaces[name] != nil || len(c.At(top)) > 0 {
		return false
	}
	if _, err := os.Stat(filepath.Join(core.StateRoot(), "zero-"+name)); err != nil {
		return false
	}
	c.Workspaces[name] = &Workspace{Folders: []string{top}}
	return true
}
