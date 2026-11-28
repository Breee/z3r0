package core

// ModelProviders are the providers zero can put behind the model proxy.
var ModelProviders = []string{"github_copilot", "openai", "anthropic", "openrouter"}

// KeyProviders need an API key; the value names the variable the proxy reads
// it from (ADR 0056).
var KeyProviders = map[string]string{
	"openai":     "OPENAI_API_KEY",
	"anthropic":  "ANTHROPIC_API_KEY",
	"openrouter": "OPENROUTER_API_KEY",
}
