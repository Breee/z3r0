package zero

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	if err := s.syncInputs(out); err != nil {
		return 0, err
	}
	if !s.exists() {
		if err := s.create(); err != nil {
			return 0, err
		}
		// Policy markers are sandbox-scoped: a fresh sandbox needs them again.
		os.Remove(s.path("zen.done"))
		os.Remove(s.path("copilot.done"))
		os.Remove(s.path("copilot-auth.done"))
	} else {
		pol, err := s.loadPolicy()
		if err != nil {
			return 0, err
		}
		ch, err := s.approvePolicy(pol)
		if err != nil {
			return 0, err
		}
		if err := s.write("caps", strings.Join(pol.Capabilities, " "), 0o600); err != nil {
			return 0, err
		}
		repaths, err := s.syncForgePaths()
		if err != nil {
			return 0, err
		}
		if rt.Stopped(s.Name) {
			resumes := s.read("forge.id") == ""
			if err := s.resumeForge(pol.Capabilities); err != nil {
				return 0, err
			}
			if resumes {
				s.forgeSynced() // minted for the new paths
				repaths = false
			}
			if err := rt.Start(s.Name); err != nil {
				return 0, fmt.Errorf("start sandbox: %w", err)
			}
			// Placeholders get a new version on start; the one stored in
			// opencode's credential db is stale.
			os.Remove(s.path("copilot-auth.done"))
		}
		if err := s.renewForge(pol.Capabilities, ch.caps || repaths); err != nil {
			return 0, err
		}
		if repaths {
			s.forgeSynced()
			fmt.Fprintf(out, "%s token: rotated (%s)\n", s.forge().Name(), strings.Join(s.forgePaths(), ", "))
		}
		if len(ch.newHosts) > 0 {
			if err := s.applyProjectPolicy(pol); err != nil {
				return 0, err
			}
		}
		if ch.goneHosts {
			fmt.Fprintln(out, "policy: removed hosts stay reachable until `zero rm` + `zero up`")
		}
		if err := s.saveApproved(pol); err != nil {
			return 0, err
		}
		if err := s.recordPolicy(); err != nil {
			return 0, err
		}
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
	if s.read("forge") != "" {
		if err := ensureServe(); err != nil {
			return 0, err
		}
	}
	if err := s.ensureCheckout(out); err != nil {
		return 0, err
	}
	if s.read("copilot") != "" && !fileExists(s.path("copilot-auth.done")) {
		if err := s.importCopilotAuth(); err != nil {
			return 0, err
		}
		if err := s.write("copilot-auth.done", "1", 0o600); err != nil {
			return 0, err
		}
	}
	os.Remove(s.path("password")) // left by versions that ran serve with a password
	var port int
	var err error
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
		startErr = s.startServe()
	}
	if err := s.waitReady(port, startErr); err != nil {
		return 0, err
	}
	fmt.Fprintf(out, "%s ready: %s dir %s\n", s.Name, link(out, s.url(port)), strings.Join(s.dirs(), " "))
	return port, nil
}

// create makes the sandbox from the dev image with the embedded filesystem
// policy (override with Z3R0_POLICY).
func (s *Sandbox) create() error {
	if s.gw != "" {
		if err := s.write("gateway", s.gw, 0o600); err != nil {
			return err
		}
	}
	policy := os.Getenv("Z3R0_POLICY")
	if policy == "" {
		policy = s.path("policy.yaml")
		if err := os.WriteFile(policy, policyYAML, 0o600); err != nil {
			return fmt.Errorf("write embedded policy: %w", err)
		}
	}
	pol, err := s.loadPolicy()
	if err != nil {
		return err
	}
	if _, err := s.approvePolicy(pol); err != nil {
		return err
	}
	var creds, env []string
	if err := s.write("caps", strings.Join(pol.Capabilities, " "), 0o600); err != nil {
		return err
	}
	prov, err := s.setupForge(pol.Capabilities)
	if err != nil {
		return err
	}
	if prov != "" {
		creds = append(creds, prov)
		env = s.forge().Env()
	}
	cp, err := s.setupCopilot()
	if err != nil {
		return err
	}
	if cp != "" {
		creds = append(creds, cp)
	}
	// The staging directory is mounted even when it is empty, so input
	// folders can be added later without a new sandbox. The VM driver has
	// no input mount (ADR 0039).
	mount := ""
	if s.gw == "" && fileExists(s.stage()) {
		mount = s.stage()
	}
	if err := rt.Create(s.Name, image(), policy, creds, env, mount, inputMount); err != nil {
		return fmt.Errorf("create sandbox (image %s): %w", image(), err)
	}
	if mount != "" {
		if err := s.write("input.mount", mount, 0o600); err != nil {
			return err
		}
	}
	if err := s.applyProjectPolicy(pol); err != nil {
		return err
	}
	if err := s.saveApproved(pol); err != nil {
		return err
	}
	return s.recordPolicy()
}

