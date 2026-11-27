package zero

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// The model path (ADR 0049, #162): with parts.models set, a workspace's
// agents reach models only through the model proxy.

const partLiteLLM = "litellm"

// modelProviders are the providers zero can put behind the model proxy.
var modelProviders = []string{"github_copilot"}

var modelNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

// checkModels validates parts.models, providers and every workspace's model
// list. A model is "provider/name" or "provider/*" of a listed provider.
func (c *Config) checkModels() error {
	if m := c.Parts.Models; m != "" && m != partLiteLLM {
		return fmt.Errorf("parts.models: unknown value %q (%s, or leave it out)", m, partLiteLLM)
	}
	for _, p := range c.Providers {
		if !slices.Contains(modelProviders, p) {
			return fmt.Errorf("providers: unknown provider %q (one of: %s)", p, strings.Join(modelProviders, ", "))
		}
	}
	if c.Parts.Models == "" && len(c.Providers) > 0 {
		return fmt.Errorf("providers needs parts.models: %s", partLiteLLM)
	}
	if c.Parts.Models != "" && len(c.Providers) == 0 {
		return fmt.Errorf("parts.models: %s needs providers (one of: %s)", partLiteLLM, strings.Join(modelProviders, ", "))
	}
	for name, w := range c.Workspaces {
		if w == nil {
			continue
		}
		if len(w.Models) > 0 && c.Parts.Models == "" {
			return fmt.Errorf("workspace %q: models needs parts.models: %s", name, partLiteLLM)
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
		if w.Model != "" && !modelListed(w.Model, w.Models) {
			return fmt.Errorf("workspace %q: model %q is not one of its models (%s)", name, w.Model, strings.Join(w.Models, ", "))
		}
	}
	return nil
}

// modelListed reports whether the default model m is a full name that the
// list names itself or covers with PROVIDER/*.
func modelListed(m string, list []string) bool {
	prov, model, ok := strings.Cut(m, "/")
	if !ok || model == "*" || !modelNameRe.MatchString(model) {
		return false
	}
	return slices.Contains(list, m) || slices.Contains(list, prov+"/*")
}
