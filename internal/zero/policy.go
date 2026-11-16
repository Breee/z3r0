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
	"gitlab.issue.read", "gitlab.mr.read", "gitlab.mr.create", "gitlab.pipeline.read",
}

// capCIRun lets pipelines start on the agent's pushes. Without it (default)
// the push gate adds ci.skip and a human starts the pipeline (ADR 0035).
const capCIRun = "ci.run"

var hostRe = regexp.MustCompile(`^(\*\.)?[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// loadPolicy reads .zero/policy.yaml from the host checkout, never from the
// sandbox, so the agent cannot change its own policy. Missing file → default.
// Any error refuses; there is no fallback to something broader.
func (s *Sandbox) loadPolicy() (*Policy, error) {
	file := filepath.Join(s.Top, ".zero", "policy.yaml")
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

// recordPolicy stores which policy the sandbox was created with.
func (s *Sandbox) recordPolicy() error {
	sha, _ := sh("git", "-C", s.Top, "rev-parse", "HEAD")
	dirty, _ := sh("git", "-C", s.Top, "status", "--porcelain", "--", ".zero/policy.yaml")
	v := strings.TrimSpace(sha)
	if strings.TrimSpace(dirty) != "" {
		v += " dirty"
	}
	return s.write("policy.sha", v, 0o600)
}

// projectBinary is the binary scope for network.allow hosts: any process in
// the sandbox (go, git, curl) may read from the listed hosts.
const projectBinary = "/**"

func (s *Sandbox) applyProjectPolicy(p *Policy) error {
	if len(p.Network.Allow) == 0 {
		return nil
	}
	if err := rt.AllowEgress(s.Name, "project", projectBinary, "read-only", p.Network.Allow); err != nil {
		return fmt.Errorf("apply .zero/policy.yaml: %w", err)
	}
	return nil
}

// policyChange is what differs between the policy and the last approved one.
type policyChange struct {
	caps      bool     // capability set differs → token must be re-minted
	newHosts  []string // hosts to open
	goneHosts bool     // hosts removed (takes effect on recreate)
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
func (s *Sandbox) approvePolicy(p *Policy) (policyChange, error) {
	old := &Policy{Capabilities: defaultCaps}
	if b, err := os.ReadFile(s.path("policy.approved")); err == nil {
		if old, err = parsePolicy(b); err != nil {
			return policyChange{}, fmt.Errorf("policy.approved: %w", err)
		}
	}
	w := widened(old, p)
	if len(w) > 0 && !s.Approve {
		fmt.Fprintf(os.Stderr, "policy widens for %s:\n", s.Name)
		for _, l := range w {
			fmt.Fprintln(os.Stderr, "  + "+l)
		}
		if !isTerminal(os.Stdin) {
			return policyChange{}, errors.New("policy widening not approved (no terminal; use `zero up --approve`)")
		}
		fmt.Fprint(os.Stderr, "approve? [y/N] ")
		ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(ans)); a != "y" && a != "yes" {
			return policyChange{}, errors.New("policy widening not approved")
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
