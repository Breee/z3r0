package zero

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// copilotEndpoints are added to the sandbox policy on first `zero code` use;
// Copilot endpoints only apply to binaries under the VS Code server dir (F05).
var copilotEndpoints = []string{
	"*.githubcopilot.com",
	"api.enterprise.githubcopilot.com",
	"api.github.com",
	"copilot-proxy.githubusercontent.com",
}

// Up creates or reuses the sandbox, clones the repo in on first use, starts
// `opencode serve` and the local forward, and prints the ready line. It
// returns the forward port. The state lock is held only for its duration.
func (s *Sandbox) Up(out io.Writer) (int, error) {
	lk, err := s.acquireLock()
	if err != nil {
		return 0, err
	}
	defer lk.release()
	return s.up(out)
}

func (s *Sandbox) up(out io.Writer) (int, error) {
	if !s.exists() {
		if err := s.create(); err != nil {
			return 0, err
		}
		// Policy markers are sandbox-scoped: a fresh sandbox needs them again.
		os.Remove(s.path("zen.done"))
		os.Remove(s.path("copilot.done"))
	}
	if !fileExists(s.path("zen.done")) {
		if err := s.applyZenPolicy(); err != nil {
			return 0, err
		}
		if err := s.write("zen.done", "1", 0o600); err != nil {
			return 0, err
		}
	}
	if err := s.writeSSHConfig(); err != nil {
		return 0, err
	}
	if err := s.ensureCheckout(); err != nil {
		return 0, err
	}
	pw, err := s.password()
	if err != nil {
		return 0, err
	}

	var port int
	if f := s.currentForward(); f != nil {
		port = f.port
	} else {
		if port, err = s.port(); err != nil {
			return 0, err
		}
		if err := s.startForward(port); err != nil {
			return 0, err
		}
	}
	running, err := s.serveRunning()
	if err != nil {
		return 0, err
	}
	var startErr error
	if !running {
		startErr = s.startServe(pw)
	}
	if err := s.waitReady(port, pw, startErr); err != nil {
		return 0, err
	}
	fmt.Fprintf(out, "%s ready: %s dir %s\n", s.Name, s.url(port), s.Dir)
	return port, nil
}

// create makes the sandbox from the dev image with the embedded filesystem
// policy (override with Z3R0_POLICY).
func (s *Sandbox) create() error {
	policy := os.Getenv("Z3R0_POLICY")
	if policy == "" {
		policy = s.path("policy.yaml")
		if err := os.WriteFile(policy, policyYAML, 0o600); err != nil {
			return fmt.Errorf("write embedded policy: %w", err)
		}
	}
	if _, err := sh("openshell", "sandbox", "create",
		"--name", s.Name, "--from", image(), "--policy", policy, "--detach"); err != nil {
		return fmt.Errorf("create sandbox (image %s): %w", image(), err)
	}
	return nil
}

// applyZenPolicy allows opencode inside the sandbox to reach its API. Runs
// until the zen.done marker exists, so a failed attempt is retried.
func (s *Sandbox) applyZenPolicy() error {
	if _, err := sh("openshell", "policy", "update", s.Name,
		"--rule-name", "zen",
		"--binary", "/usr/local/bin/opencode",
		"--add-endpoint", "opencode.ai:443:read-write:rest:enforce",
		"--wait"); err != nil {
		return fmt.Errorf("apply opencode policy: %w", err)
	}
	return nil
}

func (s *Sandbox) writeSSHConfig() error {
	conf, err := sh("openshell", "sandbox", "ssh-config", s.Name)
	if err != nil {
		return fmt.Errorf("ssh config: %w", err)
	}
	return s.write("ssh.conf", conf+"\n", 0o600)
}

