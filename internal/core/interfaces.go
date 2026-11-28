package core

import "io"

// SandboxProvider is the system the sandboxes run on.
type SandboxProvider interface {
	Name() string
	Check() []Check
	List() ([]Sandbox, error)
	Create(spec SandboxSpec) error
	Start(name string) error
	Stop(name string) error
	Delete(name string) error
	Exec(name string, cmd string, in io.Reader) ([]byte, error)
	Upload(name, src, dst string) error
	SetEgress(name, rule string, e []Egress) error // replaces the rule; nil removes it
	Bind(name string, id string, c Credential) error
	Unbind(name, id string) error
}

// Connector is optional: a provider editors can reach over ssh.
type Connector interface {
	SSHConfig(name string) (alias, config string, err error)
}

// Rotator is optional: a provider that takes a new secret without a restart.
type Rotator interface {
	Rotate(id string, c Credential) error
}

// Forge is where the code lives and where the agent pushes.
type Forge interface {
	Name() string
	Match(remote string) (project string, ok bool)
	Check() []Check
	Permissions(caps, tools []string) ([]string, error)
	Mint(label string, projects, perms []string) (c Credential, id string, err error)
	Tokens() ([]Token, error)
	Egress(projects []string) (allow, deny []Egress)
	GitUser() string
	Env() []string
	Guardrails(project string) (missing []string, err error)
	DefinitionChanged(c Credential, project, sha string) (file string, err error)
}

// ModelProvider gives a workspace its models.
type ModelProvider interface {
	Name() string
	Check() []Check
	Ensure(out io.Writer) error // start or reach it; idempotent
	Grant(ws string, models []string) (ModelAccess, error)
	Revoke(ws string) error
}

// ModelAccess is what a workspace got from its ModelProvider.
type ModelAccess struct {
	Credential Credential
	BaseURL    string // what the agent is pointed at
	API        string // "openai" | "anthropic"
	Models     []Model
	Egress     []Egress
	Warnings   []string // e.g. a provider that is not connected
}

// ToolProvider gives a workspace its tools.
type ToolProvider interface {
	Name() string
	Check() []Check
	Grant(ws string, tools []string, forge Credential) (ToolAccess, error)
	Revoke(ws string) error
}

// ToolAccess is what a workspace got from its ToolProvider.
type ToolAccess struct {
	Credential Credential
	Servers    []ToolServer
	Egress     []Egress
}

// InstructionsProvider resolves instruction references to files on the host.
type InstructionsProvider interface {
	Name() string
	Instructions(refs []string) ([]File, error)
}

// SkillProvider resolves skill references to directories with a SKILL.md.
type SkillProvider interface {
	Name() string
	Skills(refs []string) ([]Dir, error)
}

// Agent is a coding agent zero starts in a sandbox.
type Agent interface {
	Name() string
	// Configure writes the agent's own config in the sandbox from plain data.
	Configure(sb Exec, c AgentConfig) error
	// Start gives the command zero runs for the user (attach, editor, CLI).
	Start(sb Exec, dir string, args []string) (Launch, error)
	Egress() []Egress // what the agent itself needs
}

// Server is optional: an agent that keeps a server running in the sandbox.
type Server interface {
	Serve(sb Exec) error
	Ready(sb Exec) error
}
