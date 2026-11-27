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

// optionalCaps are off by default; the text says what switching one on means.
var optionalCaps = []struct{ Cap, Text string }{
	{capCIRun, "pipelines run on the agent's pushes (default: skipped, you start them)"},
	{capMRCreate, "the agent opens merge requests (needs ci.run)"},
}

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

// firstRun guides a new workspace whose directories have no policy file:
// hosts, permissions, then the file. It asks only at a terminal; the answer
// yes approves the workspace policy shown, and nothing beyond it (ADR 0058).
func (s *Sandbox) firstRun() error {
	if s.Approve || !isTerminal(os.Stdin) || !isTerminal(os.Stderr) {
		return nil
	}
	found := map[string][]preset{}
	var dirs, names []string
	for _, top := range s.repos() {
		if fileExists(filepath.Join(top, ".zero", "policy.yaml")) {
			continue
		}
		dirs = append(dirs, top)
		found[top] = detectPresets(top)
		for _, p := range found[top] {
			if !slices.Contains(names, p.Name) {
				names = append(names, p.Name)
			}
		}
	}
	if len(dirs) == 0 {
		return nil
	}
	chosen := slices.Clone(names)
	caps := slices.Clone(defaultCaps)
	var groups []*huh.Group
	if len(names) > 0 {
		var opts []huh.Option[string]
		for _, p := range presets {
			if slices.Contains(names, p.Name) {
				opts = append(opts, huh.NewOption(p.Name+": "+strings.Join(p.Hosts, ", "), p.Name).Selected(true))
			}
		}
		groups = append(groups, huh.NewGroup(huh.NewMultiSelect[string]().
			Title("Hosts the sandbox may reach").
			Description("Found in this workspace. The forge and the model endpoint are always reachable.").
			Options(opts...).Value(&chosen)))
	}
	var opts []huh.Option[string]
	for _, c := range defaultCaps {
		opts = append(opts, huh.NewOption(c, c).Selected(true))
	}
	for _, c := range optionalCaps {
		opts = append(opts, huh.NewOption(c.Cap+": "+c.Text, c.Cap))
	}
	groups = append(groups, huh.NewGroup(huh.NewMultiSelect[string]().
		Title("What the agent's token may do").
		Description("Enter keeps the defaults.").
		Options(opts...).Value(&caps).Validate(checkChosenCaps)))
	fmt.Fprintf(os.Stderr, "first run: %s has no .zero/policy.yaml yet\n", strings.Join(dirs, ", "))
	if err := huh.NewForm(groups...).WithOutput(os.Stderr).Run(); err != nil {
		return firstRunEnd(err)
	}
	files := map[string][]byte{}
	var all []*Policy
	for _, top := range s.repos() {
		if !slices.Contains(dirs, top) {
			p, err := loadFolderPolicy(top)
			if err != nil {
				return err
			}
			all = append(all, p)
			continue
		}
		files[top] = renderPolicy(firstRunPolicy(found[top], chosen, caps))
		p, err := parsePolicy(files[top])
		if err != nil {
			return fmt.Errorf("first run: policy for %s: %w", top, err)
		}
		all = append(all, p)
		fmt.Fprintf(os.Stderr, "\n%s:\n%s", filepath.Join(top, ".zero", "policy.yaml"), files[top])
	}
	shown := unionPolicy(all...)
	if len(all) > 1 {
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
			return fmt.Errorf("first run: %w (%s); make the directory writable, or answer no, or use --approve for the default", err, done)
		}
		written = append(written, f)
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
