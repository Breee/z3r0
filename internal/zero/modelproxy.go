package zero

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Breee/z3r0/internal/agent/claude"
	"github.com/Breee/z3r0/internal/agent/copilot"
	"github.com/Breee/z3r0/internal/agent/opencode"
	"github.com/Breee/z3r0/internal/core"
	"github.com/Breee/z3r0/internal/model/litellm"
)

// The model proxy (ADR 0049, #162): LiteLLM in a container next to the
// sandbox. It holds the provider logins; a sandbox gets a key of its own
// that is limited to the workspace's models and to no MCP server.

func (s *Sandbox) modelKeyAlias() string { return tokenLabel(s.Name) }

// revokeModelKey deletes the sandbox's key at the proxy. Without a key on
// record it does nothing.
func (s *Sandbox) revokeModelKey() error {
	if s.read("models.key") == "" {
		return nil
	}
	if _, err := litellm.API("", "POST", "/key/delete", map[string]any{"key_aliases": []string{s.modelKeyAlias()}}); err != nil {
		return fmt.Errorf("model key of %s not revoked: %w", s.Name, err)
	}
	return os.Remove(s.path("models.key"))
}

// syncModels sets up the model path of a running sandbox: proxy up, a fresh
// key, the egress rule, the key in opencode's service. It returns the models
// for the opencode config. Without models it takes the path away.
func (s *Sandbox) syncModels(out io.Writer, providers []string) ([]core.Model, error) {
	if len(s.Models) == 0 {
		if err := s.revokeMCP(); err != nil {
			return nil, err
		}
		if s.read("models.key") == "" && s.read("models.provider") == "" {
			return nil, nil
		}
		if err := s.revokeModelKey(); err != nil {
			return nil, err
		}
		if err := rt.SetRule(s.Name, litellm.Rule, nil); err != nil {
			return nil, err
		}
		if err := s.dropModelProvider(); err != nil {
			return nil, err
		}
		_, err := s.ssh("opencode service unset env " + litellm.KeyEnv + " && opencode service restart")
		return nil, err
	}
	if err := s.syncMCP(); err != nil {
		return nil, err
	}
	lap("  tool token")
	mp := litellm.Proxy{Providers: providers, KeyCmds: s.ProviderKeys, Alias: s.modelKeyAlias(), Tools: s.Tools, HadKey: s.read("models.key") != ""}
	if s.read("mcp.id") != "" {
		mp.ToolEntry = litellm.MCPName(s.Name)
	}
	if err := mp.Ensure(out); err != nil {
		return nil, err
	}
	lap("  proxy up")
	// A sandbox that used the Copilot CLI before the workspace got models
	// still has the rule to GitHub's model host: it goes.
	if core.FileExists(s.path("copilot-cli.rule")) {
		if err := rt.SetRule(s.Name, copilot.Rule, nil); err != nil {
			return nil, err
		}
		os.Remove(s.path("copilot-cli.rule"))
	}
	// Nothing changed: the key stays, and with it the sandbox's credential
	// and opencode's service. A sandbox made or started in this run, or one
	// whose credential is not there, is set up again.
	mp.Renew = s.fresh || !mp.HadKey || s.read("models.provider") == "" || rt.CredentialReady(s.Name, s.modelCredential()) != nil
	var provider core.ModelProvider = mp
	a, err := provider.Grant(s.Name, s.Models)
	if err != nil {
		return nil, err
	}
	s.keptKey = a.Credential.Secret == ""
	if s.keptKey {
		s.modelNotes(out, a)
		lap("  key kept")
		return a.Models, nil
	}
	if err := s.write("models.key", "1", 0o600); err != nil {
		return nil, err
	}
	lap("  key renewed")
	if err := s.bindModelKey(a.Credential.Secret); err != nil {
		return nil, err
	}
	// A new session has the placeholder in its environment; that is what
	// opencode's service keeps, not the key.
	lap("  key bound")
	env := a.Credential.Env
	if _, err := s.ssh(`[ -n "$` + env + `" ] && opencode service set env ` + env + ` "$` + env + `" >/dev/null && opencode service restart >/dev/null`); err != nil {
		return nil, fmt.Errorf("model proxy key not handed to opencode in %s: %w; check: zero logs, then zero up", s.Name, err)
	}
	lap("  service restart")
	s.modelNotes(out, a)
	return a.Models, nil
}

func (s *Sandbox) modelNotes(out io.Writer, a core.ModelAccess) {
	fmt.Fprintf(out, "models: %d through the model proxy (%s)\n", len(a.Models), strings.Join(s.Models, ", "))
	for _, w := range a.Warnings {
		fmt.Fprintf(out, "models: %s\n", w)
	}
}

func (s *Sandbox) modelCredential() string { return s.Name + "-models" }

// bindModelKey puts the key outside the sandbox. Order (PoC 20): binding,
// attach, rule that names it, then wait until the sandbox has it. A binding
// from an earlier run only gets the new key.
func (s *Sandbox) bindModelKey(key string) error {
	id := s.modelCredential()
	if s.read("models.provider") == "" || rt.RotateCredential(id, litellm.KeyEnv, key) != nil {
		if err := rt.SetRule(s.Name, litellm.Rule, nil); err != nil {
			return err
		}
		rt.DetachCredential(s.Name, id) // not attached on a first run
		if err := rt.ImportCredentialType("zero-model-key", litellm.KeyProfile); err != nil {
			return err
		}
		if err := rt.BindCredential(id, "zero-model-key", litellm.KeyEnv, key); err != nil {
			return err
		}
		if err := rt.AttachCredential(s.Name, id); err != nil {
			return err
		}
		if err := s.write("models.provider", id, 0o600); err != nil {
			return err
		}
	}
	if err := rt.SetRule(s.Name, litellm.Rule, litellm.Policy(id, modelClients)); err != nil {
		return err
	}
	return rt.CredentialReady(s.Name, id)
}

// dropModelProvider takes the binding away; the rule that names it must be
// gone or the sandbox deleted.
func (s *Sandbox) dropModelProvider() error {
	if s.read("models.provider") == "" {
		return nil
	}
	rt.DetachCredential(s.Name, s.modelCredential()) // fails when the sandbox is gone
	rt.UnbindCredential(s.modelCredential())
	return os.Remove(s.path("models.provider"))
}

// modelClients are the programs in the sandbox that may reach the model proxy.
var modelClients = []string{opencode.Bin, copilot.Bin, claude.Bin, "/usr/bin/curl"}
