package zero

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"
)

//go:embed copilot-profile.yaml
var copilotProfileYAML []byte

const copilotEnv = "COPILOT_TOKEN"

func (s *Sandbox) copilotProvider() string { return s.Name + "-copilot" }

// noModel is the line for a sandbox that was created without a model login.
const noModel = "model: the host's opencode had no GitHub Copilot login when this sandbox was created; log in (opencode auth login), then zero rm && zero up"

// modelLogin is the doctor check: "" when the host's opencode is connected to
// GitHub Copilot, else the problem with the command that fixes it.
func modelLogin() string {
	if _, err := exec.LookPath("opencode"); err != nil {
		return "opencode is not on PATH; install it, then: opencode auth login"
	}
	// The output holds secrets: nothing of it goes into the message.
	out, err := exec.Command("opencode", "auth", "export").Output()
	if err != nil {
		return "opencode auth export failed; check: opencode auth list"
	}
	if copilotToken(out) == "" {
		return "the host's opencode has no GitHub Copilot login: opencode auth login"
	}
	return ""
}

// hostCopilotToken returns the Copilot OAuth token of the host's opencode, or
// "" when opencode is missing or not connected to Copilot.
func hostCopilotToken() string {
	out, err := exec.Command("opencode", "auth", "export").Output()
	if err != nil {
		return ""
	}
	return copilotToken(out)
}

// copilotToken reads the token from the output of `opencode auth export`.
func copilotToken(out []byte) string {
	var creds []struct {
		IntegrationID string `json:"integrationID"`
		Active        bool   `json:"active"`
		Value         struct {
			Type   string `json:"type"`
			Access string `json:"access"`
		} `json:"value"`
	}
	if json.Unmarshal(out, &creds) != nil {
		return ""
	}
	for _, c := range creds {
		if c.IntegrationID == "github-copilot" && c.Active && c.Value.Type == "oauth" {
			return c.Value.Access
		}
	}
	return ""
}

// setupCopilot binds the host's Copilot token as a provider (ADR 0029). It
// returns the provider name, or "" when the host has no Copilot login.
func (s *Sandbox) setupCopilot() (string, error) {
	tok := hostCopilotToken()
	if tok == "" {
		return "", nil
	}
	if err := rt.ImportCredentialType("zero-copilot", copilotProfileYAML); err != nil {
		return "", fmt.Errorf("import copilot profile: %w", err)
	}
	if err := rt.BindCredential(s.copilotProvider(), "zero-copilot", copilotEnv, tok); err != nil {
		return "", err
	}
	return s.copilotProvider(), s.write("copilot", "1", 0o600)
}

// importCopilotAuth registers the placeholder as opencode's Copilot credential
// inside the sandbox; opencode sends it as Bearer and the proxy substitutes it.
func (s *Sandbox) importCopilotAuth() error {
	script := `[ -n "$` + copilotEnv + `" ] || { echo "` + copilotEnv + ` not set in sandbox" >&2; exit 1; }; ` +
		// import skips an existing id; after a restart the stored placeholder is stale.
		`opencode auth logout github-copilot cred_zero0copilot0000000000 >/dev/null 2>&1; ` +
		`printf '[{"id":"cred_zero0copilot0000000000","integrationID":"github-copilot","label":"OAuth","active":true,` +
		`"value":{"type":"oauth","methodID":"device","refresh":"%s","access":"%s","expires":0}}]' ` +
		`"$` + copilotEnv + `" "$` + copilotEnv + `" | opencode auth import`
	if _, err := s.ssh(script); err != nil {
		return fmt.Errorf("import copilot credential: %w", err)
	}
	return nil
}
