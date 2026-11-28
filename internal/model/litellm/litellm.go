package litellm

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/Breee/z3r0/internal/core"
)

//go:embed model-key-profile.yaml
var KeyProfile []byte

//go:embed parts/compose.yaml
var ComposeYAML []byte

const (
	Port    = 18096
	Rule    = "models"
	KeyEnv  = "LITELLM_API_KEY"
	User    = "default_user_id" // the proxy's admin, who connects the providers
	Project = "zero-parts"
)

func PartsDir() string { return filepath.Join(core.StateRoot(), "parts") }

func URL() string { return fmt.Sprintf("http://127.0.0.1:%d", Port) }

// Compose is the compose command for the opt-in parts: Docker, until the
// podman here runs health checks (ADR 0051, C58); ZERO_COMPOSE overrides it.
func Compose() []string {
	if v := strings.Fields(os.Getenv("ZERO_COMPOSE")); len(v) > 0 {
		return v
	}
	return []string{"docker", "compose"}
}

// ComposeProblem says why the compose command of the parts does not run.
func ComposeProblem() string {
	c := Compose()
	if _, err := core.Sh(c[0], append(c[1:], "version")...); err != nil {
		return fmt.Sprintf("%s does not run (%s); the model proxy needs Docker with its compose plugin and a user who may use it, or another compose command in ZERO_COMPOSE", strings.Join(c, " "), core.Clean(strings.SplitN(err.Error(), "\n", 2)[0]))
	}
	return ""
}

// Config is the proxy's config file for the given providers.
func Config(providers []string, entries []MCPEntry) string {
	var b strings.Builder
	b.WriteString("# Written by zero; change providers in zero's config.\nmodel_list:\n")
	creds := ""
	for _, p := range providers {
		if p == "github_copilot" {
			b.WriteString("  - model_name: \"github_copilot/*\"\n    litellm_params:\n      model: \"github_copilot/*\"\n      litellm_credential_name: github-copilot\n      github_copilot_auth_type: per_user_oauth\n")
			creds += "  - credential_name: github-copilot\n    credential_values:\n      github_copilot_auth_type: per_user_oauth\n    credential_info:\n      custom_llm_provider: github_copilot\n"
		}
		if env, ok := core.KeyProviders[p]; ok {
			fmt.Fprintf(&b, "  - model_name: \"%s/*\"\n    litellm_params:\n      model: \"%s/*\"\n      api_key: os.environ/%s\n", p, p, env)
		}
	}
	if creds != "" {
		b.WriteString("credential_list:\n" + creds)
	}
	b.WriteString("general_settings:\n  master_key: os.environ/LITELLM_MASTER_KEY\n  require_key_mcp_access_defined: true\nlitellm_settings:\n  telemetry: false\n  check_provider_endpoint: true\n")
	b.WriteString(MCPConfig(entries))
	return b.String()
}

