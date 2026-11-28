// Package copilot is the GitHub Copilot CLI in the sandbox.
package copilot

import "github.com/Breee/z3r0/internal/core"

// Bin and Rule: the GitHub Copilot CLI of the image and the egress rule that
// lets only that binary reach the Copilot API (ADR 0050).
const (
	Bin  = "/usr/local/bin/copilot"
	Rule = "copilot-cli"
)

// Env is the variable the sandbox sees the Copilot token's placeholder in.
const Env = "COPILOT_TOKEN"

// ProxyScript is the command run in a sandbox whose workspace uses the
// model proxy (PoC 23): the CLI talks to the proxy and asks GitHub for
// nothing. Its key is the placeholder of the sandbox's environment (PoC 20).
func ProxyScript(dir, baseURL, keyEnv string, args []string) string {
	script := "cd " + core.ShellQuote(dir) + ` && [ -n "$` + keyEnv + `" ] || { echo 'zero copilot: the sandbox has no key for the model proxy; run zero up' >&2; exit 1; } && COPILOT_AUTO_UPDATE=false COPILOT_OFFLINE=true COPILOT_PROVIDER_BASE_URL=` +
		baseURL + ` COPILOT_PROVIDER_API_KEY="$` + keyEnv + `" exec ` + Bin + " --no-auto-update"
	for _, a := range args {
		script += " " + core.ShellQuote(a)
	}
	return script
}

// Script is the command run in the sandbox. The CLI does not update
// itself there: the image pins its version.
func Script(dir string, args []string) string {
	script := "cd " + core.ShellQuote(dir) + " && COPILOT_AUTO_UPDATE=false COPILOT_GITHUB_TOKEN=\"$" + Env + "\" exec " + Bin + " --no-auto-update"
	for _, a := range args {
		script += " " + core.ShellQuote(a)
	}
	return script
}