// opencodeBin is the real executable; /usr/local/bin/opencode is a symlink and
// policy update matches binaries by resolved path (C37).
const opencodeBin = "/usr/local/libexec/opencode/node_modules/@opencode/cli/bin/opencode.exe"

// applyZenPolicy allows opencode inside the sandbox to reach its API. Runs
// until the zen.done marker exists, so a failed attempt is retried.
func (s *Sandbox) applyZenPolicy() error {
	if err := rt.AllowEgress(s.Name, "zen", opencodeBin, "read-write", []string{"opencode.ai", "models.opencode.ai"}); err != nil {
		return fmt.Errorf("apply opencode policy: %w", err)
	}
	return nil
}

func (s *Sandbox) writeSSHConfig() error {
	conf, err := rt.SSHConfig(s.Name)
	if err != nil {
		return fmt.Errorf("ssh config: %w", err)
	}
	return s.write("ssh.conf", conf+"\n", 0o600)
}

// dirs are the checkout paths of all folders inside the sandbox.
func (s *Sandbox) dirs() []string {
	var out []string
	for _, top := range s.Folders {
		out = append(out, sandboxDir(top))
	}
	return out
}

// ensureCheckout clones every folder that is not in the sandbox yet from a
// git bundle (bundles avoid copying a raw .git from the host trust domain).
// A folder removed from the workspace keeps its checkout in the sandbox.
func (s *Sandbox) ensureCheckout(w io.Writer) error {
	var paths map[string]string
	for _, top := range s.repos() {
		dir := sandboxDir(top)
		out, err := s.ssh("if test -d " + shellQuote(dir) + "; then echo yes; else echo no; fi")
		if err != nil {
			return fmt.Errorf("sandbox unreachable: %w", err)
		}
		if out == "yes" {
			continue
		}
		if paths == nil {
			if _, paths, err = s.folderForge(); err != nil {
				return err
			}
		}
		if err := s.checkout(top, dir, paths[top]); err != nil {
			return err
		}
		s.noteSessionBranch(w, dir, paths[top])
	}
	return nil
}

func (s *Sandbox) checkout(top, dir, path string) error {
	bundle := s.path("repo.bundle")
	if _, err := sh("git", "-C", top, "bundle", "create", bundle, "--all"); err != nil {
		return fmt.Errorf("git bundle %s: %w", top, err)
	}
	defer os.Remove(bundle)
	if err := rt.Upload(s.Name, bundle, "/tmp/.zero.bundle"); err != nil {
		return fmt.Errorf("upload bundle: %w", err)
	}
	script := "mkdir -p /tmp/zero && git clone -q /tmp/.zero.bundle " + shellQuote(dir) + " && rm -f /tmp/.zero.bundle"
	if _, err := s.ssh(script); err != nil {
		return fmt.Errorf("clone in sandbox: %w", err)
	}
	return s.configureRemote(top, path)
}

