package zero

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/Breee/z3r0/internal/core"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// firstItem is one tick box of the first run: a package source or a permission.
type firstItem struct {
	host  bool
	key   string
	label string
	on    bool
}

// firstView is the first run of a workspace, asked inside the screen.
type firstView struct {
	name, action string
	repos, dirs  []string
	found        map[string][]preset
	items        []firstItem
	cur          int
	preview      bool
	top          int
	more         bool // the preview has lines below the window
	files        map[string][]byte
	shown        *Policy
	err          string
}

// firstSandbox is enough of a sandbox to know the repositories and the state
// directory of a workspace.
func (m home) firstSandbox(name string) *Sandbox {
	sb := "zero-" + name
	return &Sandbox{Name: sb, Folders: m.ws[name].Folders, State: filepath.Join(core.StateRoot(), sb)}
}

// first opens the first run when the workspace has no sandbox yet and a
// directory without a policy; nil means: go on with the action.
func (m home) first(r *wsRow, action string) *firstView {
	if r.Status != "none" || m.ws[r.Name] == nil {
		return nil
	}
	repos := m.firstSandbox(r.Name).repos()
	dirs, found, names := firstRunPlan(repos)
	if len(dirs) == 0 {
		return nil
	}
	f := &firstView{name: r.Name, action: action, repos: repos, dirs: dirs, found: found}
	for _, p := range presets {
		for _, n := range names {
			if n == p.Name {
				f.items = append(f.items, firstItem{true, p.Name, presetLabel(p), true})
			}
		}
	}
	for _, c := range defaultCaps {
		f.items = append(f.items, firstItem{false, c, capLabel(c), true})
	}
	for _, c := range optionalCaps {
		f.items = append(f.items, firstItem{false, c, capLabel(c), false})
	}
	return f
}

func (f *firstView) chosen() (hosts, caps []string) {
	for _, it := range f.items {
		switch {
		case !it.on:
		case it.host:
			hosts = append(hosts, it.key)
		default:
			caps = append(caps, it.key)
		}
	}
	return hosts, caps
}

func (m home) firstKey(k string) (tea.Model, tea.Cmd) {
	f := *m.fr
	m.fr = &f
	f.err = ""
	if f.preview {
		switch k {
		case "esc":
			m.fr.preview = false
		case "up", "k":
			m.fr.top = max(f.top-1, 0)
		case "down", "j":
			m.fr.top = f.top + 1
		case "enter":
			if f.more {
				m.fr.top = f.top + 5
				return m, nil
			}
			// what was shown must still be what gets written
			if dirs, _, _ := firstRunPlan(f.repos); !slices.Equal(dirs, f.dirs) {
				m.mode, m.fr = homeList, nil
				m.msg = "first run: the directories changed meanwhile; nothing more written, start again"
				return m, nil
			}
			if err := writeFirstRun(f.dirs, f.files); err != nil {
				m.fr.err = err.Error()
				return m, nil
			}
			sb := m.firstSandbox(f.name).Name
			m.mode, m.fr = homeList, nil
			(&m).selectName(f.name)
			if f.action == "up" {
				v, err := shownValue(f.shown)
				if err != nil {
					m.msg = err.Error()
					return m, nil
				}
				return m, m.subEnv("up "+f.name, []string{shownEnv + "=" + sb + "\n" + v}, "-c", f.name, "up")
			}
			screenShown[sb] = f.shown
			m.cmd = f.action
			return m, tea.Quit
		}
		return m, nil
	}
	switch k {
	case "esc":
		m.mode, m.fr = homeList, nil
	case "up", "k":
		m.fr.cur = max(f.cur-1, 0)
	case "down", "j":
		m.fr.cur = min(f.cur+1, len(f.items)-1)
	case "space", " ", "x":
		items := append([]firstItem(nil), f.items...)
		items[f.cur].on = !items[f.cur].on
		m.fr.items = items
	case "enter":
		hosts, caps := m.fr.chosen()
		if err := checkChosenCaps(caps); err != nil {
			m.fr.err = err.Error()
			return m, nil
		}
		files, shown, err := firstRunFiles(f.repos, f.dirs, f.found, hosts, caps)
		if err != nil {
			m.fr.err = err.Error()
			return m, nil
		}
		m.fr.files, m.fr.shown, m.fr.preview, m.fr.top, m.fr.more = files, shown, true, 0, false
	}
	return m, nil
}

// wrap breaks s into lines of at most w cells, at spaces.
func wrap(s string, w int) []string {
	var out []string
	l := ""
	for _, word := range strings.Fields(s) {
		if l != "" && lipgloss.Width(l+" "+word) > w {
			out = append(out, l)
			l = word
			continue
		}
		if l != "" {
			l += " "
		}
		l += word
	}
	return append(out, l)
}

// firstLines draws the first run into h lines of w cells: the line of the
// current tick box is always among them, the key line is always the last.
func (m home) firstLines(w, h int) []string {
	f := m.fr
	tw := max(w-2, 10)
	var lines []string
	at := 0
	text := func(s string) {
		for _, l := range wrap(s, tw) {
			lines = append(lines, " "+cFaint.Render(l))
		}
	}
	foot := "space toggle · enter next · esc back"
	code := func(s string) {
		for _, l := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
			for lipgloss.Width(l) > tw-3 {
				r := []rune(l)
				n := len(r)
				for lipgloss.Width(string(r[:n])) > tw-3 {
					n--
				}
				lines = append(lines, "   "+string(r[:n]))
				l = "  " + string(r[n:])
			}
			lines = append(lines, "   "+l)
		}
	}
	if f.preview {
		foot = "enter write and start · esc back"
		for _, d := range f.dirs {
			lines = append(lines, " "+cTitle.Render(filepath.Join(d, ".zero", "policy.yaml")))
			code(string(f.files[d]))
		}
		if len(f.repos) > 1 {
			lines = append(lines, " "+cTitle.Render("the workspace policy (every directory together)"))
			code(string(renderPolicy(f.shown)))
		}
	} else {
		head := func(host bool) {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			if host {
				lines = append(lines, " "+cTitle.Render(hostsTitle))
				text(hostsText)
			} else {
				lines = append(lines, " "+cTitle.Render(capsTitle))
				text(capsText)
			}
		}
		for i, it := range f.items {
			if i == 0 || it.host != f.items[i-1].host {
				head(it.host)
			}
			box := "[ ] "
			if it.on {
				box = "[x] "
			}
			for j, l := range wrap(it.label, tw-5) {
				if j > 0 {
					box = "    "
				}
				l = " " + box + l
				if i == f.cur {
					if j == 0 {
						at = len(lines)
					}
					l = cSel.Render(pad(l, tw))
				}
				lines = append(lines, l)
			}
		}
	}
	body := max(h-1, 1)
	if f.err != "" {
		body = max(body-1, 1)
	}
	if len(lines) > body {
		top := min(max(at-body/2, 0), len(lines)-body)
		if f.preview {
			top = min(f.top, len(lines)-body)
			f.top = top
			if f.more = top+body < len(lines); f.more {
				foot = "enter more · esc back"
			}
		}
		lines = lines[top : top+body]
	}
	for len(lines) < body {
		lines = append(lines, "")
	}
	if f.err != "" {
		lines = append(lines, " "+cStatus["stopped"].Render(f.err))
	}
	return append(lines, " "+cFaint.Render(foot))
}
