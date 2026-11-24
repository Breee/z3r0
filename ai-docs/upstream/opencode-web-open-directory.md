# Feature request for anomalyco/opencode: pairing link that opens a folder or a session

**Target:** new issue in https://github.com/anomalyco/opencode (related: #55914 "New session" does nothing with no project, open; #44167 session deep link for the desktop app, open; #48592 projects kept on the server, closed).

---

Use case: `opencode serve` runs inside a sandbox in `/tmp/zero/<repo>`; the host reaches it over an SSH port forward. A wrapper runs `opencode pair --url http://127.0.0.1:<port>` in the sandbox and opens the printed link in the host's browser.

Actual: the link signs the browser in and loads Home. "Projects" is empty ("Nothing here yet"), although the server was started in the project folder and `GET /api/project` lists it. The project list is kept in the browser, so every new browser profile and every new forward port starts empty; the user has to type the server-side path into "Add project".

```
$ opencode pair --url http://127.0.0.1:37705
http://127.0.0.1:37705/auth/connect/<code>     # opens Home, no project
```

Expected: a documented way to say where the web UI opens, one of:

- `opencode pair --dir <server path>` (or `--session <id>`): the link opens the web UI in that folder or session after sign-in;
- a documented link form for a folder and for a session (the routes `/new-session` and `/server/<key>/session/<id>` exist in the source but are not documented, and the server key is not);
- Home lists the server's projects when the browser has none.

What we do meanwhile (documented API only): create one session with `POST /api/session` and `{"location":{"directory":"<path>"}}`; Home then lists it under "Recent sessions".

Versions: opencode 2.2.6 (server and web UI), Linux x86_64, Chromium.
