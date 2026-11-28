package zero

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Breee/z3r0/internal/agent/claude"
	"github.com/Breee/z3r0/internal/agent/copilot"
	"github.com/Breee/z3r0/internal/agent/opencode"
	"github.com/Breee/z3r0/internal/core"
	"github.com/Breee/z3r0/internal/gate"
	"github.com/Breee/z3r0/internal/guidance/hostfiles"
	"github.com/Breee/z3r0/internal/model/litellm"
)

// copilotEndpoints are added to the sandbox policy on first `zero vscode` use;
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
	out = stepWriter{out}
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
		os.Remove(s.path("copilot-cli.rule"))
		if s.read("models.provider") != "" {
			rt.UnbindCredential(s.modelCredential())
			os.Remove(s.path("models.provider"))
		}
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
		retools, err := s.syncTools()
		if err != nil {
			return 0, err
		}
		rerule := repaths
		if rt.Stopped(s.Name) {
			resumes := s.read("forge.id") == ""
			if err := s.resumeForge(pol.Capabilities); err != nil {
				return 0, err
			}
			if resumes {
				s.forgeSynced() // minted for the new paths
				repaths, retools = false, false
			}
			done := step("starting the sandbox")
			err := rt.Start(s.Name)
			done(err)
			if err != nil {
				return 0, fmt.Errorf("start sandbox: %w", err)
			}
			// Placeholders get a new version on start; the one stored in
			// opencode's credential db is stale.
			os.Remove(s.path("copilot-auth.done"))
		}
		// Rules before token: a failure in between leaves the narrower side.
		if rerule {
			if err := s.syncForgeRule(); err != nil {
				return 0, err
			}
		}
		if len(ch.newHosts) > 0 || ch.goneHosts {
			if err := s.syncProjectPolicy(pol); err != nil {
				return 0, err
			}
		}
		revoked := s.forge() != nil && s.read("forge.id") == ""
		if err := s.renewForge(pol.Capabilities, ch.caps || repaths || retools || revoked); err != nil {
			return 0, err
		}
		if repaths || retools {
			s.forgeSynced()
			if f := s.forge(); f != nil {
				tools := ""
				if len(s.Tools) > 0 {
					tools = "; tools: " + strings.Join(s.Tools, ", ")
				}
				fmt.Fprintf(out, "%s token: rotated (%s%s)\n", f.Name(), strings.Join(s.forgePaths(), ", "), tools)
			}
		}
		if s.forge() != nil && !core.FileExists(s.path("forge.rule")) {
			fmt.Fprintln(out, "forge: this sandbox reaches the whole forge host, not only its projects, until `zero rm` + `zero up`")
		}
		if err := s.saveApproved(pol); err != nil {
			return 0, err
		}
		if err := s.recordPolicy(); err != nil {
			return 0, err
		}
	}
	if len(s.Models) > 0 {
		// Models come through the proxy only: no other model host stays open.
		if core.FileExists(s.path("zen.done")) || core.FileExists(s.path("copilot.done")) {
			if err := rt.SetRule(s.Name, "zen", nil); err != nil {
				return 0, err
			}
			if err := rt.SetRule(s.Name, "copilot", nil); err != nil {
				return 0, err
			}
			os.Remove(s.path("zen.done"))
			os.Remove(s.path("copilot.done"))
		}
	} else if !core.FileExists(s.path("zen.done")) {
		if err := s.applyZenPolicy(); err != nil {
			return 0, err
		}
		if err := s.write("zen.done", "1", 0o600); err != nil {
			return 0, err
		}
	}
	done := step("connecting to the sandbox")
	err := s.writeSSHConfig()
	if err == nil && s.read("forge") != "" {
		err = ensureServe()
	}
	done(err)
	if err != nil {
		return 0, err
	}
	done = step("copying the repositories in")
	err = s.ensureCheckout(out)
	done(err)
	if err != nil {
		return 0, err
	}
	done = step("setting up models and instructions")
	models, err := s.syncModels(out, s.Providers)
	if err == nil {
		err = s.syncGuidance(out, models)
	}
	done(err)
	if err != nil {
		return 0, err
	}
	if s.read("copilot") != "" && len(s.Models) == 0 && !core.FileExists(s.path("copilot-auth.done")) {
		if err := s.importCopilotAuth(); err != nil {
			return 0, err
		}
		if err := s.write("copilot-auth.done", "1", 0o600); err != nil {
			return 0, err
		}
	}
	os.Remove(s.path("password")) // left by versions that ran serve with a password
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
	done = step("starting opencode in the sandbox")
	var startErr error
	if !running {
		startErr = s.startServe()
	}
	err = s.waitReady(port, startErr)
	done(err)
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(out, "%s ready: %s dir %s\n", s.Name, link(out, s.url(port)), strings.Join(s.dirs(), " "))
	if s.forge() == nil {
		fmt.Fprintln(out, noForge)
	}
	if s.read("copilot") == "" && len(s.Models) == 0 {
		fmt.Fprintln(out, noModel)
	}
	if s.read("copilot") != "" && len(s.Models) > 0 {
		fmt.Fprintln(out, "models: this sandbox still holds its own Copilot login from before; to drop it: zero rm && zero up")
	}
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
	base := policyYAML
	if custom := os.Getenv("Z3R0_POLICY"); custom != "" {
		b, err := os.ReadFile(custom)
		if err != nil {
			return fmt.Errorf("Z3R0_POLICY: %w", err)
		}
		base = b
	}
	policy := s.path("policy.yaml")
	if err := os.WriteFile(policy, base, 0o600); err != nil {
		return fmt.Errorf("write policy: %w", err)
	}
	if err := s.firstRun(); err != nil {
		return err
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
	if _, err := s.syncTools(); err != nil {
		return err
	}
	prov, err := s.setupForge(pol.Capabilities)
	if err != nil {
		return err
	}
	s.forgeSynced()
	if prov != "" {
		creds = append(creds, prov)
		env = s.forge().Env()
		if err := s.forgePolicyFile(base, prov); err != nil {
			return err
		}
		if err := s.write("forge.rule", "1", 0o600); err != nil {
			return err
		}
	}
	// With the model proxy the sandbox gets no provider login of its own.
	cp := ""
	if len(s.Models) == 0 {
		c, err := s.setupCopilot()
		if err != nil {
			return err
		}
		cp = c
	}
	if cp != "" {
		creds = append(creds, cp)
	}
	// The staging directory is mounted even when it is empty, so input
	// directories can be added later without a new sandbox. The VM driver has
	// no input mount (ADR 0039).
	mount := ""
	if s.gw == "" && core.FileExists(s.stage()) {
		mount = s.stage()
	}
	done := step("creating the sandbox (the first time pulls the image)")
	err = rt.Create(s.Name, image(), policy, creds, env, mount, inputMount)
	done(err)
	if err != nil {
		return fmt.Errorf("create sandbox (image %s): %w; check: zero doctor", image(), err)
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

// applyZenPolicy allows opencode inside the sandbox to reach its API. Runs
// until the zen.done marker exists, so a failed attempt is retried.
func (s *Sandbox) applyZenPolicy() error {
	if err := rt.AllowEgress(s.Name, "zen", opencode.Bin, "read-write", []string{"opencode.ai", "models.opencode.ai"}); err != nil {
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

// dirs are the checkout paths of all directories inside the sandbox.
func (s *Sandbox) dirs() []string {
	var out []string
	for _, top := range s.Folders {
		out = append(out, sandboxDir(top))
	}
	return out
}

// ensureCheckout clones every directory that is not in the sandbox yet from a
// git bundle (bundles avoid copying a raw .git from the host trust domain).
// A directory removed from the workspace keeps its checkout in the sandbox.
// uncommitted counts changed and untracked files of a host repo; ignored files do not count.
func uncommitted(top string) int {
	out, err := core.Sh("git", "-C", top, "status", "--porcelain")
	if err != nil || out == "" {
		return 0
	}
	return strings.Count(out, "\n") + 1
}

func (s *Sandbox) ensureCheckout(w io.Writer) error {
	var paths map[string]string
	for _, top := range s.repos() {
		dir := sandboxDir(top)
		out, err := s.ssh("if test -d " + core.ShellQuote(dir) + "; then echo yes; else echo no; fi")
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
		if n := uncommitted(top); n > 0 {
			fmt.Fprintf(w, "note: %s has %d uncommitted or untracked files; they are not in %s (the sandbox gets commits only)\n", top, n, dir)
		}
	}
	return nil
}

// bundleRepo writes branches, tags and HEAD of a host repo into a bundle.
// Not --all: that would also carry refs/stash, which can hold untracked
// files, and other private refs.
func bundleRepo(top, file string) error {
	if _, err := core.Sh("git", "-C", top, "bundle", "create", file, "--branches", "--tags", "HEAD"); err != nil {
		return fmt.Errorf("git bundle %s: %w", top, err)
	}
	return nil
}

func (s *Sandbox) checkout(top, dir, path string) error {
	bundle := s.path("repo.bundle")
	if err := bundleRepo(top, bundle); err != nil {
		return err
	}
	defer os.Remove(bundle)
	if err := rt.Upload(s.Name, bundle, "/tmp/.zero.bundle"); err != nil {
		return fmt.Errorf("upload bundle: %w", err)
	}
	script := "mkdir -p /tmp/zero && git clone -q /tmp/.zero.bundle " + core.ShellQuote(dir) + " && rm -f /tmp/.zero.bundle"
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
	branch := strings.TrimPrefix(gate.SessionBranch(s.Name), "refs/heads/")
	n, err := s.ssh("cd " + core.ShellQuote(dir) + " && git fetch -q origin " + core.ShellQuote(branch) +
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
	_, err := s.ssh("pkill -f '[s]erve --hostname' ; mkdir -p /tmp/zero; cd " + core.ShellQuote(s.Dir) + " || exit 1; " +
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
	if err := s.webSession(); err != nil {
		fmt.Fprintf(out, "web: no session prepared (%v); in the browser: Add project, %s\n", err, core.Clean(s.Dir))
	}
	text, err := s.ssh("opencode pair --url " + core.ShellQuote(s.url(port)))
	if err != nil {
		return fmt.Errorf("pairing link: %w", err)
	}
	pair := pairLink.FindString(text)
	if pair == "" {
		return fmt.Errorf("no pairing link in opencode pair output: %s; run zero web again", text)
	}
	fmt.Fprintf(out, "open (works once, 5 minutes): %s\n", link(out, pair))
	return exec.Command("xdg-open", pair).Run()
}

// webSession makes sure the service has a session in the workspace's first
// directory. The web UI opens on a page that lists sessions, and a pairing link
// cannot name a directory (C56).
func (s *Sandbox) webSession() error {
	list, err := s.ssh("opencode api get " + core.ShellQuote("/api/session?limit=1&directory="+url.QueryEscape(s.Dir)))
	if err != nil {
		return err
	}
	has, err := hasSession(list)
	if err != nil || has {
		return err
	}
	body, _ := json.Marshal(map[string]any{"location": map[string]string{"directory": s.Dir}})
	_, err = s.ssh("opencode api post /api/session --data " + core.ShellQuote(string(body)))
	return err
}

// hasSession reads the answer of the session list.
func hasSession(list string) (bool, error) {
	var r struct {
		Data *[]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(list), &r); err != nil || r.Data == nil {
		return false, fmt.Errorf("session list not readable")
	}
	return len(*r.Data) > 0, nil
}

// Attach is up + the opencode TUI, started inside the sandbox over ssh with
// this terminal (ADR 0048): the host's opencode cannot open a directory that
// exists only in the sandbox (C38).
// NeedTerminal is the check of `zero opencode`, exported so that the command
// can refuse before a workspace is made.
func NeedTerminal() error {
	if !core.IsTerminal(os.Stdin) || !core.IsTerminal(os.Stdout) {
		return fmt.Errorf("zero opencode needs a terminal; without one: zero run PROMPT, or zero opencode --web")
	}
	return nil
}

func (s *Sandbox) Attach(out io.Writer) error {
	if err := NeedTerminal(); err != nil {
		return err
	}
	if _, err := s.Up(out); err != nil {
		return err
	}
	return core.Passthrough("ssh", "-t", "-F", s.path("ssh.conf"), s.Host, "cd "+core.ShellQuote(s.Dir)+" && exec opencode")
}

// copilotHost is the Copilot API, reached only by the Copilot CLI (ADR 0050).
const copilotHost = "api.githubcopilot.com"

// CopilotNeedsTerminal is the check of `zero copilot` without arguments, for
// main to refuse before a workspace is made.
func CopilotNeedsTerminal(args []string) error {
	if len(args) == 0 && (!core.IsTerminal(os.Stdin) || !core.IsTerminal(os.Stdout)) {
		return fmt.Errorf("zero copilot needs a terminal; without one: zero copilot -p PROMPT")
	}
	return nil
}

// Copilot is up + the GitHub Copilot CLI inside the sandbox over ssh
// (ADR 0050). It uses the Copilot login zero brought in at create: the
// sandbox holds a placeholder, the token is put in outside of it. Arguments
// go to the tool; without arguments it needs a terminal.
func (s *Sandbox) Copilot(out io.Writer, args []string) error {
	tty := core.IsTerminal(os.Stdin) && core.IsTerminal(os.Stdout)
	if err := CopilotNeedsTerminal(args); err != nil {
		return err
	}
	proxy := len(s.Models) > 0
	if proxy {
		args = s.withModel(args)
	}
	if proxy && !core.HasModelArg(args) {
		return fmt.Errorf("zero copilot in this workspace needs a model: zero copilot --model PROVIDER/MODEL ..., or model: for the workspace in zero's config (models of the workspace: %s)", strings.Join(s.Models, ", "))
	}
	if _, err := s.Up(out); err != nil {
		return err
	}
	if !proxy && s.read("copilot") == "" {
		return fmt.Errorf("%s", noModel)
	}
	if _, err := s.ssh("test -x " + copilot.Bin); err != nil {
		return fmt.Errorf("the image of %s has no Copilot CLI; recreate it: zero rm && zero up", s.Name)
	}
	if proxy {
		// The models rule of the sandbox names the binary (PoC 23).
		ssh := []string{"-F", s.path("ssh.conf"), s.Host, copilot.ProxyScript(s.Dir, fmt.Sprintf("http://%s:%d", gateHost, litellm.Port)+"/v1", litellm.KeyEnv, args)}
		if tty {
			ssh = append([]string{"-t"}, ssh...)
		}
		return core.Passthrough("ssh", ssh...)
	}
	if !core.FileExists(s.path("copilot-cli.rule")) {
		if err := rt.AllowEgress(s.Name, copilot.Rule, copilot.Bin, "read-write", []string{copilotHost}); err != nil {
			return fmt.Errorf("%w; check: zero doctor, then run zero copilot again", err)
		}
		if err := s.write("copilot-cli.rule", copilotHost, 0o600); err != nil {
			return err
		}
		fmt.Fprintf(out, "copilot: %s may reach %s\n", copilot.Bin, copilotHost)
	}
	ssh := []string{"-F", s.path("ssh.conf"), s.Host, copilot.Script(s.Dir, args)}
	if tty {
		ssh = append([]string{"-t"}, ssh...)
	}
	return core.Passthrough("ssh", ssh...)
}

// ClaudeNeedsTerminal is the check of `zero claude` without arguments, for
// main to refuse before a workspace is made.
func ClaudeNeedsTerminal(args []string) error {
	if len(args) == 0 && (!core.IsTerminal(os.Stdin) || !core.IsTerminal(os.Stdout)) {
		return fmt.Errorf("zero claude needs a terminal; without one: zero claude --model PROVIDER/MODEL -p PROMPT")
	}
	return nil
}

// withModel puts the workspace's default model in front of the arguments
// when they name none (ADR 0054).
func (s *Sandbox) withModel(args []string) []string {
	if s.Model == "" || core.HasModelArg(args) {
		return args
	}
	return append([]string{"--model", s.Model}, args...)
}

// Claude is up + Claude Code inside the sandbox over ssh (ADR 0052). It
// talks to the model proxy only, so the workspace has to use the proxy.
// Arguments go to the tool; without arguments it needs a terminal.
func (s *Sandbox) Claude(out io.Writer, args []string) error {
	if err := ClaudeNeedsTerminal(args); err != nil {
		return err
	}
	if len(s.Models) == 0 {
		return fmt.Errorf("zero claude needs the model proxy: set parts.models, providers and models for this workspace in zero's config")
	}
	args = s.withModel(args)
	if !core.HasModelArg(args) {
		return fmt.Errorf("zero claude needs a model: zero claude --model PROVIDER/MODEL ..., or model: for the workspace in zero's config (models of the workspace: %s)", strings.Join(s.Models, ", "))
	}
	if _, err := s.Up(out); err != nil {
		return err
	}
	if _, err := s.ssh("test -x " + claude.Bin); err != nil {
		return fmt.Errorf("the image of %s has no Claude Code; recreate it: zero rm && zero up", s.Name)
	}
	ssh := []string{"-F", s.path("ssh.conf"), s.Host, claude.Script(s.Dir, fmt.Sprintf("http://%s:%d", gateHost, litellm.Port), litellm.KeyEnv, args)}
	if core.IsTerminal(os.Stdin) && core.IsTerminal(os.Stdout) {
		ssh = append([]string{"-t"}, ssh...)
	}
	return core.Passthrough("ssh", ssh...)
}

// Run is up (silent) + a one-shot opencode run inside the sandbox.
func (s *Sandbox) Run(out io.Writer, prompt []string) error {
	if _, err := s.Up(io.Discard); err != nil {
		return err
	}
	script := "cd " + core.ShellQuote(s.Dir) + " && exec opencode run --"
	for _, p := range prompt {
		script += " " + core.ShellQuote(p)
	}
	return core.Passthrough("ssh", "-F", s.path("ssh.conf"), s.Host, script)
}

// noForge is the line for a workspace whose repositories are on no forge
// zero knows: there is no token and no push gate.
const noForge = "forge: none, the agent cannot push (zero pushes to GitLab; a self-hosted GitLab needs a glab login before the sandbox is created)"

// errNoSandbox is the error of a command that needs an existing sandbox.
func (s *Sandbox) errNoSandbox() error {
	return fmt.Errorf("sandbox %s does not exist; start it: zero up", s.Name)
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
		fmt.Fprintf(out, "sandbox %s: none\nnext: zero up\n", s.Name)
		return nil
	}
	if f := s.currentForward(); f != nil {
		fmt.Fprintf(out, "forward: %s\n", link(out, s.url(f.port)))
	} else {
		fmt.Fprintln(out, "forward: off")
	}
	if f := s.forge(); f != nil && s.read("forge.id") != "" {
		fmt.Fprintf(out, "%s token: %s; capabilities: %s\n", f.Name(), strings.Join(s.forgePaths(), ", "), strings.Join(strings.Fields(s.read("caps")), ", "))
		if t := strings.Fields(s.read("tools")); len(t) > 0 {
			via := ""
			if s.read("mcp.id") != "" {
				via = " through the model proxy (" + litellm.MCPName(s.Name) + ", its own token)"
				if s.mcpAged() {
					via += "; that token may have ended, zero up makes a new one"
				}
			}
			fmt.Fprintf(out, "%s tools: %s%s\n", f.Name(), strings.Join(t, ", "), via)
		}
	} else if f != nil {
		fmt.Fprintf(out, "%s token: none; zero up makes a new one\n", f.Name())
	} else {
		fmt.Fprintln(out, noForge)
	}
	if len(s.Instructions) > 0 {
		fmt.Fprintf(out, "instructions: %s\n", hostfiles.BaseNames(s.Instructions))
	}
	if len(s.Skills) > 0 {
		fmt.Fprintf(out, "skills: %s\n", hostfiles.BaseNames(s.Skills))
	}
	if len(s.Models) > 0 {
		if s.Model != "" {
			fmt.Fprintf(out, "model: %s (default; --model wins, and for opencode a repository's opencode.json)\n", s.Model)
		}
		fmt.Fprintf(out, "copilot: %s may reach the model proxy only (zero copilot --model PROVIDER/MODEL, or the workspace's model)\n", copilot.Bin)
		fmt.Fprintf(out, "claude: %s may reach the model proxy only (zero claude --model PROVIDER/MODEL, or the workspace's model)\n", claude.Bin)
	} else if core.FileExists(s.path("copilot-cli.rule")) {
		fmt.Fprintf(out, "copilot: %s may reach %s\n", copilot.Bin, copilotHost)
	}
	if s.read("gate.key") != "" {
		if gateUp() {
			fmt.Fprintf(out, "push gate: on (%s and below)\n", strings.TrimPrefix(gate.SessionBranch(s.Name), "refs/heads/"))
		} else {
			fmt.Fprintln(out, "push gate: off (zero-cp not running; pushes fail until the next zero up)")
		}
	}
	if up {
		running, err := s.serveRunning()
		switch {
		case err != nil:
			fmt.Fprintln(out, "serve: unreachable; check: zero logs")
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
		return s.errNoSandbox()
	}
	if s.gw != "" {
		// The VM driver loses /tmp, and with it the checkout, on stop (C50).
		return fmt.Errorf("pause is not supported with --vm: the checkout would be lost; zero rm removes the sandbox")
	}
	if err := s.stopForward(io.Discard); err != nil {
		return err
	}
	// The tool token and the model key go first: with the forge or the proxy
	// down the pause stops here and neither outlives a stopped sandbox.
	if err := s.revokeMCP(); err != nil {
		return err
	}
	if err := s.revokeModelKey(); err != nil {
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

// Rm stops the forward and deletes the sandbox, after it printed what only
// the sandbox holds and the user typed the workspace name (yes skips that). If the delete fails the state
// dir is kept so the command can be retried.
func (s *Sandbox) Rm(out io.Writer, yes bool) error {
	lk, err := s.acquireLock()
	if err != nil {
		return err
	}
	defer lk.release()
	fmt.Fprintf(out, "unpushed: %s\n", s.unpushed())
	if !yes {
		if err := confirmName(strings.TrimPrefix(s.Name, "zero-")); err != nil {
			return err
		}
	}
	if err := s.stopForward(io.Discard); err != nil {
		return err
	}
	if s.exists() {
		if err := rt.Delete(s.Name); err != nil {
			if strings.Contains(err.Error(), "driver operation is still pending") {
				return fmt.Errorf("delete sandbox: the gateway was stopped while it worked on %s and OpenShell keeps it locked (caveat C62, NVIDIA/OpenShell#4409); state kept: %w", s.Name, err)
			}
			return fmt.Errorf("delete sandbox (state kept for retry): %w", err)
		}
	}
	rt.UnbindCredential(s.copilotProvider())
	// The binding holds the only copy of the model key. Once it is gone the
	// key cannot be used, also when the proxy is down and cannot delete it.
	// It goes before the forge is asked, so a forge that is down leaves no
	// usable key behind.
	if err := s.dropModelProvider(); err != nil {
		return fmt.Errorf("%w (state kept for retry)", err)
	}
	if err := s.revokeMCP(); err != nil {
		return fmt.Errorf("%w (state kept for retry)", err)
	}
	if err := s.revokeForge(); err != nil {
		return fmt.Errorf("%w (state kept for retry)", err)
	}
	if err := s.revokeModelKey(); err != nil {
		fmt.Fprintf(out, "models: %v; its only copy is deleted, the proxy drops the key when it ends (7 days)\n", err)
		os.Remove(s.path("models.key"))
	}
	if err := os.RemoveAll(s.stage()); err != nil {
		return fmt.Errorf("remove input copy: %w", err)
	}
	if err := os.RemoveAll(s.State); err != nil {
		return fmt.Errorf("remove state: %w", err)
	}
	fmt.Fprintf(out, "removed sandbox %s\n", s.Name)
	return nil
}

// Code is up + VS Code Remote-SSH on the same sandbox (Copilot runs inside).
func (s *Sandbox) Code(out io.Writer) error {
	if _, err := s.Up(out); err != nil {
		return err
	}
	if len(s.Models) > 0 {
		fmt.Fprintln(out, "models: this workspace uses the model proxy; Copilot inside VS Code has no login and no way out here")
	} else if !core.FileExists(s.path("copilot.done")) {
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
		return core.Passthrough("code", "--folder-uri", remote+sandboxDir(s.Folders[0]))
	}
	// Several directories open as a multi-root workspace (spec F11).
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
	if _, err := s.ssh("printf '%s' " + core.ShellQuote(string(b)) + " > " + file); err != nil {
		return fmt.Errorf("write %s: %w", file, err)
	}
	return core.Passthrough("code", "--file-uri", remote+file)
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
		return s.errNoSandbox()
	}
	if follow {
		cmd := exec.Command("ssh", "-F", s.path("ssh.conf"), s.Host,
			"tail -n 200 -f ~/.local/share/opencode/log/opencode.log")
		cmd.Stdout, cmd.Stderr = &core.CleanWriter{W: out}, &core.CleanWriter{W: os.Stderr}
		return cmd.Run()
	}
	text, err := s.ssh("if test -f ~/.local/share/opencode/log/opencode.log; then tail -n 200 ~/.local/share/opencode/log/opencode.log; " +
		"else echo 'no serve log (serve not started)'; fi")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, text)
	return nil
}
