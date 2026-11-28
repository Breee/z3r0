package workspace

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/Breee/z3r0/internal/core"
)

// The model path (ADR 0049, #162): with parts.models set, a workspace's
// agents reach models only through the model proxy.

const PartLiteLLM = "litellm"

var modelNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

// CheckModels validates parts.models, providers and every workspace's model
// list. A model is "provider/name" or "provider/*" of a listed provider.
func (c *Config) CheckModels() error {
	if m := c.Parts.Models; m != "" && m != PartLiteLLM {
		return fmt.Errorf("parts.models: unknown value %q (%s, or leave it out)", m, PartLiteLLM)
	}
	for _, p := range c.Providers {
		if !slices.Contains(core.ModelProviders, p) {
			return fmt.Errorf("providers: unknown provider %q (one of: %s)", p, strings.Join(core.ModelProviders, ", "))
		}
	}
	for _, p := range c.Providers {
		if _, ok := core.KeyProviders[p]; ok && len(strings.Fields(c.ProviderKeys[p])) == 0 {
			return fmt.Errorf("providers: %s needs provider_keys.%s, a command that prints its key (e.g. pass show NAME)", p, p)
		}
	}
	for p := range c.ProviderKeys {
		if _, ok := core.KeyProviders[p]; !ok || !slices.Contains(c.Providers, p) {
			return fmt.Errorf("provider_keys: %s is not a listed provider that takes a key", p)
		}
		if strings.ContainsAny(c.ProviderKeys[p], "\"'|;&$<>`\\") {
			return fmt.Errorf("provider_keys.%s: the command is split at spaces and run without a shell; quotes, pipes and variables do not work (put them into a script and name that)", p)
		}
	}
	if c.Parts.Models == "" && len(c.Providers) > 0 {
		return fmt.Errorf("providers needs parts.models: %s", PartLiteLLM)
	}
	if c.Parts.Models != "" && len(c.Providers) == 0 {
		return fmt.Errorf("parts.models: %s needs providers (one of: %s)", PartLiteLLM, strings.Join(core.ModelProviders, ", "))
	}
	for name, w := range c.Workspaces {
		if w == nil {
			continue
		}
		if len(w.Models) > 0 && c.Parts.Models == "" {
			return fmt.Errorf("workspace %q: models needs parts.models: %s", name, PartLiteLLM)
		}
		seen := map[string]bool{}
		for _, m := range w.Models {
			prov, model, ok := strings.Cut(m, "/")
			if !ok || !slices.Contains(c.Providers, prov) {
				return fmt.Errorf("workspace %q: model %q: want PROVIDER/MODEL or PROVIDER/* with a provider of: %s", name, m, strings.Join(c.Providers, ", "))
			}
			if model != "*" && !modelNameRe.MatchString(model) {
				return fmt.Errorf("workspace %q: model %q: the model name is not usable", name, m)
			}
			if seen[m] {
				return fmt.Errorf("workspace %q: model %q is listed twice", name, m)
			}
			seen[m] = true
		}
		if w.Model != "" && !ModelListed(w.Model, w.Models) {
			return fmt.Errorf("workspace %q: model %q is not one of its models (%s)", name, w.Model, strings.Join(w.Models, ", "))
		}
	}
	return nil
}

// ModelListed reports whether the default model m is a full name that the
// list names itself or covers with PROVIDER/*.
func ModelListed(m string, list []string) bool {
	prov, model, ok := strings.Cut(m, "/")
	if !ok || model == "*" || !modelNameRe.MatchString(model) {
		return false
	}
	return slices.Contains(list, m) || slices.Contains(list, prov+"/*")
}
