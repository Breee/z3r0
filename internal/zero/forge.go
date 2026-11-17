package zero

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	// ProfileID and Profile are the OpenShell provider profile for this host.
	ProfileID() string
	Profile() []byte
	// LoggedIn is nil when the host session can mint tokens; the error says
	// how to log in.
	LoggedIn() error
	// TokenEnv is the env var the sandbox sees the placeholder in.
	TokenEnv() string
	// GitUser is the https username git sends with the token.
	GitUser() string
	// Permissions maps policy capabilities to token permissions; unknown
	// capabilities are an error.
	Permissions(caps []string) ([]string, error)
	// Mint creates a short-lived token scoped to paths; id is used to revoke.
	Mint(label string, paths []string, perms []string) (token, id string, err error)
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

// forgeFor picks the forge of an origin URL: gitlab.com, or any other host
// glab is logged in to. Other hosts have no forge.
func forgeFor(remote string) (Forge, string) {
	host, path, ok := parseRemote(strings.TrimSpace(remote))
	if !ok {
		return nil, ""
	}
	if g := (gitlab{host}); host == gitlabCom || g.LoggedIn() == nil {
		return g, path
	}
	return nil, ""
}

// forgeOf is the forge recorded in a sandbox's state dir.
func forgeOf(dir string) Forge {
	if readState(dir, "forge") != "gitlab" {
		return nil
	}
	host := readState(dir, "forge.host")
	if host == "" {
		host = gitlabCom
	}
	return gitlab{host}
}

func (s *Sandbox) forge() Forge { return forgeOf(s.State) }

// forgePaths are the project paths the token covers (state forge.path, one
// per line).
func (s *Sandbox) forgePaths() []string { return strings.Fields(s.read("forge.path")) }

// folderForge finds the forge and the project path of every folder; a folder
// on no forge has path "". One workspace has one forge host.
func (s *Sandbox) folderForge() (Forge, map[string]string, error) {
	var forge Forge
	paths := map[string]string{}
	for _, top := range s.Folders {
		origin, _ := sh("git", "-C", top, "remote", "get-url", "origin")
		f, path := forgeFor(origin)
		if f == nil {
			continue
		}
		if forge != nil && f.Host() != forge.Host() {
			return nil, nil, fmt.Errorf("folders are on %s and %s; a workspace has one forge host", forge.Host(), f.Host())
		}
		forge, paths[top] = f, path
	}
	return forge, paths, nil
}

