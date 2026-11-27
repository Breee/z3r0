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
			rt.UnbindCredential(s.modelProvider())
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
			if err := rt.Start(s.Name); err != nil {
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
		if s.forge() != nil && !fileExists(s.path("forge.rule")) {
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
		if fileExists(s.path("zen.done")) || fileExists(s.path("copilot.done")) {
			if err := rt.SetRule(s.Name, "zen", nil); err != nil {
				return 0, err
			}
			if err := rt.SetRule(s.Name, "copilot", nil); err != nil {
				return 0, err
			}
			os.Remove(s.path("zen.done"))
			os.Remove(s.path("copilot.done"))
		}
	} else if !fileExists(s.path("zen.done")) {
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
	models, err := s.syncModels(out, s.Providers)
	if err != nil {
		return 0, err
	}
	if err := s.syncGuidance(out, models); err != nil {
		return 0, err
	}
	if s.read("copilot") != "" && len(s.Models) == 0 && !fileExists(s.path("copilot-auth.done")) {
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
	var startErr error
	if !running {
		startErr = s.startServe()
	}
	if err := s.waitReady(port, startErr); err != nil {
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
	// folders can be added later without a new sandbox. The VM driver has
	// no input mount (ADR 0039).
	mount := ""
	if s.gw == "" && fileExists(s.stage()) {
		mount = s.stage()
	}
	if err := rt.Create(s.Name, image(), policy, creds, env, mount, inputMount); err != nil {
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
// uncommitted counts changed and untracked files of a host repo; ignored files do not count.
func uncommitted(top string) int {
	out, err := sh("git", "-C", top, "status", "--porcelain")
	if err != nil || out == "" {
		return 0
	}
	return strings.Count(out, "\n") + 1
}

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
	if _, err := sh("git", "-C", top, "bundle", "create", file, "--branches", "--tags", "HEAD"); err != nil {
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
	if err := s.webSession(); err != nil {
		fmt.Fprintf(out, "web: no session prepared (%v); in the browser: Add project, %s\n", err, clean(s.Dir))
	}
	text, err := s.ssh("opencode pair --url " + shellQuote(s.url(port)))
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
// folder. The web UI opens on a page that lists sessions, and a pairing link
// cannot name a folder (C56).
func (s *Sandbox) webSession() error {
	list, err := s.ssh("opencode api get " + shellQuote("/api/session?limit=1&directory="+url.QueryEscape(s.Dir)))
	if err != nil {
		return err
	}
	has, err := hasSession(list)
	if err != nil || has {
		return err
	}
	body, _ := json.Marshal(map[string]any{"location": map[string]string{"directory": s.Dir}})
	_, err = s.ssh("opencode api post /api/session --data " + shellQuote(string(body)))
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
// this terminal (ADR 0048): the host's opencode cannot open a folder that
// exists only in the sandbox (C38).
// NeedTerminal is the check of `zero opencode`, exported so that the command
// can refuse before a workspace is made.
func NeedTerminal() error {
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) {
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
	return passthrough("ssh", "-t", "-F", s.path("ssh.conf"), s.Host, "cd "+shellQuote(s.Dir)+" && exec opencode")
}

// copilotBin and copilotRule: the GitHub Copilot CLI of the image and the
// egress rule that lets only that binary reach the Copilot API (ADR 0050).
const (
	copilotBin  = "/usr/local/bin/copilot"
	copilotRule = "copilot-cli"
	copilotHost = "api.githubcopilot.com"
)

// CopilotNeedsTerminal is the check of `zero copilot` without arguments, for
// main to refuse before a workspace is made.
func CopilotNeedsTerminal(args []string) error {
	if len(args) == 0 && (!isTerminal(os.Stdin) || !isTerminal(os.Stdout)) {
		return fmt.Errorf("zero copilot needs a terminal; without one: zero copilot -p PROMPT")
	}
	return nil
}

// Copilot is up + the GitHub Copilot CLI inside the sandbox over ssh
// (ADR 0050). It uses the Copilot login zero brought in at create: the
// sandbox holds a placeholder, the token is put in outside of it. Arguments
// go to the tool; without arguments it needs a terminal.
func (s *Sandbox) Copilot(out io.Writer, args []string) error {
	tty := isTerminal(os.Stdin) && isTerminal(os.Stdout)
	if err := CopilotNeedsTerminal(args); err != nil {
		return err
	}
	proxy := len(s.Models) > 0
	if proxy {
		args = s.withModel(args)
	}
	if proxy && !hasModelArg(args) {
		return fmt.Errorf("zero copilot in this workspace needs a model: zero copilot --model PROVIDER/MODEL ..., or model: for the workspace in zero's config (models of the workspace: %s)", strings.Join(s.Models, ", "))
	}
	if _, err := s.Up(out); err != nil {
		return err
	}
	if !proxy && s.read("copilot") == "" {
		return fmt.Errorf("%s", noModel)
	}
	if _, err := s.ssh("test -x " + copilotBin); err != nil {
		return fmt.Errorf("the image of %s has no Copilot CLI; recreate it: zero rm && zero up", s.Name)
	}
	if proxy {
		// The models rule of the sandbox names the binary (PoC 23).
		ssh := []string{"-F", s.path("ssh.conf"), s.Host, copilotProxyScript(s.Dir, args)}
		if tty {
			ssh = append([]string{"-t"}, ssh...)
		}
		return passthrough("ssh", ssh...)
	}
	if !fileExists(s.path("copilot-cli.rule")) {
		if err := rt.AllowEgress(s.Name, copilotRule, copilotBin, "read-write", []string{copilotHost}); err != nil {
			return fmt.Errorf("%w; check: zero doctor, then run zero copilot again", err)
		}
		if err := s.write("copilot-cli.rule", copilotHost, 0o600); err != nil {
			return err
		}
		fmt.Fprintf(out, "copilot: %s may reach %s\n", copilotBin, copilotHost)
	}
	ssh := []string{"-F", s.path("ssh.conf"), s.Host, copilotScript(s.Dir, args)}
	if tty {
		ssh = append([]string{"-t"}, ssh...)
	}
	return passthrough("ssh", ssh...)
}

// claudeBin is Claude Code in the image. /usr/local/bin/claude is a link to
// it; the egress rule has to name the program itself.
const claudeBin = "/usr/local/libexec/claude/node_modules/@anthropic-ai/claude-code/bin/claude.exe"

// ClaudeNeedsTerminal is the check of `zero claude` without arguments, for
// main to refuse before a workspace is made.
func ClaudeNeedsTerminal(args []string) error {
	if len(args) == 0 && (!isTerminal(os.Stdin) || !isTerminal(os.Stdout)) {
		return fmt.Errorf("zero claude needs a terminal; without one: zero claude --model PROVIDER/MODEL -p PROMPT")
	}
	return nil
}

// withModel puts the workspace's default model in front of the arguments
// when they name none (ADR 0054).
func (s *Sandbox) withModel(args []string) []string {
	if s.Model == "" || hasModelArg(args) {
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
	if !hasModelArg(args) {
		return fmt.Errorf("zero claude needs a model: zero claude --model PROVIDER/MODEL ..., or model: for the workspace in zero's config (models of the workspace: %s)", strings.Join(s.Models, ", "))
	}
	if _, err := s.Up(out); err != nil {
		return err
	}
	if _, err := s.ssh("test -x " + claudeBin); err != nil {
		return fmt.Errorf("the image of %s has no Claude Code; recreate it: zero rm && zero up", s.Name)
	}
	ssh := []string{"-F", s.path("ssh.conf"), s.Host, claudeScript(s.Dir, args)}
	if isTerminal(os.Stdin) && isTerminal(os.Stdout) {
		ssh = append([]string{"-t"}, ssh...)
	}
	return passthrough("ssh", ssh...)
}

// claudeScript is the command run in the sandbox (PoC 24): Claude Code talks
// to the model proxy with the placeholder of the sandbox's environment as
// its key and asks Anthropic for nothing; the image pins its version. The
// model of --model is also the one for its background calls.
func claudeScript(dir string, args []string) string {
	script := "cd " + shellQuote(dir) + ` && [ -n "$` + modelKeyEnv + `" ] || { echo 'zero claude: the sandbox has no key for the model proxy; run zero up' >&2; exit 1; } && ANTHROPIC_BASE_URL=` +
		fmt.Sprintf("http://%s:%d", gateHost, modelPort) + ` ANTHROPIC_AUTH_TOKEN="$` + modelKeyEnv + `" ANTHROPIC_MODEL=` + shellQuote(modelArg(args)) + ` ANTHROPIC_DEFAULT_HAIKU_MODEL=` + shellQuote(modelArg(args)) +
		` CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 CLAUDE_CODE_DISABLE_OFFICIAL_MARKETPLACE_AUTOINSTALL=1 DISABLE_UPDATES=1 DISABLE_AUTOUPDATER=1 DISABLE_TELEMETRY=1 DISABLE_ERROR_REPORTING=1 exec ` + claudeBin
	for _, a := range args {
		script += " " + shellQuote(a)
	}
	return script
}

// modelArg is the model the arguments for the Copilot CLI or Claude Code
// name with --model, or "". A value that starts with - is the next option,
// not a model.
func modelArg(args []string) string {
	for i, a := range args {
		v := ""
		if a == "--model" && i+1 < len(args) {
			v = args[i+1]
		} else if strings.HasPrefix(a, "--model=") {
			v = strings.TrimPrefix(a, "--model=")
		}
		if v != "" && !strings.HasPrefix(v, "-") {
			return v
		}
	}
	return ""
}

func hasModelArg(args []string) bool { return modelArg(args) != "" }

// copilotProxyScript is the command run in a sandbox whose workspace uses the
// model proxy (PoC 23): the CLI talks to the proxy and asks GitHub for
// nothing. Its key is the placeholder of the sandbox's environment (PoC 20).
func copilotProxyScript(dir string, args []string) string {
	script := "cd " + shellQuote(dir) + ` && [ -n "$` + modelKeyEnv + `" ] || { echo 'zero copilot: the sandbox has no key for the model proxy; run zero up' >&2; exit 1; } && COPILOT_AUTO_UPDATE=false COPILOT_OFFLINE=true COPILOT_PROVIDER_BASE_URL=` +
		fmt.Sprintf("http://%s:%d/v1", gateHost, modelPort) + ` COPILOT_PROVIDER_API_KEY="$` + modelKeyEnv + `" exec ` + copilotBin + " --no-auto-update"
	for _, a := range args {
		script += " " + shellQuote(a)
	}
	return script
}

// copilotScript is the command run in the sandbox. The CLI does not update
// itself there: the image pins its version.
func copilotScript(dir string, args []string) string {
	script := "cd " + shellQuote(dir) + " && COPILOT_AUTO_UPDATE=false COPILOT_GITHUB_TOKEN=\"$" + copilotEnv + "\" exec " + copilotBin + " --no-auto-update"
	for _, a := range args {
		script += " " + shellQuote(a)
	}
	return script
}

// Run is up (silent) + a one-shot opencode run inside the sandbox.
func (s *Sandbox) Run(out io.Writer, prompt []string) error {
	if _, err := s.Up(io.Discard); err != nil {
		return err
	}
	script := "cd " + shellQuote(s.Dir) + " && exec opencode run --"
	for _, p := range prompt {
		script += " " + shellQuote(p)
	}
	return passthrough("ssh", "-F", s.path("ssh.conf"), s.Host, script)
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
			fmt.Fprintf(out, "%s tools: %s\n", f.Name(), strings.Join(t, ", "))
		}
	} else if f != nil {
		fmt.Fprintf(out, "%s token: none; zero up makes a new one\n", f.Name())
	} else {
		fmt.Fprintln(out, noForge)
	}
	if len(s.Instructions) > 0 {
		fmt.Fprintf(out, "instructions: %s\n", baseNames(s.Instructions))
	}
	if len(s.Skills) > 0 {
		fmt.Fprintf(out, "skills: %s\n", baseNames(s.Skills))
	}
	if len(s.Models) > 0 {
		if s.Model != "" {
			fmt.Fprintf(out, "model: %s (default; a repository's opencode.json and --model win)\n", s.Model)
		}
		fmt.Fprintf(out, "copilot: %s may reach the model proxy only (zero copilot --model PROVIDER/MODEL)\n", copilotBin)
		fmt.Fprintf(out, "claude: %s may reach the model proxy only (zero claude --model PROVIDER/MODEL)\n", claudeBin)
	} else if fileExists(s.path("copilot-cli.rule")) {
		fmt.Fprintf(out, "copilot: %s may reach %s\n", copilotBin, copilotHost)
	}
	if s.read("gate.key") != "" {
		if gateUp() {
			fmt.Fprintf(out, "push gate: on (%s and below)\n", strings.TrimPrefix(sessionBranch(s.Name), "refs/heads/"))
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
	// The model key goes first: with the proxy down the pause stops here and
	// no key outlives a sandbox that was stopped.
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
			return fmt.Errorf("delete sandbox (state kept for retry): %w", err)
		}
	}
	if err := s.revokeForge(); err != nil {
		return fmt.Errorf("%w (state kept for retry)", err)
	}
	rt.UnbindCredential(s.copilotProvider())
	// The binding holds the only copy of the model key. Once it is gone the
	// key cannot be used, also when the proxy is down and cannot delete it.
	if err := s.dropModelProvider(); err != nil {
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
	} else if !fileExists(s.path("copilot.done")) {
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
		return s.errNoSandbox()
	}
	if follow {
		cmd := exec.Command("ssh", "-F", s.path("ssh.conf"), s.Host,
			"tail -n 200 -f ~/.local/share/opencode/log/opencode.log")
		cmd.Stdout, cmd.Stderr = &cleanWriter{w: out}, &cleanWriter{w: os.Stderr}
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
