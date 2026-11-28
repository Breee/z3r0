package core

import (
	"regexp"
	"slices"
	"strings"
)

// ModelProviders are the providers zero can put behind the model proxy.
var ModelProviders = []string{"github_copilot", "openai", "anthropic", "openrouter"}

// KeyProviders need an API key; the value names the variable the proxy reads
// it from (ADR 0056).
var KeyProviders = map[string]string{
	"openai":     "OPENAI_API_KEY",
	"anthropic":  "ANTHROPIC_API_KEY",
	"openrouter": "OPENROUTER_API_KEY",
}

// ModelNameRe is what the name part of a model may look like.
var ModelNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

// ModelListed reports whether the default model m is a full name that the
// list names itself or covers with PROVIDER/*.
func ModelListed(m string, list []string) bool {
	prov, model, ok := strings.Cut(m, "/")
	if !ok || model == "*" || !ModelNameRe.MatchString(model) {
		return false
	}
	return slices.Contains(list, m) || slices.Contains(list, prov+"/*")
}
