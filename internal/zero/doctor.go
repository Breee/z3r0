package zero

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
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

	fmt.Fprintf(out, "zero %s\n", Version())

	// The gateway state belongs to the dedicated user; only root can read it.
	if os.Getuid() == 0 {
		check("credential store", credentialStore())
		fmt.Fprintln(out, "skip  all other checks: run `zero doctor` without sudo")
		if fails > 0 {
			return fmt.Errorf("%d check(s) failed", fails)
		}
		return nil
	}
	fmt.Fprintln(out, "skip  credential store: run `sudo zero doctor`")

	if _, err := exec.LookPath("openshell"); err != nil {
		check("openshell cli", "not on PATH")
	} else {
		withGateway("", func() {
			if _, err := sh("openshell", "gateway", "info"); err != nil {
				check("openshell gateway", "unreachable: "+err.Error())
			} else {
				check("openshell gateway", "")
			}
		})
	}
	check("gateway user", gatewayUser())
	check("client key", clientKey())
	if gateUp() {
		check("zero-cp", "")
	} else {
		check("zero-cp", "not running: systemctl --user start zero-cp (or zero up)")
	}

	check("model login", modelLogin())
	if c, err := loadConfig(); err == nil && c.Parts.Models != "" {
		check("model proxy runtime", composeProblem())
		check("model proxy login", proxyLogin())
	}

	if top := gitTop(); top != "" {
		remote, _ := sh("git", "-C", top, "remote", "get-url", "origin")
		f, path := forgeFor(remote)
		if g, ok := f.(gitlab); ok && g.LoggedIn() == nil && g.PlaintextLogin() {
			fmt.Fprintf(out, "warn  glab login for %s is stored in plain text: glab auth login --hostname %s --use-keyring\n", g.host, g.host)
		}
		if f == nil {
			fmt.Fprintf(out, "na    forge: %s\n", noForgeHere(remote))
		} else if err := f.LoggedIn(); err != nil {
			check("forge login", err.Error())
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

// notGitlab are hosts for which a glab login is no fix.
var notGitlab = []string{"github.com", "bitbucket.org", "codeberg.org", "dev.azure.com"}

// noForgeHere says why a repository has no forge checks. It is no failure of
// the host setup: the sandbox works, the agent cannot push.
func noForgeHere(remote string) string {
	remote = clean(strings.TrimSpace(remote))
	if remote == "" {
		return "this repository has no origin; a sandbox for it cannot push"
	}
	host, _, ok := parseRemote(remote)
	if !ok || slices.Contains(notGitlab, host) {
		return "origin " + remote + " is not on GitLab; a sandbox for it cannot push"
	}
	return "origin is on " + host + ", no glab login for it; if it is a GitLab: glab auth login --hostname " + host + " --use-keyring. Without it a sandbox cannot push and the project's guardrails are not checked"
}

// credentialStore fails unless every running gateway has an owner-only
// key-encryption key and no forge or Copilot token in plaintext in its state.
func credentialStore() string {
	procs, _ := filepath.Glob("/proc/[0-9]*/comm")
	found := false
	for _, c := range procs {
		b, err := os.ReadFile(c)
		if err != nil || strings.TrimSpace(string(b)) != "openshell-gatew" {
			continue
		}
		found = true
		dir := filepath.Dir(c)
		fi, err := os.Stat(dir)
		if err != nil {
			continue
		}
		uid := fi.Sys().(*syscall.Stat_t).Uid
		state := ""
		env, _ := os.ReadFile(filepath.Join(dir, "environ"))
		for _, kv := range bytes.Split(env, []byte{0}) {
			if v, ok := bytes.CutPrefix(kv, []byte("XDG_STATE_HOME=")); ok {
				state = string(v)
			}
		}
		if state == "" {
			u, err := user.LookupId(strconv.Itoa(int(uid)))
			if err != nil {
				return err.Error()
			}
			state = filepath.Join(u.HomeDir, ".local/state")
		}
		gw := filepath.Join(state, "openshell/gateway")
		kek := filepath.Join(gw, "credentials/key-encryption-key.bin")
		ki, err := os.Stat(kek)
		if err != nil {
			return "no key-encryption key: " + err.Error()
		}
		if ki.Mode().Perm()&0o077 != 0 || ki.Sys().(*syscall.Stat_t).Uid != uid {
			return kek + " must be 0600 and owned by the gateway user"
		}
		plain := ""
		filepath.WalkDir(gw, func(p string, d os.DirEntry, err error) error {
			if err != nil || !d.Type().IsRegular() {
				return nil
			}
			data, _ := os.ReadFile(p)
			for _, pre := range []string{"glpat-", "gho_", "ghu_", "ghp_"} {
				if bytes.Contains(data, []byte(pre)) {
					plain = "plaintext token (" + pre + ") in " + p
				}
			}
			return nil
		})
		if plain != "" {
			return plain
		}
	}
	if !found {
		return "no openshell-gateway process found"
	}
	return ""
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
