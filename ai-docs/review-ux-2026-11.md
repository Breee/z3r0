# zero — ease-of-use review (2026-11)

Scope: CLI wording, steps and decisions a user must take; no security check is proposed for removal or weakening. Verified against the repo build and the installed `/usr/local/bin/zero` (its usage text is identical; only the last commit differs).

## The path of a new user

Fresh clone to an agent prompt, as README + `getting-started.md` describe it:

1. `git clone … && cd z3r0` — 1 command.
2. `mkdir -p bin; go build -o bin/zero ./cmd/zero` — 1 command, needs Go 1.26.
3. `go build -o bin/zero-cp ./cmd/zero-cp` — 1 command; nothing says why (README.md:13).
4. `sudo bin/zero install` — 1 command, sudo; needs podman, uidmap, passthru, curl, systemd.
5. `zero doctor` — 1 command; can end in `FAIL` (exit 1) on a non-GitLab repo.
6. `cd ~/src/project` — 1 command.
7. `zero up` — 1 command; prints `zero-project ready: …` and exits. It does **not** open the agent.
8. `zero attach` — 1 command; now the prompt appears.

Before step 1 the user must already have: rootless podman stack, Go, `glab` logged in to the right GitLab host with keyring, `opencode` on the host connected to GitHub Copilot, and sudo. Two of these (Copilot login, `glab` on the right host) are not checked by anything before the agent is opened.

Unclear-failure points found: a GitHub repo (`doctor` FAILs with a fix that cannot work; `up` says nothing), a missing Copilot login (no message at all), and a typo in a flag (silently ignored).

## Findings

### High

#### U1 — `stop` stops a tunnel, not the sandbox
- Evidence: `cmd/zero/main.go:29` `stop  end the host port forward (sandbox keeps running)`; `internal/zero/commands.go:494-502`; the word "stop" is otherwise the natural opposite of "up".
- Why it costs: a user who wants to stop the agent work types `stop`, sees nothing (no output at all when no tunnel runs, `forward.go:91-95`), and believes the sandbox is off while it still runs and still holds a live token.
- Change: rename the command to `disconnect` (keep `stop` as a one-release alias that prints `the sandbox still runs; zero pause stops it`). One word, one meaning; nothing removed.
- security effect: none (behaviour unchanged, the alias keeps scripts working).

#### U2 — `up` looks like "start the agent" but only prints a URL
- Evidence: `main.go:20` vs `main.go:24`; observed `zero up` → `zero-z3r0 ready: http://127.0.0.1:33733 dir /tmp/zero/z3r0`, exit 0, no next step.
- Why it costs: the shortest-sounding command leaves the user at a shell with a URL they did not ask for; they must know that `attach` is the real entry point.
- Change: after the ready line print `next: zero attach` (or `zero web`, `zero code` when stdin is a terminal), and one line in usage: `up = start only; attach = start + open the agent`. Do not change `up`'s behaviour (scripts and `run` depend on it).
- security effect: none.

#### U3 — the model login is never checked, and its absence is silent
- Evidence: `doctor.go:18-73` checks openshell, gateway user, client key, zero-cp, forge — never `opencode` on PATH nor a Copilot login; `copilot.go:45-49` returns `""` without a word when `opencode auth export` has no active `github-copilot` entry; `getting-started.md:15` lists the login as a prerequisite only.
- Why it costs: the sandbox starts, `attach` opens, and the failure (no model) appears later as an opencode error inside the TUI — after the slowest step of the whole setup.
- Change: `zero doctor` gains `opencode on PATH` and `Copilot login` checks naming `opencode auth login github-copilot`; `zero up` prints `note: no Copilot login on the host; the agent has no model`.
- security effect: none.

#### U4 — `doctor` FAILs on a GitHub repo and names the wrong fix
- Evidence: observed in this repo: `FAIL  forge: origin git@github.com:Breee/z3r0.git is not a supported forge (GitLab hosts need a glab login)`, exit 1 (`doctor.go:65`).
- Why it costs: GitHub is the most common origin; the parenthetical sends the user to `glab auth login`, which cannot help, and a `FAIL` reads as "my install is broken" while the install is fine. `docs/content/docs/_index.md` already states "GitLab only", so this is not a hidden gap — only the message is wrong.
- Change: `na    forge: GitHub (not supported yet; this workspace has no token and no push)` and do not count it as a failure; exit 0.
- security effect: none (the check that matters, guardrails for GitLab, is unchanged).

