package zero

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Forge is a git hosting strategy (D20). zero owns the lifecycle — profile
// import, OpenShell provider, sandbox remote, state — and a Forge only knows
// its host and how to mint and revoke a project-scoped token. Add a forge by
// implementing this interface and registering it in forges.
type Forge interface {
	// Name is the short id used for the profile (zero-<name>), provider
	// (<sandbox>-<name>) and state.
	Name() string
	// Host is the https host the token is valid for, e.g. gitlab.com.
	Host() string
	// Match returns the project path of an origin URL on this forge.
	Match(remote string) (path string, ok bool)
	// Profile is the OpenShell provider profile YAML with id zero-<name>.
	Profile() []byte
	// TokenEnv is the env var the sandbox sees the placeholder in.
	TokenEnv() string
	// GitUser is the https username git sends with the token.
	GitUser() string
	// Mint creates a short-lived token scoped to path; id is used to revoke.
	Mint(label, path string) (token, id string, err error)
	// Revoke revokes a token by id; an expired or revoked token is not an error.
	Revoke(id string) error
	// MCP is the forge's remote MCP endpoint, or "" if it has none. The agent
	// reaches it with "Authorization: Bearer <token placeholder>".
	MCP() string
}

var forges = []Forge{gitlab{}}

func forgeFor(remote string) (Forge, string) {
	for _, f := range forges {
		if path, ok := f.Match(strings.TrimSpace(remote)); ok {
			return f, path
		}
	}
	return nil, ""
}

func forgeByName(name string) Forge {
	for _, f := range forges {
		if f.Name() == name {
			return f
		}
	}
	return nil
}

func (s *Sandbox) forgeProvider(f Forge) string { return s.Name + "-" + f.Name() }

// setupForge mints a token for origin's forge and stores it as an OpenShell
// provider. It returns the provider name, or "" when origin is on no known forge.
func (s *Sandbox) setupForge() (string, error) {
	origin, _ := sh("git", "-C", s.Top, "remote", "get-url", "origin")
	f, path := forgeFor(origin)
	if f == nil {
		return "", nil
	}
	if err := importProfile(f); err != nil {
		return "", err
	}
	// A leftover token from a failed create is revoked before minting anew.
	if err := s.revokeForge(); err != nil {
		return "", err
	}
	tok, id, err := f.Mint(s.Name, path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", f.Name(), err)
	}
	for k, v := range map[string]string{"forge": f.Name(), "forge.id": id, "forge.path": path} {
		if err := s.write(k, v, 0o600); err != nil {
			return "", err
		}
	}
	prov := s.forgeProvider(f)
	sh("openshell", "provider", "delete", prov)
	// The token goes in via env lookup, never in argv.
	cmd := exec.Command("openshell", "provider", "create", "--name", prov,
		"--type", "zero-"+f.Name(), "--credential", f.TokenEnv())
	cmd.Env = append(os.Environ(), f.TokenEnv()+"="+tok)
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("provider create: %v: %s", err, strings.TrimSpace(string(b)))
	}
	return prov, nil
}

func importProfile(f Forge) error {
	tmp, err := os.CreateTemp("", "zero-"+f.Name()+"-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(f.Profile()); err != nil {
		return err
	}
	tmp.Close()
	// An existing profile is kept; after editing a profile run
	// `openshell profile delete zero-<forge>` once.
	if _, err := sh("openshell", "profile", "import", "-f", tmp.Name()); err != nil &&
		!strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("import %s profile: %w", f.Name(), err)
	}
	return nil
}

// revokeForge revokes the stored token and deletes the provider.
func (s *Sandbox) revokeForge() error {
	f, id := forgeByName(s.read("forge")), s.read("forge.id")
	if f == nil || id == "" {
		return nil
	}
	if err := f.Revoke(id); err != nil {
		return fmt.Errorf("revoke %s token %s: %w", f.Name(), id, err)
	}
	sh("openshell", "provider", "delete", s.forgeProvider(f))
	os.Remove(s.path("forge.id"))
	return nil
}

// configureRemote points origin in the sandbox at https and makes git send
// the placeholder from the token env; the proxy substitutes the token.
func (s *Sandbox) configureRemote() error {
	f, path := forgeByName(s.read("forge")), s.read("forge.path")
	if f == nil || path == "" {
		return nil
	}
	helper := fmt.Sprintf(`!f() { echo username=%s; echo "password=$%s"; }; f`, f.GitUser(), f.TokenEnv())
	script := "cd " + shellQuote(s.Dir) + " && " +
		"git remote set-url origin " + shellQuote("https://"+f.Host()+"/"+path+".git") + " && " +
		"git config credential.helper " + shellQuote(helper)
	for _, k := range []string{"user.name", "user.email"} {
		if v, err := sh("git", "-C", s.Top, "config", k); err == nil && strings.TrimSpace(v) != "" {
			script += " && git config " + k + " " + shellQuote(strings.TrimSpace(v))
		}
	}
	if _, err := s.ssh(script); err != nil {
		return fmt.Errorf("configure %s remote: %w", f.Name(), err)
	}
	return s.configureMCP(f)
}

// configureMCP registers the forge MCP server in the sandbox's global opencode
// config. The header holds the placeholder; opencode expands {env:...}.
func (s *Sandbox) configureMCP(f Forge) error {
	if f.MCP() == "" {
		return nil
	}
	cfg, _ := json.Marshal(map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"mcp": map[string]any{f.Name(): map[string]any{
			"type": "remote", "url": f.MCP(), "enabled": true, "oauth": false,
			"headers": map[string]string{"Authorization": "Bearer {env:" + f.TokenEnv() + "}"},
		}},
	})
	script := "mkdir -p ~/.config/opencode && printf '%s' " + shellQuote(string(cfg)) + " > ~/.config/opencode/opencode.json"
	if _, err := s.ssh(script); err != nil {
		return fmt.Errorf("configure %s mcp: %w", f.Name(), err)
	}
	return nil
}
