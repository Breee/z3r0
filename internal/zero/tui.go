package zero

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Breee/z3r0/internal/core"
	"github.com/Breee/z3r0/internal/policy"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// The mark of zero, drawn in half blocks.
const logo = ` ▄█████▄
▄██   ██▀
██  ▄█▀██
██▄█▀  ██
▄█▀   ▄██
 ▀█████▀`

type homeMode int

const (
	homeList homeMode = iota
	homeFilter
	homeCommand
	homeDescribe
	homeHelp
	homeFirst
)

type keyHint struct{ key, what string }

var homeHints = []keyHint{
	{"enter", "opencode"}, {"v", "vscode"}, {"u", "up"}, {"s", "stop"}, {"r", "remove"}, {"n", "new"},
	{"d", "describe"}, {"/", "filter"}, {":", "command"}, {"?", "help"}, {"w", "switch to"}, {"q", "quit"},
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
		"none":    cFaint,
	}
)

type (
	homeLoaded struct {
		o   *overview
		err error
	}
	homeTick struct{}
	homeRan  struct {
		what, last string
		err        error
	}
)

// home is the workspace screen behind a bare `zero` (ADR 0059).
type home struct {
	exe              string
	rows             []wsRow
	ws               map[string]*Workspace
	runtime, gateway string
	warn             []string
	ready            bool
	sel, w, h        int
	mode             homeMode
	input, filter    string
	msg              string
	cmd              string     // opencode, vscode or new: what to do after the screen
	want             string     // the workspace to stay on while the filter changes
	loading, again   bool       // one load at a time; again: one more after it
	fr               *firstView // the first run being asked (mode homeFirst)
}

func homeLoad() tea.Msg {
	o, err := loadOverview()
	return homeLoaded{o, err}
}

func homeTicker() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return homeTick{} })
}

func (m home) Init() tea.Cmd { return tea.Batch(homeLoad, homeTicker()) }

// load starts a load unless one runs; then it asks for one more after it,
// so an older answer never replaces a newer one.
func (m *home) load() tea.Cmd {
	if m.loading {
		m.again = true
		return nil
	}
	m.loading = true
	return homeLoad
}

func (m home) shown() []wsRow {
	if m.filter == "" {
		return m.rows
	}
	var out []wsRow
	for _, r := range m.rows {
		if strings.Contains(r.Name, m.filter) {
			out = append(out, r)
		}
	}
	return out
}

func (m home) cur() *wsRow {
	rs := m.shown()
	if m.sel < 0 || m.sel >= len(rs) {
		return nil
	}
	return &rs[m.sel]
}

// selectName puts the selection on the named workspace, if it is shown.
func (m *home) selectName(name string) bool {
	for i, r := range m.shown() {
		if r.Name == name {
			m.sel = i
			return true
		}
	}
	return false
}

func (m *home) clamp() {
	m.sel = max(min(m.sel, len(m.shown())-1), 0)
}

func (m home) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case homeTick:
		if m.loading {
			return m, homeTicker()
		}
		c := m.load()
		return m, tea.Batch(c, homeTicker())
	case homeLoaded:
		m.loading = false
		if m.again {
			m.again = false
			c := m.load()
			return m, c
		}
		if msg.err != nil {
			m.msg = msg.err.Error()
			return m, nil
		}
		name := ""
		if c := m.cur(); c != nil {
			name = c.Name
		}
		m.rows, m.ws, m.warn = msg.o.rows, msg.o.c.Workspaces, msg.o.warn
		m.gateway = "ok"
		for _, w := range m.warn {
			if strings.HasPrefix(w, "gateway not reachable") {
				m.gateway = "unreachable"
			}
		}
		if !m.ready {
			m.ready = true
			for _, r := range m.rows {
				if r.Current {
					name = r.Name
				}
			}
		}
		if !m.selectName(name) {
			m.clamp()
		}
	case homeRan:
		m.msg = msg.what + ": done"
		if msg.err != nil {
			m.msg = msg.what + ": " + msg.err.Error()
			if msg.last != "" {
				m.msg = msg.what + ": " + msg.last
			}
		}
		c := m.load()
		return m, c
	case tea.KeyPressMsg:
		k := msg.String()
		switch {
		case k == "ctrl+c":
			return m, tea.Quit
		case m.mode == homeFilter || m.mode == homeCommand:
			return m.typed(k, msg.Text)
		case m.mode == homeFirst && m.fr != nil:
			return m.firstKey(k)
		case m.mode == homeDescribe || m.mode == homeHelp:
			if k == "esc" || k == "q" || k == "enter" {
				m.mode = homeList
			}
			return m, nil
		}
		return m.key(k)
	}
	return m, nil
}

