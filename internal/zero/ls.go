package zero

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

// Ls prints every workspace with its live state, and what is left over
// without a workspace: tokens, sandboxes, state dirs and input copies.
func Ls(out io.Writer) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}
	names := make([]string, 0, len(c.Workspaces))
	for n := range c.Workspaces {
		names = append(names, n)
	}
	sort.Strings(names)

	// Forges in use: recorded in a state dir, or the remote of a folder.
	forges := map[string]Forge{}
	dirs, _ := filepath.Glob(filepath.Join(stateRoot(), "zero-*"))
	for _, d := range dirs {
		if f := forgeOf(d); f != nil {
			forges[f.Name()+"@"+f.Host()] = f
		}
	}
	for _, n := range names {
		if f, _, err := (&Sandbox{Folders: c.Workspaces[n].Folders}).folderForge(); err == nil && f != nil {
			forges[f.Name()+"@"+f.Host()] = f
		}
	}
	var warn []string
	tokens := map[string][]Token{}
	for k, f := range forges {
		toks, err := f.Tokens()
		if err != nil {
			warn = append(warn, fmt.Sprintf("%s tokens not listed: %v", k, err))
			continue
		}
		tokens[k] = toks
	}

	// Sandboxes per gateway ("" = default).
	sandboxes := map[string][]string{}
	for _, gw := range []string{"", vmGateway} {
		withGateway(gw, func() {
			if !rt.Reachable() {
				if gw == "" {
					warn = append(warn, "gateway not reachable: sandbox status unknown")
				}
				return
			}
			l, err := rt.List()
			if err != nil {
				warn = append(warn, err.Error())
				return
			}
			sandboxes[gw] = l
		})
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  NAME\tSTATUS\tFOLDERS\tTOKEN\tPOLICY")
	liveTokens := map[string]bool{}
	for _, n := range names {
		sb := "zero-" + n
		dir := filepath.Join(stateRoot(), sb)
		gw := readState(dir, "gateway")
		status := "none"
		if l, ok := sandboxes[gw]; !ok {
			status = "unknown"
		} else if slices.Contains(l, sb) {
			status = "running"
			withGateway(gw, func() {
				if rt.Stopped(sb) {
					status = "stopped"
				}
			})
		}
		token := "-"
		if f := forgeOf(dir); f != nil {
			token = f.Name() + " none"
			k := f.Name() + "@" + f.Host()
			if _, ok := tokens[k]; !ok {
				token = f.Name() + " unknown"
			}
			for _, t := range tokens[k] {
				if t.ID == readState(dir, "forge.id") {
					token = f.Name() + " " + until(t.Expires)
					liveTokens[k+"/"+t.ID] = true
				}
			}
		}
		policy := "-"
		if lines := strings.Split(readState(dir, "policy.sha"), "\n"); len(lines[0]) >= 6 {
			policy = lines[0][:6]
			if slices.ContainsFunc(lines, func(l string) bool { return slices.Contains(strings.Fields(l), "dirty") }) {
				policy += " dirty"
			}
		}
		mark := " "
		if n == c.Current {
			mark = "*"
		}
		fmt.Fprintf(tw, "%s %s\t%s\t%d\t%s\t%s\n", mark, n, status, len(c.Workspaces[n].Folders), token, policy)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	known := func(sb string) bool {
		n, ok := strings.CutPrefix(sb, "zero-")
		return ok && c.Workspaces[n] != nil
	}
	var orphans []string
	suffix := tokenLabel("")
	for _, k := range slices.Sorted(mapKeys(tokens)) {
		for _, t := range tokens[k] {
			name, ok := strings.CutSuffix(t.Name, suffix)
			if !ok || !strings.HasPrefix(name, "zero-") || liveTokens[k+"/"+t.ID] {
				continue
			}
			why := "no sandbox"
			if readState(filepath.Join(stateRoot(), name), "forge.id") == t.ID {
				why = "no workspace"
			}
			orphans = append(orphans, fmt.Sprintf("token %s %s (%s)", t.ID, t.Name, why))
		}
	}
	for _, gw := range []string{"", vmGateway} {
		for _, sb := range sandboxes[gw] {
			if strings.HasPrefix(sb, "zero-") && !known(sb) {
				orphans = append(orphans, fmt.Sprintf("sandbox %s (no workspace)", sb))
			}
		}
	}
	for _, d := range dirs {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() && !known(filepath.Base(d)) {
			orphans = append(orphans, fmt.Sprintf("state %s (no workspace)", d))
		}
	}
	staged, _ := filepath.Glob(filepath.Join(inputRoot, "zero-*"))
	for _, d := range staged {
		if !known(filepath.Base(d)) {
			orphans = append(orphans, fmt.Sprintf("input copy %s (no workspace)", d))
		}
	}
	for _, o := range orphans {
		fmt.Fprintf(out, "orphans: %s\n", o)
	}
	for _, w := range warn {
		fmt.Fprintf(out, "warning: %s\n", w)
	}
	return nil
}

func mapKeys[V any](m map[string]V) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// until formats the time left: hours below two days, days above.
func until(t time.Time) string {
	d := time.Until(t)
	switch {
	case d <= 0:
		return "expired"
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// unpushedScript prints one line per thing that exists only in the checkout:
// branches with commits on no remote branch, and uncommitted files.
const unpushedScript = `git for-each-ref --format='%(refname:short)' refs/heads | while read -r b; do ` +
	`n=$(git rev-list --count "$b" --not --remotes); ` +
	`[ "$n" = 0 ] || echo "$b ($n commits)"; done; ` +
	`n=$(git status --porcelain | wc -l); [ "$n" = 0 ] || echo "uncommitted changes ($n files)"`

// unpushedIn reports what only the sandbox checkout of top holds; "none" if
// nothing. Remote branches are as of the last fetch or push in the sandbox.
func (s *Sandbox) unpushedIn(top string) string {
	if !s.exists() {
		return "no sandbox"
	}
	if rt.Stopped(s.Name) {
		return "unknown (sandbox paused)"
	}
	dir := sandboxDir(top)
	res, err := s.ssh("cd " + shellQuote(dir) + " 2>/dev/null || { echo 'no checkout'; exit 0; }; " + unpushedScript)
	if err != nil {
		return "unknown (sandbox not reachable)"
	}
	if res = strings.TrimSpace(res); res == "" {
		return "none"
	}
	return strings.Join(strings.Split(res, "\n"), ", ")
}

// unpushed is the summary over all repo folders, as printed before a delete.
func (s *Sandbox) unpushed() string {
	var parts []string
	for _, top := range s.repos() {
		parts = append(parts, filepath.Base(top)+": "+s.unpushedIn(top))
	}
	if len(parts) == 0 {
		return "no repo folders"
	}
	return strings.Join(parts, "; ")
}

// confirmName asks the user to type name; anything else aborts.
func confirmName(name string) error {
	fmt.Fprintf(os.Stderr, "type workspace name to delete: ")
	ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if strings.TrimSpace(ans) != name {
		return fmt.Errorf("not removed: the name typed is not %q (zero rm -y skips the question)", name)
	}
	return nil
}