func SecretFile(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil && len(bytes.TrimSpace(b)) > 0 {
		return strings.TrimSpace(string(b)), nil
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	v := "sk-" + hex.EncodeToString(raw)
	return v, os.WriteFile(path, []byte(v+"\n"), 0o600)
}

func MasterKey() (string, error) { return SecretFile(filepath.Join(PartsDir(), "models.key")) }

func Healthy() bool {
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(URL() + "/health/liveliness")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// Start writes the Compose and config files and starts the proxy;
// a changed config restarts it.
func Start(out io.Writer, providers []string, keyCmds map[string]string) error {
	dir := PartsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// One zero at a time sets the parts up: two would write two master keys.
	lf, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lf.Close()
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	key, err := MasterKey()
	if err != nil {
		return err
	}
	pg, err := SecretFile(filepath.Join(dir, "models-db.key"))
	if err != nil {
		return err
	}
	entries, err := MCPEntries()
	if err != nil {
		return err
	}
	cfg, env := Config(providers, entries), MCPEnv(entries)
	keys, err := ProviderKeys(keyCmds)
	if err != nil {
		return err
	}
	sum := KeysSum(keys)
	oldSum, _ := os.ReadFile(filepath.Join(dir, "keys.sha"))
	old, _ := os.ReadFile(filepath.Join(dir, "litellm.yaml"))
	oldCompose, _ := os.ReadFile(filepath.Join(dir, "compose.yaml"))
	oldEnv, _ := os.ReadFile(filepath.Join(dir, "mcp.env"))
	same := string(old) == cfg && bytes.Equal(oldCompose, ComposeYAML) && string(oldEnv) == env && string(oldSum) == sum
	if same && Healthy() {
		return nil
	}
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), ComposeYAML, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "litellm.yaml"), []byte(cfg), 0o644); err != nil {
		return err
	}
	// The tool tokens of the MCP entries; only this file holds them.
	os.Remove(filepath.Join(dir, "mcp.env"))
	if err := os.WriteFile(filepath.Join(dir, "mcp.env"), []byte(env), 0o600); err != nil {
		return err
	}
	// Written only once the proxy answers with these keys; until then a
	// failed start is tried again at the next zero up.
	os.Remove(filepath.Join(dir, "keys.sha"))
	fmt.Fprintf(out, "model proxy: starting (LiteLLM on 127.0.0.1:%d)\n", Port)
	c := Compose()
	args := append(c[1:], "-p", Project, "-f", filepath.Join(dir, "compose.yaml"), "--profile", "models", "up", "-d")
	if !same {
		args = append(args, "--force-recreate")
	}
	cmd := exec.Command(c[0], args...)
	cmd.Env = append(os.Environ(), "ZERO_MODELS_KEY="+key, "ZERO_MODELS_DB_PASSWORD="+pg)
	for p, v := range keys {
		cmd.Env = append(cmd.Env, "ZERO_KEY_"+strings.ToUpper(p)+"="+v)
	}
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("model proxy: %s failed: %v\n%s", strings.Join(c, " "), err, core.Clean(string(b)))
	}
	for i := 0; i < 150; i++ {
		if Healthy() {
			return os.WriteFile(filepath.Join(dir, "keys.sha"), []byte(sum), 0o600)
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("model proxy not answering on 127.0.0.1:%d after 5 minutes; check: %s -p %s logs models", Port, strings.Join(c, " "), Project)
}

// API calls the proxy with the given key ("" = master key).
func API(key, method, path string, body any) ([]byte, error) {
	if key == "" {
		k, err := MasterKey()
		if err != nil {
			return nil, err
		}
		key = k
	}
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, URL()+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	c := http.Client{Timeout: 30 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("model proxy: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("model proxy: %s %s: %s", method, path, resp.Status)
	}
	return b, nil
}

// KeyRequest is the body that mints a sandbox's key: its models only, no
// MCP server, owned by the proxy's admin (who connected the providers).
func KeyRequest(alias string, models []string, entry string, tools []string) map[string]any {
	perm := map[string]any{"mcp_servers": []string{"no-mcp-servers"}}
	if entry != "" && len(tools) > 0 {
		perm = map[string]any{"mcp_servers": []string{entry}, "mcp_tool_permissions": map[string]any{entry: tools}}
	}
	return map[string]any{
		"key_alias":         alias,
		"user_id":           User,
		"models":            models,
		"duration":          "7d",
		"object_permission": perm,
	}
}

// Model is one model a key sees at the proxy, with the token limits the
// proxy reports for it (0 when it reports none).
type Model struct {
	ID      string
	In, Out int
}

// KeyLeft says how long the key with this alias is still valid at the proxy
// and a sum of what it allows. No such key: zero time.
func KeyLeft(alias string) (time.Duration, string, error) {
	b, err := API("", "GET", "/key/list?return_full_object=true&key_alias="+url.QueryEscape(alias), nil)
	if err != nil {
		return 0, "", err
	}
	left, allows := keyState(b)
	return left, allows, nil
}

// keyState reads the answer of /key/list: the time left and what the one
// key allows. Zero time: no single usable key, or a key that is not as zero
// makes it (another owner, a team, or a permission zero never asks for).
func keyState(b []byte) (time.Duration, string) {
	var v struct {
		Keys []struct {
			Expires time.Time                  `json:"expires"`
			Blocked bool                       `json:"blocked"`
			Models  []string                   `json:"models"`
			User    string                     `json:"user_id"`
			Team    *string                    `json:"team_id"`
			Routes  []string                   `json:"allowed_routes"`
			Perm    map[string]json.RawMessage `json:"object_permission"`
		} `json:"keys"`
	}
	if json.Unmarshal(b, &v) != nil || len(v.Keys) != 1 {
		return 0, ""
	}
	k := v.Keys[0]
	if k.Blocked || k.User != User || k.Team != nil || len(k.Routes) > 0 || k.Expires.IsZero() {
		return 0, ""
	}
	var servers []string
	var tools map[string][]string
	for name, raw := range k.Perm {
		switch name {
		case "object_permission_id":
		case "mcp_servers":
			if json.Unmarshal(raw, &servers) != nil {
				return 0, ""
			}
		case "mcp_tool_permissions":
			if json.Unmarshal(raw, &tools) != nil {
				return 0, ""
			}
		default:
			if t := strings.TrimSpace(string(raw)); t != "null" && t != "[]" && t != "{}" && t != "false" {
				return 0, ""
			}
		}
	}
	return time.Until(k.Expires), allowSum(k.Models, servers, tools)
}

// allowSum is one text for what a key allows, the same for every order.
func allowSum(models, servers []string, tools map[string][]string) string {
	models, servers = slices.Clone(models), slices.Clone(servers)
	slices.Sort(models)
	slices.Sort(servers)
	perm := map[string][]string{}
	for e, t := range tools {
		t = slices.Clone(t)
		slices.Sort(t)
		perm[e] = t
	}
	b, _ := json.Marshal([]any{append([]string{}, models...), append([]string{}, servers...), perm})
	return string(b)
}

// KeyAllows is the sum KeyLeft gives for a key made from this request.
func KeyAllows(models []string, entry string, tools []string) string {
	if entry != "" && len(tools) > 0 {
		return allowSum(models, []string{entry}, map[string][]string{entry: tools})
	}
	return allowSum(models, []string{"no-mcp-servers"}, nil)
}

// Models reads the model list a key sees. Wildcard entries are dropped,
// and so is every model the proxy marks with a mode opencode cannot chat with
// (embedding, image_generation, ...). A model without a mode is kept.
func Models(list []byte) ([]Model, error) {
	var v struct {
		Data []struct {
			ID   string `json:"id"`
			Mode string `json:"mode"`
			In   int    `json:"max_input_tokens"`
			Out  int    `json:"max_output_tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(list, &v); err != nil {
		return nil, fmt.Errorf("model list not readable")
	}
	var ms []Model
	for _, d := range v.Data {
		if d.ID == "" || strings.Contains(d.ID, "*") {
			continue
		}
		if d.Mode != "" && d.Mode != "chat" && d.Mode != "responses" {
			continue
		}
		ms = append(ms, Model{d.ID, d.In, d.Out})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].ID < ms[j].ID })
	return ms, nil
}

// OpencodeProvider is the opencode provider entry for the proxy. opencode finds
// no model of the proxy by itself (PoC 21), so every model is written, with
// the proxy's limits where it reports them.
func OpencodeProvider(ms []core.Model) map[string]any {
	models := map[string]any{}
	for _, pm := range ms {
		m := map[string]any{"name": pm.ID}
		if pm.Context > 0 && pm.Output > 0 {
			m["limit"] = map[string]any{"context": pm.Context, "output": pm.Output}
		}
		if strings.Contains(pm.ID, "claude") {
			m["compatibility"] = map[string]any{"cacheFormat": "anthropic"}
		}
		models[pm.ID] = m
	}
	return map[string]any{
		"name":    "LiteLLM",
		"env":     []string{KeyEnv},
		"package": "@opencode/ai/providers/openai-compatible",
		"settings": map[string]any{
			"baseURL": fmt.Sprintf("http://%s:%d/v1", core.SandboxHost, Port),
			"apiKey":  "{env:" + KeyEnv + "}",
		},
		"models": models,
	}
}

// Policy lets the sandbox's model clients reach the proxy and binds the
// sandbox's key to it: the sandbox holds a placeholder, the real key is put
// in on the way to the proxy and nowhere else (PoC 20).
func Policy(provider string, binaries []string) map[string]any {
	var rules []map[string]any
	for _, r := range Routes {
		rules = append(rules, map[string]any{"allow": r.Matcher()})
	}
	var bins []map[string]any
	for _, b := range binaries {
		bins = append(bins, map[string]any{"path": b})
	}
	return map[string]any{
		"name": Rule,
		"endpoints": []map[string]any{{
			"host": core.SandboxHost, "port": Port, "protocol": "rest", "enforcement": "enforce",
			"credential_binding": map[string]any{"provider": provider},
			"rules":              rules,
		}},
		"binaries": bins,
	}
}

// Routes is all of the proxy a sandbox may call: the model list and the
// calls that ask a model. The key is one of the proxy's admin user, so every
// other path (keys, users, credentials, MCP servers) is closed here, outside
// the sandbox.
var Routes = []core.EgressRule{
	{Method: "GET", Path: "/v1/models"},
	{Method: "POST", Path: "/v1/chat/completions"},
	{Method: "POST", Path: "/v1/responses"},
	{Method: "POST", Path: "/v1/messages"},
	{Method: "POST", Path: "/v1/messages/count_tokens"},
	{Method: "POST", Path: "/v1/embeddings"},
	{Method: "POST", Path: "/mcp"},
}

// Login is the doctor check of a setup with the model proxy: the
// provider logins live in the proxy, not in the host's opencode.
func Login() string {
	ui := fmt.Sprintf("http://127.0.0.1:%d/ui/", Port)
	b, err := API("", "GET", "/credentials/user_connections", nil)
	if err != nil {
		return "the model proxy does not answer; zero up in a workspace with models starts it"
	}
	if n := NotConnected(b); len(n) > 0 {
		return strings.Join(n, ", ") + " not connected; connect it in the proxy's UI: " + ui + " (Models + Endpoints, LLM Credentials, Connect)"
	}
	return ""
}

func Conns() []byte {
	b, _ := API("", "GET", "/credentials/user_connections", nil)
	return b
}

// NotConnected names the provider logins the proxy's user has not made yet.
func NotConnected(list []byte) []string {
	var v struct {
		Connections []struct {
			Name      string `json:"credential_name"`
			Connected bool   `json:"connected"`
		} `json:"connections"`
	}
	json.Unmarshal(list, &v)
	var out []string
	for _, c := range v.Connections {
		if !c.Connected {
			out = append(out, c.Name)
		}
	}
	return out
}

// ProviderKeys runs each provider's key command (no shell) and takes the
// first line it prints. The key goes to the proxy through the environment of
// the compose command only; zero writes it to no file (ADR 0056).
// KeyTimeout bounds a key command; it runs under the lock of the parts.
const KeyTimeout = 30 * time.Second

func ProviderKeys(cmds map[string]string) (map[string]string, error) {
	keys := map[string]string{}
	for p, c := range cmds {
		f := strings.Fields(c)
		if len(f) == 0 {
			return nil, fmt.Errorf("provider_keys.%s: no command", p)
		}
		ctx, cancel := context.WithTimeout(context.Background(), KeyTimeout)
		out, err := exec.CommandContext(ctx, f[0], f[1:]...).Output()
		cancel()
		line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
		if line = strings.TrimSpace(line); err != nil || line == "" {
			// Neither the command nor what it printed: both can hold the key.
			return nil, fmt.Errorf("provider_keys.%s: its command gave no key within %s; run it by hand to see why", p, KeyTimeout)
		}
		keys[p] = line
	}
	return keys, nil
}

// KeysSum lets zero see that a key changed without keeping the key.
func KeysSum(keys map[string]string) string {
	names := make([]string, 0, len(keys))
	for p := range keys {
		names = append(names, p)
	}
	slices.Sort(names)
	h := sha256.New()
	for _, p := range names {
		fmt.Fprintf(h, "%s=%s\n", p, keys[p])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ProviderKeysProblem is the doctor check: every key command prints a key.
func ProviderKeysProblem(cmds map[string]string) string {
	if _, err := ProviderKeys(cmds); err != nil {
		return err.Error()
	}
	return ""
}
