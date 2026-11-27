// PoC 32 (#181): a full-screen workspace screen in the style of k9s, with
// bubbletea v2 and lipgloss v2, on made-up workspaces.
//
//	poc32                     the screen (needs a terminal, else exit 2)
//	poc32 frame W H [KEY...]  print the frame after these keys, no terminal
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

const logo = ` ▄█████▄
▄██   ██▀
██  ▄█▀██
██▄█▀  ██
▄█▀   ▄██
 ▀█████▀ `

type row struct {
	Name, Status, Token, Policy string
	Dirs                        []string
	Current                     bool
}

func fake() []row {
	return []row{
		{"apps", "none", "-", "-", []string{"/home/me/repos/apps"}, false},
		{"cve", "stopped", "ok", "default", []string{"/home/me/repos/cve"}, false},
		{"developer", "running", "ok", "approved", []string{"/home/me/repos/developer", "/home/me/repos/docs"}, true},
		{"z3r0", "running", "expires in 2d", "approved", []string{"/home/me/repos/z3r0"}, false},
	}
}

type mode int

const (
	list mode = iota
	filtering
	command
	describe
	help
)

type keyHint struct{ key, what string }

var hints = []keyHint{
	{"enter", "opencode"}, {"v", "vscode"}, {"u", "up"}, {"s", "stop"},
	{"r", "remove"}, {"n", "new"}, {"d", "describe"}, {"/", "filter"},
	{":", "command"}, {"?", "help"}, {"ctrl-r", "refresh"}, {"q", "quit"},
}

var (
	cTitle  = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	cKey    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	cLabel  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	cFrame  = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	cFaint  = lipgloss.NewStyle().Faint(true)
	cHead   = lipgloss.NewStyle().Bold(true)
	cSel    = lipgloss.NewStyle().Reverse(true)
	cLogo   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	cStatus = map[string]lipgloss.Style{
		"running": lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		"stopped": lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		"none":    lipgloss.NewStyle().Faint(true),
	}
)

type refreshed time.Time
type ran struct {
	what string
	err  error
}

type model struct {
	rows   []row
	sel    int
	w, h   int
	mode   mode
	input  string
	filter string
	msg    string
	ticks  int
	action string // what to start after the screen closes
}

func tick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg { return refreshed(t) })
}

func (m model) Init() tea.Cmd { return tick() }

func (m model) shown() []row {
	if m.filter == "" {
		return m.rows
	}
	var out []row
	for _, r := range m.rows {
		if strings.Contains(r.Name, m.filter) {
			out = append(out, r)
		}
	}
	return out
}

func (m model) cur() *row {
	rs := m.shown()
	if m.sel < 0 || m.sel >= len(rs) {
		return nil
	}
	return &rs[m.sel]
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case refreshed:
		m.ticks++
		return m, tick()
	case ran:
		m.msg = msg.what + ": done"
		if msg.err != nil {
			m.msg = msg.what + ": " + msg.err.Error()
		}
	case tea.KeyPressMsg:
		k := msg.String()
		if k == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.mode {
		case filtering, command:
			return m.typed(k, msg.Text)
		case describe, help:
			if k == "esc" || k == "q" || k == "enter" {
				m.mode = list
			}
			return m, nil
		}
		return m.key(k)
	}
	return m, nil
}

func (m model) typed(k, text string) (tea.Model, tea.Cmd) {
	switch k {
	case "esc":
		if m.mode == filtering {
			m.filter = ""
		}
		m.mode, m.input = list, ""
	case "backspace":
		if n := len(m.input); n > 0 {
			m.input = m.input[:n-1]
		}
	case "enter":
		in := strings.TrimSpace(m.input)
		was := m.mode
		m.mode, m.input = list, ""
		if was == command {
			return m.run(in)
		}
	default:
		m.input += text
	}
	if m.mode == filtering {
		m.filter, m.sel = m.input, 0
	}
	return m, nil
}

func (m model) run(c string) (tea.Model, tea.Cmd) {
	switch c {
	case "":
	case "q", "quit":
		return m, tea.Quit
	case "help":
		m.mode = help
	case "all":
		m.filter, m.sel = "", 0
	default:
		for i, r := range m.shown() {
			if r.Name == c {
				m.sel = i
				return m, nil
			}
		}
		m.msg = "no command or workspace: " + c
	}
	return m, nil
}

func (m model) key(k string) (tea.Model, tea.Cmd) {
	n := len(m.shown())
	m.msg = ""
	switch k {
	case "q":
		return m, tea.Quit
	case "esc":
		m.filter, m.sel = "", 0
	case "down", "j":
		if m.sel < n-1 {
			m.sel++
		}
	case "up", "k":
		if m.sel > 0 {
			m.sel--
		}
	case "g":
		m.sel = 0
	case "G":
		m.sel = max(n-1, 0)
	case "/":
		m.mode, m.input = filtering, m.filter
	case ":":
		m.mode, m.input = command, ""
	case "?":
		m.mode = help
	case "ctrl+r":
		m.msg = "refreshed"
	}
	r := m.cur()
	if r == nil {
		return m, nil
	}
	switch k {
	case "d":
		m.mode = describe
	case "enter", "o":
		m.action = "-c " + r.Name + " opencode"
		return m, tea.Quit
	case "v":
		m.action = "-c " + r.Name + " vscode"
		return m, tea.Quit
	case "r":
		// The real command keeps its own questions: the screen steps aside.
		what := "rm " + r.Name
		c := exec.Command("sh", "-c", `printf 'type workspace name to delete: '; read -r x; [ "$x" = "$0" ]`, r.Name)
		return m, tea.ExecProcess(c, func(err error) tea.Msg { return ran{what, err} })
	}
	return m, nil
}

