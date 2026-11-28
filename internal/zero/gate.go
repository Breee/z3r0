package zero

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Breee/z3r0/internal/core"
	"github.com/Breee/z3r0/internal/gate"
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
	gateHost = core.SandboxHost
)

const commandTimeout = 30 * time.Second

// Limits on one push (#139). One gate serves all sandboxes, so a sandbox must
// not be able to hold it with endless or stalled requests.
const (
	maxPushBytes = 2 << 30          // request body, as sent and unpacked
	pushIdle     = 2 * time.Minute  // longest pause while the body arrives
	pushTimeout  = 30 * time.Minute // a whole request
	maxPushes    = 2                // pushes of one sandbox at a time
)

var errPushTooLarge = errors.New("push larger than 2 GiB")

// limitReader fails with errPushTooLarge after n bytes; before each read
// touch (if set) moves the read deadline of the connection.
type limitReader struct {
	r     io.Reader
	n     int64
	touch func()
	hit   bool
}

func (l *limitReader) Read(p []byte) (int, error) {
	if l.n <= 0 {
		// the end exactly at the limit is not an error
		if n, err := l.r.Read(make([]byte, 1)); n == 0 && err != nil {
			return 0, err
		}
		l.hit = true
		return 0, errPushTooLarge
	}
	if l.touch != nil {
		l.touch()
	}
	if int64(len(p)) > l.n {
		p = p[:l.n]
	}
	n, err := l.r.Read(p)
	l.n -= int64(n)
	return n, err
}

var (
	pushMu  sync.Mutex
	pushing = map[string]int{}
)

// pushSlot takes one of the sandbox's push slots; free gives it back.
func pushSlot(name string) (free func(), ok bool) {
	pushMu.Lock()
	defer pushMu.Unlock()
	if pushing[name] >= maxPushes {
		return nil, false
	}
	pushing[name]++
	return func() {
		pushMu.Lock()
		defer pushMu.Unlock()
		if pushing[name]--; pushing[name] == 0 {
			delete(pushing, name)
		}
	}, true
}

type gateSession struct {
	name, state, upstream string
	repos                 []string // /<group/project>.git of every workspace repo
	ciRun                 bool     // capability ci.run: pipelines start on push
	mrCreate              bool     // capability gitlab.mr.create: merge_request.* push options pass
}

