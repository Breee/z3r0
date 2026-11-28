package zero

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Breee/z3r0/internal/core"
	"github.com/Breee/z3r0/internal/policy"

	"go.yaml.in/yaml/v3"
)

// loadPolicy reads .zero/policy.yaml from the host directories, never from the
// sandbox, so the agent cannot change its own policy. Missing file → default.
// Several directories give the union: the token has one permission set for all
// projects. Any error refuses; there is no fallback to something broader.
func (s *Sandbox) loadPolicy() (*policy.Policy, error) {
	var ps []*policy.Policy
	// Input directories are no policy source (spec §6).
	for _, top := range s.repos() {
		p, err := policy.LoadDir(top)
		if err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	u := policy.Union(ps...)
	if slices.Contains(u.Capabilities, core.CapMRCreate) && !slices.Contains(u.Capabilities, core.CapCIRun) {
		return nil, fmt.Errorf(".zero/policy.yaml: %s needs %s: a merge request opened by the agent starts a pipeline; add %s or take %s out", core.CapMRCreate, core.CapCIRun, core.CapCIRun, core.CapMRCreate)
	}
	if f, _, err := s.folderForge(); err != nil {
		return nil, err
	} else if f != nil {
		if err := policy.CheckForgeHost(f.Host(), u.Network.Allow); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// recordPolicy stores which policy the sandbox was created with: one line
// per directory, "<HEAD sha> [dirty]"; further directories add their name.
func (s *Sandbox) recordPolicy() error {
	var lines []string
	for i, top := range s.repos() {
		sha, _ := core.Sh("git", "-C", top, "rev-parse", "HEAD")
		dirty, _ := core.Sh("git", "-C", top, "status", "--porcelain", "--", ".zero/policy.yaml")
		v := strings.TrimSpace(sha)
		if strings.TrimSpace(dirty) != "" {
			v += " dirty"
		}
		if i > 0 {
			v += " " + filepath.Base(top)
		}
		lines = append(lines, v)
	}
	return s.write("policy.sha", strings.Join(lines, "\n"), 0o600)
}

// projectBinary is the binary scope for network.allow hosts: any process in
// the sandbox (go, git, curl) may read from the listed hosts.
const projectBinary = "/**"

// projectRule is the name of the network rule for network.allow.
const projectRule = "project"

// projectPolicy is the network rule for the hosts of network.allow; nil
// when there is none.
func projectPolicy(hosts []string) map[string]any {
	if len(hosts) == 0 {
		return nil
	}
	var eps []map[string]any
	for _, h := range hosts {
		eps = append(eps, map[string]any{
			"host": h, "port": 443, "protocol": "rest", "enforcement": "enforce", "access": "read-only",
		})
	}
	return map[string]any{
		"name":      projectRule,
		"endpoints": eps,
		"binaries":  []map[string]any{{"path": projectBinary}},
	}
}

// syncProjectPolicy replaces the rule of a running sandbox, so a host taken
// out of network.allow is no longer reachable.
func (s *Sandbox) syncProjectPolicy(p *policy.Policy) error {
	if err := rt.SetRule(s.Name, projectRule, projectPolicy(p.Network.Allow)); err != nil {
		return fmt.Errorf("apply .zero/policy.yaml: %w", err)
	}
	return nil
}

func (s *Sandbox) applyProjectPolicy(p *policy.Policy) error {
	if len(p.Network.Allow) == 0 {
		return nil
	}
	if err := rt.AllowEgress(s.Name, projectRule, projectBinary, "read-only", p.Network.Allow); err != nil {
		return fmt.Errorf("apply .zero/policy.yaml: %w", err)
	}
	return nil
}

// policyChange is what differs between the policy and the last approved one.
type policyChange struct {
	caps      bool     // capability set differs → token must be re-minted
	newHosts  []string // hosts to open
	goneHosts bool     // hosts removed
}

// approvePolicy compares p with the policy last approved for this sandbox
// (state policy.approved; none yet = the default). A wider policy needs a yes
// from the human at the terminal, or --approve. Narrowing needs nothing.
// The yes at the first run covers what was shown there and nothing beyond.
func (s *Sandbox) approvePolicy(p *policy.Policy) (policyChange, error) {
	old := &policy.Policy{Capabilities: policy.DefaultCaps}
	if b, err := os.ReadFile(s.path("policy.approved")); err == nil {
		if old, err = policy.Parse(b); err != nil {
			return policyChange{}, fmt.Errorf("policy.approved: %w", err)
		}
	}
	w := policy.Widened(old, p)
	if len(w) > 0 {
		fmt.Fprintf(os.Stderr, "policy widens for %s:\n", s.Name)
		for _, l := range w {
			fmt.Fprintln(os.Stderr, "  + "+l)
		}
	}
	if s.shown == nil {
		s.shown = s.takeShown()
	}
	seen := s.shown != nil && len(policy.Widened(s.shown, p)) == 0
	if len(w) > 0 && !s.Approve && !seen {
		if !core.IsTerminal(os.Stdin) {
			return policyChange{}, errors.New("policy widening not approved: no terminal to ask at; to accept the lines above: zero up --approve")
		}
		fmt.Fprint(os.Stderr, "approve? [y/N] ")
		ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(ans)); a != "y" && a != "yes" {
			return policyChange{}, errors.New("policy widening not approved; nothing changed")
		}
	}
	ch := policyChange{}
	for _, l := range w {
		if h, ok := strings.CutPrefix(l, "network "); ok {
			ch.newHosts = append(ch.newHosts, h)
		}
	}
	a, b := slices.Sorted(slices.Values(old.Capabilities)), slices.Sorted(slices.Values(p.Capabilities))
	ch.caps = !slices.Equal(a, b)
	for _, h := range old.Network.Allow {
		if !slices.Contains(p.Network.Allow, h) {
			ch.goneHosts = true
		}
	}
	return ch, nil
}

// The yes given in the workspace screen reaches exactly the command the screen
// starts next: in this process through screenShown, in a child through its
// environment. Nothing is stored, so no later run can use it.
const shownEnv = "ZERO_SHOWN_POLICY"

var screenShown = map[string]*policy.Policy{}

func shownValue(p *policy.Policy) (string, error) {
	out, err := yaml.Marshal(p)
	return string(out), err
}

func (s *Sandbox) takeShown() *policy.Policy {
	if p := screenShown[s.Name]; p != nil {
		delete(screenShown, s.Name)
		return p
	}
	v := os.Getenv(shownEnv)
	os.Unsetenv(shownEnv)
	name, body, ok := strings.Cut(v, "\n")
	if !ok || name != s.Name {
		return nil
	}
	var p policy.Policy
	if yaml.Unmarshal([]byte(body), &p) != nil {
		return nil
	}
	return &p
}

func (s *Sandbox) saveApproved(p *policy.Policy) error {
	out, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return s.write("policy.approved", string(out), 0o600)
}
