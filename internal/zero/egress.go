package zero

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

// EgressRule matches requests by method and path glob ("*" within a
// segment, "**" across segments, at least one). Query is optional.
type EgressRule struct {
	Method string
	Path   string
	Query  map[string]string
}

const forgeRule = "forge"

// forgeBinaries may reach the forge with the token.
var forgeBinaries = []string{"/usr/bin/git", "/usr/lib/git-core/git-remote-https", "/usr/bin/curl", "/usr/local/bin/glab", opencodeBin}

// forgePolicy is the sandbox's network rule for its forge: only the
// workspace's projects on the forge host, and the push gate. The token is
// bound to these two endpoints and to no other (#52).
func forgePolicy(f Forge, provider string, paths []string) (map[string]any, error) {
	allow, deny, err := f.Egress(paths)
	if err != nil {
		return nil, err
	}
	var rules, denies []map[string]any
	for _, r := range allow {
		rules = append(rules, map[string]any{"allow": matcher(r)})
	}
	for _, r := range deny {
		denies = append(denies, matcher(r))
	}
	bind := map[string]any{"provider": provider}
	var bins []map[string]any
	for _, b := range forgeBinaries {
		bins = append(bins, map[string]any{"path": b})
	}
	return map[string]any{
		"name": forgeRule,
		"endpoints": []map[string]any{{
			"host": f.Host(), "port": 443, "protocol": "rest", "enforcement": "enforce",
			"allow_encoded_slash": true, "credential_binding": bind,
			"rules": rules, "deny_rules": denies,
		}, {
			"host": gateHost, "port": gatePort, "protocol": "rest", "enforcement": "enforce",
			"access": "read-write", "credential_binding": bind,
		}},
		"binaries": bins,
	}, nil
}

func matcher(r EgressRule) map[string]any {
	m := map[string]any{"method": r.Method, "path": r.Path}
	if len(r.Query) > 0 {
		m["query"] = r.Query
	}
	return m
}

// policyWith returns the policy file base with network rule name set to
// rule; a nil rule removes it.
func policyWith(base []byte, name string, rule map[string]any) ([]byte, error) {
	var p map[string]any
	if err := yaml.Unmarshal(base, &p); err != nil {
		return nil, fmt.Errorf("policy: %w", err)
	}
	np, _ := p["network_policies"].(map[string]any)
	if np == nil {
		np = map[string]any{}
	}
	if rule == nil {
		delete(np, name)
	} else {
		np[name] = rule
	}
	p["network_policies"] = np
	return yaml.Marshal(p)
}
