// Package openshell runs sandboxes through an OpenShell gateway.
package openshell

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Breee/z3r0/internal/core"
)

// openshell is the Runtime backed by the NVIDIA OpenShell CLI.
// Provider is the OpenShell sandbox system.
type Provider struct{}

// VMGateway is the second OpenShell gateway with the MicroVM driver (E3, ADR 0015).
const VMGateway = "openshell-vm"

// DefaultGateway is the gateway `zero install` registers. State records it as "".
const DefaultGateway = "openshell"

// GatewayName is the openshell gateway for a recorded state value. zero always
// names the gateway, so OPENSHELL_GATEWAY of the caller and `openshell gateway
// select` do not decide where a sandbox is looked up.
func GatewayName(gw string) string {
	if gw == "" {
		return DefaultGateway
	}
	return gw
}

func (Provider) Name() string { return "openshell" }

func (Provider) Exists(name string) bool {
	_, err := core.Sh("openshell", "sandbox", "get", name)
	return err == nil
}

func (Provider) Reachable() bool {
	_, err := core.Sh("openshell", "gateway", "info")
	return err == nil
}

func (Provider) List() ([]string, error) {
	out, err := core.Sh("openshell", "sandbox", "list", "--names", "--page-size", "1000")
	if err != nil {
		return nil, fmt.Errorf("list sandboxes: %w", err)
	}
	return strings.Fields(out), nil
}

func (Provider) Create(name, image, policyFile string, creds, env []string, mountSrc, mountDst string) error {
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
		drv, err := ComputeDriver()
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]any{drv: map[string]any{"mounts": []mount{{"bind", mountSrc, mountDst, true}}}})
		args = append(args, "--driver-config-json", string(b))
	}
	_, err := core.Sh("openshell", args...)
	return err
}

func (Provider) Delete(name string) error {
	_, err := core.Sh("openshell", "sandbox", "delete", name)
	return err
}

func (Provider) Stop(name string) error {
	_, err := core.Sh("openshell", "sandbox", "stop", name)
	return err
}

// Start waits until the sandbox is ready again.
func (Provider) Start(name string) error {
	if _, err := core.Sh("openshell", "sandbox", "start", name); err != nil {
		return err
	}
	for i := 0; i < 60; i++ {
		if sandboxPhase(name) == "Ready" {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("sandbox %s not ready 60s after start (phase %s); check: zero doctor", name, sandboxPhase(name))
}

func (Provider) Stopped(name string) bool { return sandboxPhase(name) == "Stopped" }

func sandboxPhase(name string) string {
	out, _ := core.Sh("openshell", "sandbox", "get", name)
	for _, l := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), "Phase:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (Provider) SSHConfig(name string) (string, error) {
	return core.Sh("openshell", "sandbox", "ssh-config", name)
}

func (Provider) HostAlias(name string) string { return "openshell-" + name + ".default" }

func (Provider) Upload(name, src, dst string) error {
	_, err := core.Sh("openshell", "sandbox", "upload", name, src, dst)
	return err
}

func (Provider) AllowEgress(name, rule, binary, access string, hosts []string) error {
	// One endpoint per call; each --wait costs seconds, so only the last waits.
	for i, h := range hosts {
		args := []string{"policy", "update", name,
			"--rule-name", rule,
			"--binary", binary,
			"--add-endpoint", h + ":443:" + access + ":rest:enforce"}
		if i == len(hosts)-1 {
			args = append(args, "--wait")
		}
		if _, err := core.Sh("openshell", args...); err != nil {
			return fmt.Errorf("%s: %w", h, err)
		}
	}
	return nil
}

func (Provider) SetRule(name, rule string, spec map[string]any) error {
	out, err := core.Sh("openshell", "policy", "get", name, "--base")
	if err != nil {
		return err
	}
	_, base, ok := strings.Cut(out, "\n---\n")
	if !ok {
		return fmt.Errorf("policy get %s: no policy in the answer", name)
	}
	b, err := PolicyWith([]byte(base), rule, spec)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp("", name+"-policy-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		return err
	}
	tmp.Close()
	_, err = core.Sh("openshell", "policy", "set", name, "--policy", tmp.Name(), "--wait")
	return err
}

// ImportCredentialType imports a provider profile. `profile update` needs a
// resource_version, so an existing profile is kept; after editing one run
// `openshell profile delete <id>` once.
func (Provider) ImportCredentialType(id string, spec []byte) error {
	tmp, err := os.CreateTemp("", id+"-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(spec); err != nil {
		return err
	}
	tmp.Close()
	if _, err := core.Sh("openshell", "profile", "import", "-f", tmp.Name()); err != nil &&
		!strings.Contains(err.Error(), "already exists") {
		return err
	}
	return nil
}

// BindCredential passes the secret via env lookup, never in argv.
func (Provider) BindCredential(id, typ, env, secret string) error {
	core.Sh("openshell", "provider", "delete", id)
	cmd := exec.Command("openshell", "provider", "create", "--name", id, "--type", typ, "--credential", env)
	cmd.Env = append(os.Environ(), env+"="+secret)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("provider create: %v: %s", err, strings.TrimSpace(string(b)))
	}
	return nil
}

func (Provider) UnbindCredential(id string) { core.Sh("openshell", "provider", "delete", id) }

// RotateCredential passes the secret via env lookup, never in argv.
func (Provider) RotateCredential(id, env, secret string) error {
	cmd := exec.Command("openshell", "provider", "update", id, "--credential", env)
	cmd.Env = append(os.Environ(), env+"="+secret)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("provider update: %v: %s", err, strings.TrimSpace(string(b)))
	}
	return nil
}

// AttachCredential does not wait: until a rule binds the credential to a
// host the sandbox withholds it, and the rule can only name an attached one
// (PoC 20).
func (Provider) AttachCredential(name, id string) error {
	_, err := core.Sh("openshell", "sandbox", "provider", "attach", name, id)
	return err
}

func (Provider) DetachCredential(name, id string) {
	core.Sh("openshell", "sandbox", "provider", "detach", "--wait", name, id)
}

func (Provider) CredentialReady(name, id string) error {
	_, err := core.Sh("openshell", "sandbox", "provider", "status", "--wait", name, id)
	return err
}

// ComputeDriver names the gateway's compute driver (podman or docker); the
// mount settings are passed under that name (ADR 0057).
func ComputeDriver() (string, error) {
	out, err := core.Sh("openshell", "gateway", "info")
	if err != nil {
		return "", err
	}
	if d := ParseComputeDriver(out); d != "" {
		return d, nil
	}
	return "", fmt.Errorf("openshell gateway info names no compute driver zero knows (podman, docker)")
}

func ParseComputeDriver(info string) string {
	lines := strings.Split(info, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "Compute drivers:" && i+1 < len(lines) {
			if d := strings.TrimSpace(lines[i+1]); d == "docker" || d == "podman" {
				return d
			}
		}
	}
	return ""
}
