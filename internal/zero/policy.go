package zero

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
