package zero

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// homeKeys maps a key of the workspace list to the command it runs.
var homeKeys = map[string]string{
	"enter": "opencode", "o": "opencode", "v": "vscode",
	"s": "stop", "r": "rm", "n": "new",
}

type home struct {
	rows []wsRow
	sel  int
	cmd  string
}

func (m home) Init() tea.Cmd { return nil }

func (m home) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key := k.String(); key {
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
	default:
		if c, ok := homeKeys[key]; ok && (c == "new" || len(m.rows) > 0) {
			m.cmd = c
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m home) View() tea.View {
	bold := lipgloss.NewStyle().Bold(true)
	faint := lipgloss.NewStyle().Faint(true)
	var b strings.Builder
	b.WriteString(bold.Render("zero") + faint.Render("  workspaces") + "\n\n")
	if len(m.rows) == 0 {
		b.WriteString("  no workspace yet; n makes one\n")
	}
	w := 9
	for _, r := range m.rows {
		w = max(w, len(r.Name))
	}
	for i, r := range m.rows {
		color := "8"
		switch r.Status {
		case "running":
			color = "2"
		case "stopped":
			color = "3"
		}
		line := fmt.Sprintf("%-*s  %-8s %d dir  token %-8s policy %s", w, r.Name, r.Status, r.Folders, r.Token, r.Policy)
		cur := "  "
		if i == m.sel {
			cur = bold.Render("> ")
			line = bold.Render(line)
		}
		b.WriteString(cur + lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render("● ") + line + "\n")
	}
	b.WriteString("\n" + faint.Render("enter opencode · v vscode · s stop · r remove · n new · q quit") + "\n")
	return tea.NewView(b.String())
}

// Interactive: a person at a terminal on both ends.
func Interactive() bool { return isTerminal(os.Stdin) && isTerminal(os.Stdout) }

// Home is zero without a command on a terminal: the workspace list. It runs
// the commands a person would type. stop, rm and new come back to the list;
// for an agent it returns the arguments to run zero with.
func Home(exe string) ([]string, error) {
	sel := -1
	for {
		o, err := loadOverview()
		if err != nil {
			return nil, err
		}
		m := home{rows: o.rows, sel: sel}
		if sel < 0 || sel >= len(m.rows) {
			m.sel = 0
			for i, r := range m.rows {
				if r.Current {
					m.sel = i
				}
			}
		}
		res, err := tea.NewProgram(m).Run()
		if err != nil {
			return nil, err
		}
		m = res.(home)
		sel = m.sel
		var args []string
		switch m.cmd {
		case "":
			return nil, nil
		case "new":
			if args, err = newWorkspaceArgs(); err != nil {
				return nil, err
			}
			if args == nil {
				continue
			}
		case "opencode", "vscode":
			return []string{"-c", m.rows[m.sel].Name, m.cmd}, nil
		default:
			args = []string{"-c", m.rows[m.sel].Name, m.cmd}
		}
		c := exec.Command(exe, args...)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := c.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "zero %s: %v\nenter to go on ", strings.Join(args, " "), err)
			fmt.Scanln()
		}
	}
}

// newWorkspaceArgs asks for a name and a directory; nil when left with esc.
func newWorkspaceArgs() ([]string, error) {
	name := ""
	dir, _ := os.Getwd()
	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Name of the new workspace").Value(&name).Validate(func(v string) error {
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("a name is needed")
			}
			return nil
		}),
		huh.NewInput().Title("Directory").Value(&dir).Validate(func(v string) error {
			if st, err := os.Stat(v); err != nil || !st.IsDir() {
				return fmt.Errorf("not a directory")
			}
			return nil
		}),
	)).WithOutput(os.Stderr).Run()
	if err == huh.ErrUserAborted {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	args := []string{"ctx", "new", strings.TrimSpace(name)}
	if d := strings.TrimSpace(dir); d != "" {
		args = append(args, d)
	}
	return args, nil
}
