package zero

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
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

// Limits on what the gate parses from a sandbox (#100). A normal push names a
// handful of refs in short lines.
const (
	maxRefUpdates  = 32
	maxCommandLine = 1024
	commandTimeout = 30 * time.Second
)

type refUpdate struct{ Old, New, Ref string }

// readCommands reads the ref update commands of a receive-pack request up to
// the first flush packet. caps is the capability list of the first command.
func readCommands(r *bufio.Reader) (ups []refUpdate, caps string, err error) {
	for {
		head := make([]byte, 4)
		if _, err := io.ReadFull(r, head); err != nil {
			return nil, "", fmt.Errorf("truncated request")
		}
		n, err := strconv.ParseUint(string(head), 16, 16)
		if err != nil {
			return nil, "", fmt.Errorf("bad pkt-line")
		}
		if n == 0 {
			return ups, caps, nil
		}
		if n < 4 {
			return nil, "", fmt.Errorf("unsupported pkt-line")
		}
		if n > maxCommandLine {
			return nil, "", fmt.Errorf("command line longer than %d bytes", maxCommandLine)
		}
		if len(ups) == maxRefUpdates {
			return nil, "", fmt.Errorf("more than %d ref updates in one push", maxRefUpdates)
		}
		line := make([]byte, n-4)
		if _, err := io.ReadFull(r, line); err != nil {
			return nil, "", fmt.Errorf("truncated request")
		}
		s, c, _ := strings.Cut(string(line), "\x00")
		if len(ups) == 0 {
			caps = strings.TrimSpace(c)
		}
		f := strings.Fields(s)
		if len(f) != 3 || len(f[0]) != 40 || len(f[1]) != 40 {
			return nil, "", fmt.Errorf("unsupported command %q", s)
		}
		ups = append(ups, refUpdate{f[0], f[1], f[2]})
	}
}

// readOptions reads the push options that follow the commands when the
// client announced push-options, up to their flush packet.
func readOptions(r *bufio.Reader) ([]string, error) {
	var opts []string
	for {
		head := make([]byte, 4)
		if _, err := io.ReadFull(r, head); err != nil {
			return nil, fmt.Errorf("truncated request")
		}
		n, err := strconv.ParseUint(string(head), 16, 16)
		if err != nil || (n != 0 && n < 4) {
			return nil, fmt.Errorf("unsupported pkt-line")
		}
		if n == 0 {
			return opts, nil
		}
		if n > maxCommandLine || len(opts) == maxRefUpdates {
			return nil, fmt.Errorf("push options too long or too many")
		}
		line := make([]byte, n-4)
		if _, err := io.ReadFull(r, line); err != nil {
			return nil, fmt.Errorf("truncated request")
		}
		opts = append(opts, strings.TrimSuffix(string(line), "\n"))
	}
}

// checkCaps refuses a capability that git reads as push-options but the gate
// would not ("push-options=x"): its options would reach the forge unchecked.
func checkCaps(caps string) error {
	for _, c := range strings.Fields(caps) {
		if c != "push-options" && strings.HasPrefix(strings.ToLower(c), "push-options") {
			return fmt.Errorf("capability %q not allowed", c)
		}
	}
	return nil
}

// checkOptions refuses the agent's push options: the forge acts on them with
// only push permission (merge_request.create opens a merge request, whose
// pipeline ci.skip does not prevent, #134). merge_request.* passes when
// gitlab.mr.create and ci.run are granted (ADR 0044).
func checkOptions(opts []string, mrCreate bool) error {
	for _, o := range opts {
		if !mrCreate || !strings.HasPrefix(o, "merge_request.") || strings.ContainsAny(o, "\n\x00") {
			return fmt.Errorf("push option %q not allowed", o)
		}
	}
	return nil
}

func pktLine(s string) string { return fmt.Sprintf("%04x%s", len(s)+4, s) }

// writeCommands is the command section of a receive-pack request for ups.
// With skipCI the push option ci.skip is added (#102): the forge records the
// pipeline as skipped, so agent-written CI code runs only when a human starts
// it. The rest of the request must follow: the client's own push options if
// it announced push-options (writeOptions), then the pack.
func writeCommands(ups []refUpdate, caps string, skipCI bool) string {
	hadOpts := slices.Contains(strings.Fields(caps), "push-options")
	if skipCI && !hadOpts {
		caps = strings.TrimSpace(caps + " push-options")
	}
	var b strings.Builder
	for i, u := range ups {
		l := u.Old + " " + u.New + " " + u.Ref
		if i == 0 {
			l += "\x00" + caps
		}
		b.WriteString(pktLine(l + "\n"))
	}
	b.WriteString("0000")
	if skipCI {
		b.WriteString(pktLine("ci.skip\n"))
		if !hadOpts {
			b.WriteString("0000")
		}
	}
	return b.String()
}

// writeOptions is the client's push option section after writeCommands.
func writeOptions(opts []string) string {
	var b strings.Builder
	for _, o := range opts {
		b.WriteString(pktLine(o + "\n"))
	}
	return b.String() + "0000"
}