#### U5 — a repo with no supported forge starts a sandbox that cannot hand over work, silently
- Evidence: `forge.go:196-203` (`setupForge` returns `""` when `folderForge` is nil); `commands.go:37-176` never mentions the absence; observed `zero status` for such a sandbox shows `sandbox …: up / forward: … / serve: running` and no token line.
- Why it costs: the agent works, produces commits, and the only exit (`git push`) does not exist. The user finds out when the agent says it cannot push, or not at all.
- Change: one notice at the end of `up` and one line in `status`: `forge: none (no origin on a supported forge); this workspace can only work locally`.
- security effect: none.

#### U6 — unknown flags are silently ignored
- Evidence: observed `zero up --bogus` → started the sandbox, `ready:` line, exit 0 (`main.go:118-136` reads flags with `slices.Contains`/`slices.Index` and ignores the rest).
- Why it costs: `--aprove`, `--max-imput` or `--vmv` change nothing and say nothing; a user who meant `--approve` gets `policy widening not approved` later, a user who meant `--max-input` gets the default limit.
- Change: reject unknown arguments per command: `zero up: unknown option "--bogus" (options: --approve, --max-input MiB, --vm)`, exit 2.
- security effect: none (strictly more predictable).

### Medium

#### U7 — no way to see the version
- Evidence: observed `zero --version`, `zero version`, `zero -v` → `zero: unknown command "--version"`, exit 2; `doctor` prints no version either.
- Why it costs: the first question in any bug report; `docs/AGENTS.md` itself asks for the build that produced an output.
- Change: `zero version` and `--version` print `zero <version> (<commit>)`; `doctor` prints it as its first line.
- security effect: none.

#### U8 — three words for one thing: workspace, ctx/context, sandbox
- Evidence: `main.go:19` `[-c WORKSPACE]`; `config.go:216` `ctx: developer (flag)`; `commands.go:174` `… ready: …`; `ls.go:135` header `NAME`; spec §1 defines Workspace and Context separately.
- Why it costs: `-c` takes a workspace, the line printed is `ctx:`, `ls` calls it `NAME`, and `status`/`rm`/`pause` print `zero-developer` (sandbox name). The user must learn that all four are the same object.
- Change: use `workspace` in all user-facing text: `workspace: developer (from -c)`, `zero ls` header `WORKSPACE`, `zero rm` asks for the workspace name it just listed. Keep `-c`/`ZERO_CONTEXT` as is.
- security effect: none.

#### U9 — duplicate commands: `switch` = `ctx use`, `ctx rm` ≈ `ctx remove`
- Evidence: `switch.go:76-104` says "like `zero ctx use`"; `config.go:322-326` vs `373-386`; `main.go:35-36` lists `remove [PATH...]` and `rm [-y] NAME` on adjacent lines.
- Why it costs: two spellings of "select" and two near-identical spellings of two very different actions (drop a folder vs delete the sandbox and its state).
- Change: in usage show only `switch [NAME]`, `ctx new`, `ctx add`, `ctx remove`; keep `ctx use`/`ctx rm` as hidden aliases. Rename `ctx remove` → `ctx drop PATH...` (or `ctx remove-folder`) so no two commands differ by two letters.
- security effect: none.

#### U10 — the same state has two names: `paused` and `stopped`
- Evidence: `commands.go:454` prints `paused (zero up continues)`; `ls.go:101` sets `status = "stopped"` for the same `rt.Stopped` condition; docs `commands.md:74` shows `stopped`.
- Change: print `paused` in both.
- security effect: none.

#### U11 — `zero status` with nothing there is a dead end
- Evidence: observed `zero status` in a repo with no sandbox → `sandbox zero-z3r0: none` + `forward: off`, exit 0, no serve line, no hint.
- Why it costs: `status` is the first command a returning user runs; "none" does not say what to do.
- Change: append `next: zero up` when there is no sandbox, and `next: zero up` when the sandbox is paused (already partly there).
- security effect: none.