func pad(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func (m model) header() string {
	cur := "-"
	for _, r := range m.rows {
		if r.Current {
			cur = r.Name
		}
	}
	info := []string{
		cLabel.Render("Workspace: ") + cur,
		cLabel.Render("Runtime:   ") + "docker-rootless",
		cLabel.Render("Gateway:   ") + "ok",
		cLabel.Render("Zero:      ") + "v0.0.0-poc",
		cLabel.Render("Refresh:   ") + strconv.Itoa(m.ticks),
		"",
	}
	parts := []string{strings.Join(info, "\n")}
	left := 30
	if m.w >= 72 {
		var cols []string
		for i := 0; i < len(hints); i += 6 {
			var c []string
			for _, h := range hints[i:min(i+6, len(hints))] {
				c = append(c, cKey.Render("<"+h.key+">")+strings.Repeat(" ", 9-len(h.key)-2)+cFaint.Render(h.what))
			}
			cols = append(cols, lipgloss.NewStyle().Width(20).Render(strings.Join(c, "\n")))
		}
		parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, cols...))
		left += 40
	}
	parts[0] = lipgloss.NewStyle().Width(30).Render(parts[0])
	if m.w >= 44 {
		gap := max(m.w-left-10, 0)
		parts = append(parts, strings.Repeat(" ", gap), cLogo.Render(logo))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// box frames the lines, with the title in the top edge, to w by h cells.
func box(title string, lines []string, w, h int) string {
	in := w - 2
	t := " " + title + " "
	fill := max(in-lipgloss.Width(t), 0)
	out := []string{cFrame.Render("┌"+strings.Repeat("─", fill/2)) + cTitle.Render(t) + cFrame.Render(strings.Repeat("─", fill-fill/2)+"┐")}
	for i := 0; i < h-2; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		l = lipgloss.NewStyle().MaxWidth(in).Render(l)
		out = append(out, cFrame.Render("│")+pad(l, in)+cFrame.Render("│"))
	}
	out = append(out, cFrame.Render("└"+strings.Repeat("─", in)+"┘"))
	return strings.Join(out, "\n")
}

func (m model) table(w int) []string {
	rs := m.shown()
	nw := len("NAME")
	for _, r := range rs {
		nw = max(nw, len(r.Name))
	}
	line := func(name, status, dirs, token, policy string) string {
		return fmt.Sprintf(" %-*s  %-8s  %-4s  %-14s  %s", nw+1, name, status, dirs, token, policy)
	}
	out := []string{cHead.Render(line("NAME", "STATUS", "DIRS", "TOKEN", "POLICY"))}
	for i, r := range rs {
		name := r.Name
		if r.Current {
			name += "*"
		}
		l := pad(line(name, r.Status, strconv.Itoa(len(r.Dirs)), r.Token, r.Policy), w)
		switch {
		case i == m.sel:
			l = cSel.Render(l)
		default:
			l = cStatus[r.Status].Render(l)
		}
		out = append(out, l)
	}
	return out
}

func (m model) frame() string {
	w, h := m.w, m.h
	if w < 20 || h < 8 {
		return "zero: the window is too small"
	}
	var top []string
	headH := 0
	if h >= 16 {
		top = append(top, m.header())
		headH = 6
	}
	switch m.mode {
	case filtering:
		top = append(top, cKey.Render("/")+m.input+"█")
		headH++
	case command:
		top = append(top, cKey.Render(":")+m.input+"█")
		headH++
	}
	bh := h - headH - 1
	var body string
	switch m.mode {
	case describe:
		r := m.cur()
		ls := []string{"", cLabel.Render(" Status:      ") + r.Status, cLabel.Render(" Token:       ") + r.Token,
			cLabel.Render(" Policy:      ") + r.Policy, cLabel.Render(" Directories:")}
		for _, d := range r.Dirs {
			ls = append(ls, "   "+d)
		}
		body = box("describe("+r.Name+")", ls, w, bh)
	case help:
		ls := []string{""}
		for _, hh := range hints {
			ls = append(ls, " "+cKey.Render(pad("<"+hh.key+">", 10))+hh.what)
		}
		ls = append(ls, "", " :NAME jumps to a workspace, :all clears the filter, :q quits")
		body = box("help", ls, w, bh)
	default:
		scope := "all"
		if m.filter != "" {
			scope = "/" + m.filter
		}
		body = box(fmt.Sprintf("workspaces(%s)[%d]", scope, len(m.shown())), m.table(w-2), w, bh)
	}
	status := cFaint.Render(lipgloss.NewStyle().MaxWidth(w).Render(" " + m.msg))
	return strings.Join(append(top, body, status), "\n")
}

func (m model) View() tea.View {
	v := tea.NewView(m.frame())
	v.AltScreen = true
	v.WindowTitle = "zero"
	return v
}

func press(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	}
	r := []rune(k)[0]
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func main() {
	m := model{rows: fake(), sel: 2}
	if len(os.Args) > 1 && os.Args[1] == "frame" {
		w, _ := strconv.Atoi(os.Args[2])
		h, _ := strconv.Atoi(os.Args[3])
		var t tea.Model = m
		t, _ = t.Update(tea.WindowSizeMsg{Width: w, Height: h})
		for _, k := range os.Args[4:] {
			t, _ = t.Update(press(k))
		}
		fmt.Println(t.(model).frame())
		return
	}
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr, "poc32: no terminal")
		os.Exit(2)
	}
	end, err := tea.NewProgram(m).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "poc32:", err)
		os.Exit(1)
	}
	if a := end.(model).action; a != "" {
		fmt.Println("would run: zero " + a)
	}
}
