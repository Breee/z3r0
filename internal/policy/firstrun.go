package policy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Breee/z3r0/internal/core"
)

// Preset is a set of hosts a kind of project needs, found by a file in it.
type Preset struct {
	Name  string
	Files []string
	Hosts []string
}

var Presets = []Preset{
	{"Go modules", []string{"go.mod"}, []string{"proxy.golang.org", "sum.golang.org"}},
	{"npm", []string{"package.json"}, []string{"registry.npmjs.org"}},
	{"Python", []string{"requirements.txt", "pyproject.toml"}, []string{"pypi.org", "files.pythonhosted.org"}},
	{"Rust", []string{"Cargo.toml"}, []string{"crates.io", "index.crates.io", "static.crates.io"}},
}

// CapText says in plain words what each permission lets the agent do. The
// first-run questions, the workspace screen and the guide use these texts.
var CapText = map[string]string{
	"git.read":             "read the repositories of this workspace",
	"git.commit":           "commit inside the sandbox",
	"git.push":             "push, only to its own branch zero/...",
	"gitlab.issue.read":    "read issues",
	"gitlab.mr.read":       "read merge requests and their comments",
	"gitlab.pipeline.read": "read pipelines and job logs",
	core.CapCIRun:          "pipelines run on the agent's pushes (off: skipped, you start them)",
	core.CapMRCreate:       "open merge requests (needs ci.run: a merge request starts a pipeline)",
}

const (
	HostsTitle = "Hosts the sandbox may reach"
	HostsText  = "Package sources found in this workspace. The forge and the model endpoint are always reachable; every other host is refused."
	CapsTitle  = "What the agent may do"
	CapsText   = "Ticked is the safe default. Enter keeps it."
)

// OptionalCaps are off by default.
var OptionalCaps = []string{core.CapCIRun, core.CapMRCreate}

func CapLabel(c string) string { return c + ": " + CapText[c] }

func PresetLabel(p Preset) string { return p.Name + ": " + strings.Join(p.Hosts, ", ") }

func DetectPresets(dir string) []Preset {
	var found []Preset
	for _, p := range Presets {
		if slices.ContainsFunc(p.Files, func(f string) bool { return core.FileExists(filepath.Join(dir, f)) }) {
			found = append(found, p)
		}
	}
	return found
}

func CheckChosenCaps(caps []string) error {
	if len(caps) == 0 {
		return errors.New("choose at least one permission")
	}
	if slices.Contains(caps, core.CapMRCreate) && !slices.Contains(caps, core.CapCIRun) {
		return fmt.Errorf("%s needs %s", core.CapMRCreate, core.CapCIRun)
	}
	return nil
}

// FirstRunPolicy is the policy file for one directory: the chosen presets
// that were found in it, and the chosen permissions in their usual order.
func FirstRunPolicy(found []Preset, chosen, caps []string) *Policy {
	p := &Policy{}
	for _, pr := range found {
		if slices.Contains(chosen, pr.Name) {
			p.Network.Allow = append(p.Network.Allow, pr.Hosts...)
		}
	}
	for _, c := range append(slices.Clone(DefaultCaps), core.CapCIRun, core.CapMRCreate) {
		if slices.Contains(caps, c) {
			p.Capabilities = append(p.Capabilities, c)
		}
	}
	return p
}

func Render(p *Policy) []byte {
	var b strings.Builder
	if len(p.Network.Allow) > 0 {
		b.WriteString("network:\n  allow:\n")
		for _, h := range p.Network.Allow {
			b.WriteString("    - " + h + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("capabilities:\n")
	for _, c := range p.Capabilities {
		b.WriteString("  - " + c + "\n")
	}
	return []byte(b.String())
}

// FirstRunPlan finds the directories without a policy file and the presets
// in them.
func FirstRunPlan(repos []string) (dirs []string, found map[string][]Preset, names []string) {
	found = map[string][]Preset{}
	for _, top := range repos {
		if core.FileExists(filepath.Join(top, ".zero", "policy.yaml")) {
			continue
		}
		dirs = append(dirs, top)
		found[top] = DetectPresets(top)
	}
	for _, p := range Presets {
		for _, top := range dirs {
			if slices.ContainsFunc(found[top], func(f Preset) bool { return f.Name == p.Name }) {
				names = append(names, p.Name)
				break
			}
		}
	}
	return dirs, found, names
}

// FirstRunFiles makes the policy file of every directory in dirs and the
// workspace policy: these files and the existing ones together.
func FirstRunFiles(repos, dirs []string, found map[string][]Preset, chosen, caps []string) (map[string][]byte, *Policy, error) {
	files := map[string][]byte{}
	var all []*Policy
	for _, top := range repos {
		if !slices.Contains(dirs, top) {
			p, err := LoadDir(top)
			if err != nil {
				return nil, nil, err
			}
			all = append(all, p)
			continue
		}
		files[top] = Render(FirstRunPolicy(found[top], chosen, caps))
		p, err := Parse(files[top])
		if err != nil {
			return nil, nil, fmt.Errorf("first run: policy for %s: %w", top, err)
		}
		all = append(all, p)
	}
	return files, Union(all...), nil
}

func WriteFirstRun(dirs []string, files map[string][]byte) error {
	var written []string
	for _, top := range dirs {
		f := filepath.Join(top, ".zero", "policy.yaml")
		err := os.MkdirAll(filepath.Dir(f), 0o755)
		if err == nil {
			err = os.WriteFile(f, files[top], 0o644)
		}
		if err != nil {
			done := "nothing written"
			if len(written) > 0 {
				done = "already written: " + strings.Join(written, ", ")
			}
			return fmt.Errorf("first run: %w (%s); make the directory writable, or go on without the file: the default applies", err, done)
		}
		written = append(written, f)
	}
	return nil
}
