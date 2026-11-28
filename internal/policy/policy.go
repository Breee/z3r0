package policy

import (
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

var DefaultCaps = []string{
	"git.read", "git.commit", "git.push",
	"gitlab.issue.read", "gitlab.mr.read", "gitlab.pipeline.read",
}

var HostRe = regexp.MustCompile(`^(\*\.)?[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// Union: every host and every capability of ps, each once, in order.
func Union(ps ...*Policy) *Policy {
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

// CheckForgeHost refuses a network.allow entry for the forge host: it would
// add up with the forge rule and open every project on the host again
// (ADR 0046).
func CheckForgeHost(host string, allow []string) error {
	for _, h := range allow {
		if h == host || strings.HasPrefix(h, "*.") && strings.HasSuffix(host, h[1:]) {
			return fmt.Errorf(".zero/policy.yaml: network.allow: %s covers the forge host; zero limits that host to the workspace's projects; take the entry out", h)
		}
	}
	return nil
}

func LoadDir(top string) (*Policy, error) {
	file := filepath.Join(top, ".zero", "policy.yaml")
	b, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return &Policy{Capabilities: DefaultCaps}, nil
	}
	if err != nil {
		return nil, err
	}
	p, err := Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return p, nil
}

func Parse(b []byte) (*Policy, error) {
	var p Policy
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	for _, h := range p.Network.Allow {
		if !HostRe.MatchString(h) {
			return nil, fmt.Errorf("network.allow: invalid host %q", h)
		}
	}
	return &p, nil
}

// Widened returns the entries of p that old does not grant, as "kind value".
func Widened(old, p *Policy) []string {
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
