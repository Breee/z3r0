package zero

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/huh/v2"
)

// preset is a set of hosts a kind of project needs, found by a file in it.
type preset struct {
	Name  string
	Files []string
	Hosts []string
}

var presets = []preset{
	{"Go modules", []string{"go.mod"}, []string{"proxy.golang.org", "sum.golang.org"}},
	{"npm", []string{"package.json"}, []string{"registry.npmjs.org"}},
	{"Python", []string{"requirements.txt", "pyproject.toml"}, []string{"pypi.org", "files.pythonhosted.org"}},
	{"Rust", []string{"Cargo.toml"}, []string{"crates.io", "index.crates.io", "static.crates.io"}},
}

// capText says in plain words what each permission lets the agent do. The
// first-run questions, the workspace screen and the guide use these texts.
var capText = map[string]string{
	"git.read":             "read the repositories of this workspace",
	"git.commit":           "commit inside the sandbox",
	"git.push":             "push, only to its own branch zero/...",
	"gitlab.issue.read":    "read issues",
	"gitlab.mr.read":       "read merge requests and their comments",
	"gitlab.pipeline.read": "read pipelines and job logs",
	capCIRun:               "pipelines run on the agent's pushes (off: skipped, you start them)",
	capMRCreate:            "open merge requests (needs ci.run: a merge request starts a pipeline)",
}

const (
	hostsTitle = "Hosts the sandbox may reach"
	hostsText  = "Package sources found in this workspace. The forge and the model endpoint are always reachable; every other host is refused."
	capsTitle  = "What the agent may do"
	capsText   = "Ticked is the safe default. Enter keeps it."
)

// optionalCaps are off by default.
var optionalCaps = []string{capCIRun, capMRCreate}

func capLabel(c string) string { return c + ": " + capText[c] }

func presetLabel(p preset) string { return p.Name + ": " + strings.Join(p.Hosts, ", ") }

func detectPresets(dir string) []preset {
	var found []preset
	for _, p := range presets {
		if slices.ContainsFunc(p.Files, func(f string) bool { return fileExists(filepath.Join(dir, f)) }) {
			found = append(found, p)
		}
	}
	return found
}

func checkChosenCaps(caps []string) error {
	if len(caps) == 0 {
		return errors.New("choose at least one permission")
	}
	if slices.Contains(caps, capMRCreate) && !slices.Contains(caps, capCIRun) {
		return fmt.Errorf("%s needs %s", capMRCreate, capCIRun)
	}
	return nil
}

// firstRunPolicy is the policy file for one directory: the chosen presets
// that were found in it, and the chosen permissions in their usual order.
func firstRunPolicy(found []preset, chosen, caps []string) *Policy {
	p := &Policy{}
	for _, pr := range found {
		if slices.Contains(chosen, pr.Name) {
			p.Network.Allow = append(p.Network.Allow, pr.Hosts...)
		}
	}
	for _, c := range append(slices.Clone(defaultCaps), capCIRun, capMRCreate) {
		if slices.Contains(caps, c) {
			p.Capabilities = append(p.Capabilities, c)
		}
	}
	return p
}