// gateLookup finds the sandbox owning key by scanning the state dirs, so a
// removed sandbox loses access at once.
func gateLookup(key string) *gateSession {
	if len(key) != 32 {
		return nil
	}
	dirs, _ := filepath.Glob(filepath.Join(core.StateRoot(), "*", "gate.key"))
	for _, p := range dirs {
		b, err := os.ReadFile(p)
		if err != nil || subtle.ConstantTimeCompare(bytes.TrimSpace(b), []byte(key)) != 1 {
			continue
		}
		dir := filepath.Dir(p)
		read := func(n string) string {
			b, _ := os.ReadFile(filepath.Join(dir, n))
			return strings.TrimSpace(string(b))
		}
		f := forgeOf(dir)
		var repos []string
		for _, p := range strings.Fields(read("forge.path")) {
			repos = append(repos, "/"+p+".git")
		}
		if f == nil || len(repos) == 0 {
			return nil
		}
		caps := strings.Fields(read("caps"))
		return &gateSession{filepath.Base(dir), dir, "https://" + f.Host(), repos,
			slices.Contains(caps, core.CapCIRun), slices.Contains(caps, core.CapMRCreate)}
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

func gateRecord(gs *gateSession, u gate.Update, decision, reason string) {
	gateLogMu.Lock()
	defer gateLogMu.Unlock()
	log.Printf("gate %s: %s %s %s", gs.name, decision, u.Ref, reason)
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

// gateRefuse answers 403 and records the request: in the journal, and in the
// sandbox's gate.log if the key was valid. The key itself is not logged.
func gateRefuse(w http.ResponseWriter, r *http.Request, gs *gateSession, key, path, reason string) {
	who := "unknown key"
	if len(key) > 4 {
		who = "key " + key[:4] + "..."
	}
	if gs != nil {
		who = gs.name
	}
	path = strings.ToValidUTF8(path, "?")
	if len(path) > 200 {
		path = path[:200]
	}
	log.Printf("gate: refused %s %q from %s (%s): %s", r.Method, path, r.RemoteAddr, who, reason)
	if gs != nil {
		gateLogMu.Lock()
		if f, err := os.OpenFile(filepath.Join(gs.state, "gate.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			json.NewEncoder(f).Encode(map[string]string{
				"time": time.Now().UTC().Format(time.RFC3339), "sandbox": gs.name, "peer": r.RemoteAddr,
				"method": r.Method, "path": path, "decision": "refused", "reason": reason,
			})
			f.Close()
		}
		gateLogMu.Unlock()
	}
	http.Error(w, "zero: "+reason, http.StatusForbidden)
}

func gateHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		fmt.Fprintln(w, serveGreeting)
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/s/")
	key, rest, _ := strings.Cut(rest, "/")
	gs := gateLookup(key)
	if !ok || gs == nil {
		if !ok {
			key, rest = "", r.URL.Path
		}
		gateRefuse(w, r, nil, key, rest, "unknown sandbox")
		return
	}
	rest = "/" + rest
	ctx, cancel := context.WithTimeout(r.Context(), pushTimeout)
	defer cancel()
	r = r.WithContext(ctx)
	target, _ := url.Parse(gs.upstream)
	proxy := httputil.NewSingleHostReverseProxy(target)
	r.URL.Path, r.URL.RawPath, r.Host = rest, "", target.Host

	repo, op, _ := strings.Cut(rest, ".git/")
	if repo += ".git"; !slices.Contains(gs.repos, repo) {
		op = ""
	}
	switch {
	case r.Method == http.MethodGet && op == "info/refs" && r.URL.Query().Get("service") == "git-receive-pack":
		proxy.ServeHTTP(w, r)
	case r.Method == http.MethodPost && op == "git-receive-pack":
		free, ok := pushSlot(gs.name)
		if !ok {
			err := fmt.Errorf("%d pushes of this sandbox are running; try again when one has ended", maxPushes)
			gateRecord(gs, gate.Update{}, "deny", err.Error())
			gateDeny(w, err.Error())
			return
		}
		defer free()
		rc := http.NewResponseController(w)
		rc.SetWriteDeadline(time.Now().Add(pushTimeout))
		if r.ContentLength > maxPushBytes {
			gateRecord(gs, gate.Update{}, "deny", errPushTooLarge.Error())
			gateDeny(w, errPushTooLarge.Error())
			return
		}
		// the limit counts the bytes as sent and, if compressed, as unpacked
		sent := &limitReader{r: r.Body, n: maxPushBytes}
		body := io.Reader(sent)
		if r.Header.Get("Content-Encoding") == "gzip" {
			zr, err := gzip.NewReader(sent)
			if err != nil {
				http.Error(w, "bad gzip body", http.StatusBadRequest)
				return
			}
			body = zr
			r.Header.Del("Content-Encoding")
		}
		lim := &limitReader{r: body, n: maxPushBytes}
		tooLarge := func() bool { return sent.hit || lim.hit }
		// the command list must arrive promptly; the pack may take longer,
		// but not stall
		rc.SetReadDeadline(time.Now().Add(commandTimeout))
		br := bufio.NewReader(lim)
		ups, caps, err := gate.ReadCommands(br)
		if err == nil {
			err = gate.CheckRefs(ups, gate.SessionBranch(gs.name))
		}
		hasOpts := slices.Contains(strings.Fields(caps), "push-options")
		var opts []string
		if err == nil {
			err = gate.CheckCaps(caps)
		}
		if err == nil && hasOpts {
			if opts, err = gate.ReadOptions(br); err == nil {
				err = gate.CheckOptions(opts, gs.mrCreate && gs.ciRun)
			}
		}
		sent.touch = func() { rc.SetReadDeadline(time.Now().Add(pushIdle)) }
		sent.touch()
		if err != nil {
			for _, u := range ups {
				gateRecord(gs, u, "deny", err.Error())
			}
			if len(ups) == 0 {
				gateRecord(gs, gate.Update{}, "deny", err.Error())
			}
			gateDeny(w, err.Error())
			return
		}
		skip, note := !gs.ciRun, "ci.skip added"
		pack := io.Reader(br)
		if gs.ciRun {
			// ci.run: the pipeline starts unless the branch changes a pipeline
			// definition file (#126, ADR 0045)
			tmp, why, err := gs.checkDefinition(r, repo, ups, br)
			if tmp != "" {
				defer gs.dropRef(r, repo, ups[0].New, tmp)
			}
			if err != nil {
				if tooLarge() {
					err = errPushTooLarge
				}
				gateRecord(gs, ups[0], "deny", err.Error())
				gateDeny(w, err.Error())
				return
			}
			if tmp != "" {
				pack = bytes.NewReader(gate.EmptyPack())
			}
			if skip, note = why != "", "checked: no pipeline definition change"; skip {
				note = "ci.skip added: " + why
				proxy.ModifyResponse = gateNote(caps, "zero: pipeline skipped: "+why)
			}
			// a merge request opened by push option would run the changed
			// definition in its own pipeline (ADR 0044)
			if skip && len(opts) > 0 {
				err := fmt.Errorf("push option %q not allowed: %s", opts[0], why)
				gateRecord(gs, ups[0], "deny", err.Error())
				gateDeny(w, err.Error())
				return
			}
		}
		cmds := gate.WriteCommands(ups, caps, skip)
		if hasOpts {
			cmds += gate.WriteOptions(opts)
		}
		for _, u := range ups {
			gateRecord(gs, u, "allow", note)
		}
		r.Body = io.NopCloser(io.MultiReader(strings.NewReader(cmds), pack))
		r.ContentLength = -1
		r.Header.Del("Content-Length")
		proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
			if tooLarge() {
				gateRecord(gs, gate.Update{}, "deny", errPushTooLarge.Error())
				gateDeny(w, errPushTooLarge.Error())
				return
			}
			gateRecord(gs, gate.Update{}, "error", "push not forwarded: "+err.Error())
			w.WriteHeader(http.StatusBadGateway)
		}
		proxy.ServeHTTP(w, r)
	default:
		gateRefuse(w, r, gs, key, rest, "the gate only serves pushes to "+strings.Join(gs.repos, ", "))
	}
}

