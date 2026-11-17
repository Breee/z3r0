package zero

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// openshell is the Runtime backed by the NVIDIA OpenShell CLI.
type openshell struct{}

func (openshell) Name() string { return "openshell" }

func (openshell) Exists(name string) bool {
	_, err := sh("openshell", "sandbox", "get", name)
	return err == nil
}

func (openshell) Reachable() bool {
	_, err := sh("openshell", "gateway", "info")
	return err == nil
}

func (openshell) Create(name, image, policyFile string, creds, env []string, mountSrc, mountDst string) error {
	args := []string{"sandbox", "create", "--name", name, "--from", image, "--policy", policyFile, "--detach"}
	for _, c := range creds {
		args = append(args, "--provider", c)
	}
	for _, e := range env {
		args = append(args, "--env", e)
	}
	if mountSrc != "" {
		type mount struct {
			Type     string `json:"type"`
			Source   string `json:"source"`
			Target   string `json:"target"`
			ReadOnly bool   `json:"read_only"`
		}
		b, _ := json.Marshal(map[string]any{"podman": map[string]any{"mounts": []mount{{"bind", mountSrc, mountDst, true}}}})
		args = append(args, "--driver-config-json", string(b))
	}
	_, err := sh("openshell", args...)
	return err
}

func (openshell) Delete(name string) error {
	_, err := sh("openshell", "sandbox", "delete", name)
	return err
}

func (openshell) Stop(name string) error {
	_, err := sh("openshell", "sandbox", "stop", name)
	return err
}

// Start waits until the sandbox is ready again.
func (openshell) Start(name string) error {
	if _, err := sh("openshell", "sandbox", "start", name); err != nil {
		return err
	}
	for i := 0; i < 60; i++ {
		if sandboxPhase(name) == "Ready" {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("sandbox %s not ready 60s after start (phase %s)", name, sandboxPhase(name))
}

func (openshell) Stopped(name string) bool { return sandboxPhase(name) == "Stopped" }

func sandboxPhase(name string) string {
	out, _ := sh("openshell", "sandbox", "get", name)
	for _, l := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), "Phase:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
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
	// One endpoint per call; each --wait costs seconds, so only the last waits.
	for i, h := range hosts {
		args := []string{"policy", "update", name,
			"--rule-name", rule,
			"--binary", binary,
			"--add-endpoint", h + ":443:" + access + ":rest:enforce"}
		if i == len(hosts)-1 {
			args = append(args, "--wait")
		}
		if _, err := sh("openshell", args...); err != nil {
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

// RotateCredential passes the secret via env lookup, never in argv.
func (openshell) RotateCredential(id, env, secret string) error {
	cmd := exec.Command("openshell", "provider", "update", id, "--credential", env)
	cmd.Env = append(os.Environ(), env+"="+secret)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("provider update: %v: %s", err, strings.TrimSpace(string(b)))
	}
	return nil
}
