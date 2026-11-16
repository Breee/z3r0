package zero

import (
	"bufio"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// The push gate (L4, #95) is a git smart-HTTP proxy on the host. Sandboxes
// cannot push to the forge directly (provider profile deny rule); they push to
// the gate, which forwards a push only if every ref update stays inside the
// sandbox's session branches. It holds no credential: the Authorization header
// (filled in by the supervisor) is passed through.
//
// One gate serves all sandboxes: the port is fixed because the provider
// profile that declares the endpoint is shared. A sandbox is identified by the
// secret key in its push URL: /s/<key>/<group/project>.git
const (
	gatePort = 18095
	gateHost = "host.openshell.internal"
	zeroSHA  = "0000000000000000000000000000000000000000"
)

type refUpdate struct{ Old, New, Ref string }

// readCommands reads the ref update commands of a receive-pack request up to
// the first flush packet.
func readCommands(r *bufio.Reader) ([]refUpdate, error) {
	var ups []refUpdate
	for {
		head := make([]byte, 4)
		if _, err := io.ReadFull(r, head); err != nil {
			return nil, fmt.Errorf("truncated request")
		}
		n, err := strconv.ParseUint(string(head), 16, 16)
		if err != nil {
			return nil, fmt.Errorf("bad pkt-line")
		}
		if n == 0 {
			return ups, nil
		}
		if n < 4 {
			return nil, fmt.Errorf("unsupported pkt-line")
		}
		line := make([]byte, n-4)
		if _, err := io.ReadFull(r, line); err != nil {
			return nil, fmt.Errorf("truncated request")
		}
		s, _, _ := strings.Cut(string(line), "\x00")
		f := strings.Fields(s)
		if len(f) != 3 || len(f[0]) != 40 || len(f[1]) != 40 {
			return nil, fmt.Errorf("unsupported command %q", s)
		}
		ups = append(ups, refUpdate{f[0], f[1], f[2]})
	}
}

// checkRefs allows only updates of branch (refs/heads/zero/<name>) and
// branches below it; no deletes.
func checkRefs(ups []refUpdate, branch string) error {
	if len(ups) == 0 {
		return fmt.Errorf("no ref updates")
	}
	for _, u := range ups {
		if u.Ref != branch && !strings.HasPrefix(u.Ref, branch+"/") {
			return fmt.Errorf("%s: only %s and %s/* may be pushed", u.Ref, branch, branch)
		}
		if u.New == zeroSHA {
			return fmt.Errorf("%s: delete not allowed", u.Ref)
		}
	}
	return nil
}

// sessionBranch is the ref a sandbox may push.
func sessionBranch(sandbox string) string {
	return "refs/heads/zero/" + strings.TrimPrefix(sandbox, "zero-")
}

type gateSession struct {
	name, state, upstream, repo string
}

// gateLookup finds the sandbox owning key by scanning the state dirs, so a
// removed sandbox loses access at once.
func gateLookup(key string) *gateSession {
	if len(key) != 32 {
		return nil
	}
	dirs, _ := filepath.Glob(filepath.Join(stateRoot(), "*", "gate.key"))
	for _, p := range dirs {
		b, err := os.ReadFile(p)
		if err != nil || strings.TrimSpace(string(b)) != key {
			continue
		}
		dir := filepath.Dir(p)
		read := func(n string) string {
			b, _ := os.ReadFile(filepath.Join(dir, n))
			return strings.TrimSpace(string(b))
		}
		f := forgeByName(read("forge"))
		if f == nil || read("forge.path") == "" {
			return nil
		}
		return &gateSession{filepath.Base(dir), dir, "https://" + f.Host(), "/" + read("forge.path") + ".git"}
	}
	return nil
}

// gateDeny answers in the receive-pack result format so git shows the reason.
func gateDeny(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
	line := "\x03zero: push denied: " + msg + "\n"
	fmt.Fprintf(w, "%04x%s0000", len(line)+4, line)
}

var gateLogMu sync.Mutex

func gateRecord(gs *gateSession, u refUpdate, decision, reason string) {
	gateLogMu.Lock()
	defer gateLogMu.Unlock()
	f, err := os.OpenFile(filepath.Join(gs.state, "gate.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(map[string]string{
		"time": time.Now().UTC().Format(time.RFC3339), "sandbox": gs.name,
		"ref": u.Ref, "old": u.Old, "new": u.New, "decision": decision, "reason": reason,
	})
}

func gateHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		fmt.Fprintln(w, "zero gate")
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/s/")
	key, rest, _ := strings.Cut(rest, "/")
	gs := gateLookup(key)
	if !ok || gs == nil {
		http.Error(w, "zero: unknown sandbox", http.StatusForbidden)
		return
	}
	rest = "/" + rest
	target, _ := url.Parse(gs.upstream)
	proxy := httputil.NewSingleHostReverseProxy(target)
	r.URL.Path, r.URL.RawPath, r.Host = rest, "", target.Host

	switch {
	case r.Method == http.MethodGet && rest == gs.repo+"/info/refs" && r.URL.Query().Get("service") == "git-receive-pack":
		proxy.ServeHTTP(w, r)
	case r.Method == http.MethodPost && rest == gs.repo+"/git-receive-pack":
		body := io.Reader(r.Body)
		if r.Header.Get("Content-Encoding") == "gzip" {
			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "bad gzip body", http.StatusBadRequest)
				return
			}
			body = zr
			r.Header.Del("Content-Encoding")
		}
		br := bufio.NewReader(body)
		var seen strings.Builder
		ups, err := readCommands(bufio.NewReader(io.TeeReader(br, &seen)))
		if err == nil {
			err = checkRefs(ups, sessionBranch(gs.name))
		}
		if err != nil {
			for _, u := range ups {
				gateRecord(gs, u, "deny", err.Error())
			}
			if len(ups) == 0 {
				gateRecord(gs, refUpdate{}, "deny", err.Error())
			}
			gateDeny(w, err.Error())
			return
		}
		for _, u := range ups {
			gateRecord(gs, u, "allow", "")
		}
		// The tee'd reader may have buffered past the commands: replay
		// everything read so far, then the rest.
		r.Body = io.NopCloser(io.MultiReader(strings.NewReader(seen.String()), br))
		r.ContentLength = -1
		r.Header.Del("Content-Length")
		proxy.ServeHTTP(w, r)
	default:
		http.Error(w, "zero: the gate only serves pushes to "+gs.repo, http.StatusForbidden)
	}
}