func renderPolicy(p *Policy) []byte {
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

// firstRunPlan finds the directories without a policy file and the presets
// in them.
func firstRunPlan(repos []string) (dirs []string, found map[string][]preset, names []string) {
	found = map[string][]preset{}
	for _, top := range repos {
		if fileExists(filepath.Join(top, ".zero", "policy.yaml")) {
			continue
		}
		dirs = append(dirs, top)
		found[top] = detectPresets(top)
	}
	for _, p := range presets {
		for _, top := range dirs {
			if slices.ContainsFunc(found[top], func(f preset) bool { return f.Name == p.Name }) {
				names = append(names, p.Name)
				break
			}
		}
	}
	return dirs, found, names
}

// firstRunFiles makes the policy file of every directory in dirs and the
// workspace policy: these files and the existing ones together.
func firstRunFiles(repos, dirs []string, found map[string][]preset, chosen, caps []string) (map[string][]byte, *Policy, error) {
	files := map[string][]byte{}
	var all []*Policy
	for _, top := range repos {
		if !slices.Contains(dirs, top) {
			p, err := loadFolderPolicy(top)
			if err != nil {
				return nil, nil, err
			}
			all = append(all, p)
			continue
		}
		files[top] = renderPolicy(firstRunPolicy(found[top], chosen, caps))
		p, err := parsePolicy(files[top])
		if err != nil {
			return nil, nil, fmt.Errorf("first run: policy for %s: %w", top, err)
		}
		all = append(all, p)
	}
	return files, unionPolicy(all...), nil
}

func writeFirstRun(dirs []string, files map[string][]byte) error {
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

// firstRun guides a new workspace whose directories have no policy file:
// hosts, permissions, then the file. It asks only at a terminal; the answer
// yes approves the workspace policy shown, and nothing beyond it (ADR 0058).
func (s *Sandbox) firstRun() error {
	if s.Approve || !isTerminal(os.Stdin) || !isTerminal(os.Stderr) {
		return nil
	}
	dirs, found, names := firstRunPlan(s.repos())
	if len(dirs) == 0 {
		return nil
	}
	chosen := slices.Clone(names)
	caps := slices.Clone(defaultCaps)
	// The height is set by hand: without it huh hides options (caveat C63).
	var groups []*huh.Group
	if len(names) > 0 {
		var opts []huh.Option[string]
		for _, p := range presets {
			if slices.Contains(names, p.Name) {
				opts = append(opts, huh.NewOption(presetLabel(p), p.Name).Selected(true))
			}
		}
		groups = append(groups, huh.NewGroup(huh.NewMultiSelect[string]().
			Title(hostsTitle).Description(hostsText).
			Options(opts...).Height(2*len(opts)+5).Value(&chosen)))
	}
	var opts []huh.Option[string]
	for _, c := range defaultCaps {
		opts = append(opts, huh.NewOption(capLabel(c), c).Selected(true))
	}
	for _, c := range optionalCaps {
		opts = append(opts, huh.NewOption(capLabel(c), c))
	}
	groups = append(groups, huh.NewGroup(huh.NewMultiSelect[string]().
		Title(capsTitle).Description(capsText).
		Options(opts...).Height(2*len(opts)+4).Value(&caps).Validate(checkChosenCaps)))
	fmt.Fprintf(os.Stderr, "first run: %s has no .zero/policy.yaml yet\n", strings.Join(dirs, ", "))
	if err := huh.NewForm(groups...).WithOutput(os.Stderr).Run(); err != nil {
		return firstRunEnd(err)
	}
	files, shown, err := firstRunFiles(s.repos(), dirs, found, chosen, caps)
	if err != nil {
		return err
	}
	for _, top := range dirs {
		fmt.Fprintf(os.Stderr, "\n%s:\n%s", filepath.Join(top, ".zero", "policy.yaml"), files[top])
	}
	if len(s.repos()) > 1 {
		fmt.Fprintf(os.Stderr, "\nthe workspace policy (every directory together):\n%s", renderPolicy(shown))
	}
	ok := true
	if err := huh.NewForm(huh.NewGroup(huh.NewConfirm().
		Title("Write and use this policy?").
		Description("Enter: yes. No: nothing is written, the default applies.").
		Value(&ok))).WithOutput(os.Stderr).Run(); err != nil {
		return firstRunEnd(err)
	}
	if !ok {
		return nil
	}
	if err := writeFirstRun(dirs, files); err != nil {
		return err
	}
	s.shown = shown
	return nil
}

// firstRunEnd: leaving the questions (esc, ctrl+c) stops the command.
func firstRunEnd(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		return errors.New("stopped; nothing written")
	}
	return err
}
