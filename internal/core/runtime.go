package core

// Runtime is the sandbox substrate zero runs on. zero owns the workflow,
// credentials and policy intent; a Runtime only executes them.
type Runtime interface {
	Name() string
	Exists(name string) bool
	// Reachable reports whether the runtime answers at all; Exists is only meaningful if it does.
	Reachable() bool
	// List returns the names of all sandboxes.
	List() ([]string, error)
	// Create starts a sandbox from image with a filesystem policy file and
	// credential bindings (ids returned by BindCredential). A non-empty
	// mountSrc (host directory) is mounted read-only at mountDst.
	Create(name, image, policyFile string, creds, env []string, mountSrc, mountDst string) error
	Delete(name string) error
	// Stop halts a sandbox and keeps its files; Start brings it back with
	// fresh processes. Stopped reports that state.
	Stop(name string) error
	Start(name string) error
	Stopped(name string) bool
	// SSHConfig returns an ssh_config snippet; HostAlias is its Host entry.
	SSHConfig(name string) (string, error)
	HostAlias(name string) string
	Upload(name, src, dst string) error
	// AllowEgress lets binary reach host:443 (REST, enforced) under rule;
	// access is read-only or read-write.
	AllowEgress(name, rule, binary, access string, hosts []string) error
	// SetRule replaces the network rule named rule of a running sandbox
	// and keeps the other rules.
	SetRule(name, rule string, spec map[string]any) error
	// ImportCredentialType registers how a credential is bound (endpoints,
	// header, binaries). An existing type is kept.
	ImportCredentialType(id string, spec []byte) error
	// BindCredential stores secret under id for type, exposed as env in the
	// sandbox as a placeholder only.
	BindCredential(id, typ, env, secret string) error
	UnbindCredential(id string)
	// RotateCredential replaces the secret of an existing binding.
	RotateCredential(id, env, secret string) error
	// AttachCredential gives a running sandbox a binding made after it was
	// created; the sandbox holds it back until a rule names it.
	// CredentialReady waits until the sandbox has applied it.
	AttachCredential(name, id string) error
	DetachCredential(name, id string)
	CredentialReady(name, id string) error
}

// EgressRule matches requests by method and path glob ("*" within a
// segment, "**" across segments, at least one). Query is optional.
type EgressRule struct {
	Method string
	Path   string
	Query  map[string]string
}

// Matcher is the policy form of the rule.
func (r EgressRule) Matcher() map[string]any {
	m := map[string]any{"method": r.Method, "path": r.Path}
	if len(r.Query) > 0 {
		m["query"] = r.Query
	}
	return m
}
