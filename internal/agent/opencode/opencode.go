// Package opencode is opencode in the sandbox.
package opencode

// Bin is the real executable; /usr/local/bin/opencode is a symlink and the
// policy update matches binaries by resolved path (C37).
const Bin = "/usr/local/libexec/opencode/node_modules/@opencode/cli/bin/opencode.exe"

// ConfigPath is the opencode user config inside the sandbox.
const ConfigPath = "$HOME/.config/opencode/opencode.json"
