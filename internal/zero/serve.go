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

// zero serve (#97) is the one long-running host process: it hosts the push
// gate for all sandboxes and keeps forge tokens renewed and orphans revoked,
// so neither depends on the next `zero up`.

const (
	serveUnit     = "zero.service"
	upkeepEvery   = 10 * time.Minute
	serveGreeting = "zero serve"
)

// Serve runs the broker in the foreground.
func Serve() error {
	log.SetFlags(log.LstdFlags | log.LUTC)
	go func() {
		for {
			upkeep()
			time.Sleep(upkeepEvery)
		}
	}()
	addr := fmt.Sprintf("127.0.0.1:%d", gatePort)
	log.Printf("serve: push gate on %s, upkeep every %s", addr, upkeepEvery)
	srv := &http.Server{
		Addr:              addr,
		Handler:           http.HandlerFunc(gateHandler),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    64 << 10,
	}
	return srv.ListenAndServe()
}

// withGateway runs fn with OPENSHELL_GATEWAY set as recorded for a sandbox
// ("" = the default gateway) and restores the environment afterwards.
func withGateway(gw string, fn func()) {
	old, had := os.LookupEnv("OPENSHELL_GATEWAY")
	if gw != "" {
		os.Setenv("OPENSHELL_GATEWAY", gw)
	} else {
		os.Unsetenv("OPENSHELL_GATEWAY")
	}
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

// ensureServe starts zero serve if it is not answering: through its systemd
// user unit when installed, else detached.
func ensureServe() error {
	if gateUp() {
		return nil
	}
	if err := os.MkdirAll(stateRoot(), 0o700); err != nil {
		return err
	}
	out := filepath.Join(stateRoot(), "serve.out")
	if _, err := sh("systemctl", "--user", "cat", serveUnit); err == nil {
		out = "journalctl --user -u " + serveUnit
		if _, err := sh("systemctl", "--user", "start", serveUnit); err != nil {
			return err
		}
	} else {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		logf, err := os.OpenFile(out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer logf.Close()
		cmd := exec.Command(exe, "serve")
		cmd.Stdout, cmd.Stderr = logf, logf
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start zero serve: %w", err)
		}
		go cmd.Wait()
	}
	for i := 0; i < 50; i++ {
		if gateUp() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("zero serve not answering on 127.0.0.1:%d (see %s)", gatePort, out)
}
