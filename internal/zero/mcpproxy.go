package zero

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// What goes into the proxy's config and env file is checked first: a host or
// token with other characters would break out of its line.
var (
	mcpHostRe  = regexp.MustCompile(`^[A-Za-z0-9.-]+(:[0-9]+)?$`)
	mcpTokenRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// mcpEntries lists the entries of every sandbox that holds a tool token.
func mcpEntries() ([]mcpEntry, error) {
	dirs, _ := filepath.Glob(filepath.Join(stateRoot(), "zero-*"))
	sort.Strings(dirs)
	var out []mcpEntry
	for _, d := range dirs {
		tok, host := readState(d, "mcp.token"), readState(d, "forge.host")
		if tok == "" {
			continue
		}
		n := filepath.Base(d)
		if !mcpHostRe.MatchString(host) || !mcpTokenRe.MatchString(tok) {
			return nil, fmt.Errorf("%s: tool token or forge host not usable for the model proxy; zero -c %s up makes them anew", n, strings.TrimPrefix(n, "zero-"))
		}
		out = append(out, mcpEntry{mcpName(n), host, mcpEnvName(n), tok})
	}
	return out, nil
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
	// The marker lets revokeUnrecorded find the token if zero dies before
	// its ID is written down.
	if err := s.write("forge.minting", "", 0o600); err != nil {
		return err
	}
	tok, nid, err := f.Mint(tokenLabel(s.Name), s.forgePaths(), perms)
	if err != nil {
		return fmt.Errorf("token for the workspace's tools: %w", err)
	}
	// The token file appears whole or not at all: another zero up reads it
	// without this sandbox's lock. The ID comes last.
	tmp := s.path("mcp.token.new")
	err = os.WriteFile(tmp, []byte(tok), 0o600)
	if err == nil {
		err = os.Rename(tmp, s.path("mcp.token"))
	}
	for _, kv := range [][2]string{{"mcp.tools", want}, {"mcp.minted", time.Now().UTC().Format(time.RFC3339)}, {"mcp.id", nid}} {
		if err == nil {
			err = s.write(kv[0], kv[1], 0o600)
		}
	}
	if err != nil {
		f.Revoke(nid)
		return err
	}
	os.Remove(s.path("forge.minting"))
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
	for _, n := range []string{"mcp.id", "mcp.token", "mcp.tools", "mcp.minted"} {
		os.Remove(s.path(n))
	}
	return nil
}

// mcpAged says whether the tool token may have ended: the forge ends it one
// to two days after it was made, and only zero up makes a new one.
func (s *Sandbox) mcpAged() bool {
	t, err := time.Parse(time.RFC3339, s.read("mcp.minted"))
	return err != nil || time.Since(t) > 24*time.Hour
}