// typed handles a key while a filter or a command is being typed.
func (m home) typed(k, text string) (tea.Model, tea.Cmd) {
	switch k {
	case "esc":
		if m.mode == homeFilter {
			m.filter = ""
		}
		m.mode, m.input = homeList, ""
	case "backspace":
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
	case "enter":
		c := m.input
		mode := m.mode
		m.mode, m.input = homeList, ""
		if mode == homeCommand {
			return m.run(strings.TrimSpace(c))
		}
		return m, nil
	default:
		m.input += text
	}
	if m.mode == homeFilter {
		m.filter = m.input
	}
	if !m.selectName(m.want) {
		m.clamp()
	}
	return m, nil
}

// run does a command typed after the colon.
func (m home) run(c string) (tea.Model, tea.Cmd) {
	switch c {
	case "":
	case "q", "quit":
		return m, tea.Quit
	case "help":
		m.mode = homeHelp
	case "all":
		m.filter, m.sel = "", 0
	case "new":
		m.cmd = "new"
		return m, tea.Quit
	default:
		f := m.filter
		m.filter = ""
		if !m.selectName(c) {
			m.filter = f
			m.msg = "no command or workspace: " + c
		}
	}
	return m, nil
}

// key handles a key on the list.
func (m home) key(k string) (tea.Model, tea.Cmd) {
	m.msg = ""
	m.want = ""
	if c := m.cur(); c != nil {
		m.want = c.Name
	}
	n := len(m.shown())
	switch k {
	case "q":
		return m, tea.Quit
	case "esc":
		m.filter = ""
		if !m.selectName(m.want) {
			m.clamp()
		}
	case "down", "j":
		m.sel = min(m.sel+1, max(n-1, 0))
	case "up", "k":
		m.sel = max(m.sel-1, 0)
	case "g":
		m.sel = 0
	case "G":
		m.sel = max(n-1, 0)
	case "/":
		m.mode, m.input = homeFilter, m.filter
	case ":":
		m.mode, m.input = homeCommand, ""
	case "?":
		m.mode = homeHelp
	case "ctrl+r":
		m.msg = "refreshing"
		c := m.load()
		return m, c
	case "n":
		m.cmd = "new"
		return m, tea.Quit
	}
	c := m.cur()
	if c == nil {
		return m, nil
	}
	switch k {
	case "d":
		m.mode = homeDescribe
	case "enter", "o", "v", "u":
		action := map[string]string{"enter": "opencode", "o": "opencode", "v": "vscode", "u": "up"}[k]
		if f := m.first(c, action); f != nil {
			m.mode, m.fr = homeFirst, f
			return m, nil
		}
		if action == "up" {
			return m, m.exec(c.Name, "up")
		}
		m.cmd = action
		return m, tea.Quit
	case "w":
		return m, m.sub("switch "+c.Name, "switch", c.Name)
	case "s", "r":
		cmd := map[string]string{"s": "stop", "r": "rm"}[k]
		return m, m.exec(c.Name, cmd)
	}
	return m, nil
}

// exec runs `zero -c NAME CMD` with the terminal handed over, so the command
// asks and prints as it always does, and comes back to the screen.
func (m home) exec(name, cmd string) tea.Cmd {
	return m.sub(cmd+" "+name, "-c", name, cmd)
}

// sub runs zero with args, the terminal handed over, and reports under what.
func (m home) sub(what string, args ...string) tea.Cmd {
	return m.subEnv(what, nil, args...)
}

