package zero

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"
	"time"
)

// forward is a running local SSH tunnel: host port -> sandbox servePort.
type forward struct {
	pid  int
	port int
}

// fwdRe matches our -L spec inside /proc/<pid>/cmdline. The pid alone is
// never trusted — a recycled pid only counts if its cmdline proves it is our
// tunnel.
var fwdRe = regexp.MustCompile(fmt.Sprintf(`127\.0\.0\.1:(\d+):127\.0\.0\.1:%d`, servePort))

// currentForward reports the running forward for this sandbox, if any. A
// healthy match also heals a missing or stale port file.
func (s *Sandbox) currentForward() *forward {
	pid, err := strconv.Atoi(s.read("fwd.pid"))
	if err != nil || pid <= 0 || !pidAlive(pid) {
		return nil
	}
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return nil
	}
	m := fwdRe.FindStringSubmatch(string(cmdline))
	if m == nil {
		return nil
	}
	port, err := strconv.Atoi(m[1])
	if err != nil {
		return nil
	}
	if s.read("port") != m[1] {
		s.write("port", m[1], 0o600)
	}
	return &forward{pid: pid, port: port}
}

func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

// startForward opens the background SSH tunnel and waits until the local port
// accepts connections (or the tunnel dies).
func (s *Sandbox) startForward(port int) error {
	spec := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", port, servePort)
	cmd := exec.Command("ssh", "-F", s.path("ssh.conf"), "-N",
		"-o", "ExitOnForwardFailure=yes", "-L", spec, s.Host)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start forward: %w", err)
	}
	pid := cmd.Process.Pid
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	if err := s.write("fwd.pid", strconv.Itoa(pid), 0o600); err != nil {
		syscall.Kill(pid, syscall.SIGKILL)
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			os.Remove(s.path("fwd.pid"))
			return fmt.Errorf("ssh forward exited early: %v", err)
		default:
		}
		if dialOK(port) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	syscall.Kill(pid, syscall.SIGKILL)
	os.Remove(s.path("fwd.pid"))
	return fmt.Errorf("forward not listening on %s after 10s", s.url(port))
}

func dialOK(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// stopForward ends the local tunnel (the sandbox keeps running).
func (s *Sandbox) stopForward(out io.Writer) error {
	f := s.currentForward()
	if f == nil {
		os.Remove(s.path("fwd.pid"))
		return nil
	}
	syscall.Kill(f.pid, syscall.SIGTERM)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && pidAlive(f.pid) {
		time.Sleep(100 * time.Millisecond)
	}
	if pidAlive(f.pid) {
		syscall.Kill(f.pid, syscall.SIGKILL)
	}
	os.Remove(s.path("fwd.pid"))
	fmt.Fprintln(out, "forward stopped")
	return nil
}
