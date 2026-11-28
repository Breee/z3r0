package core

// ForgeHost is the forge interface in use until the parts move to Forge.
// It is a git hosting strategy (D20). zero owns the lifecycle — profile
// import, OpenShell provider, sandbox remote, state — and a Forge only knows
// its host and how to mint and revoke a project-scoped token. Add a forge by
// implementing this interface and registering it in forges.
type ForgeHost interface {
	// Name is the short id used for the profile (zero-<name>), provider
	// (<sandbox>-<name>) and state.
	Name() string
	// Host is the https host the token is valid for, e.g. gitlab.com.
	Host() string
	// ProfileID and Profile are the OpenShell provider profile of the token.
	ProfileID() string
	Profile() []byte
	// Egress is what the sandbox may reach on the forge host for the
	// projects at paths; anything not allowed is refused.
	Egress(paths []string) (allow, deny []EgressRule, err error)
	// LoggedIn is nil when the host session can mint tokens; the error says
	// how to log in.
	LoggedIn() error
	// TokenEnv is the env var the sandbox sees the placeholder in.
	TokenEnv() string
	// GitUser is the https username git sends with the token.
	GitUser() string
	// Permissions maps policy capabilities and MCP tools to token
	// permissions; an unknown capability or tool is an error.
	Permissions(caps, tools []string) ([]string, error)
	// Mint creates a short-lived token scoped to paths; id is used to revoke.
	Mint(label string, paths []string, perms []string) (token, id string, err error)
	// Revoke revokes a token by id; an expired or revoked token is not an error.
	Revoke(id string) error
	// Tokens lists the user's active tokens.
	Tokens() ([]Token, error)
	// Env is extra non-secret sandbox env the forge's tools need.
	Env() []string
	// DefinitionChanged names a pipeline definition file that commit sha
	// changes against the default branch of the project at path, or "".
	// token is the sandbox's token. An error means: not known.
	DefinitionChanged(token, path, sha string) (file string, err error)
	// Guardrails lists missing DevOps protections on the project (ADR 0024):
	// default branch push-protected, merge only with green pipeline.
	Guardrails(path string) (missing []string, err error)
}

// CapMRCreate is not in the default set: opening a merge request starts a
// merge request pipeline, which ci.skip on the push does not prevent. It is
// accepted only together with CapCIRun (ADR 0044).
const CapMRCreate = "gitlab.mr.create"

// CapCIRun lets pipelines start on the agent's pushes. Without it (default)
// the push gate adds ci.skip and a human starts the pipeline (ADR 0035).
const CapCIRun = "ci.run"