// checkRefs allows only updates of branch (refs/heads/zero/<name>) and
// branches below it; no deletes.
func checkRefs(ups []refUpdate, branch string) error {
	if len(ups) == 0 {
		return fmt.Errorf("no ref updates")
	}
	for _, u := range ups {
		if u.Ref != branch && !strings.HasPrefix(u.Ref, branch+"/") || strings.Contains(u.Ref, "..") {
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
			slices.Contains(caps, capCIRun), slices.Contains(caps, capMRCreate)}
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

func gateHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		fmt.Fprintln(w, serveGreeting)
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

	repo, op, _ := strings.Cut(rest, ".git/")
	if repo += ".git"; !slices.Contains(gs.repos, repo) {
		op = ""
	}
	switch {
	case r.Method == http.MethodGet && op == "info/refs" && r.URL.Query().Get("service") == "git-receive-pack":
		proxy.ServeHTTP(w, r)
	case r.Method == http.MethodPost && op == "git-receive-pack":
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
		// the command list must arrive promptly; the pack may take longer
		rc := http.NewResponseController(w)
		rc.SetReadDeadline(time.Now().Add(commandTimeout))
		br := bufio.NewReader(body)
		ups, caps, err := readCommands(br)
		if err == nil {
			err = checkRefs(ups, sessionBranch(gs.name))
		}
		hasOpts := slices.Contains(strings.Fields(caps), "push-options")
		var opts []string
		if err == nil {
			err = checkCaps(caps)
		}
		if err == nil && hasOpts {
			if opts, err = readOptions(br); err == nil {
				err = checkOptions(opts, gs.mrCreate && gs.ciRun)
			}
		}
		rc.SetReadDeadline(time.Time{})
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
				gateRecord(gs, ups[0], "deny", err.Error())
				gateDeny(w, err.Error())
				return
			}
			if tmp != "" {
				pack = bytes.NewReader(emptyPack())
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
		cmds := writeCommands(ups, caps, skip)
		if hasOpts {
			cmds += writeOptions(opts)
		}
		for _, u := range ups {
			gateRecord(gs, u, "allow", note)
		}
		r.Body = io.NopCloser(io.MultiReader(strings.NewReader(cmds), pack))
		r.ContentLength = -1
		r.Header.Del("Content-Length")
		proxy.ServeHTTP(w, r)
	default:
		http.Error(w, "zero: the gate only serves pushes to "+strings.Join(gs.repos, ", "), http.StatusForbidden)
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
func (gs *gateSession) checkDefinition(r *http.Request, repo string, ups []refUpdate, pack io.Reader) (tmp, why string, err error) {
	_, token, ok := r.BasicAuth()
	u := ups[0]
	f := forgeOf(gs.state)
	switch {
	case len(ups) != 1 || u.Ref != sessionBranch(gs.name):
		return "", "the check needs a push of " + sessionBranch(gs.name) + " alone", nil
	case !ok || token == "" || len(u.New) != len(zeroSHA):
		return "", "the check needs a token and a sha1 repository", nil
	}
	b := make([]byte, 6)
	rand.Read(b)
	tmp = "refs/heads/zero-check/" + strings.TrimPrefix(gs.name, "zero-") + "-" + hex.EncodeToString(b)
	cmds := writeCommands([]refUpdate{{zeroSHA, u.New, tmp}}, "report-status", true)
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
	return readReport(bufio.NewReader(io.LimitReader(resp.Body, 1<<16)), ref)
}

// readReport is nil when a report-status answer says the pack was unpacked
// and ref was updated; otherwise the error carries the forge's line.
func readReport(r *bufio.Reader, ref string) error {
	lines, err := readOptions(r)
	if err != nil {
		return fmt.Errorf("forge answer not understood")
	}
	if len(lines) > 0 && lines[0] == "unpack ok" && slices.Contains(lines[1:], "ok "+ref) {
		return nil
	}
	return fmt.Errorf("forge refused the push: %s", strings.Join(lines, "; "))
}

// dropRef deletes the temporary ref; a leftover is logged, not an error.
func (gs *gateSession) dropRef(r *http.Request, repo, sha, ref string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), commandTimeout)
	defer cancel()
	cmds := writeCommands([]refUpdate{{sha, zeroSHA, ref}}, "report-status delete-refs", false)
	if err := gs.push(r.WithContext(ctx), repo, ref, strings.NewReader(cmds)); err != nil {
		gateRecord(gs, refUpdate{sha, zeroSHA, ref}, "error", "temporary branch not deleted (or never created): "+err.Error())
	}
}

// emptyPack is a pack without objects: the objects of a checked push are
// already on the forge.
func emptyPack() []byte {
	p := []byte("PACK\x00\x00\x00\x02\x00\x00\x00\x00")
	sum := sha1.Sum(p)
	return append(p, sum[:]...)
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
			}{io.MultiReader(strings.NewReader(pktLine("\x02"+msg+"\n")), resp.Body), resp.Body}
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
