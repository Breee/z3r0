package zero

// Runtime is the sandbox substrate zero runs on. zero owns the workflow,
// credentials and policy intent; a Runtime only executes them.
type Runtime interface {
	Name() string
	Exists(name string) bool
	// Create starts a sandbox from image with a filesystem policy file and
	// credential bindings (ids returned by BindCredential).
	Create(name, image, policyFile string, creds []string) error
	Delete(name string) error
	// SSHConfig returns an ssh_config snippet; HostAlias is its Host entry.
	SSHConfig(name string) (string, error)
	HostAlias(name string) string
	Upload(name, src, dst string) error
	// AllowEgress lets binary reach host:443 (REST, enforced) under rule.
	AllowEgress(name, rule, binary string, hosts []string) error
	// ImportCredentialType registers how a credential is bound (endpoints,
	// header, binaries). An existing type is kept.
	ImportCredentialType(id string, spec []byte) error
	// BindCredential stores secret under id for type, exposed as env in the
	// sandbox as a placeholder only.
	BindCredential(id, typ, env, secret string) error
	UnbindCredential(id string)
}

var rt Runtime = openshell{}