#### U12 — errors that do not say what to do next
- Evidence (each quoted from the code):
  - `config.go:224` `folders %s and %s have the same name; a workspace needs distinct folder names` — no command.
  - `config.go:359` `%s is not in workspace %s` — no command.
  - `commands.go:513` `sandbox %s: none` (also `commands.go:646` for `logs`) — no `zero up`.
  - `ls.go:75` `gateway not reachable: sandbox status unknown` — no `sudo zero install` / `systemctl` hint.
  - `forge.go:114` `folders are on %s and %s; a workspace has one forge host` — no `zero ctx new`.
- Why it costs: exactly the errors a user meets while building a multi-folder workspace are the ones that stop and explain nothing.
- Change: each ends with the command: `…; rename a folder or use zero ctx new NAME`.
- security effect: none.

#### U13 — internal ids in user-facing messages
- Evidence: `commands.go:517` `pause is not supported with --vm: the checkout would be lost (C50)`; `input.go:236` `… not supported on the %s gateway (ADR 0039)`; `policy.go:66` `… (ADR 0044)`; `policy.go:84` `… (ADR 0046)`; `forge.go:146` `the forge of %s changed (was %v, now %v)` prints a Go slice.
- Why it costs: `C50` and `ADR 0044` mean nothing without the repo; `%v` of `[]string` prints `[a b]`.
- Change: plain sentence, no id; put the id in a trailing `see docs …` only where it helps. Format slices with `strings.Join(…, ", ")`.
- security effect: none.

#### U14 — `--approve` accepts a wider policy without printing what it accepted
- Evidence: `policy.go:188` — the `policy widens for …` header and the `+ network …` list are printed only inside `if len(w) > 0 && !s.Approve`.
- Why it costs: the one moment the user should be able to see what the agent may now reach is skipped by the flag, and `up` then prints only `token: rotated …`.
- Change: always print the `+` list; `--approve` only skips the question. Add `approved: 2 additions` to the ready line.
- security effect: none (display only; still nothing widened without the flag or a `y`).

#### U15 — the `POLICY` column is a 6-hex sha
- Evidence: `ls.go:120-124`; observed `POLICY 9e841a`; `commands.md:92` explains it as "the commit the policy was read from".
- Why it costs: the column is unreadable at a glance; what the user wants to know is "is my policy the one that is applied".
- Change: `POLICY` → `ok`, `changed` (uncommitted) or `-`; keep the sha in `zero status` where there is room.
- security effect: none.

#### U16 — usage text is one flat list of 22 lines, no examples, no per-command help
- Evidence: `main.go:17-43`; observed `zero help` prints the same text; `zero help up` is `unknown command "up"?`-style failure (dispatch only accepts `help` bare, `main.go:71-73`).
- Why it costs: `install`/`doctor`/`completion` sit between `rm` and `ls`; the four "up + something" commands are not grouped, so the difference between `up` and `attach` is easy to miss; no example at all.
- Change: four groups (`start/stop`, `agent`, `workspaces`, `host`), a `Examples:` block of 3 lines, and `zero help <command>` printing one command's line plus its flags.
- security effect: none.

#### U17 — docs say `zero rm` deletes the workspace; it only deletes the sandbox
- Evidence: `commands.go:537-568` deletes sandbox, token, state; the `Workspace` entry stays in `~/.config/zero/config.yaml` (`Ctx`/`rm` differ: `config.go:373-386` deletes both).
- Why it costs: after `zero rm`, `zero ls` still lists the workspace (`STATUS none`) and `zero up` recreates it — the opposite of what "rm" suggests; users then hunt for the "real" delete (`ctx rm`).
- Change: docs line + `removed` output: `removed sandbox zero-api; workspace api kept (zero ctx rm api deletes it)`. Keep both commands.
- security effect: none.

### Low

