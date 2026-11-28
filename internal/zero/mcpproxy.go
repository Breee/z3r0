package zero

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Breee/z3r0/internal/forge/gitlab"
)

// Tool path through the model proxy (ADR 0055, PoC 27): a workspace with
// models and tools gets its own entry among the proxy's MCP servers. The
// entry points at the forge's MCP endpoint and carries a token of its own,
// minted for the tools only; the token lives in the proxy's environment,
// never in the sandbox. The sandbox's proxy key names the entry and the tools.

// proxyTools says whether the workspace's tools go through the model proxy.
func (s *Sandbox) proxyTools() bool { return len(s.Models) > 0 && len(s.Tools) > 0 }

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
	perms, err := gitlab.ToolPermissions(s.Tools)
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