// ensureCheckout clones the repo into the sandbox from a git bundle on first
// use (bundles avoid copying a raw .git from the host trust domain).
func (s *Sandbox) ensureCheckout() error {
	out, err := s.ssh("if test -d " + shellQuote(s.Dir) + "; then echo yes; else echo no; fi")
	if err != nil {
		return fmt.Errorf("sandbox unreachable: %w", err)
	}
	if out == "yes" {
		return nil
	}
	bundle := s.path("repo.bundle")
	if _, err := sh("git", "-C", s.Top, "bundle", "create", bundle, "--all"); err != nil {
		return fmt.Errorf("git bundle: %w", err)
	}
	defer os.Remove(bundle)
	if _, err := sh("openshell", "sandbox", "upload", s.Name, bundle, "/sandbox/.zero.bundle"); err != nil {
		return fmt.Errorf("upload bundle: %w", err)
	}
	script := "git clone -q /sandbox/.zero.bundle " + shellQuote(s.Dir) + " && rm -f /sandbox/.zero.bundle"
	if _, err := s.ssh(script); err != nil {
		return fmt.Errorf("clone in sandbox: %w", err)
	}
	return nil
}

func (s *Sandbox) serveRunning() (bool, error) {
	out, err := s.ssh("if pgrep -x opencode >/dev/null; then echo yes; else echo no; fi")
	if err != nil {
		return false, fmt.Errorf("sandbox unreachable: %w", err)
	}
	return out == "yes", nil
}

// startServe launches `opencode serve` inside the sandbox. The password is
// passed on stdin, never in argv. ssh may report a teardown race even when
// serve started fine — the readiness probe is authoritative.
func (s *Sandbox) startServe(pw string) error {
	script := "read -r pw; cd " + shellQuote(s.Dir) + " || exit 1; " +
		"OPENCODE_SERVER_PASSWORD=\"$pw\" nohup opencode serve --hostname 127.0.0.1 --port " +
		fmt.Sprint(servePort) + " >/tmp/serve.log 2>&1 </dev/null &"
	cmd := exec.Command("ssh", "-F", s.path("ssh.conf"), s.Host, script)
	cmd.Stdin = strings.NewReader(pw + "\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return err
	}
	return nil
}

// waitReady polls /config through the forward until serve answers 200.
func (s *Sandbox) waitReady(port int, pw string, startErr error) error {
	url := s.url(port)
	last := "connection refused"
	for i := 0; i < 30; i++ {
		status, err := probeConfig(port, pw)
		if err == nil && status == 200 {
			return nil
		}
		if err != nil {
			last = err.Error()
		} else {
			last = fmt.Sprintf("HTTP %d", status)
		}
		time.Sleep(time.Second)
	}
	msg := fmt.Sprintf("serve not reachable at %s (%s)", url, last)
	if startErr != nil {
		msg += fmt.Sprintf("; start: %v", startErr)
	}
	return fmt.Errorf("%s; check: zero logs", msg)
}

// Attach is up + the opencode TUI attached to the sandbox.
func (s *Sandbox) Attach(out io.Writer) error {
	port, err := s.Up(out)
	if err != nil {
		return err
	}
	pw, err := s.password()
	if err != nil {
		return err
	}
	return passthrough("opencode", "attach", s.url(port), "-p", pw, "--dir", s.Dir)
}

// Run is up (silent) + a one-shot opencode run.
func (s *Sandbox) Run(out io.Writer, prompt []string) error {
	port, err := s.Up(io.Discard)
	if err != nil {
		return err
	}
	pw, err := s.password()
	if err != nil {
		return err
	}
	args := append([]string{"run", "--attach", s.url(port), "-p", pw, "--dir", s.Dir}, prompt...)
	return passthrough("opencode", args...)
}

// Status prints sandbox / forward / serve state.
func (s *Sandbox) Status(out io.Writer) error {
	up := s.exists()
	if up {
		fmt.Fprintf(out, "sandbox %s: up\n", s.Name)
	} else {
		fmt.Fprintf(out, "sandbox %s: none\n", s.Name)
	}
	if f := s.currentForward(); f != nil {
		fmt.Fprintf(out, "forward: %s\n", s.url(f.port))
	} else {
		fmt.Fprintln(out, "forward: off")
	}
	if up {
		running, err := s.serveRunning()
		switch {
		case err != nil:
			fmt.Fprintln(out, "serve: unreachable")
		case running:
			fmt.Fprintln(out, "serve: running")
		default:
			fmt.Fprintln(out, "serve: stopped")
		}
	}
	return nil
}

