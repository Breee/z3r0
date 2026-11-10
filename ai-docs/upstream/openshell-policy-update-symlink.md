# `policy update --binary` keeps symlink paths that provider profiles resolve, so the rule silently never matches

**Target:** new issue on https://github.com/NVIDIA/OpenShell (related: #4834, #2883)

## Description
A binary given as a symlink is handled differently depending on where it comes from:
- **Provider profile `binaries`**: stored by resolved path (`/usr/local/bin/opencode` → `/usr/local/libexec/opencode/node_modules/@opencode/cli/bin/opencode.exe`). Traffic matches.
- **`openshell policy update --binary /usr/local/bin/opencode`**: stored as the literal symlink path. The running process's exe is the resolved path, so the rule never matches. The sandbox gets `ECONNREFUSED`, and `policy update` gives no warning.

## Reproduction
1. Image where `/usr/local/bin/opencode` is a symlink to `.../opencode.exe`.
2. `openshell policy update SB --rule-name zen --binary /usr/local/bin/opencode --add-endpoint opencode.ai:443:read-write:rest:enforce --wait`
3. Inside SB: `opencode run "hi"` → `ECONNREFUSED` to opencode.ai.
4. `openshell policy get SB --full`: the provider rule lists `.../opencode.exe`, and the `zen` rule lists `/usr/local/bin/opencode`.
5. Re-running step 2 with the real exe path fixes it.

## Expected
`policy update --binary` resolves symlinks the same way profile import does, or it at least warns that the path is a symlink and will not match.

## Versions
openshell 0.1.4 (deb), podman compute driver, Linux x86_64.
