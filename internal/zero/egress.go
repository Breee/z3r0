package zero

import (
	"github.com/Breee/z3r0/internal/agent/opencode"
	"github.com/Breee/z3r0/internal/core"
	"github.com/Breee/z3r0/internal/sandbox/openshell"
)

const forgeRule = "forge"

// forgeBinaries may reach the forge with the token.
var forgeBinaries = []string{"/usr/bin/git", "/usr/lib/git-core/git-remote-https", "/usr/bin/curl", "/usr/local/bin/glab", opencode.Bin}

// forgePolicy is the sandbox's network rule for its forge: only the
// workspace's projects on the forge host, and the push gate. The token is
// bound to these two endpoints and to no other (#52).
func forgePolicy(f core.ForgeHost, provider string, paths []string) (map[string]any, error) {
	allow, deny, err := f.Egress(paths)
	if err != nil {
		return nil, err
	}
	var rules, denies []map[string]any
	for _, r := range allow {
		rules = append(rules, map[string]any{"allow": openshell.Matcher(r)})
	}
	for _, r := range deny {
		denies = append(denies, openshell.Matcher(r))
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
