package zero

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Breee/z3r0/internal/core"
	"github.com/Breee/z3r0/internal/policy"
	"github.com/Breee/z3r0/internal/sandbox/openshell"
	"github.com/Breee/z3r0/internal/workspace"
)

// wsRow is one workspace with its live state.
type wsRow struct {
	Name, Status, Token, Policy string
	Folders                     int
	Current                     bool
}

// overview is the live state behind `zero ls` and `zero switch`.
type overview struct {
	c          *workspace.Config
	rows       []wsRow
	tokens     map[string][]core.Token
	liveTokens map[string]bool
	sandboxes  map[string][]string
	dirs       []string
	warn       []string
}

func loadOverview() (*overview, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(c.Workspaces))
	for n := range c.Workspaces {
		names = append(names, n)
	}
	sort.Strings(names)

	// Forges in use: recorded in a state dir, or the remote of a directory.
	forges := map[string]core.ForgeHost{}
	dirs, _ := filepath.Glob(filepath.Join(core.StateRoot(), "zero-*"))
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
	// The forges are asked while the gateways are: they take the longest.
	type listed struct {
		toks []core.Token
		err  error
	}
	asked := map[string]chan listed{}
	for k, f := range forges {
		ch := make(chan listed, 1)
		asked[k] = ch
		go func() {
			toks, err := f.Tokens()
			ch <- listed{toks, err}
		}()
	}
	var warn []string

	// Sandboxes per gateway ("" = default).
	sandboxes := map[string][]string{}
	for _, gw := range []string{"", openshell.VMGateway} {
		withGateway(gw, func() {
			if !rt.Reachable() {
				if gw == "" {
					warn = append(warn, "gateway not reachable: sandbox status unknown; check: zero doctor")
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

	tokens := map[string][]core.Token{}
	for _, k := range slices.Sorted(mapKeys(asked)) {
		l := <-asked[k]
		if l.err != nil {
			warn = append(warn, fmt.Sprintf("%s tokens not listed: %v", k, l.err))
			continue
		}
		tokens[k] = l.toks
	}

	var rows []wsRow
	liveTokens := map[string]bool{}
	for _, n := range names {
		sb := "zero-" + n
		dir := filepath.Join(core.StateRoot(), sb)
		gw := readState(dir, "gateway")
		status := "none"
		if l, ok := sandboxes[gw]; !ok {
			status = "unknown"
		} else if slices.Contains(l, sb) {
			status = "running"
			withGateway(gw, func() {
				if rt.Stopped(sb) {
					status = "paused"
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
		if core.FileExists(filepath.Join(dir, "policy.sha")) {
			policy = sandboxOf(c, n).policyState()
		}
		rows = append(rows, wsRow{Name: n, Status: status, Token: token, Policy: policy,
			Folders: len(c.Workspaces[n].Folders), Current: n == c.Current})
	}
	return &overview{c, rows, tokens, liveTokens, sandboxes, dirs, warn}, nil
}

// policyState compares the policy in the workspace's directories with the one
// the sandbox runs with: "ok", or "changed" until the next zero up applies it
// (also when the policy cannot be read; zero up says why).
func (s *Sandbox) policyState() string {
	p, err := s.loadPolicy()
	if err != nil {
		return "changed"
	}
	old := &policy.Policy{Capabilities: policy.DefaultCaps}
	if b, err := os.ReadFile(s.path("policy.approved")); err == nil {
		if old, err = policy.Parse(b); err != nil {
			return "changed"
		}
	}
	same := func(a, b []string) bool {
		return slices.Equal(slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b)))
	}
	if same(old.Capabilities, p.Capabilities) && same(old.Network.Allow, p.Network.Allow) {
		return "ok"
	}
	return "changed"
}

// lsLegend explains the table; it goes to stderr, the rows stay as they are
// for scripts.
const lsLegend = "legend: * current workspace; TOKEN: forge and time left; POLICY: ok, or changed = the directories' .zero/policy.yaml differs from what runs; zero up applies it or says why not"

// writeRows prints the workspace table.
func writeRows(out io.Writer, rows []wsRow) error {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  WORKSPACE\tSTATUS\tDIRS\tTOKEN\tPOLICY")
	for _, r := range rows {
		mark := " "
		if r.Current {
			mark = "*"
		}
		fmt.Fprintf(tw, "%s %s\t%s\t%d\t%s\t%s\n", mark, r.Name, r.Status, r.Folders, r.Token, r.Policy)
	}
	return tw.Flush()
}

// Ls prints every workspace with its live state, and what is left over
// without a workspace: tokens, sandboxes, state dirs and input copies.
func Ls(out io.Writer) error {
	o, err := loadOverview()
	if err != nil {
		return err
	}
	var table bytes.Buffer
	if err := writeRows(&table, o.rows); err != nil {
		return err
	}
	t := table.String()
	if Fancy(out) {
		t = styleRows(t, o.rows)
	}
	if _, err := io.WriteString(out, t); err != nil {
		return err
	}
	c, tokens, liveTokens, sandboxes, dirs, warn := o.c, o.tokens, o.liveTokens, o.sandboxes, o.dirs, o.warn

	known := func(sb string) bool {
		n, ok := strings.CutPrefix(sb, "zero-")
		return ok && c.Workspaces[n] != nil
	}
	adopt := func(sb string) string {
		return "; to remove it: zero ctx rm " + strings.TrimPrefix(sb, "zero-")
	}
	var orphans []string
	suffix := tokenLabel("")
	for _, k := range slices.Sorted(mapKeys(tokens)) {
		for _, t := range tokens[k] {
			name, ok := strings.CutSuffix(t.Name, suffix)
			if !ok || !strings.HasPrefix(name, "zero-") || liveTokens[k+"/"+t.ID] {
				continue
			}
			why := "no sandbox); zero-cp revokes it within 10 minutes, else revoke it on the forge"
			if readState(filepath.Join(core.StateRoot(), name), "forge.id") == t.ID {
				why = "no workspace)" + adopt(name)
			}
			orphans = append(orphans, fmt.Sprintf("token %s %s (%s", t.ID, t.Name, why))
		}
	}
	for _, gw := range []string{"", openshell.VMGateway} {
		for _, sb := range sandboxes[gw] {
			if strings.HasPrefix(sb, "zero-") && !known(sb) {
				orphans = append(orphans, fmt.Sprintf("sandbox %s (no workspace)%s", sb, adopt(sb)))
			}
		}
	}
	for _, d := range dirs {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() && !known(filepath.Base(d)) {
			orphans = append(orphans, fmt.Sprintf("state %s (no workspace)%s", d, adopt(filepath.Base(d))))
		}
	}
	staged, _ := filepath.Glob(filepath.Join(inputRoot, "zero-*"))
	for _, d := range staged {
		if !known(filepath.Base(d)) {
			orphans = append(orphans, fmt.Sprintf("input copy %s (no workspace)%s", d, adopt(filepath.Base(d))))
		}
	}
	for _, o := range orphans {
		fmt.Fprintf(out, "leftover: %s\n", o)
	}
	for _, w := range warn {
		fmt.Fprintf(out, "warning: %s\n", w)
	}
	fmt.Fprintln(os.Stderr, lsLegend)
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
	res, err := s.ssh("cd " + core.ShellQuote(dir) + " 2>/dev/null || { echo 'no checkout'; exit 0; }; " + unpushedScript)
	if err != nil {
		return "unknown (sandbox not reachable)"
	}
	if res = strings.TrimSpace(res); res == "" {
		return "none"
	}
	return strings.Join(strings.Split(res, "\n"), ", ")
}

// unpushed is the summary over all repo directories, as printed before a delete.
func (s *Sandbox) unpushed() string {
	var parts []string
	for _, top := range s.repos() {
		parts = append(parts, filepath.Base(top)+": "+s.unpushedIn(top))
	}
	if len(parts) == 0 {
		return "no repo directories"
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
