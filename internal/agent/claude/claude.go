// Package claude is Claude Code in the sandbox.
package claude

import (
	"github.com/Breee/z3r0/internal/core"
)

// Bin is Claude Code in the image. /usr/local/bin/claude is a link to it;
// the egress rule has to name the program itself.
const Bin = "/usr/local/libexec/claude/node_modules/@anthropic-ai/claude-code/bin/claude.exe"

// Script is the command run in the sandbox (PoC 24): Claude Code talks
// to the model proxy with the placeholder of the sandbox's environment as
// its key and asks Anthropic for nothing; the image pins its version. The
// model of --model is also the one for its background calls.
func Script(dir, baseURL, keyEnv string, args []string) string {
	script := "cd " + core.ShellQuote(dir) + ` && [ -n "$` + keyEnv + `" ] || { echo 'zero claude: the sandbox has no key for the model proxy; run zero up' >&2; exit 1; } && ANTHROPIC_BASE_URL=` +
		baseURL + ` ANTHROPIC_AUTH_TOKEN="$` + keyEnv + `" ANTHROPIC_MODEL=` + core.ShellQuote(core.ModelArg(args)) + ` ANTHROPIC_DEFAULT_HAIKU_MODEL=` + core.ShellQuote(core.ModelArg(args)) +
		` CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 CLAUDE_CODE_DISABLE_OFFICIAL_MARKETPLACE_AUTOINSTALL=1 DISABLE_UPDATES=1 DISABLE_AUTOUPDATER=1 DISABLE_TELEMETRY=1 DISABLE_ERROR_REPORTING=1 exec ` + Bin
	for _, a := range args {
		script += " " + core.ShellQuote(a)
	}
	return script
}
