package workspace

import "strings"

// SandboxName maps a repo basename to the sandbox name: lowercase, every
// character outside [a-z0-9] becomes '-' (matches the original shell version).
func SandboxName(repo string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(repo) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return "zero-" + b.String()
}