// ServeGate runs the push gate in the foreground (`zero gate`).
func ServeGate() error {
	return http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", gatePort), http.HandlerFunc(gateHandler))
}

// gateUp reports whether the gate answers on its port.
func gateUp() bool {
	c := http.Client{Timeout: time.Second}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", gatePort))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 32))
	return strings.TrimSpace(string(b)) == "zero gate"
}

// ensureGate starts the shared gate detached if it is not running.
func ensureGate() error {
	if gateUp() {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateRoot(), 0o700); err != nil {
		return err
	}
	logf, err := os.OpenFile(filepath.Join(stateRoot(), "gate.out"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer logf.Close()
	cmd := exec.Command(exe, "gate")
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start push gate: %w", err)
	}
	go cmd.Wait()
	for i := 0; i < 50; i++ {
		if gateUp() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("push gate not answering on 127.0.0.1:%d (see %s)", gatePort, logf.Name())
}

// gateURL is the sandbox's push URL; the key is created on first use.
func (s *Sandbox) gateURL() (string, error) {
	key := s.read("gate.key")
	if key == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		key = hex.EncodeToString(b)
		if err := s.write("gate.key", key, 0o600); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("http://%s:%d/s/%s/%s.git", gateHost, gatePort, key, s.read("forge.path")), nil
}
