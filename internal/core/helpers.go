package core

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// StateRoot is the directory zero keeps its state in.
func StateRoot() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return filepath.Join(v, "zero")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".local", "state", "zero")
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Sh runs a command capturing stdout and stderr. On failure the error carries
// the full command line and stderr — failures are never swallowed.
func Sh(name string, args ...string) (string, error) {
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

// Passthrough runs a command on the caller's terminal.
func Passthrough(name string, args ...string) error {
	return PassthroughEnv(nil, name, args...)
}

// PassthroughEnv is Passthrough with extra environment (secrets stay out of
// argv); the exit error is kept.
// PassthroughEnv is Passthrough with extra environment; the exit error is kept.
func PassthroughEnv(env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func IsTerminal(f *os.File) bool {
	var t syscall.Termios
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TCGETS, uintptr(unsafe.Pointer(&t)))
	return e == 0
}

// RealPath returns p absolute with symlinks resolved.
func RealPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}
