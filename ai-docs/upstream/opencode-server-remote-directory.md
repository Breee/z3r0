# Comment for anomalyco/opencode#47665

**Target:** comment on https://github.com/anomalyco/opencode/issues/47665 (and ask to reopen PR #47818).

---

Same problem with `opencode run --server`, which has no directory argument at all. Use case: the agent runs inside a sandbox (`opencode serve` in `/tmp/zero/<repo>` on the sandbox side), and the host attaches over an SSH port forward. The host path of the checkout and the sandbox path differ by design.

```
$ OPENCODE_SERVER_PASSWORD=... opencode run --server http://127.0.0.1:41849 "hi"
Error: Location not found: /tmp/opencode/developer      # client cwd, exists only on the host
```

1.x supported this with `opencode attach <url> --dir <remote-dir>` and `opencode run --attach <url> --dir <remote-dir>`. In 2.x, neither `run` nor the TUI can pick a server-side directory, so remote/sandboxed servers are unusable from the CLI unless both sides share the same absolute path.

Request: with `--server`, a `--dir <remote path>` flag (or the positional directory) for both the TUI and `run`, sent as-is to the location API and resolved on the server. Or, if the server was started in a project dir, use that as the default location instead of the client cwd.

Versions: opencode 2.2.6 on both client and server, Linux x86_64.
