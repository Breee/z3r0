package litellm

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Breee/z3r0/internal/core"
)

// MCPEntry is one MCP server of the model proxy.
type MCPEntry struct{ Name, Host, Env, Token string }

// MCPName is the workspace's entry among the proxy's MCP servers.
func MCPName(sandbox string) string {
	// LiteLLM refuses a '-' in a server name; workspace names hold no '_'.
	return "gitlab_" + strings.ReplaceAll(strings.TrimPrefix(sandbox, "zero-"), "-", "_")
}

// MCPEnvName is the proxy's environment variable holding the entry's token.
func MCPEnvName(sandbox string) string {
	n := strings.ToUpper(strings.TrimPrefix(sandbox, "zero-"))
	return "ZERO_MCP_" + strings.ReplaceAll(n, "-", "_")
}

// What goes into the proxy's config and env file is checked first: a host or
// token with other characters would break out of its line.
var (
	MCPHostRe  = regexp.MustCompile(`^[A-Za-z0-9.-]+(:[0-9]+)?$`)
	MCPTokenRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// MCPEntries lists the entries of every sandbox that holds a tool token.
func MCPEntries() ([]MCPEntry, error) {
	dirs, _ := filepath.Glob(filepath.Join(core.StateRoot(), "zero-*"))
	sort.Strings(dirs)
	var out []MCPEntry
	for _, d := range dirs {
		tok, host := state(d, "mcp.token"), state(d, "forge.host")
		if tok == "" {
			continue
		}
		n := filepath.Base(d)
		if !MCPHostRe.MatchString(host) || !MCPTokenRe.MatchString(tok) {
			return nil, fmt.Errorf("%s: tool token or forge host not usable for the model proxy; zero -c %s up makes them anew", n, strings.TrimPrefix(n, "zero-"))
		}
		out = append(out, MCPEntry{MCPName(n), host, MCPEnvName(n), tok})
	}
	return out, nil
}

// MCPConfig is the mcp_servers block of the proxy's config.
func MCPConfig(entries []MCPEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("mcp_servers:\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "  %s:\n    url: \"https://%s/api/v4/mcp\"\n    transport: \"http\"\n    auth_type: \"bearer_token\"\n    auth_value: os.environ/%s\n", e.Name, e.Host, e.Env)
	}
	return b.String()
}

// MCPEnv is the env file that hands the entries' tokens to the proxy.
func MCPEnv(entries []MCPEntry) string {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%s=%s\n", e.Env, e.Token)
	}
	return b.String()
}

// state reads one state file of a sandbox; empty when it is missing.
func state(dir, name string) string {
	b, _ := os.ReadFile(filepath.Join(dir, name))
	return strings.TrimSpace(string(b))
}