func (m home) subEnv(what string, env []string, args ...string) tea.Cmd {
	var buf bytes.Buffer
	c := exec.Command(m.exe, args...)
	if env != nil {
		c.Env = append(os.Environ(), env...)
	}
	c.Stderr = io.MultiWriter(os.Stderr, &buf)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		last := ""
		for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				last = l
			}
		}
		return homeRan{what, last, err}
	})
}

func pad(s string, w int) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func (m home) header() []string {
	current := "-"
	for _, r := range m.rows {
		if r.Current {
			current = r.Name
		}
	}
	info := [][2]string{{"Workspace", current}, {"Runtime", m.runtime}, {"Gateway", m.gateway}, {"Zero", strings.Fields(Version() + " -")[0]}}
	lines := make([]string, 6)
	for i := range lines {
		if i < len(info) {
			lines[i] = cLabel.Render(fmt.Sprintf("%-11s", info[i][0]+":")) + lipgloss.NewStyle().MaxWidth(16).Render(info[i][1])
		}
		lines[i] = pad(lines[i], 28)
	}
	left := 28
	if m.w >= 78 {
		for i := range lines {
			for _, h := range []keyHint{homeHints[i], homeHints[i+6]} {
				lines[i] += pad(cKey.Render("<"+h.key+">")+" "+h.what, 18)
			}
		}
		left += 36
	}
	if m.w >= 48 {
		gap := strings.Repeat(" ", max(m.w-left-14, 0))
		for i, l := range strings.Split(logo, "\n") {
			word := "    "
			if i == 2 {
				word = cHead.Render("Z3R0")
			}
			lines[i] += gap + word + " " + cLogo.Render(l)
		}
	}
	return lines
}

// box frames the lines, with the title in the top edge, to w by h cells.
func box(title string, lines []string, w, h int) string {
	in := w - 2
	t := lipgloss.NewStyle().MaxWidth(max(in-2, 0)).Render(" " + title + " ")
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

// table is the header line and the rows that fit in h lines, scrolled so
// that the selected row is among them.
func (m home) table(w, h int) []string {
	rs := m.shown()
	nw := len("NAME")
	for _, r := range rs {
		nw = max(nw, lipgloss.Width(r.Name))
	}
	line := func(name, status, dirs, token, policy string) string {
		return " " + pad(name, nw+1) + fmt.Sprintf("  %-8s  %-4s  %-14s  %s", status, dirs, token, policy)
	}
	out := []string{cHead.Render(line("NAME", "STATUS", "DIRS", "TOKEN", "POLICY"))}
	if !m.ready {
		return append(out, cFaint.Render(" loading..."))
	}
	off := max(m.sel-max(h-1, 1)+1, 0)
	for i, r := range rs {
		if i < off {
			continue
		}
		name := r.Name
		if r.Current {
			name += "*"
		}
		l := pad(line(name, r.Status, strconv.Itoa(r.Folders), r.Token, r.Policy), w)
		if i == m.sel {
			l = cSel.Render(l)
		} else {
			l = cStatus[r.Status].Render(l)
		}
		out = append(out, l)
	}
	return out
}

func (m home) describe(r *wsRow) []string {
	out := []string{"", " " + cLabel.Render("Status:  ") + r.Status, " " + cLabel.Render("Token:   ") + r.Token, " " + cLabel.Render("Policy:  ") + r.Policy}
	list := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		out = append(out, "", " "+cLabel.Render(title+":"))
		for _, i := range items {
			out = append(out, "   "+i)
		}
	}
	if w := m.ws[r.Name]; w != nil {
		if w.Model != "" {
			out = append(out, " "+cLabel.Render("Model:   ")+w.Model)
		}
		list("Directories", w.Folders)
		var ps []*policy.Policy
		for _, d := range w.Folders {
			p, err := policy.LoadDir(d)
			if err != nil {
				out = append(out, "", " "+cLabel.Render("Policy file: ")+err.Error())
				continue
			}
			ps = append(ps, p)
		}
		u := policy.Union(ps...)
		list("Permissions (.zero/policy.yaml of the directories)", u.Capabilities)
		list("Hosts allowed besides the forge", u.Network.Allow)
		list("Tools", w.Tools)
		list("Models", w.Models)
	}
	return append(out, "", cFaint.Render(" esc back"))
}