// noteSessionBranch tells the user when the forge still has the session
// branch of an earlier sandbox with commits the new checkout lacks: a push
// is then rejected (the gate allows no delete), the old work stays.
func (s *Sandbox) noteSessionBranch(out io.Writer, dir, path string) {
	if path == "" {
		return
	}
	branch := strings.TrimPrefix(sessionBranch(s.Name), "refs/heads/")
	n, err := s.ssh("cd " + shellQuote(dir) + " && git fetch -q origin " + shellQuote(branch) +
		" 2>/dev/null && git rev-list --count HEAD..FETCH_HEAD")
	if err != nil || n == "" || n == "0" {
		return
	}
	fmt.Fprintf(out, "note: %s has branch %s with %s commits that are not in this checkout; in %s: git pull origin %s, or delete the branch on the forge\n",
		path, branch, n, dir, branch)
}

func (s *Sandbox) serveRunning() (bool, error) {
	out, err := s.ssh("opencode service status")
	if err != nil {
		return false, fmt.Errorf("sandbox unreachable: %w", err)
	}
	// opencode starts the service by itself on a random port (e.g. on
	// "opencode auth import"); only the forwarded port counts as running.
	return out == fmt.Sprintf("http://127.0.0.1:%d", servePort), nil
}

// startServe starts the opencode background service inside the sandbox on
// the forwarded port. It has no password: browsers get in with a one-time
// pairing link (zero web). A leftover password-mode serve is stopped first.
func (s *Sandbox) startServe() error {
	_, err := s.ssh("pkill -f '[s]erve --hostname' ; mkdir -p /tmp/zero; cd " + shellQuote(s.Dir) + " || exit 1; " +
		"opencode service set hostname 127.0.0.1 >/dev/null && " +
		"opencode service set port " + fmt.Sprint(servePort) + " >/dev/null && " +
		"{ opencode service stop >/dev/null 2>&1; opencode service start >/dev/null; }")
	return err
}

// waitReady polls the API through the forward until the service answers.
func (s *Sandbox) waitReady(port int, startErr error) error {
	url := s.url(port)
	last := "connection refused"
	for i := 0; i < 30; i++ {
		err := probeServe(port)
		if err == nil {
			return nil
		}
		last = err.Error()
		time.Sleep(time.Second)
	}
	msg := fmt.Sprintf("serve not reachable at %s (%s)", url, last)
	if startErr != nil {
		msg += fmt.Sprintf("; start: %v", startErr)
	}
	return fmt.Errorf("%s; check: zero logs", msg)
}

var pairLink = regexp.MustCompile(`http://127\.0\.0\.1:\d+/auth/connect/\S+`)

// Web is up + the opencode web UI in the browser, opened with a one-time
// pairing link from the sandbox service.
func (s *Sandbox) Web(out io.Writer) error {
	port, err := s.Up(out)
	if err != nil {
		return err
	}
	text, err := s.ssh("opencode pair --url " + shellQuote(s.url(port)))
	if err != nil {
		return fmt.Errorf("pairing link: %w", err)
	}
	pair := pairLink.FindString(text)
	if pair == "" {
		return fmt.Errorf("no pairing link in opencode pair output: %s", text)
	}
	fmt.Fprintf(out, "open (works once, 5 minutes): %s\n", link(out, pair))
	return exec.Command("xdg-open", pair).Run()
}

// Attach is up + the opencode TUI attached to the sandbox.
func (s *Sandbox) Attach(out io.Writer) error {
	port, err := s.Up(out)
	if err != nil {
		return err
	}
	return passthrough("opencode", "--server", s.url(port), s.Dir)
}

// Run is up (silent) + a one-shot opencode run.
func (s *Sandbox) Run(out io.Writer, prompt []string) error {
	port, err := s.Up(io.Discard)
	if err != nil {
		return err
	}
	args := append([]string{"run", "--server", s.url(port)}, prompt...)
	return passthrough("opencode", args...)
}

