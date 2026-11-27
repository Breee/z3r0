// PoC 31 (#185): the first-run questions and the workspace list, with
// Charm v2 (huh, bubbletea, lipgloss).
//
//	poc31 firstrun DIR   detect presets in DIR, ask, print the policy
//	poc31 tui            workspace list
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

type preset struct {
	name  string
	files []string
	hosts []string
}

var presets = []preset{
	{"Go modules", []string{"go.mod"}, []string{"proxy.golang.org", "sum.golang.org"}},
	{"npm", []string{"package.json"}, []string{"registry.npmjs.org"}},
	{"Python packages", []string{"requirements.txt", "pyproject.toml"}, []string{"pypi.org", "files.pythonhosted.org"}},
	{"Rust crates", []string{"Cargo.toml"}, []string{"crates.io", "index.crates.io", "static.crates.io"}},
}

type perm struct {
	name, text string
	on         bool
}

var perms = []perm{
	{"git.read", "read the repository", true},
	{"git.commit", "commit in the sandbox", true},
	{"git.push", "push to its own branch", true},
	{"gitlab.issue.read", "read issues", true},
	{"gitlab.mr.read", "read merge requests", true},
	{"gitlab.pipeline.read", "read pipeline results", true},
	{"ci.run", "start pipelines: CI runs with the project's secrets", false},
	{"gitlab.mr.create", "open merge requests (needs ci.run)", false},
}

func detect(dir string) (found []preset) {
	for _, p := range presets {
		for _, f := range p.files {
			if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
				found = append(found, p)
				break
			}
		}
	}
	return found
}

func policy(hosts, caps []string) string {
	var b strings.Builder
	b.WriteString("network:\n  allow:")
	if len(hosts) == 0 {
		b.WriteString(" []")
	}
	for _, h := range hosts {
		b.WriteString("\n    - " + h)
	}
	b.WriteString("\ncapabilities:\n")
	for _, c := range caps {
		b.WriteString("  - " + c + "\n")
	}
	return b.String()
}

func firstRun(dir string) error {
	var hosts, caps []string
	var hostOpts []huh.Option[string]
	for _, p := range detect(dir) {
		for _, h := range p.hosts {
			hosts = append(hosts, h)
			hostOpts = append(hostOpts, huh.NewOption(h+"  ("+p.name+")", h).Selected(true))
		}
	}
	var capOpts []huh.Option[string]
	for _, p := range perms {
		if p.on {
			caps = append(caps, p.name)
		}
		capOpts = append(capOpts, huh.NewOption(fmt.Sprintf("%-22s %s", p.name, p.text), p.name).Selected(p.on))
	}

	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stderr.Fd()) {
		fmt.Fprintln(os.Stderr, "no terminal: no questions, defaults used")
		fmt.Print(policy(hosts, caps))
		return nil
	}

	var groups []*huh.Group
	if len(hostOpts) > 0 {
		groups = append(groups, huh.NewGroup(huh.NewMultiSelect[string]().
			Title("Hosts the agent may reach").
			Description("Found in this repository. Space toggles, Enter goes on.").
			Options(hostOpts...).Value(&hosts)))
	}
	groups = append(groups, huh.NewGroup(huh.NewMultiSelect[string]().
		Title("What the agent may do").
		Description("The safe set is selected. The last two reach beyond the sandbox.").
		Options(capOpts...).Value(&caps).
		Validate(func(v []string) error {
			has := func(c string) bool {
				for _, x := range v {
					if x == c {
						return true
					}
				}
				return false
			}
			if has("gitlab.mr.create") && !has("ci.run") {
				return errors.New("gitlab.mr.create needs ci.run")
			}
			return nil
		})))
	if err := huh.NewForm(groups...).WithOutput(os.Stderr).Run(); err != nil {
		return err
	}
	sort.Strings(hosts)
	ok := true
	fmt.Fprint(os.Stderr, lipgloss.NewStyle().Faint(true).Render(policy(hosts, caps)))
	if err := huh.NewForm(huh.NewGroup(huh.NewConfirm().
		Title("Write .zero/policy.yaml and start?").Value(&ok))).WithOutput(os.Stderr).Run(); err != nil {
		return err
	}
	if !ok {
		return errors.New("nothing written")
	}
	fmt.Print(policy(hosts, caps))
	return nil
}

type ws struct{ name, status, dirs, policy string }

type list struct {
	rows   []ws
	sel    int
	chosen string
}

func (m list) Init() tea.Cmd { return nil }

func (m list) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.sel > 0 {
				m.sel--
			}
		case "down", "j":
			if m.sel < len(m.rows)-1 {
				m.sel++
			}
		case "enter", "o", "v", "s", "r":
			m.chosen = k.String() + " " + m.rows[m.sel].name
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m list) View() tea.View {
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Faint(true)
	var b strings.Builder
	b.WriteString(bold.Render("zero") + dim.Render("  workspaces") + "\n\n")
	for i, r := range m.rows {
		dot := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("○")
		if r.status == "running" {
			dot = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("●")
		}
		line := fmt.Sprintf("%s %-12s %-8s %-4s %s", dot, r.name, r.status, r.dirs, r.policy)
		if i == m.sel {
			line = bold.Render("› " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + dim.Render("enter/o opencode · v vscode · s stop · r remove · q quit") + "\n")
	return tea.NewView(b.String())
}

func tui() error {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr, "no terminal: help text instead of the list")
		os.Exit(2)
	}
	m, err := tea.NewProgram(list{rows: []ws{
		{"developer", "running", "3", "approved"},
		{"cve", "running", "1", "approved"},
		{"apps", "none", "2", "default"},
	}}).Run()
	if err != nil {
		return err
	}
	fmt.Println("chosen:", m.(list).chosen)
	return nil
}

func main() {
	var err error
	switch {
	case len(os.Args) == 3 && os.Args[1] == "firstrun":
		err = firstRun(os.Args[2])
	case len(os.Args) == 2 && os.Args[1] == "tui":
		err = tui()
	default:
		err = errors.New("usage: poc31 firstrun DIR | poc31 tui")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "poc31:", err)
		os.Exit(1)
	}
}
