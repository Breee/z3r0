// Package zero implements the zero CLI: a per-repo OpenShell sandbox running
// `opencode serve`, with a local SSH forward for the web UI; the terminal UI runs inside the sandbox over ssh (ADR 0048).
package zero

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Breee/z3r0/internal/core"
	"github.com/Breee/z3r0/internal/policy"
	"github.com/Breee/z3r0/internal/sandbox/openshell"
)

//go:embed policy.yaml
var policyYAML []byte

// servePort is where opencode serve listens inside the sandbox; the local
// forward maps a host port onto it.
const servePort = 4096

// Sandbox is the per-repo zero environment: one OpenShell sandbox plus local
// state (port, forward pid, ssh config) under the state dir.
type Sandbox struct {
	Name         string   // zero-<workspace>
	Folders      []string // host directories of the workspace
	Tools        []string // forge MCP tools of the workspace, "default" expanded
	Instructions []string // instruction files on the host, absolute
	Skills       []string // skill directories on the host, absolute
	// Models the agents may use through the model proxy; empty: no proxy.
	Models    []string
	Model     string
	Providers []string
	// ProviderKeys: per key provider the command that prints its key.
	ProviderKeys map[string]string
	Repo         string // basename of the first directory
	Top          string // first directory on the host
	Dir          string // checkout path of the first directory inside the sandbox
	State        string // local state dir (0700)
	Host         string // ssh host alias from `openshell sandbox ssh-config`
	gw           string // OpenShell gateway; empty = active gateway (podman, E1)

	Approve bool
	// shown is the workspace policy the human said yes to at the first run.
	shown *policy.Policy
	// MaxInput is the size limit for input directories in bytes; 0 means the default.
	MaxInput int64 // --approve: accept a wider policy without asking
}

// rt is the sandbox system zero runs on.
var rt core.Runtime = openshell.Provider{}

// New resolves the enclosing git repo into a sandbox description. zero only
// operates inside a git work tree; the sandbox name derives from the repo name.
// UseVM selects the MicroVM gateway for a new sandbox. An existing sandbox
// keeps the gateway it was created on.
func (s *Sandbox) UseVM() error {
	if gw := s.read("gateway"); gw != openshell.VMGateway && rt.Exists(s.Name) {
		return fmt.Errorf("%s exists on the podman gateway; zero rm first", s.Name)
	}
	s.setGateway(openshell.VMGateway)
	return nil
}

// setGateway routes every openshell call (and ssh ProxyCommand) to gw.
func (s *Sandbox) setGateway(gw string) {
	s.gw = gw
	os.Setenv("OPENSHELL_GATEWAY", openshell.GatewayName(gw))
}

// SandboxName maps a repo basename to the sandbox name: lowercase, every
// character outside [a-z0-9] becomes '-' (matches the original shell version).
func SandboxName(repo string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(repo) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return "zero-" + b.String()
}

func image() string {
	if v := os.Getenv("Z3R0_IMAGE"); v != "" {
		return v
	}
	return devImage
}

func (s *Sandbox) path(parts ...string) string {
	return filepath.Join(append([]string{s.State}, parts...)...)
}

func (s *Sandbox) url(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func (s *Sandbox) ensureState() error {
	if err := os.MkdirAll(s.State, 0o700); err != nil {
		return fmt.Errorf("state dir %s: %w", s.State, err)
	}
	return nil
}

// acquireLock takes an exclusive flock on the state dir so concurrent zero
// commands on the same repo cannot interleave sandbox mutations.
func (s *Sandbox) acquireLock() (*lock, error) {
	if err := s.ensureState(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(s.path("lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another zero command is running for %s; run this one again when it has ended", s.Name)
	}
	return &lock{f: f}, nil
}

type lock struct{ f *os.File }

func (l *lock) release() {
	syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	l.f.Close()
}

func (s *Sandbox) read(name string) string {
	b, err := os.ReadFile(s.path(name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (s *Sandbox) write(name, val string, mode os.FileMode) error {
	return os.WriteFile(s.path(name), []byte(val), mode)
}

// port returns the local forward port: the stored one if still free,
// otherwise a freshly allocated port (guards against port collisions).
func (s *Sandbox) port() (int, error) {
	if raw := s.read("port"); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil && portFree(p) {
			return p, nil
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("allocate port: %w", err)
	}
	p := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	if err := s.write("port", strconv.Itoa(p), 0o600); err != nil {
		return 0, err
	}
	return p, nil
}

func portFree(p int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func (s *Sandbox) exists() bool {
	return rt.Exists(s.Name)
}

// ssh runs a script on the sandbox via the generated ssh config. Output and
// error text come from the sandbox and are returned without control
// characters.
func (s *Sandbox) ssh(script string) (string, error) {
	out, err := core.Sh("ssh", "-F", s.path("ssh.conf"), s.Host, script)
	if err != nil {
		err = errors.New(core.Clean(err.Error()))
	}
	return core.Clean(out), err
}

// probeServe asks the API through the forward. Without a paired browser the
// service answers 401 as JSON; the web UI answers 200 HTML on other paths.
func probeServe(port int) error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/session", port))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		return fmt.Errorf("HTTP %d, content type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	return nil
}

// devImage is built and cosign-signed by .github/workflows/dev-image.yml (ADR 0028).
const devImage = "ghcr.io/breee/z3r0-dev@sha256:a6d406ae281bd0065f873b77a3521316f0d687df4cde3465298dc3163dfcf1af"
