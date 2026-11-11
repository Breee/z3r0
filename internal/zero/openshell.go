package zero

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// openshell is the Runtime backed by the NVIDIA OpenShell CLI.
type openshell struct{}

func (openshell) Name() string { return "openshell" }

func (openshell) Exists(name string) bool {
	_, err := sh("openshell", "sandbox", "get", name)
	return err == nil
}

func (openshell) Create(name, image, policyFile string, creds, env []string) error {
	args := []string{"sandbox", "create", "--name", name, "--from", image, "--policy", policyFile, "--detach"}
	for _, c := range creds {
		args = append(args, "--provider", c)
	}
	for _, e := range env {
		args = append(args, "--env", e)
	}
	_, err := sh("openshell", args...)
	return err
}

func (openshell) Delete(name string) error {
	_, err := sh("openshell", "sandbox", "delete", name)
	return err
}

func (openshell) SSHConfig(name string) (string, error) {
	return sh("openshell", "sandbox", "ssh-config", name)
}

func (openshell) HostAlias(name string) string { return "openshell-" + name + ".default" }

func (openshell) Upload(name, src, dst string) error {
	_, err := sh("openshell", "sandbox", "upload", name, src, dst)
	return err
}

func (openshell) AllowEgress(name, rule, binary, access string, hosts []string) error {
	for _, h := range hosts {
		if _, err := sh("openshell", "policy", "update", name,
			"--rule-name", rule,
			"--binary", binary,
			"--add-endpoint", h+":443:"+access+":rest:enforce",
			"--wait"); err != nil {
			return fmt.Errorf("%s: %w", h, err)
		}
	}
	return nil
}

// ImportCredentialType imports a provider profile. `profile update` needs a
// resource_version, so an existing profile is kept; after editing one run
// `openshell profile delete <id>` once.
func (openshell) ImportCredentialType(id string, spec []byte) error {
	tmp, err := os.CreateTemp("", id+"-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(spec); err != nil {
		return err
	}
	tmp.Close()
	if _, err := sh("openshell", "profile", "import", "-f", tmp.Name()); err != nil &&
		!strings.Contains(err.Error(), "already exists") {
		return err
	}
	return nil
}

// BindCredential passes the secret via env lookup, never in argv.
func (openshell) BindCredential(id, typ, env, secret string) error {
	sh("openshell", "provider", "delete", id)
	cmd := exec.Command("openshell", "provider", "create", "--name", id, "--type", typ, "--credential", env)
	cmd.Env = append(os.Environ(), env+"="+secret)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("provider create: %v: %s", err, strings.TrimSpace(string(b)))
	}
	return nil
}

func (openshell) UnbindCredential(id string) { sh("openshell", "provider", "delete", id) }
