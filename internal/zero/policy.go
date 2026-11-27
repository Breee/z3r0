package zero

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Policy is the project policy (docs/architecture/07a-policy-sources-and-lifecycle.md).
type Policy struct {
	Network struct {
		Allow []string `yaml:"allow"`
	} `yaml:"network"`
	Capabilities []string `yaml:"capabilities"`
}

var defaultCaps = []string{
	"git.read", "git.commit", "git.push",
	"gitlab.issue.read", "gitlab.mr.read", "gitlab.pipeline.read",
}

// capMRCreate is not in the default set: opening a merge request starts a
// merge request pipeline, which ci.skip on the push does not prevent. It is
// accepted only together with ci.run (ADR 0044).
const capMRCreate = "gitlab.mr.create"

// capCIRun lets pipelines start on the agent's pushes. Without it (default)
// the push gate adds ci.skip and a human starts the pipeline (ADR 0035).
const capCIRun = "ci.run"

var hostRe = regexp.MustCompile(`^(\*\.)?[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// loadPolicy reads .zero/policy.yaml from the host directories, never from the
// sandbox, so the agent cannot change its own policy. Missing file → default.
// Several directories give the union: the token has one permission set for all
// projects. Any error refuses; there is no fallback to something broader.
func (s *Sandbox) loadPolicy() (*Policy, error) {
	var ps []*Policy
	// Input directories are no policy source (spec §6).
	for _, top := range s.repos() {
		p, err := loadFolderPolicy(top)
		if err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	u := unionPolicy(ps...)
	if slices.Contains(u.Capabilities, capMRCreate) && !slices.Contains(u.Capabilities, capCIRun) {
		return nil, fmt.Errorf(".zero/policy.yaml: %s needs %s: a merge request opened by the agent starts a pipeline; add %s or take %s out", capMRCreate, capCIRun, capCIRun, capMRCreate)
	}
	if f, _, err := s.folderForge(); err != nil {
		return nil, err
	} else if f != nil {
		if err := checkForgeHost(f.Host(), u.Network.Allow); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// unionPolicy: every host and every capability of ps, each once, in order.
func unionPolicy(ps ...*Policy) *Policy {
	u := &Policy{}
	for _, p := range ps {
		for _, h := range p.Network.Allow {
			if !slices.Contains(u.Network.Allow, h) {
				u.Network.Allow = append(u.Network.Allow, h)
			}
		}
		for _, c := range p.Capabilities {
			if !slices.Contains(u.Capabilities, c) {
				u.Capabilities = append(u.Capabilities, c)
			}
		}
	}
	return u
}

// checkForgeHost refuses a network.allow entry for the forge host: it would
// add up with the forge rule and open every project on the host again
// (ADR 0046).
func checkForgeHost(host string, allow []string) error {
	for _, h := range allow {
		if h == host || strings.HasPrefix(h, "*.") && strings.HasSuffix(host, h[1:]) {
			return fmt.Errorf(".zero/policy.yaml: network.allow: %s covers the forge host; zero limits that host to the workspace's projects; take the entry out", h)
		}
	}
	return nil
}

func loadFolderPolicy(top string) (*Policy, error) {
	file := filepath.Join(top, ".zero", "policy.yaml")
	b, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return &Policy{Capabilities: defaultCaps}, nil
	}
	if err != nil {
		return nil, err
	}
	p, err := parsePolicy(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return p, nil
}

func parsePolicy(b []byte) (*Policy, error) {
	var p Policy
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	for _, h := range p.Network.Allow {
		if !hostRe.MatchString(h) {
			return nil, fmt.Errorf("network.allow: invalid host %q", h)
		}
	}
	return &p, nil
}

// recordPolicy stores which policy the sandbox was created with: one line
// per directory, "<HEAD sha> [dirty]"; further directories add their name.
func (s *Sandbox) recordPolicy() error {
	var lines []string
	for i, top := range s.repos() {
		sha, _ := sh("git", "-C", top, "rev-parse", "HEAD")
		dirty, _ := sh("git", "-C", top, "status", "--porcelain", "--", ".zero/policy.yaml")
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
func (s *Sandbox) syncProjectPolicy(p *Policy) error {
	if err := rt.SetRule(s.Name, projectRule, projectPolicy(p.Network.Allow)); err != nil {
		return fmt.Errorf("apply .zero/policy.yaml: %w", err)
	}
	return nil
}

func (s *Sandbox) applyProjectPolicy(p *Policy) error {
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

// widened returns the entries of p that old does not grant, as "kind value".
func widened(old, p *Policy) []string {
	var w []string
	for _, h := range p.Network.Allow {
		if !slices.Contains(old.Network.Allow, h) {
			w = append(w, "network "+h)
		}
	}
	for _, c := range p.Capabilities {
		if !slices.Contains(old.Capabilities, c) {
			w = append(w, "capability "+c)
		}
	}
	return w
}

// approvePolicy compares p with the policy last approved for this sandbox
// (state policy.approved; none yet = the default). A wider policy needs a yes
// from the human at the terminal, or --approve. Narrowing needs nothing.
// The yes at the first run covers what was shown there and nothing beyond.
func (s *Sandbox) approvePolicy(p *Policy) (policyChange, error) {
	old := &Policy{Capabilities: defaultCaps}
	if b, err := os.ReadFile(s.path("policy.approved")); err == nil {
		if old, err = parsePolicy(b); err != nil {
			return policyChange{}, fmt.Errorf("policy.approved: %w", err)
		}
	}
	w := widened(old, p)
	if len(w) > 0 {
		fmt.Fprintf(os.Stderr, "policy widens for %s:\n", s.Name)
		for _, l := range w {
			fmt.Fprintln(os.Stderr, "  + "+l)
		}
	}
	seen := s.shown != nil && len(widened(s.shown, p)) == 0
	if len(w) > 0 && !s.Approve && !seen {
		if !isTerminal(os.Stdin) {
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

// saveApproved records p as approved, once it is applied.
func (s *Sandbox) saveApproved(p *Policy) error {
	out, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return s.write("policy.approved", string(out), 0o600)
}