// frame is the whole screen as text.
func (m home) frame() string {
	if m.w == 0 {
		return ""
	}
	if m.w < 20 || m.h < 8 {
		return "zero: the window is too small"
	}
	var out []string
	headH := 0
	// the first run needs the room: every option on one screen
	if m.h >= 16 && m.mode != homeFirst {
		out = append(out, m.header()...)
		headH = 6
	}
	switch m.mode {
	case homeFilter:
		out = append(out, cKey.Render("/")+m.input+"█")
		headH++
	case homeCommand:
		out = append(out, cKey.Render(":")+m.input+"█")
		headH++
	}
	bh := m.h - headH - 1
	switch c := m.cur(); {
	case m.mode == homeDescribe && c != nil:
		out = append(out, box("describe("+c.Name+")", m.describe(c), m.w, bh))
	case m.mode == homeFirst && m.fr != nil:
		out = append(out, box("first run("+m.fr.name+")", m.firstLines(m.w-2, bh-2), m.w, bh))
	case m.mode == homeHelp:
		lines := []string{""}
		for i := 0; i < 6; i++ {
			l := " "
			for _, h := range []keyHint{homeHints[i], homeHints[i+6]} {
				l += pad(pad(cKey.Render("<"+h.key+">"), 10)+h.what, 23)
			}
			lines = append(lines, l)
		}
		lines = append(lines, " "+pad(cKey.Render("<j/k>"), 10)+"move, g first, G last",
			" :NAME goes there, :all, :new, :q", " "+pad(cKey.Render("<ctrl-r>"), 10)+"reload now", "", cFaint.Render(" esc back"))
		out = append(out, box("help", lines, m.w, bh))
	default:
		scope := "all"
		if m.filter != "" {
			scope = "/" + m.filter
		}
		out = append(out, box(fmt.Sprintf("workspaces(%s)[%d]", scope, len(m.shown())), m.table(m.w-2, bh-2), m.w, bh))
	}
	status := m.msg
	if status == "" && len(m.warn) > 0 {
		status = m.warn[0]
	}
	out = append(out, cFaint.Render(lipgloss.NewStyle().MaxWidth(m.w).Render(" "+status)))
	return strings.Join(out, "\n")
}

func (m home) View() tea.View {
	v := tea.NewView(m.frame())
	v.AltScreen = true
	v.WindowTitle = "zero"
	return v
}

// Interactive reports whether zero talks to a person at a terminal.
func Interactive() bool { return core.IsTerminal(os.Stdin) && core.IsTerminal(os.Stdout) }

// Home shows the workspace screen until the person picks something to run
// outside it; it returns the arguments for zero, nil to leave.
func Home(exe string) ([]string, error) {
	m := home{exe: exe, runtime: "-", gateway: "-", loading: true}
	if b, err := os.ReadFile("/var/lib/zero/runtime"); err == nil && strings.TrimSpace(string(b)) != "" {
		m.runtime = strings.TrimSpace(string(b))
	}
	for {
		m.cmd, m.mode, m.fr = "", homeList, nil
		res, err := tea.NewProgram(m).Run()
		if err != nil {
			return nil, err
		}
		m = res.(home)
		switch c := m.cur(); {
		case m.cmd == "new":
			args, err := newWorkspaceArgs()
			if err != nil {
				return nil, err
			}
			if args == nil {
				continue
			}
			p := exec.Command(exe, args...)
			p.Stdin, p.Stdout, p.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "zero %s: %v\nenter to go on ", strings.Join(args, " "), err)
				fmt.Scanln()
			}
		case m.cmd != "" && c != nil:
			return []string{"-c", c.Name, m.cmd}, nil
		default:
			return nil, nil
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