// orderedPaths lists the project paths in folder order.
func (s *Sandbox) orderedPaths(paths map[string]string) []string {
	var out []string
	for _, top := range s.Folders {
		if p := paths[top]; p != "" && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// syncForgePaths records the workspace's project paths and reports whether
// the token must be minted anew for them. The marker forge.stale stays until
// forgeSynced, so a failed mint is retried on the next zero up.
func (s *Sandbox) syncForgePaths() (bool, error) {
	f, byTop, err := s.folderForge()
	if err != nil {
		return false, err
	}
	paths, old := s.orderedPaths(byTop), s.forgePaths()
	if slices.Equal(paths, old) {
		return fileExists(s.path("forge.stale")), nil
	}
	cur := s.forge()
	if cur == nil || len(paths) == 0 || f.Host() != cur.Host() {
		return false, fmt.Errorf("the forge of %s changed (was %v, now %v); recreate: zero rm && zero up", s.Name, old, paths)
	}
	if err := s.write("forge.stale", "1", 0o600); err != nil {
		return false, err
	}
	return true, s.write("forge.path", strings.Join(paths, "\n"), 0o600)
}

// forgeSynced marks the token as minted for the recorded paths.
func (s *Sandbox) forgeSynced() { os.Remove(s.path("forge.stale")) }

func (s *Sandbox) forgeProvider(f Forge) string { return s.Name + "-" + f.Name() }

// setupForge mints a token for origin's forge and stores it as an OpenShell
// provider. It returns the provider name, or "" when origin is on no known forge.
func (s *Sandbox) setupForge(caps []string) (string, error) {
	f, byTop, err := s.folderForge()
	if err != nil {
		return "", err
	}
	if f == nil {
		return "", nil
	}
	paths := s.orderedPaths(byTop)
	if err := f.LoggedIn(); err != nil {
		return "", err
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
	tok, id, err := f.Mint(tokenLabel(s.Name), paths, perms)
	if err != nil {
		return "", fmt.Errorf("%s: %w", f.Name(), err)
	}
	for k, v := range map[string]string{"forge": f.Name(), "forge.host": f.Host(), "forge.id": id, "forge.path": strings.Join(paths, "\n")} {
		if err := s.write(k, v, 0o600); err != nil {
			return "", err
		}
	}
	prov := s.forgeProvider(f)
	if err := rt.BindCredential(prov, f.ProfileID(), f.TokenEnv(), tok); err != nil {
		return "", err
	}
	return prov, nil
}

func importProfile(f Forge) error {
	if err := rt.ImportCredentialType(f.ProfileID(), f.Profile()); err != nil {
		return fmt.Errorf("import %s profile: %w", f.Name(), err)
	}
	return nil
}

// revokeForge revokes the stored token and deletes the provider.
func (s *Sandbox) revokeForge() error {
	f, id := s.forge(), s.read("forge.id")
	if f == nil {
		return nil
	}
	if id != "" {
		if err := f.Revoke(id); err != nil {
			return fmt.Errorf("revoke %s token %s: %w", f.Name(), id, err)
		}
	}
	rt.UnbindCredential(s.forgeProvider(f))
	os.Remove(s.path("forge.id"))
	return nil
}

// pauseForge revokes the token and keeps the binding: the stopped sandbox
// still refers to it, resumeForge fills it again.
func (s *Sandbox) pauseForge() error {
	f, id := s.forge(), s.read("forge.id")
	if f == nil || id == "" {
		return nil
	}
	if err := f.Revoke(id); err != nil {
		return fmt.Errorf("revoke %s token %s: %w", f.Name(), id, err)
	}
	return os.Remove(s.path("forge.id"))
}

// resumeForge mints a token for a paused sandbox. It runs before the sandbox
// starts, so every process sees the new token (no C44).
func (s *Sandbox) resumeForge(caps []string) error {
	f, paths := s.forge(), s.forgePaths()
	if f == nil || s.read("forge.id") != "" {
		return nil
	}
	perms, err := f.Permissions(caps)
	if err != nil {
		return err
	}
	tok, id, err := f.Mint(tokenLabel(s.Name), paths, perms)
	if err != nil {
		return fmt.Errorf("%s: %w", f.Name(), err)
	}
	if err := rt.RotateCredential(s.forgeProvider(f), f.TokenEnv(), tok); err != nil {
		f.Revoke(id)
		return err
	}
	return s.write("forge.id", id, 0o600)
}

// renewForge re-mints the token of a running sandbox when it expires within
// renewBefore. New processes get the new token; running ones keep the old (C44).
func (s *Sandbox) renewForge(caps []string, force bool) error {
	f, id, paths := s.forge(), s.read("forge.id"), s.forgePaths()
	if f == nil || id == "" {
		return nil
	}
	toks, err := f.Tokens()
	if err != nil {
		return err
	}
	for _, t := range toks {
		if !force && t.ID == id && time.Until(t.Expires) > renewBefore {
			return nil
		}
	}
	perms, err := f.Permissions(caps)
	if err != nil {
		return err
	}
	tok, nid, err := f.Mint(tokenLabel(s.Name), paths, perms)
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
		dir := filepath.Join(stateRoot(), name)
		if _, err := os.Stat(dir); err == nil {
			// A sandbox being created or renewed holds its lock: not an orphan.
			l, err := (&Sandbox{Name: name, State: dir}).acquireLock()
			if err != nil {
				continue
			}
			live := false
			withGateway(readState(dir, "gateway"), func() {
				// An unreachable gateway says nothing about the sandbox: keep the token.
				live = readState(dir, "forge.id") == t.ID && (rt.Exists(name) || !rt.Reachable())
			})
			l.release()
			if live {
				continue
			}
		}
		if err := f.Revoke(t.ID); err != nil {
			return fmt.Errorf("revoke orphan %s token %s (%s): %w", f.Name(), t.ID, t.Name, err)
		}
	}
	return nil
}

// configureRemote points origin of the checkout of top at https and makes git
// send the placeholder from the token env; the proxy substitutes the token.
func (s *Sandbox) configureRemote(top, path string) error {
	f := s.forge()
	if f == nil || path == "" {
		return nil
	}
	helper := fmt.Sprintf(`!f() { echo username=%s; echo "password=$%s"; }; f`, f.GitUser(), f.TokenEnv())
	push, err := s.gateURL(path)
	if err != nil {
		return err
	}
	script := "cd " + shellQuote(sandboxDir(top)) + " && " +
		"git remote set-url origin " + shellQuote("https://"+f.Host()+"/"+path+".git") + " && " +
		"git config remote.origin.pushurl " + shellQuote(push) + " && " +
		"git config credential.helper " + shellQuote(helper)
	for _, k := range []string{"user.name", "user.email"} {
		if v, err := sh("git", "-C", top, "config", k); err == nil && strings.TrimSpace(v) != "" {
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
