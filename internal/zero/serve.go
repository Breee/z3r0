package zero

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// zero-cp (#97, #119) is the zero service, the one long-running host process: it hosts the push
// gate for all sandboxes and keeps forge tokens renewed and orphans revoked,
// so neither depends on the next `zero up`.

const (
	serveUnit     = "zero-cp.service"
	upkeepEvery   = 10 * time.Minute
	serveGreeting = "zero-cp"
)

// Serve runs the service; it is the whole of the zero-cp binary.
func Serve() error {
	log.SetFlags(log.LstdFlags | log.LUTC)
	go func() {
		for {
			upkeep()
			time.Sleep(upkeepEvery)
		}
	}()
	addr := fmt.Sprintf("127.0.0.1:%d", gatePort)
	log.Printf("zero-cp: push gate on %s, upkeep every %s", addr, upkeepEvery)
	srv := &http.Server{
		Addr:              addr,
		Handler:           http.HandlerFunc(gateHandler),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       pushTimeout,
		WriteTimeout:      pushTimeout,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    64 << 10,
	}
	return srv.ListenAndServe()
}

// withGateway runs fn with OPENSHELL_GATEWAY set as recorded for a sandbox
// ("" = the podman gateway) and restores the environment afterwards.
func withGateway(gw string, fn func()) {
	old, had := os.LookupEnv("OPENSHELL_GATEWAY")
	os.Setenv("OPENSHELL_GATEWAY", gatewayName(gw))
	defer func() {
		if had {
			os.Setenv("OPENSHELL_GATEWAY", old)
		} else {
			os.Unsetenv("OPENSHELL_GATEWAY")
		}
	}()
	fn()
}

func readState(dir, name string) string {
	b, _ := os.ReadFile(filepath.Join(dir, name))
	return strings.TrimSpace(string(b))
}

// upkeep renews the token of every live forge sandbox and revokes tokens no
// sandbox owns. A sandbox another zero command holds the lock on is skipped.
func upkeep() {
	ids, _ := filepath.Glob(filepath.Join(stateRoot(), "*", "forge.id"))
	used := map[string]Forge{}
	// A sandbox whose token could not be revoked has no forge.id left; its
	// forge is still swept for orphans.
	marks, _ := filepath.Glob(filepath.Join(stateRoot(), "*", "forge"))
	for _, p := range marks {
		if f := forgeOf(filepath.Dir(p)); f != nil {
			used[f.Name()+"@"+f.Host()] = f
		}
	}
	for _, p := range ids {
		dir := filepath.Dir(p)
		s := &Sandbox{Name: filepath.Base(dir), State: dir}
		f := s.forge()
		if f == nil {
			continue
		}
		used[f.Name()+"@"+f.Host()] = f
		caps := strings.Fields(s.read("caps"))
		if len(caps) == 0 {
			log.Printf("upkeep %s: no recorded capabilities, run zero up once", s.Name)
			continue
		}
		l, err := s.acquireLock()
		if err != nil {
			continue
		}
		withGateway(s.read("gateway"), func() {
			if !rt.Exists(s.Name) || rt.Stopped(s.Name) {
				return
			}
			s.Host = rt.HostAlias(s.Name)
			old := s.read("forge.id")
			if err := s.renewForge(caps, false); err != nil {
				log.Printf("upkeep %s: renew: %v", s.Name, err)
			} else if id := s.read("forge.id"); id != old {
				log.Printf("upkeep %s: %s token renewed %s -> %s", s.Name, f.Name(), old, id)
			}
		})
		l.release()
	}
	for _, f := range used {
		if err := revokeOrphans(f); err != nil {
			log.Printf("upkeep: %v", err)
		}
	}
}

// cpBinary finds zero-cp: next to this zero binary, else on PATH.
func cpBinary() (string, error) {
	if exe, err := os.Executable(); err == nil {
		if p := filepath.Join(filepath.Dir(exe), "zero-cp"); fileExists(p) {
			return p, nil
		}
	}
	p, err := exec.LookPath("zero-cp")
	if err != nil {
		return "", fmt.Errorf("zero-cp not found next to zero or on PATH: sudo zero install")
	}
	return p, nil
}

// ensureServe starts zero-cp if it is not answering: through its systemd
// user unit when installed, else detached.
func ensureServe() error {
	if gateUp() {
		return nil
	}
	if err := os.MkdirAll(stateRoot(), 0o700); err != nil {
		return err
	}
	out := filepath.Join(stateRoot(), "zero-cp.out")
	if _, err := sh("systemctl", "--user", "cat", serveUnit); err == nil {
		out = "journalctl --user -u " + serveUnit
		if _, err := sh("systemctl", "--user", "start", serveUnit); err != nil {
			return err
		}
	} else {
		exe, err := cpBinary()
		if err != nil {
			return err
		}
		logf, err := os.OpenFile(out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer logf.Close()
		cmd := exec.Command(exe)
		cmd.Stdout, cmd.Stderr = logf, logf
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start zero-cp: %w", err)
		}
		go cmd.Wait()
	}
	for i := 0; i < 50; i++ {
		if gateUp() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("zero-cp not answering on 127.0.0.1:%d (see %s)", gatePort, out)
}
