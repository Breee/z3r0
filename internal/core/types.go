// Package core holds what every part of zero shares: the types that cross
// between parts, the interfaces the parts implement (docs/spec/interfaces.md)
// and a few helpers. It imports no other package of zero.
package core

import "time"

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

// Sandbox is one sandbox as its provider lists it.
type Sandbox struct{ Name, State string } // State: running, stopped, unknown

// Mount is a host directory shown read-only in the sandbox.
type Mount struct{ Source, Target string }

// SandboxSpec is what a new sandbox is made from.
type SandboxSpec struct {
	Name, Image string
	Policy      []byte // file policy and first egress rules, in the provider's form
	Env         []string
	Mounts      []Mount
	Credentials []string // ids bound before, attached at creation
}

// Token is a forge token as the forge lists it.
type Token struct {
	ID, Name string
	Expires  time.Time
}

// Model is a model the agent may list.
type Model struct {
	ID, Name        string
	Context, Output int // token limits; 0: unknown
}

// ToolServer is one tool server written into the agent's config.
type ToolServer struct{ Name, URL string }

// File is a regular file on the host, by absolute path.
type File struct{ Path string }

// Dir is a directory on the host, by absolute path.
type Dir struct{ Path string }

// Exec runs a command in one sandbox.
type Exec interface {
	Run(script string) (string, error)
}

// Launch is the command zero runs on the host for the user.
type Launch struct {
	Name string
	Args []string
	Env  []string
}

// AgentConfig is everything an agent writes its own config from.
type AgentConfig struct {
	Models       ModelAccess
	Tools        ToolAccess
	Instructions []string // paths in the sandbox
	Skills       []string // paths in the sandbox
	Model        string   // default model; empty: the agent's own
}

// SandboxHost is the name a sandbox reaches the host's own services by.
const SandboxHost = "host.openshell.internal"
