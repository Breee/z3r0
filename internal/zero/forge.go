package zero

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Token is an active forge token as listed by the forge.
type Token struct {
	ID, Name string
	Expires  time.Time
}

// renewBefore is how long before expiry zero up re-mints the token.
const renewBefore = 12 * time.Hour

// tokenLabel names tokens <sandbox>@<host>, so orphans of this machine are
// recognisable without touching tokens of other machines.
func tokenLabel(sandbox string) string {
	h, _ := os.Hostname()
	return sandbox + "@" + h
}

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
	// Permissions maps policy capabilities to token permissions; unknown
	// capabilities are an error.
	Permissions(caps []string) ([]string, error)
	// Mint creates a short-lived token scoped to path; id is used to revoke.
	Mint(label, path string, perms []string) (token, id string, err error)
	// Revoke revokes a token by id; an expired or revoked token is not an error.
	Revoke(id string) error
	// Tokens lists the user's active tokens.
	Tokens() ([]Token, error)
	// MCP is the forge's remote MCP endpoint, or "" if it has none. The agent
	// reaches it with "Authorization: Bearer <token placeholder>".
	MCP() string
	// Env is extra non-secret sandbox env the forge's tools need.
	Env() []string
	// Guardrails lists missing DevOps protections on the project (ADR 0024):
	// default branch push-protected, merge only with green pipeline.
	Guardrails(path string) (missing []string, err error)
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
func (s *Sandbox) setupForge(caps []string) (string, error) {
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
	if err := revokeOrphans(f); err != nil {
		return "", err
	}
	perms, err := f.Permissions(caps)
	if err != nil {
		return "", err
	}
	tok, id, err := f.Mint(tokenLabel(s.Name), path, perms)
	if err != nil {
		return "", fmt.Errorf("%s: %w", f.Name(), err)
	}
	for k, v := range map[string]string{"forge": f.Name(), "forge.id": id, "forge.path": path} {
		if err := s.write(k, v, 0o600); err != nil {
			return "", err
		}
	}
	prov := s.forgeProvider(f)
	if err := rt.BindCredential(prov, "zero-"+f.Name(), f.TokenEnv(), tok); err != nil {
		return "", err
	}
	return prov, nil
}

func importProfile(f Forge) error {
	if err := rt.ImportCredentialType("zero-"+f.Name(), f.Profile()); err != nil {
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
	rt.UnbindCredential(s.forgeProvider(f))
	os.Remove(s.path("forge.id"))
	return nil
}

// renewForge re-mints the token of a running sandbox when it expires within
// renewBefore. New processes get the new token; running ones keep the old (C44).
func (s *Sandbox) renewForge(caps []string) error {
	f, id, path := forgeByName(s.read("forge")), s.read("forge.id"), s.read("forge.path")
	if f == nil || id == "" {
		return nil
	}
	toks, err := f.Tokens()
	if err != nil {
		return err
	}
	for _, t := range toks {
		if t.ID == id && time.Until(t.Expires) > renewBefore {
			return nil
		}
	}
	perms, err := f.Permissions(caps)
	if err != nil {
		return err
	}
	tok, nid, err := f.Mint(tokenLabel(s.Name), path, perms)
	if err != nil {
		return fmt.Errorf("%s: %w", f.Name(), err)
	}
	probe := "printf %s \"$" + f.TokenEnv() + "\""
	before, err := s.ssh(probe)
	if err != nil {
		f.Revoke(nid)
		return err
	}
	if err := rt.RotateCredential(s.forgeProvider(f), f.TokenEnv(), tok); err != nil {
		f.Revoke(nid)
		return err
	}
	if err := s.write("forge.id", nid, 0o600); err != nil {
		return err
	}
	// The sandbox polls provider revisions; keep the old token valid until new
	// sessions resolve the new placeholder.
	for i := 0; ; i++ {
		if now, err := s.ssh(probe); err == nil && now != before {
			break
		}
		if i == 60 {
			return fmt.Errorf("sandbox did not pick up new %s token in 60s; old token %s kept", f.Name(), id)
		}
		time.Sleep(time.Second)
	}
	if err := f.Revoke(id); err != nil {
		return fmt.Errorf("revoke old %s token %s: %w", f.Name(), id, err)
	}
	return nil
}

// revokeOrphans revokes this machine's zero tokens that no live sandbox owns,
// e.g. left behind by a crash or a sandbox deleted outside zero.
func revokeOrphans(f Forge) error {
	toks, err := f.Tokens()
	if err != nil {
		return err
	}
	suffix := tokenLabel("")
	for _, t := range toks {
		name, ok := strings.CutSuffix(t.Name, suffix)
		if !ok || !strings.HasPrefix(name, "zero-") {
			continue
		}
		b, _ := os.ReadFile(filepath.Join(stateRoot(), name, "forge.id"))
		if strings.TrimSpace(string(b)) == t.ID && rt.Exists(name) {
			continue
		}
		if err := f.Revoke(t.ID); err != nil {
			return fmt.Errorf("revoke orphan %s token %s (%s): %w", f.Name(), t.ID, t.Name, err)
		}
	}
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
