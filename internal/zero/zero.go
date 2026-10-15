// Package zero implements the zero CLI: a per-repo OpenShell sandbox running
// `opencode serve`, with a local SSH forward the host opencode attaches to (F17).
package zero

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//go:embed policy.yaml
var policyYAML []byte

// servePort is where opencode serve listens inside the sandbox; the local
// forward maps a host port onto it.
const servePort = 4096

// Sandbox is the per-repo zero environment: one OpenShell sandbox plus local
// state (password, port, forward pid, ssh config) under the state dir.
type Sandbox struct {
	Name  string // zero-<repo>
	Repo  string // repo basename
	Top   string // git toplevel on the host
	Dir   string // checkout path inside the sandbox
	State string // local state dir (0700)
	Host  string // ssh host alias from `openshell sandbox ssh-config`
}

// New resolves the enclosing git repo into a sandbox description. zero only
// operates inside a git work tree; the sandbox name derives from the repo name.
func New() (*Sandbox, error) {
	top, err := sh("git", "rev-parse", "--show-toplevel")
	if err != nil || top == "" {
		return nil, fmt.Errorf("not inside a git repository (run zero from a repo)")
	}
	repo := filepath.Base(top)
	name := SandboxName(repo)
	return &Sandbox{
		Name:  name,
		Repo:  repo,
		Top:   top,
		Dir:   "/sandbox/" + repo,
		State: filepath.Join(stateRoot(), name),
		Host:  "openshell-" + name + ".default",
	}, nil
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

func stateRoot() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return filepath.Join(v, "zero")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".local", "state", "zero")
}

func image() string {
	if v := os.Getenv("Z3R0_IMAGE"); v != "" {
		return v
	}
	return "localhost/z3r0-dev:latest"
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
		return nil, fmt.Errorf("another zero command is running for %s", s.Name)
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// password reads or creates the random serve password (0600 state file).
func (s *Sandbox) password() (string, error) {
	if p := s.read("password"); p != "" {
		return p, nil
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("password: %w", err)
	}
	p := base64.RawStdEncoding.EncodeToString(buf)
	if err := s.write("password", p, 0o600); err != nil {
		return "", err
	}
	return p, nil
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
	_, err := sh("openshell", "sandbox", "get", s.Name)
	return err == nil
}

// ssh runs a script on the sandbox via the generated ssh config.
func (s *Sandbox) ssh(script string) (string, error) {
	return sh("ssh", "-F", s.path("ssh.conf"), s.Host, script)
}

// sh runs a command capturing stdout and stderr. On failure the error carries
// the full command line and stderr — failures are never swallowed.
func sh(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(errBuf.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), detail)
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// passthrough runs a host command attached to the terminal; the underlying
// ExitError is preserved so main can propagate the child's exit code.
func passthrough(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// shellQuote single-quotes s for a POSIX remote shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// probeConfig hits /config through the forward with basic auth.
func probeConfig(port int, pw string) (int, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/config", port), nil)
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth("opencode", pw)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, nil
}