// Stop ends the local forward; the sandbox keeps running.
func (s *Sandbox) Stop(out io.Writer) error {
	lk, err := s.acquireLock()
	if err != nil {
		return err
	}
	defer lk.release()
	return s.stopForward(out)
}

// Rm stops the forward and deletes the sandbox. If the delete fails the state
// dir is kept so the command can be retried.
func (s *Sandbox) Rm(out io.Writer) error {
	lk, err := s.acquireLock()
	if err != nil {
		return err
	}
	defer lk.release()
	if err := s.stopForward(io.Discard); err != nil {
		return err
	}
	if s.exists() {
		if _, err := sh("openshell", "sandbox", "delete", s.Name); err != nil {
			return fmt.Errorf("delete sandbox (state kept for retry): %w", err)
		}
	}
	if err := os.RemoveAll(s.State); err != nil {
		return fmt.Errorf("remove state: %w", err)
	}
	fmt.Fprintf(out, "removed %s\n", s.Name)
	return nil
}

// Code is up + VS Code Remote-SSH on the same sandbox (Copilot runs inside).
func (s *Sandbox) Code(out io.Writer) error {
	if _, err := s.Up(out); err != nil {
		return err
	}
	if !fileExists(s.path("copilot.done")) {
		for _, ep := range copilotEndpoints {
			if _, err := sh("openshell", "policy", "update", s.Name,
				"--rule-name", "copilot",
				"--binary", "/sandbox/.vscode-server/**",
				"--add-endpoint", ep+":443:read-write:rest:enforce",
				"--wait"); err != nil {
				return fmt.Errorf("copilot policy (%s): %w", ep, err)
			}
		}
		if err := s.write("copilot.done", "1", 0o600); err != nil {
			return err
		}
	}
	if err := s.ensureHostSSHConfig(); err != nil {
		return err
	}
	return passthrough("code", "--folder-uri", "vscode-remote://ssh-remote+"+s.Host+s.Dir)
}

// ensureHostSSHConfig appends our Host entry to the openshell ssh_config the
// same way `sandbox connect --editor vscode` writes it (connect itself opens
// a second window that races, so we write the entry directly).
func (s *Sandbox) ensureHostSSHConfig() error {
	cfg := filepath.Join(os.Getenv("HOME"), ".config", "openshell", "ssh_config")
	want := "Host " + s.Host
	if data, err := os.ReadFile(cfg); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimRight(line, " \t") == want {
				return nil
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(cfg), 0o700); err != nil {
		return err
	}
	conf, err := os.ReadFile(s.path("ssh.conf"))
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if data, err := os.ReadFile(cfg); err == nil && len(data) > 0 {
		buf.Write(data)
		if !strings.HasSuffix(string(data), "\n") {
			buf.WriteByte('\n')
		}
	}
	buf.Write(conf)
	if !strings.HasSuffix(string(conf), "\n") {
		buf.WriteByte('\n')
	}
	return os.WriteFile(cfg, buf.Bytes(), 0o600)
}

// Logs prints serve.log from inside the sandbox; follow tails it live.
func (s *Sandbox) Logs(out io.Writer, follow bool) error {
	if !s.exists() {
		return fmt.Errorf("sandbox %s: none", s.Name)
	}
	if follow {
		return passthrough("ssh", "-F", s.path("ssh.conf"), s.Host,
			"tail -n 200 -f /tmp/serve.log")
	}
	text, err := s.ssh("if test -f /tmp/serve.log; then tail -n 200 /tmp/serve.log; " +
		"else echo 'no serve log (serve not started)'; fi")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, text)
	return nil
}
