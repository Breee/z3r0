package zero

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Doctor checks the host setup and, inside a git repo, the forge guardrails.
// It only reads; every failed check names the fix.
func Doctor(out io.Writer) error {
	fails := 0
	check := func(name string, problem string) {
		if problem == "" {
			fmt.Fprintf(out, "ok    %s\n", name)
			return
		}
		fails++
		fmt.Fprintf(out, "FAIL  %s: %s\n", name, problem)
	}

	if _, err := exec.LookPath("openshell"); err != nil {
		check("openshell cli", "not on PATH")
	} else if _, err := sh("openshell", "gateway", "info"); err != nil {
		check("openshell gateway", "unreachable: "+err.Error())
	} else {
		check("openshell gateway", "")
	}
	check("gateway user", gatewayUser())
	check("docker access", dockerAccess())
	check("client key", clientKey())

	if s, err := New(); err == nil {
		remote, _ := sh("git", "-C", s.Top, "remote", "get-url", "origin")
		if f, path := forgeFor(remote); f == nil {
			check("forge", "origin "+remote+" is not a supported forge")
		} else if missing, err := f.Guardrails(path); err != nil {
			check("guardrails "+path, err.Error())
		} else {
			check("guardrails "+path, strings.Join(missing, "; "))
		}
	}
	if fails > 0 {
		return fmt.Errorf("%d check(s) failed", fails)
	}
	return nil
}

// gatewayUser fails if the gateway runs as the login user: a sandbox breakout
// would then reach the user's files and keys (F09).
func gatewayUser() string {
	procs, _ := filepath.Glob("/proc/[0-9]*/comm")
	found := false
	for _, c := range procs {
		b, err := os.ReadFile(c)
		if err != nil || strings.TrimSpace(string(b)) != "openshell-gatew" {
			continue
		}
		found = true
		fi, err := os.Stat(filepath.Dir(c))
		if err != nil {
			continue
		}
		if fi.Sys().(*syscall.Stat_t).Uid == uint32(os.Getuid()) {
			return "gateway runs as the login user; run it as a dedicated user"
		}
	}
	if !found {
		return "no openshell-gateway process found"
	}
	return ""
}

// dockerAccess fails on docker group membership or a writable docker socket
// (root-equivalent, P03).
func dockerAccess() string {
	if g, err := user.LookupGroup("docker"); err == nil {
		gids, _ := os.Getgroups()
		id, _ := strconv.Atoi(g.Gid)
		for _, x := range gids {
			if x == id {
				return "login user is in group docker (root-equivalent)"
			}
		}
	}
	if syscall.Access("/var/run/docker.sock", 2) == nil {
		return "/var/run/docker.sock is writable (root-equivalent)"
	}
	return ""
}

// clientKey fails if a gateway mTLS client key is readable by others.
func clientKey() string {
	home, _ := os.UserHomeDir()
	keys, _ := filepath.Glob(filepath.Join(home, ".config/openshell/gateways/*/mtls/tls.key"))
	for _, k := range keys {
		fi, err := os.Stat(k)
		if err == nil && fi.Mode().Perm()&0o077 != 0 {
			return k + " mode " + fi.Mode().Perm().String() + " (want 0600)"
		}
	}
	return ""
}
