package openshell

import (
	"fmt"

	"github.com/Breee/z3r0/internal/core"
	"go.yaml.in/yaml/v3"
)

// Matcher is the policy form of one egress rule.
func Matcher(r core.EgressRule) map[string]any {
	m := map[string]any{"method": r.Method, "path": r.Path}
	if len(r.Query) > 0 {
		m["query"] = r.Query
	}
	return m
}

// PolicyWith returns the policy file base with network rule name set to
// rule; a nil rule removes it.
func PolicyWith(base []byte, name string, rule map[string]any) ([]byte, error) {
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
