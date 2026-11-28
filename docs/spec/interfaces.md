# Interfaces

Status: proposed (#190). No code is moved before this is read and agreed.

zero owns the workflow: workspaces, policy and its approval, the push gate,
state, the order of steps in `zero up`. Everything else is a part that can be
replaced. Each part sits behind one interface, written from what zero needs of
it. A new part is one new package and one line that registers it.

## 1. Rules for every interface

1. **Requirement first.** A method exists because zero's workflow needs the
   answer, not because today's implementation offers it.
2. **Small.** Optional abilities are separate interfaces, asked for with a type
   assertion (`if r, ok := p.(Rotator); ok`). No method that half the
   implementations answer with "not supported".
3. **No real secret into the sandbox.** A provider hands secrets to zero as a
   `Credential`; only zero binds it, and the sandbox sees a placeholder
   (Z-INV-2). No provider writes into the sandbox by itself.
4. **Egress is declared, not opened.** A provider says which hosts and paths it
   needs (`[]Egress`); zero turns that into rules and shows it in the policy.
5. **Providers do not know each other.** They get plain data (names, paths,
   lists), never another provider. zero connects them.
6. **Errors say what to do.** A provider that is not usable says the command
   that makes it usable (`LoggedIn() error` style); `zero doctor` prints it.
7. **State belongs to zero.** A provider gets a `State` (read, write, remove
   under the workspace's state directory) instead of making its own files.
8. **Named and registered.** `Name()` is the word used in config and output.
   Config picks an implementation by that name.

Shared types:

```go
// Credential is a secret zero binds to a sandbox as a placeholder.
type Credential struct {
	Env     string   // variable the sandbox sees the placeholder in
	Secret  string   // never logged, never written into the sandbox
	Hosts   []string // the only hosts the real value is sent to
	Header  string   // header it is sent in
	Expires time.Time
	Revoke  func() error // nil: nothing to revoke
}

// Egress is one thing the sandbox may reach.
type Egress struct {
	Host   string
	Paths  []string // empty: every path
	Write  bool     // false: read-only methods
	Binary string   // empty: every binary
}

// Check is one line of zero doctor.
type Check struct{ Name, Problem string } // Problem empty: ok
```

## 2. The parts

| Interface | zero needs it to | Today | Checked against |
|---|---|---|---|
| `SandboxProvider` | run an isolated machine with a policy, bind credentials as placeholders | OpenShell (podman, Docker) | nono (#180) |
| `Forge` | give a short-lived token for exactly these projects; say what the remote protects | GitLab | GitHub App (#186) |
| `ModelProvider` | give the sandbox models without the provider's key | LiteLLM | Copilot directly (exists as a special case) |
| `ToolProvider` | give the sandbox tools (MCP) without their tokens | LiteLLM's MCP gateway | MCP servers reached directly (#178) |
| `InstructionsProvider` | resolve named instructions to files | host files | a git repository |
| `SkillProvider` | resolve named skills to directories | host directories | a git repository |
| `Agent` | set up and start one coding agent in the sandbox | opencode, VS Code, Copilot CLI, Claude Code | each other |

### 2.1 SandboxProvider

Today `Runtime` (22 methods, OpenShell-shaped: `SetRule(spec map[string]any)`,
`ImportCredentialType(spec []byte)` take OpenShell's own formats).

Requirements: make, start, stop, remove a sandbox; run a command in it and
copy files in; enforce a file policy and a list of `Egress`; hold a
`Credential` and put a placeholder in; say whether it is reachable.

```go
type SandboxProvider interface {
	Name() string
	Check() []Check
	List() ([]Sandbox, error)                // name + state
	Create(spec SandboxSpec) error           // image, file policy, env, mounts, egress, credentials
	Start(name string) error
	Stop(name string) error
	Delete(name string) error
	Exec(name string, cmd string, in io.Reader) ([]byte, error)
	Upload(name, src, dst string) error
	SetEgress(name, rule string, e []Egress) error // replaces the rule; nil removes it
	Bind(name string, id string, c Credential) error
	Unbind(name, id string) error
}

// Optional.
type Connector interface{ SSHConfig(name string) (alias, config string, err error) } // editors
type Rotator interface{ Rotate(id string, c Credential) error }                      // without restart
```

Change against today: zero stops building OpenShell rule maps and profile
YAML; that moves into the OpenShell package. `Exec` replaces the ssh calls
spread over the code. Security: a provider that cannot enforce `Egress` per
path or cannot keep a credential out of the sandbox must fail `Create`, not
weaken it.

### 2.2 Forge

Today's interface is close. Requirements: recognise a remote; mint and revoke
a token limited to projects and permissions; say what the sandbox must reach;
say what the remote protects (guardrails) and whether a commit changes the
pipeline definition (push gate).

```go
type Forge interface {
	Name() string
	Match(remote string) (project string, ok bool)
	Check() []Check                                    // logged in?
	Permissions(caps, tools []string) ([]string, error)
	Mint(label string, projects, perms []string) (Credential, id string, err error)
	Tokens() ([]Token, error)                          // for zero clean
	Egress(projects []string) (allow, deny []Egress)
	GitUser() string
	Env() []string
	Guardrails(project string) (missing []string, err error)
	DefinitionChanged(c Credential, project, sha string) (file string, err error)
}
```

Change: `ProfileID`/`Profile` (OpenShell formats) go; `Mint` returns a
`Credential`. Capability names (`gitlab.mr.read`) become forge-neutral names
with the old ones kept as aliases: decided in #186.

### 2.3 ModelProvider

Today: LiteLLM in `modelproxy.go`, called directly from `up`.

Requirements: be running (or say why not); list the models a workspace may
use; give one credential per workspace limited to those models; take it back.

```go
type ModelProvider interface {
	Name() string
	Check() []Check
	Ensure(out io.Writer) error                         // start or reach it; idempotent
	Grant(ws string, models []string) (ModelAccess, error)
	Revoke(ws string) error
}

type ModelAccess struct {
	Credential Credential
	BaseURL    string   // what the agent is pointed at
	API        string   // "openai" | "anthropic"
	Models     []Model  // id, name, limits: what the agent may list
	Egress     []Egress
	Warnings   []string // e.g. a provider that is not connected
}
```

Second case: Copilot directly is `Grant` returning GitHub's host and the
user's Copilot token as the credential; it has no `Ensure` work. Security: the
upstream provider keys stay with the provider; `Grant` must return a key that
is limited to `models` and to this workspace.

### 2.4 ToolProvider

Today: MCP servers through LiteLLM (`mcpproxy.go`), tied to the model proxy.

```go
type ToolProvider interface {
	Name() string
	Check() []Check
	Grant(ws string, tools []string, forge Credential) (ToolAccess, error)
	Revoke(ws string) error
}

type ToolAccess struct {
	Credential Credential
	Servers    []ToolServer // name, URL: written into the agent's config
	Egress     []Egress
}
```

One implementation may serve both `ModelProvider` and `ToolProvider`
(LiteLLM does); zero treats them as two.

### 2.5 InstructionsProvider and SkillProvider

Today: `guidance.go` resolves paths on the host. Both needs are "turn the names
in the workspace config into local files, checked"; they differ in the shape
(file or directory with `SKILL.md`), so they are two interfaces over one
shared source type.

```go
type InstructionsProvider interface {
	Name() string
	Instructions(refs []string) ([]File, error) // absolute host paths, regular files
}

type SkillProvider interface {
	Name() string
	Skills(refs []string) ([]Dir, error) // directories with a SKILL.md
}
```

A reference is `name:rest` (`git:https://…#path`); without a prefix it is a
host path, as today. zero copies the result into the sandbox read-only; a
provider only fetches to the host. Content is input, never trusted: it cannot
change policy.

### 2.6 Agent

Today: opencode is assumed in `syncModels`/`syncGuidance` (`opencode service
…`, `opencode.json`); VS Code, Copilot CLI and Claude Code are scripts in
`commands.go`.

```go
type Agent interface {
	Name() string
	// Configure writes the agent's own config in the sandbox from plain data.
	Configure(sb Exec, c AgentConfig) error // models, tools, instructions, skills, default model
	// Start gives the command zero runs for the user (attach, editor, CLI).
	Start(sb Exec, dir string, args []string) (Launch, error)
	Egress() []Egress // what the agent itself needs (updates: none by default)
}

// Optional: an agent that keeps a server running in the sandbox.
type Server interface {
	Serve(sb Exec) error
	Ready(sb Exec) error
}
```

`AgentConfig` holds `ModelAccess`, `ToolAccess`, the copied instruction and
skill paths: this is the one place where the parts meet, and zero fills it.

## 3. Package layout

```
cmd/zero/                 argument parsing only
internal/
  core/                   Credential, Egress, Check, State; the interfaces above
  workspace/              config, workspaces, switch, ls
  policy/                 policy file, approval, first run
  gate/                   push gate
  up/                     the workflow: up, stop, rm, clean; connects the parts
  sandbox/openshell/      SandboxProvider
  forge/gitlab/           Forge        (forge/github/ with #186)
  model/litellm/          ModelProvider + ToolProvider
  model/copilot/          ModelProvider
  guidance/hostfiles/     InstructionsProvider + SkillProvider
  agent/{opencode,vscode,copilot,claude}/
  tui/                    workspace screen, first-run view, progress
  ui/                     web UI
  install/                install, doctor
```

Imports go one way: `cmd` → `up`, `tui`, `ui`, `install` → parts → `core`. A
part never imports another part or `up`.

## 4. Order of work

1. `core` with the shared types and interfaces; no behaviour change.
2. One component per commit, tests green each time: workspace, policy, gate,
   sandbox/openshell, forge/gitlab, model/litellm, guidance, agents, tui, ui,
   install; `up` last.
3. Attack suite and `zero doctor` after the last move.

Tests move with their component; `zero_test.go` (2500 lines) is split the
same way.

## 5. Not in this work

- New implementations (GitHub #186, nono #180, tools without proxy #178).
- Loading providers from outside the binary (plugins). The interfaces allow
  it later; nothing asks for it now.
- Renaming config keys.

## 6. Decided by the user (2026-11-28)

1. One `ModelProvider` per workspace, chosen in the workspace config. More than
   one at once is unlikely to be needed and is not built.
2. `InstructionsProvider` and `SkillProvider` are two interfaces with a shared
   source.
3. Permission names depend on what the forge offers: each `Forge` lists the
   permissions it has (name and explaining line); only `git.read`,
   `git.commit` and `git.push` are the same for every forge. A policy naming a
   permission its forge does not offer is refused. GitHub's names come with
   #186.
