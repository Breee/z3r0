package zero

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// The model proxy (ADR 0049, #162): LiteLLM in a container next to the
// sandbox. It holds the provider logins; a sandbox gets a key of its own
// that is limited to the workspace's models and to no MCP server.

//go:embed model-key-profile.yaml
var modelKeyProfile []byte

//go:embed parts/compose.yaml
var partsCompose []byte

const (
	modelPort    = 18096
	modelRule    = "models"
	modelKeyEnv  = "LITELLM_API_KEY"
	modelUser    = "default_user_id" // the proxy's admin, who connects the providers
	modelProject = "zero-parts"
)

func partsDir() string { return filepath.Join(stateRoot(), "parts") }

func modelURL() string { return fmt.Sprintf("http://127.0.0.1:%d", modelPort) }

// compose is the compose command for the opt-in parts: Docker, until the
// podman here runs health checks (ADR 0051, C58); ZERO_COMPOSE overrides it.
func compose() []string {
	if v := strings.Fields(os.Getenv("ZERO_COMPOSE")); len(v) > 0 {
		return v
	}
	return []string{"docker", "compose"}
}

// composeProblem says why the compose command of the parts does not run.
func composeProblem() string {
	c := compose()
	if _, err := sh(c[0], append(c[1:], "version")...); err != nil {
		return fmt.Sprintf("%s does not run (%s); the model proxy needs Docker with its compose plugin and a user who may use it, or another compose command in ZERO_COMPOSE", strings.Join(c, " "), clean(strings.SplitN(err.Error(), "\n", 2)[0]))
	}
	return ""
}

// modelConfig is the proxy's config file for the given providers.
func modelConfig(providers []string) string {
	var b strings.Builder
	b.WriteString("# Written by zero; change providers in zero's config.\nmodel_list:\n")
	creds := ""
	for _, p := range providers {
		if p == "github_copilot" {
			b.WriteString("  - model_name: \"github_copilot/*\"\n    litellm_params:\n      model: \"github_copilot/*\"\n      litellm_credential_name: github-copilot\n      github_copilot_auth_type: per_user_oauth\n")
			creds += "  - credential_name: github-copilot\n    credential_values:\n      github_copilot_auth_type: per_user_oauth\n    credential_info:\n      custom_llm_provider: github_copilot\n"
		}
	}
	if creds != "" {
		b.WriteString("credential_list:\n" + creds)
	}
	b.WriteString("general_settings:\n  master_key: os.environ/LITELLM_MASTER_KEY\n  require_key_mcp_access_defined: true\nlitellm_settings:\n  telemetry: false\n  check_provider_endpoint: true\n")
	return b.String()
}