// checkDefinition decides whether a push with ci.run may start its pipeline.
// The gate cannot see file names in the pack, so it pushes the pack to a
// temporary branch with ci.skip and asks the forge which files the commit
// changes against the default branch. why is the reason to skip, "" to run.
// tmp is the temporary ref if the pack was sent: the caller then updates the
// session branch with an empty pack and drops tmp (also on error: the forge
// may have created it). Every failure before the pack is sent skips the
// pipeline; a pack the forge refuses is an error.
func (gs *gateSession) checkDefinition(r *http.Request, repo string, ups []gate.Update, pack io.Reader) (tmp, why string, err error) {
	_, token, ok := r.BasicAuth()
	u := ups[0]
	f := forgeOf(gs.state)
	switch {
	case len(ups) != 1 || u.Ref != gate.SessionBranch(gs.name):
		return "", "the check needs a push of " + gate.SessionBranch(gs.name) + " alone", nil
	case !ok || token == "" || len(u.New) != len(gate.ZeroSHA):
		return "", "the check needs a token and a sha1 repository", nil
	}
	b := make([]byte, 6)
	rand.Read(b)
	tmp = "refs/heads/zero-check/" + strings.TrimPrefix(gs.name, "zero-") + "-" + hex.EncodeToString(b)
	cmds := gate.WriteCommands([]gate.Update{{Old: gate.ZeroSHA, New: u.New, Ref: tmp}}, "report-status", true)
	if err := gs.push(r, repo, tmp, io.MultiReader(strings.NewReader(cmds), pack)); err != nil {
		return tmp, "", err // the branch may exist all the same
	}
	file, err := f.DefinitionChanged(token, strings.TrimSuffix(repo[1:], ".git"), u.New)
	if err != nil {
		return tmp, "changed files unknown (" + err.Error() + ")", nil
	}
	if file != "" {
		return tmp, "the branch changes the pipeline definition (" + file + "); a human starts it", nil
	}
	return tmp, "", nil
}

// push sends a receive-pack request of the gate's own for one ref with the
// client's credentials and reads the plain report-status answer.
func (gs *gateSession) push(r *http.Request, repo, ref string, body io.Reader) error {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, gs.upstream+repo+"/git-receive-pack", body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("Content-Type", "application/x-git-receive-pack-request")
	req.Header.Set("Accept", "application/x-git-receive-pack-result")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("forge not reached")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("forge answered %d", resp.StatusCode)
	}
	return gate.ReadReport(bufio.NewReader(io.LimitReader(resp.Body, 1<<16)), ref)
}

// dropRef deletes the temporary ref; a leftover is logged, not an error.
func (gs *gateSession) dropRef(r *http.Request, repo, sha, ref string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), commandTimeout)
	defer cancel()
	cmds := gate.WriteCommands([]gate.Update{{Old: sha, New: gate.ZeroSHA, Ref: ref}}, "report-status delete-refs", false)
	if err := gs.push(r.WithContext(ctx), repo, ref, strings.NewReader(cmds)); err != nil {
		gateRecord(gs, gate.Update{Old: sha, New: gate.ZeroSHA, Ref: ref}, "error", "temporary branch not deleted (or never created): "+err.Error())
	}
}

// gateNote puts a message line in front of the forge's answer, which git
// prints as "remote: ..." if the client reads side bands.
func gateNote(caps, msg string) func(*http.Response) error {
	return func(resp *http.Response) error {
		c := strings.Fields(caps)
		if resp.StatusCode == http.StatusOK && (slices.Contains(c, "side-band-64k") || slices.Contains(c, "side-band")) {
			resp.Body = struct {
				io.Reader
				io.Closer
			}{io.MultiReader(strings.NewReader(gate.PktLine("\x02"+msg+"\n")), resp.Body), resp.Body}
			resp.ContentLength = -1
			resp.Header.Del("Content-Length")
		}
		return nil
	}
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
	return strings.TrimSpace(string(b)) == serveGreeting
}

// gateURL is the sandbox's push URL; the key is created on first use.
func (s *Sandbox) gateURL(path string) (string, error) {
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
	return fmt.Sprintf("http://%s:%d/s/%s/%s.git", gateHost, gatePort, key, path), nil
}
