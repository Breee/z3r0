package zero

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// Tool path through the model proxy (ADR 0055, PoC 27): a workspace with
// models and tools gets its own entry among the proxy's MCP servers. The
// entry points at the forge's MCP endpoint and carries a token of its own,
// minted for the tools only; the token lives in the proxy's environment,
// never in the sandbox. The sandbox's proxy key names the entry and the tools.

// mcpEntry is one MCP server of the model proxy.
type mcpEntry struct{ Name, Host, Env, Token string }

// proxyTools says whether the workspace's tools go through the model proxy.
func (s *Sandbox) proxyTools() bool { return len(s.Models) > 0 && len(s.Tools) > 0 }

// mcpName is the workspace's entry among the proxy's MCP servers.
func mcpName(sandbox string) string {
	// LiteLLM refuses a '-' in a server name; workspace names hold no '_'.
	return "gitlab_" + strings.ReplaceAll(strings.TrimPrefix(sandbox, "zero-"), "-", "_")
}

// mcpEnvName is the proxy's environment variable holding the entry's token.
func mcpEnvName(sandbox string) string {
	n := strings.ToUpper(strings.TrimPrefix(sandbox, "zero-"))
	return "ZERO_MCP_" + strings.ReplaceAll(n, "-", "_")
}

// mcpEntries lists the entries of every sandbox that holds a tool token.
func mcpEntries() []mcpEntry {
	dirs, _ := filepath.Glob(filepath.Join(stateRoot(), "zero-*"))
	sort.Strings(dirs)
	var out []mcpEntry
	for _, d := range dirs {
		tok, host := readState(d, "mcp.token"), readState(d, "forge.host")
		if tok == "" || host == "" {
			continue
		}
		n := filepath.Base(d)
		out = append(out, mcpEntry{mcpName(n), host, mcpEnvName(n), tok})
	}
	return out
}

// mcpConfig is the mcp_servers block of the proxy's config.
func mcpConfig(entries []mcpEntry) string {
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

// mcpEnv is the env file that hands the entries' tokens to the proxy.
func mcpEnv(entries []mcpEntry) string {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%s=%s\n", e.Env, e.Token)
	}
	return b.String()
}

// syncMCP makes the workspace's tool token match its tool list: minted when
// missing, when the list changed or when it ends soon; revoked when the
// tools no longer go through the proxy.
func (s *Sandbox) syncMCP() error {
	f := s.forge()
	if !s.proxyTools() || f == nil {
		return s.revokeMCP()
	}
	want, id := strings.Join(s.Tools, " "), s.read("mcp.id")
	if id != "" && s.read("mcp.tools") == want && s.read("mcp.token") != "" {
		toks, err := f.Tokens()
		if err != nil {
			return err
		}
		for _, t := range toks {
			if t.ID == id && time.Until(t.Expires) > renewBefore {
				return nil
			}
		}
	}
	perms, err := gitlabToolPermissions(s.Tools)
	if err != nil {
		return err
	}
	if !slices.Contains(perms, "read_project") {
		perms = append([]string{"read_project"}, perms...)
	}
	tok, nid, err := f.Mint(tokenLabel(s.Name), s.forgePaths(), perms)
	if err != nil {
		return fmt.Errorf("token for the workspace's tools: %w", err)
	}
	for k, v := range map[string]string{"mcp.id": nid, "mcp.token": tok, "mcp.tools": want} {
		if err := s.write(k, v, 0o600); err != nil {
			f.Revoke(nid)
			return err
		}
	}
	if id != "" {
		return f.Revoke(id)
	}
	return nil
}

// revokeMCP revokes the workspace's tool token and forgets it.
func (s *Sandbox) revokeMCP() error {
	id := s.read("mcp.id")
	if id != "" {
		f := s.forge()
		if f == nil {
			return fmt.Errorf("tool token %s is not revoked, the forge is not recorded: revoke it on the forge", id)
		}
		if err := f.Revoke(id); err != nil {
			return err
		}
	}
	for _, n := range []string{"mcp.id", "mcp.token", "mcp.tools"} {
		os.Remove(s.path(n))
	}
	return nil
}