func secretFile(path string) (string, error) {
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

func modelMasterKey() (string, error) { return secretFile(filepath.Join(partsDir(), "models.key")) }

func modelHealthy() bool {
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(modelURL() + "/health/liveliness")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// startModelProxy writes the compose and config files and starts the proxy;
// a changed config restarts it.
func startModelProxy(out io.Writer, providers []string) error {
	dir := partsDir()
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
	key, err := modelMasterKey()
	if err != nil {
		return err
	}
	pg, err := secretFile(filepath.Join(dir, "models-db.key"))
	if err != nil {
		return err
	}
	cfg := modelConfig(providers)
	old, _ := os.ReadFile(filepath.Join(dir, "litellm.yaml"))
	oldCompose, _ := os.ReadFile(filepath.Join(dir, "compose.yaml"))
	same := string(old) == cfg && bytes.Equal(oldCompose, partsCompose)
	if same && modelHealthy() {
		return nil
	}
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), partsCompose, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "litellm.yaml"), []byte(cfg), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "model proxy: starting (LiteLLM on 127.0.0.1:%d)\n", modelPort)
	c := compose()
	args := append(c[1:], "-p", modelProject, "-f", filepath.Join(dir, "compose.yaml"), "--profile", "models", "up", "-d")
	if !same {
		args = append(args, "--force-recreate")
	}
	cmd := exec.Command(c[0], args...)
	cmd.Env = append(os.Environ(), "ZERO_MODELS_KEY="+key, "ZERO_MODELS_DB_PASSWORD="+pg)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("model proxy: %s failed: %v\n%s", strings.Join(c, " "), err, clean(string(b)))
	}
	for i := 0; i < 150; i++ {
		if modelHealthy() {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("model proxy not answering on 127.0.0.1:%d after 5 minutes; check: %s -p %s logs models", modelPort, strings.Join(c, " "), modelProject)
}

// modelAPI calls the proxy with the given key ("" = master key).
func modelAPI(key, method, path string, body any) ([]byte, error) {
	if key == "" {
		k, err := modelMasterKey()
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
	req, err := http.NewRequest(method, modelURL()+path, r)
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

func (s *Sandbox) modelKeyAlias() string { return tokenLabel(s.Name) }

// keyRequest is the body that mints a sandbox's key: its models only, no
// MCP server, owned by the proxy's admin (who connected the providers).
func keyRequest(alias string, models []string) map[string]any {
	return map[string]any{
		"key_alias": alias,
		"user_id":   modelUser,
		"models":    models,
		"duration":  "7d",
		"object_permission": map[string]any{
			"mcp_servers": []string{"no-mcp-servers"},
		},
	}
}

// revokeModelKey deletes the sandbox's key at the proxy. Without a key on
// record it does nothing.
func (s *Sandbox) revokeModelKey() error {
	if s.read("models.key") == "" {
		return nil
	}
	if _, err := modelAPI("", "POST", "/key/delete", map[string]any{"key_aliases": []string{s.modelKeyAlias()}}); err != nil {
		return fmt.Errorf("model key of %s not revoked: %w", s.Name, err)
	}
	return os.Remove(s.path("models.key"))
}

// proxyModel is one model a key sees at the proxy, with the token limits the
// proxy reports for it (0 when it reports none).
type proxyModel struct {
	ID      string
	In, Out int
}

// proxyModels reads the model list a key sees. Wildcard entries are dropped,
// and so is every model the proxy marks with a mode opencode cannot chat with
// (embedding, image_generation, ...). A model without a mode is kept.
func proxyModels(list []byte) ([]proxyModel, error) {
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
	var ms []proxyModel
	for _, d := range v.Data {
		if d.ID == "" || strings.Contains(d.ID, "*") {
			continue
		}
		if d.Mode != "" && d.Mode != "chat" && d.Mode != "responses" {
			continue
		}
		ms = append(ms, proxyModel{d.ID, d.In, d.Out})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].ID < ms[j].ID })
	return ms, nil
}

// modelProvider is the opencode provider entry for the proxy. opencode finds
// no model of the proxy by itself (PoC 21), so every model is written, with
// the proxy's limits where it reports them.
func modelProvider(ms []proxyModel) map[string]any {
	models := map[string]any{}
	for _, pm := range ms {
		m := map[string]any{"name": pm.ID}
		if pm.In > 0 && pm.Out > 0 {
			m["limit"] = map[string]any{"context": pm.In, "output": pm.Out}
		}
		if strings.Contains(pm.ID, "claude") {
			m["compatibility"] = map[string]any{"cacheFormat": "anthropic"}
		}
		models[pm.ID] = m
	}
	return map[string]any{
		"name":    "LiteLLM",
		"env":     []string{modelKeyEnv},
		"package": "@opencode/ai/providers/openai-compatible",
		"settings": map[string]any{
			"baseURL": fmt.Sprintf("http://%s:%d/v1", gateHost, modelPort),
			"apiKey":  "{env:" + modelKeyEnv + "}",
		},
		"models": models,
	}
}

// modelPolicy lets the sandbox's model clients reach the proxy and binds the
// sandbox's key to it: the sandbox holds a placeholder, the real key is put
// in on the way to the proxy and nowhere else (PoC 20).
func modelPolicy(provider string) map[string]any {
	var rules []map[string]any
	for _, r := range modelRoutes {
		rules = append(rules, map[string]any{"allow": matcher(r)})
	}
	return map[string]any{
		"name": modelRule,
		"endpoints": []map[string]any{{
			"host": gateHost, "port": modelPort, "protocol": "rest", "enforcement": "enforce",
			"credential_binding": map[string]any{"provider": provider},
			"rules":              rules,
		}},
		"binaries": []map[string]any{{"path": opencodeBin}, {"path": copilotBin}, {"path": "/usr/bin/curl"}},
	}
}

// modelRoutes is all of the proxy a sandbox may call: the model list and the
// calls that ask a model. The key is one of the proxy's admin user, so every
// other path (keys, users, credentials, MCP servers) is closed here, outside
// the sandbox.
var modelRoutes = []EgressRule{
	{Method: "GET", Path: "/v1/models"},
	{Method: "POST", Path: "/v1/chat/completions"},
	{Method: "POST", Path: "/v1/responses"},
	{Method: "POST", Path: "/v1/messages"},
	{Method: "POST", Path: "/v1/messages/count_tokens"},
	{Method: "POST", Path: "/v1/embeddings"},
}

// syncModels sets up the model path of a running sandbox: proxy up, a fresh
// key, the egress rule, the key in opencode's service. It returns the models
// for the opencode config. Without models it takes the path away.
func (s *Sandbox) syncModels(out io.Writer, providers []string) ([]proxyModel, error) {
	if len(s.Models) == 0 {
		if s.read("models.key") == "" && s.read("models.provider") == "" {
			return nil, nil
		}
		if err := s.revokeModelKey(); err != nil {
			return nil, err
		}
		if err := rt.SetRule(s.Name, modelRule, nil); err != nil {
			return nil, err
		}
		if err := s.dropModelProvider(); err != nil {
			return nil, err
		}
		_, err := s.ssh("opencode service unset env " + modelKeyEnv + " && opencode service restart")
		return nil, err
	}
	if err := startModelProxy(out, providers); err != nil {
		return nil, err
	}
	alias := s.modelKeyAlias()
	if _, err := modelAPI("", "POST", "/key/delete", map[string]any{"key_aliases": []string{alias}}); err != nil && s.read("models.key") != "" {
		return nil, err
	}
	b, err := modelAPI("", "POST", "/key/generate", keyRequest(alias, s.Models))
	if err != nil {
		return nil, err
	}
	var k struct {
		Key string `json:"key"`
	}
	if json.Unmarshal(b, &k) != nil || k.Key == "" {
		return nil, fmt.Errorf("model proxy: no key in the answer")
	}
	if err := s.write("models.key", "1", 0o600); err != nil {
		return nil, err
	}
	if err := s.bindModelKey(k.Key); err != nil {
		return nil, err
	}
	// A sandbox that used the Copilot CLI before the workspace got models
	// still has the rule to GitHub's model host: it goes.
	if fileExists(s.path("copilot-cli.rule")) {
		if err := rt.SetRule(s.Name, copilotRule, nil); err != nil {
			return nil, err
		}
		os.Remove(s.path("copilot-cli.rule"))
	}
	// A new session has the placeholder in its environment; that is what
	// opencode's service keeps, not the key.
	if _, err := s.ssh(`[ -n "$` + modelKeyEnv + `" ] && opencode service set env ` + modelKeyEnv + ` "$` + modelKeyEnv + `" >/dev/null && opencode service restart >/dev/null`); err != nil {
		return nil, fmt.Errorf("model proxy key not handed to opencode in %s: %w; check: zero logs, then zero up", s.Name, err)
	}
	list, err := modelAPI(k.Key, "GET", "/v1/models", nil)
	if err != nil {
		return nil, err
	}
	ids, err := proxyModels(list)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(out, "models: %d through the model proxy (%s)\n", len(ids), strings.Join(s.Models, ", "))
	for _, n := range notConnected(conns()) {
		fmt.Fprintf(out, "models: %s is not connected, no answer will come; connect it in the proxy's UI: %s/ui/ (user admin, password in %s), Models + Endpoints, LLM Credentials, Connect\n", n, modelURL(), filepath.Join(partsDir(), "models.key"))
	}
	return ids, nil
}

func (s *Sandbox) modelProvider() string { return s.Name + "-models" }

// bindModelKey puts the key outside the sandbox. Order (PoC 20): binding,
// attach, rule that names it, then wait until the sandbox has it. A binding
// from an earlier run only gets the new key.
func (s *Sandbox) bindModelKey(key string) error {
	id := s.modelProvider()
	if s.read("models.provider") == "" || rt.RotateCredential(id, modelKeyEnv, key) != nil {
		if err := rt.SetRule(s.Name, modelRule, nil); err != nil {
			return err
		}
		rt.DetachCredential(s.Name, id) // not attached on a first run
		if err := rt.ImportCredentialType("zero-model-key", modelKeyProfile); err != nil {
			return err
		}
		if err := rt.BindCredential(id, "zero-model-key", modelKeyEnv, key); err != nil {
			return err
		}
		if err := rt.AttachCredential(s.Name, id); err != nil {
			return err
		}
		if err := s.write("models.provider", id, 0o600); err != nil {
			return err
		}
	}
	if err := rt.SetRule(s.Name, modelRule, modelPolicy(id)); err != nil {
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
	rt.DetachCredential(s.Name, s.modelProvider()) // fails when the sandbox is gone
	rt.UnbindCredential(s.modelProvider())
	return os.Remove(s.path("models.provider"))
}

func conns() []byte {
	b, _ := modelAPI("", "GET", "/credentials/user_connections", nil)
	return b
}

// notConnected names the provider logins the proxy's user has not made yet.
func notConnected(list []byte) []string {
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