// Status prints sandbox / forward / serve state.
func (s *Sandbox) Status(out io.Writer) error {
	up := s.exists()
	if up && rt.Stopped(s.Name) {
		fmt.Fprintf(out, "sandbox %s: paused (zero up continues)\n", s.Name)
		return nil
	}
	if up {
		fmt.Fprintf(out, "sandbox %s: up\n", s.Name)
	} else {
		fmt.Fprintf(out, "sandbox %s: none\n", s.Name)
	}
	if f := s.currentForward(); f != nil {
		fmt.Fprintf(out, "forward: %s\n", link(out, s.url(f.port)))
	} else {
		fmt.Fprintln(out, "forward: off")
	}
	if s.read("gate.key") != "" {
		if gateUp() {
			fmt.Fprintf(out, "push gate: on (%s and below)\n", strings.TrimPrefix(sessionBranch(s.Name), "refs/heads/"))
		} else {
			fmt.Fprintln(out, "push gate: off (zero serve not running; pushes fail until the next zero up)")
		}
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

// Pause closes the forward, stops the sandbox and revokes its forge token.
// Files stay; `zero up` starts it again with a new token.
func (s *Sandbox) Pause(out io.Writer) error {
	lk, err := s.acquireLock()
	if err != nil {
		return err
	}
	defer lk.release()
	if !s.exists() {
		return fmt.Errorf("sandbox %s: none", s.Name)
	}
	if s.gw != "" {
		// The VM driver loses /tmp, and with it the checkout, on stop (C50).
		return fmt.Errorf("pause is not supported with --vm: the checkout would be lost (C50)")
	}
	if err := s.stopForward(io.Discard); err != nil {
		return err
	}
	if !rt.Stopped(s.Name) {
		if err := rt.Stop(s.Name); err != nil {
			return fmt.Errorf("stop sandbox: %w", err)
		}
	}
	if err := s.pauseForge(); err != nil {
		return err
	}
	fmt.Fprintf(out, "paused %s\n", s.Name)
	return nil
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
		if err := rt.Delete(s.Name); err != nil {
			return fmt.Errorf("delete sandbox (state kept for retry): %w", err)
		}
	}
	if err := s.revokeForge(); err != nil {
		return fmt.Errorf("%w (state kept for retry)", err)
	}
	rt.UnbindCredential(s.copilotProvider())
	if err := os.RemoveAll(s.stage()); err != nil {
		return fmt.Errorf("remove input copy: %w", err)
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
		if err := rt.AllowEgress(s.Name, "copilot", "/go/.vscode-server/**", "read-write", copilotEndpoints); err != nil {
			return fmt.Errorf("copilot policy: %w", err)
		}
		if err := s.write("copilot.done", "1", 0o600); err != nil {
			return err
		}
	}
	if err := s.ensureHostSSHConfig(); err != nil {
		return err
	}
	remote := "vscode-remote://ssh-remote+" + s.Host
	if len(s.Folders) == 1 {
		return passthrough("code", "--folder-uri", remote+sandboxDir(s.Folders[0]))
	}
	// Several folders open as a multi-root workspace (spec F11).
	type folder struct {
		Path string `json:"path"`
	}
	var ws struct {
		Folders []folder `json:"folders"`
	}
	for _, d := range s.dirs() {
		ws.Folders = append(ws.Folders, folder{d})
	}
	b, _ := json.Marshal(ws)
	const file = "/tmp/zero/zero.code-workspace"
	if _, err := s.ssh("printf '%s' " + shellQuote(string(b)) + " > " + file); err != nil {
		return fmt.Errorf("write %s: %w", file, err)
	}
	return passthrough("code", "--file-uri", remote+file)
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

// Logs prints the opencode service log from inside the sandbox; follow tails it live.
func (s *Sandbox) Logs(out io.Writer, follow bool) error {
	if !s.exists() {
		return fmt.Errorf("sandbox %s: none", s.Name)
	}
	if follow {
		return passthrough("ssh", "-F", s.path("ssh.conf"), s.Host,
			"tail -n 200 -f ~/.local/share/opencode/log/opencode.log")
	}
	text, err := s.ssh("if test -f ~/.local/share/opencode/log/opencode.log; then tail -n 200 ~/.local/share/opencode/log/opencode.log; " +
		"else echo 'no serve log (serve not started)'; fi")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, text)
	return nil
}