#### U18 — `*` in `ls`, `*` and `+` in `ctx`, no legend in either
- Evidence: `ls.go:135-142` only `*`; `config.go:306-314` `*` and `+`; `commands.md:75` explains both only for `ctx`.
- Change: show `*` (current) and `+` (this folder) in both tables, one legend line at the bottom when used.
- security effect: none.

#### U19 — `zero ctx` and `zero ls` are two tables of the same data
- Evidence: `config.go:299-316` (name + folders) vs `ls.go:133-201` (name, status, folders count, token, policy, orphans).
- Change: `zero ctx` prints the `ls` table plus a `FOLDERS` path column; keep `ctx` as the script/name-resolution view. One table to learn.
- security effect: none.

#### U20 — orphan and warning lines have no action
- Evidence: `ls.go:174`, `180`, `186`, `192` print `orphans: token … (no sandbox)`, `orphans: sandbox …`, `orphans: state …`, `orphans: input copy …`; `ls.go:198` `warning: …`; spec §4 says `zero-cp` revokes token orphans.
- Change: one closing line `next: zero-cp revokes token orphans; zero rm -y <workspace> removes the rest`, and singular `orphan:` per line.
- security effect: none.

#### U21 — exit codes are not stated anywhere
- Evidence: observed 0 for `help`, `ls`, `status` (even `none`), `switch` + `q`; 1 for runtime errors (`-c nope status`, `doctor` FAIL, `completion zsh`); 2 for missing/unknown command (`zero nonsense`, `zero --version`).
- Why it costs: a script cannot tell `status: none` (exit 0) from a healthy sandbox, and the 1-vs-2 split is invisible.
- Change: one line in `commands.md` and in usage: `exit 0 ok, 1 failed, 2 usage`; say that `status`/`ls` report state in the text, not the exit code.
- security effect: none.

#### U22 — small gaps in the text
- `main.go:37` documents `completion bash` only; `zero completion zsh` answers `usage: zero completion bash` (exit 1) without saying "only bash is supported".
- `config.go:300` accepts `zero ctx list` but usage and `commands.md` never mention it.
- `main.go:35-36` does not give the `ctx new` default (`PATH...` defaults to the current folder, `config.go:267-270`).
- `main.go:41-42` names `zero ctx use` in the resolution order but not `switch`, which does the same thing.

## Top 5 changes that make zero simpler

1. **U6 + U7: strict flags and a version.** Reject unknown options (a typo currently starts a sandbox), add `zero version`. Two small patches, both remove surprises.
2. **U3: check the model login in `doctor` and warn in `up`.** The only prerequisite that fails after the slowest step today is the one nothing checks.
3. **U2 + U11: always print the next command.** `up` ends with `next: zero attach`; `status` with nothing there ends with `next: zero up`. Removes the "what now" moment for every new and returning user.
4. **U4 + U5: tell the truth about a non-GitLab origin.** `na` in `doctor` instead of `FAIL`, and one `forge: none` notice in `up`/`status`, so the user knows the agent cannot hand over work before it starts.
5. **U9 + U8 + U10: one name per thing.** Hide `ctx use` (keep `switch`), separate `ctx remove` from `ctx rm`, print `workspace:` instead of `ctx:`, `paused` in both tables. Pure wording; nothing to relearn afterwards.

## Fine as it is

- `rm` asks for the typed workspace name and prints what is unpushed first (`ls.go:265-273`, `commands.go:543-548`).
- Policy widening needs a `y` at the terminal; no terminal means no widening (`policy.go:193-201`). Do not relax this.
- The `ctx: name (source)` line: the idea is right, only the word is wrong (U8).
- `zero switch` without a name: arrows/`j`/`k`, `q` leaves everything unchanged and exits 0 (`switch.go:13`, `switch.go:107-166`).
- `doctor`'s `ok`/`FAIL`/`skip` prefixes, and `skip credential store: run sudo zero doctor`.
- Usage text states the workspace resolution order in three lines (`main.go:41-42`).
- Input folder summary and the `excluded:` list (`input.go:294-301`); `--max-input` error names the fix.
- `zero pause` prints `paused x` and its usage line says `zero up` continues.
- Exit 0 for `help`, exit 2 for no arguments: conventional, keep.
- `zero completion bash` output completes commands and workspace names.
