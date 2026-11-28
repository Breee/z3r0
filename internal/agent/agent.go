// Package agent holds what the agents of the sandbox share.
package agent

import "strings"

// ModelArg is the model the arguments for the Copilot CLI or Claude Code
// name with --model, or "". A value that starts with - is the next option,
// not a model.
func ModelArg(args []string) string {
	for i, a := range args {
		if a == "--" {
			break
		}
		v := ""
		if a == "--model" && i+1 < len(args) {
			v = args[i+1]
		} else if strings.HasPrefix(a, "--model=") {
			v = strings.TrimPrefix(a, "--model=")
		}
		if v != "" && !strings.HasPrefix(v, "-") {
			return v
		}
	}
	return ""
}

// HasModelArg reports whether the arguments name a model.
func HasModelArg(args []string) bool { return ModelArg(args) != "" }
