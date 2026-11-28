package litellm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/Breee/z3r0/internal/core"
)

// keyRenewBefore is how much life a key must have left to be kept.
const keyRenewBefore = 12 * time.Hour

// Proxy is the LiteLLM model proxy as a core.ModelProvider.
//
// Grant takes the workspace's tools and the state of its sandbox from the
// fields: the interface has no place for them.
type Proxy struct {
	Providers []string          // model providers the proxy serves
	KeyCmds   map[string]string // provider -> command that prints its key
	Alias     string            // name of the workspace's key at the proxy
	ToolEntry string            // the workspace's tool entry at the proxy, or ""
	Tools     []string          // tools the key may call through ToolEntry
	Renew     bool              // make a new key even if the old one would do
	HadKey    bool              // a key was made before: its deletion must work
}

var _ core.ModelProvider = Proxy{}

func (Proxy) Name() string { return "litellm" }

func (p Proxy) Check() []core.Check {
	return []core.Check{
		{Name: "model proxy runtime", Problem: ComposeProblem()},
		{Name: "model proxy login", Problem: Login()},
		{Name: "model provider keys", Problem: ProviderKeysProblem(p.KeyCmds)},
	}
}

func (p Proxy) Ensure(out io.Writer) error { return Start(out, p.Providers, p.KeyCmds) }

// Grant returns the access of one workspace. The secret is empty when the
// key the workspace already has was kept.
func (p Proxy) Grant(ws string, models []string) (core.ModelAccess, error) {
	a := core.ModelAccess{
		Credential: core.Credential{Env: KeyEnv},
		BaseURL:    fmt.Sprintf("http://%s:%d", core.SandboxHost, Port),
		API:        "openai",
	}
	var ms []Model
	if !p.Renew && p.keep(models) {
		list, err := API("", "GET", "/v1/models", nil)
		if err != nil {
			return a, err
		}
		all, err := Models(list)
		if err != nil {
			return a, err
		}
		for _, m := range all {
			if core.ModelListed(m.ID, models) {
				ms = append(ms, m)
			}
		}
	} else {
		if _, err := API("", "POST", "/key/delete", map[string]any{"key_aliases": []string{p.Alias}}); err != nil && p.HadKey {
			return a, err
		}
		b, err := API("", "POST", "/key/generate", KeyRequest(p.Alias, models, p.ToolEntry, p.Tools))
		if err != nil {
			return a, err
		}
		var k struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(b, &k); err != nil || k.Key == "" {
			return a, errors.New("model proxy: no key in the answer")
		}
		a.Credential.Secret = k.Key
		list, err := API(k.Key, "GET", "/v1/models", nil)
		if err != nil {
			return a, err
		}
		if ms, err = Models(list); err != nil {
			return a, err
		}
	}
	a.Models = CoreModels(ms)
	for _, n := range NotConnected(Conns()) {
		a.Warnings = append(a.Warnings, fmt.Sprintf("%s is not connected, no answer will come; connect it in the proxy's UI: %s/ui/ (user admin, password in %s), Models + Endpoints, LLM Credentials, Connect", n, URL(), filepath.Join(PartsDir(), "models.key")))
	}
	return a, nil
}

func (p Proxy) keep(models []string) bool {
	left, allows, err := KeyLeft(p.Alias)
	return err == nil && keepKey(left, allows, KeyAllows(models, p.ToolEntry, p.Tools))
}

// keepKey reports whether a key with this much life left and these rights
// is the key zero would make now.
func keepKey(left time.Duration, allows, want string) bool {
	return left > keyRenewBefore && allows == want
}

func (p Proxy) Revoke(ws string) error {
	_, err := API("", "POST", "/key/delete", map[string]any{"key_aliases": []string{p.Alias}})
	return err
}

// CoreModels gives the proxy's models in the form every part knows.
func CoreModels(ms []Model) []core.Model {
	var out []core.Model
	for _, m := range ms {
		out = append(out, core.Model{ID: m.ID, Name: m.ID, Context: m.In, Output: m.Out})
	}
	return out
}
