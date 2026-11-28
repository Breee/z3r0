package zero

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Breee/z3r0/internal/core"
	"github.com/Breee/z3r0/internal/sandbox/openshell"
	"go.yaml.in/yaml/v3"
)

func TestSandboxName(t *testing.T) {
	cases := map[string]string{
		"myrepo":     "zero-myrepo",
		"My.Repo":    "zero-my-repo",
		"Z3R0":       "zero-z3r0",
		"foo_bar":    "zero-foo-bar",
		"UPPER CASE": "zero-upper-case",
	}
	for in, want := range cases {
		if got := SandboxName(in); got != want {
			t.Errorf("SandboxName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStateRoot(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/xdg")
	if got, want := core.StateRoot(), "/tmp/xdg/zero"; got != want {
		t.Errorf("stateRoot() = %q, want %q", got, want)
	}
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/u")
	if got, want := core.StateRoot(), "/home/u/.local/state/zero"; got != want {
		t.Errorf("stateRoot() = %q, want %q", got, want)
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"simple":            "'simple'",
		"it's":              `'it'\''s'`,
		"/tmp/zero/My Repo": `'/tmp/zero/My Repo'`,
	}
	for in, want := range cases {
		if got := core.ShellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestForwardCmdlineMatch(t *testing.T) {
	// /proc cmdline is NUL-separated.
	cmdline := "ssh\x00-F\x00/state/ssh.conf\x00-N\x00-L\x00127.0.0.1:25123:127.0.0.1:4096\x00openshell-zero-x.default\x00"
	m := fwdRe.FindStringSubmatch(cmdline)
	if m == nil {
		t.Fatal("expected forward cmdline to match")
	}
	if m[1] != "25123" {
		t.Errorf("captured port = %q, want 25123", m[1])
	}
	if fwdRe.MatchString("ssh\x00-s\x00some-other-session\x00") {
		t.Error("unrelated cmdline must not match")
	}
}

func TestGitlabHost(t *testing.T) {
	h, p, ok := parseRemote("ssh://git@git.example.org:2222/a/b.git")
	if !ok || h != "git.example.org" || p != "a/b" {
		t.Fatalf("parseRemote = %q %q %v", h, p, ok)
	}
	if strings.Contains(string(gitlab{h}.Profile()), "endpoints") {
		t.Error("the token profile must not carry an endpoint: profile endpoints add to the sandbox's rules")
	}
}

func TestForgeRule(t *testing.T) {
	rules := gitlabProjectRules("grp/sub/app", 42)
	var got []string
	for _, r := range rules {
		got = append(got, r.Method+" "+r.Path)
	}
	for _, want := range []string{"GET /api/v4/projects/42", "POST /api/v4/projects/42/**",
		"GET /api/v4/projects/grp%2Fsub%2Fapp", "HEAD /api/v4/projects/grp%2Fsub%2Fapp/**",
		"GET /grp/sub/app.git/info/refs", "POST /grp/sub/app.git/git-upload-pack"} {
		if !slices.Contains(got, want) {
			t.Errorf("rule %q missing in %v", want, got)
		}
	}
	for _, r := range got {
		if m, _, _ := strings.Cut(r, " "); !slices.Contains([]string{"GET", "HEAD", "POST"}, m) {
			t.Errorf("rule %q: method not needed", r)
		}
	}
	if _, _, err := (gitlab{host: "gitlab.com"}).Egress([]string{"grp/a*"}); err == nil {
		t.Error("project path with a glob character accepted")
	}
	for host, want := range map[string]bool{"gitlab.com": false, "*.example.com": false, "proxy.golang.org": true, "*.gitlab.com": true} {
		forge := "gitlab.com"
		if strings.Contains(host, "example") {
			forge = "git.example.com"
		}
		if err := checkForgeHost(forge, []string{host}); (err == nil) != want {
			t.Errorf("network.allow %s with forge %s: %v", host, forge, err)
		}
	}
	for _, r := range got {
		if strings.HasPrefix(r, "DELETE") || strings.Contains(r, "receive-pack") ||
			!(strings.Contains(r, "/42") || strings.Contains(r, "grp%2Fsub%2Fapp") || strings.Contains(r, "/grp/sub/app.git/")) {
			t.Errorf("rule %q reaches beyond the project", r)
		}
	}
	b, err := openshell.PolicyWith(policyYAML, forgeRule, map[string]any{"name": forgeRule})
	if err != nil {
		t.Fatal(err)
	}
	b, err = openshell.PolicyWith(b, "zen", map[string]any{"name": "zen"})
	if err != nil {
		t.Fatal(err)
	}
	y := string(b)
	if !strings.Contains(y, "forge:") || !strings.Contains(y, "zen:") || !strings.Contains(y, "filesystem_policy:") {
		t.Errorf("policy lost a part:\n%s", y)
	}
}

func TestForgeFor(t *testing.T) {
	for in, want := range map[string]string{
		"git@gitlab.com:corewire/technology-and-research/developer.git": "corewire/technology-and-research/developer",
		"https://gitlab.com/a/b.git":                                    "a/b",
		"https://user:x@gitlab.com/a/b":                                 "a/b",
		"ssh://git@gitlab.com/a/b.git":                                  "a/b",
		"git@nope.invalid:Breee/z3r0.git":                               "",
	} {
		f, got := forgeFor(in)
		if got != want || (want != "" && f.Name() != "gitlab") || (want == "" && f != nil) {
			t.Errorf("forgeFor(%q) = %v %q, want gitlab %q", in, f, got, want)
		}
	}
}

func TestParsePolicy(t *testing.T) {
	p, err := parsePolicy([]byte("network:\n  allow: [proxy.golang.org]\ncapabilities: [git.read]\n"))
	if err != nil || p.Network.Allow[0] != "proxy.golang.org" || p.Capabilities[0] != "git.read" {
		t.Fatalf("got %+v %v", p, err)
	}
	for _, bad := range []string{"bogus: 1\n", "network:\n  allow: [\"evil.com:22\"]\n", "network:\n  allow: [\"*\"]\n"} {
		if _, err := parsePolicy([]byte(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if _, err := (gitlab{}).Permissions([]string{"container.build"}, nil); err == nil {
		t.Error("unknown capability accepted")
	}
}

func TestCheckRefs(t *testing.T) {
	b := sessionBranch("zero-dev")
	a, n := strings.Repeat("a", 40), strings.Repeat("b", 40)
	ok := [][]refUpdate{
		{{a, n, "refs/heads/zero/dev"}},
		{{zeroSHA, n, "refs/heads/zero/dev/fix"}, {a, n, "refs/heads/zero/dev"}},
	}
	bad := [][]refUpdate{
		nil,
		{{a, n, "refs/heads/main"}},
		{{a, n, "refs/heads/zero/developer"}},
		{{a, n, "refs/heads/zero/other"}},
		{{a, n, "refs/tags/v1"}},
		{{a, zeroSHA, "refs/heads/zero/dev"}},
		{{a, n, "refs/heads/zero/dev"}, {a, n, "refs/heads/main"}},
		{{a, n, "refs/heads/zero/dev/../../main"}},
	}
	for _, u := range ok {
		if err := checkRefs(u, b); err != nil {
			t.Errorf("%v: %v", u, err)
		}
	}
	for _, u := range bad {
		if checkRefs(u, b) == nil {
			t.Errorf("%v allowed", u)
		}
	}
}

func TestReadCommands(t *testing.T) {
	a, n := strings.Repeat("a", 40), strings.Repeat("b", 40)
	line := a + " " + n + " refs/heads/x\x00report-status\n"
	in := fmt.Sprintf("%04x%s0000PACK", len(line)+4, line)
	ups, _, err := readCommands(bufio.NewReader(strings.NewReader(in)))
	if err != nil || len(ups) != 1 || ups[0].Ref != "refs/heads/x" {
		t.Fatalf("%v %v", ups, err)
	}
	if _, _, err := readCommands(bufio.NewReader(strings.NewReader("0005"))); err == nil {
		t.Fatal("truncated accepted")
	}
}

func TestWidened(t *testing.T) {
	pol := func(hosts, caps []string) *Policy {
		p := &Policy{Capabilities: caps}
		p.Network.Allow = hosts
		return p
	}
	old := pol([]string{"a.example"}, []string{"git.read", "git.push"})
	if w := widened(old, pol(nil, []string{"git.read"})); len(w) != 0 {
		t.Errorf("narrowing reported as widening: %v", w)
	}
	if w := widened(old, pol([]string{"a.example"}, []string{"git.push", "git.read"})); len(w) != 0 {
		t.Errorf("same policy reported as widening: %v", w)
	}
	w := widened(old, pol([]string{"a.example", "b.example"}, []string{"git.read", "gitlab.mr.create"}))
	if fmt.Sprint(w) != "[network b.example capability gitlab.mr.create]" {
		t.Errorf("widened = %v", w)
	}
}

func TestWriteCommands(t *testing.T) {
	a, n := strings.Repeat("a", 40), strings.Repeat("b", 40)
	ups := []refUpdate{{a, n, "refs/heads/zero/x"}}
	cmd := func(caps string) string { return pktLine(a + " " + n + " refs/heads/zero/x\x00" + caps + "\n") }
	skip := pktLine("ci.skip\n")
	for _, c := range []struct{ caps, skipCI, want string }{
		{"report-status", "n", cmd("report-status") + "0000"},
		{"report-status", "y", cmd("report-status push-options") + "0000" + skip + "0000"},
		{"report-status push-options", "y", cmd("report-status push-options") + "0000" + skip},
	} {
		if got := writeCommands(ups, c.caps, c.skipCI == "y"); got != c.want {
			t.Errorf("caps %q skip %s:\n got %q\nwant %q", c.caps, c.skipCI, got, c.want)
		}
	}
	// what was written parses back to the same updates
	got, _, err := readCommands(bufio.NewReader(strings.NewReader(writeCommands(ups, "x", true))))
	if err != nil || len(got) != 1 || got[0] != ups[0] {
		t.Fatalf("%v %v", got, err)
	}
}

func TestPushOptions(t *testing.T) {
	in := pktLine("merge_request.create\n") + pktLine("merge_request.target=main") + "0000PACK"
	br := bufio.NewReader(strings.NewReader(in))
	opts, err := readOptions(br)
	if err != nil || !slices.Equal(opts, []string{"merge_request.create", "merge_request.target=main"}) {
		t.Fatalf("%q %v", opts, err)
	}
	if rest, _ := io.ReadAll(br); string(rest) != "PACK" {
		t.Errorf("rest %q", rest)
	}
	if got := writeOptions(opts); got != pktLine("merge_request.create\n")+pktLine("merge_request.target=main\n")+"0000" {
		t.Errorf("written %q", got)
	}
	if checkOptions(opts, false) == nil {
		t.Error("merge_request.create passes without gitlab.mr.create")
	}
	if err := checkOptions(opts, true); err != nil {
		t.Error(err)
	}
	for _, o := range []string{"ci.variable=A=b", "ci.skip", "merge_request.title=x\nci.variable=A=b", ""} {
		if checkOptions([]string{o}, true) == nil {
			t.Errorf("option %q passes", o)
		}
	}
	for caps, ok := range map[string]bool{"report-status push-options": true, "report-status": true, "push-options=1": false, "a Push-Options": false, "push-options-x": false} {
		if (checkCaps(caps) == nil) != ok {
			t.Errorf("checkCaps(%q) ok = %v", caps, !ok)
		}
	}
	if checkOptions(nil, false) != nil {
		t.Error("no options refused")
	}
	for _, bad := range []string{"", "0003", "zzzz", "0009ab"} {
		if _, err := readOptions(bufio.NewReader(strings.NewReader(bad))); err == nil {
			t.Errorf("readOptions(%q) passes", bad)
		}
	}
}

// FuzzGateCommands feeds arbitrary bytes to the gate's parser (#100): it must
// not panic, and whatever it accepts must survive the rewrite unchanged.
func FuzzGateCommands(f *testing.F) {
	a, n := strings.Repeat("a", 40), strings.Repeat("b", 40)
	f.Add(pktLine(a+" "+n+" refs/heads/zero/dev\x00report-status push-options\n") + "0000PACK")
	f.Add(pktLine(a+" "+n+" refs/heads/main\n") + pktLine(a+" "+zeroSHA+" refs/heads/zero/dev\n") + "0000")
	f.Add("0000")
	f.Add("ffff")
	f.Fuzz(func(t *testing.T, in string) {
		ups, caps, err := readCommands(bufio.NewReader(strings.NewReader(in)))
		if err != nil {
			return
		}
		if len(ups) > maxRefUpdates {
			t.Fatalf("%d updates accepted", len(ups))
		}
		checkRefs(ups, "refs/heads/zero/dev")
		for _, skip := range []bool{false, true} {
			again, _, err := readCommands(bufio.NewReader(strings.NewReader(writeCommands(ups, caps, skip))))
			if err != nil || !slices.Equal(again, ups) {
				t.Fatalf("rewrite changed the commands: %v %v -> %v", err, ups, again)
			}
		}
	})
}

func TestReadCommandsLimits(t *testing.T) {
	a := strings.Repeat("a", 40)
	one := pktLine(a + " " + a + " refs/heads/zero/dev\n")
	if _, _, err := readCommands(bufio.NewReader(strings.NewReader(strings.Repeat(one, maxRefUpdates+1) + "0000"))); err == nil {
		t.Error("too many ref updates accepted")
	}
	long := pktLine(a + " " + a + " refs/heads/zero/" + strings.Repeat("x", maxCommandLine) + "\n")
	if _, _, err := readCommands(bufio.NewReader(strings.NewReader(long + "0000"))); err == nil {
		t.Error("overlong command accepted")
	}
}

func TestLinkPlainWhenNotTerminal(t *testing.T) {
	var b strings.Builder
	if got := link(&b, "http://127.0.0.1:1"); got != "http://127.0.0.1:1" {
		t.Fatalf("got %q", got)
	}
	f, err := os.CreateTemp(t.TempDir(), "x")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if got := link(f, "http://127.0.0.1:1"); got != "http://127.0.0.1:1" {
		t.Fatalf("file: got %q", got)
	}
}

func TestResolveContext(t *testing.T) {
	c := &Config{Current: "team", Workspaces: map[string]*Workspace{
		"api":    {Folders: []string{"/src/api"}},
		"team":   {Folders: []string{"/src/web", "/src/shared"}},
		"hotfix": {Folders: []string{"/src/shared"}},
		"other":  {Folders: []string{"/src/api2"}},
	}}
	cases := []struct{ flag, env, cwd, name, source string }{
		{"api", "team", "/src/web", "api", "flag"},
		{"", "hotfix", "/src/api", "hotfix", "env"},
		{"", "", "/src/api/cmd", "api", "cwd"},
		{"", "", "/src/api2", "other", "cwd"},
		{"", "", "/src/shared", "team", "current"},
		{"", "", "/elsewhere", "team", "current"},
	}
	for _, tc := range cases {
		name, source, err := c.resolve(tc.flag, tc.env, tc.cwd)
		if err != nil || name != tc.name || source != tc.source {
			t.Errorf("resolve(%q,%q,%q) = %q %q %v, want %q %q", tc.flag, tc.env, tc.cwd, name, source, err, tc.name, tc.source)
		}
	}
	if _, _, err := c.resolve("nope", "", "/src/api"); err == nil {
		t.Error("unknown -c accepted")
	}
	c.Current = "api"
	if _, _, err := c.resolve("", "", "/src/shared"); err == nil || !strings.Contains(err.Error(), "hotfix, team") {
		t.Errorf("ambiguous cwd: %v", err)
	}
	c.Current = ""
	if name, _, err := c.resolve("", "", "/elsewhere"); name != "" || err != nil {
		t.Errorf("no context: %q %v", name, err)
	}
	for in, want := range map[string]string{"developer": "developer", "My_Repo.go": "my-repo-go", "---": ""} {
		if got := wsName(in); got != want {
			t.Errorf("wsName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadPolicyUnion(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(a, ".zero"), 0o700)
	os.WriteFile(filepath.Join(a, ".zero", "policy.yaml"),
		[]byte("network:\n  allow: [proxy.golang.org]\ncapabilities: [git.read, ci.run]\n"), 0o600)
	// The input directory c is no policy source.
	c := t.TempDir()
	os.MkdirAll(filepath.Join(a, ".git"), 0o700)
	os.MkdirAll(filepath.Join(b, ".git"), 0o700)
	os.MkdirAll(filepath.Join(c, ".zero"), 0o700)
	os.WriteFile(filepath.Join(c, ".zero", "policy.yaml"), []byte("network:\n  allow: [evil.example]\ncapabilities: [git.read]\n"), 0o600)
	p, err := (&Sandbox{Folders: []string{a, b, c}}).loadPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Network.Allow, []string{"proxy.golang.org"}) {
		t.Errorf("allow = %v", p.Network.Allow)
	}
	// a's file plus the default of b, which has no file
	if want := append([]string{"git.read", "ci.run"}, defaultCaps[1:]...); !slices.Equal(p.Capabilities, want) {
		t.Errorf("caps = %v, want %v", p.Capabilities, want)
	}
}

func TestMRCreateNeedsCIRun(t *testing.T) {
	if slices.Contains(defaultCaps, capMRCreate) {
		t.Error("gitlab.mr.create is in the default capabilities")
	}
	for caps, ok := range map[string]bool{
		"[git.read, gitlab.mr.create]":         false,
		"[git.read, gitlab.mr.create, ci.run]": true,
		"[git.read]":                           true,
	} {
		a := t.TempDir()
		os.MkdirAll(filepath.Join(a, ".zero"), 0o700)
		os.MkdirAll(filepath.Join(a, ".git"), 0o700)
		os.WriteFile(filepath.Join(a, ".zero", "policy.yaml"), []byte("capabilities: "+caps+"\n"), 0o600)
		_, err := (&Sandbox{Folders: []string{a}}).loadPolicy()
		if (err == nil) != ok {
			t.Errorf("%s: err = %v, want ok %v", caps, err, ok)
		}
	}
}

func TestGateSeveralRepos(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := filepath.Join(core.StateRoot(), "zero-team")
	os.MkdirAll(dir, 0o700)
	key := strings.Repeat("ab", 16)
	for k, v := range map[string]string{"gate.key": key, "forge": "gitlab", "forge.path": "g/api\ng/web\n"} {
		os.WriteFile(filepath.Join(dir, k), []byte(v), 0o600)
	}
	gs := gateLookup(key)
	if gs == nil || !slices.Equal(gs.repos, []string{"/g/api.git", "/g/web.git"}) {
		t.Fatalf("gateLookup = %+v", gs)
	}
	// a repo outside the workspace is refused before anything is forwarded
	for _, path := range []string{"/g/other.git/info/refs?service=git-receive-pack", "/g/api.git.git/info/refs?service=git-receive-pack", "/g/api.git/info/refs?service=git-upload-pack"} {
		w := httptest.NewRecorder()
		gateHandler(w, httptest.NewRequest("GET", "/s/"+key+path, nil))
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", path, w.Code)
		}
	}
}

func TestGateLimits(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := filepath.Join(core.StateRoot(), "zero-team")
	os.MkdirAll(dir, 0o700)
	key := strings.Repeat("ab", 16)
	for k, v := range map[string]string{"gate.key": key, "forge": "gitlab", "forge.path": "g/api\n"} {
		os.WriteFile(filepath.Join(dir, k), []byte(v), 0o600)
	}
	gateLog := func() string {
		b, _ := os.ReadFile(filepath.Join(dir, "gate.log"))
		return string(b)
	}

	// refused requests are recorded, the key is not
	w := httptest.NewRecorder()
	gateHandler(w, httptest.NewRequest("GET", "/s/"+key+"/g/other.git/info/refs?service=git-receive-pack", nil))
	if l := gateLog(); w.Code != http.StatusForbidden || !strings.Contains(l, `"decision":"refused"`) ||
		!strings.Contains(l, `"path":"/g/other.git/info/refs"`) || !strings.Contains(l, `"peer":"`) || strings.Contains(l, key) {
		t.Errorf("refused request: status %d, log %q", w.Code, l)
	}
	w = httptest.NewRecorder()
	gateHandler(w, httptest.NewRequest("GET", "/s/"+strings.Repeat("cd", 16)+"/g/api.git/info/refs", nil))
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "unknown sandbox") {
		t.Errorf("unknown key: status %d, body %q", w.Code, w.Body)
	}

	// a body announced larger than the limit is refused before it is read
	post := func() *http.Request {
		return httptest.NewRequest("POST", "/s/"+key+"/g/api.git/git-receive-pack", strings.NewReader("0000"))
	}
	r := post()
	r.ContentLength = maxPushBytes + 1
	w = httptest.NewRecorder()
	gateHandler(w, r)
	if !strings.Contains(w.Body.String(), "push denied: push larger than") || !strings.Contains(gateLog(), "push larger than") {
		t.Errorf("large push: body %q", w.Body)
	}

	// no more than maxPushes pushes of one sandbox at a time
	var frees []func()
	for range maxPushes {
		free, ok := pushSlot("zero-team")
		if !ok {
			t.Fatal("no slot although below the limit")
		}
		frees = append(frees, free)
	}
	if _, ok := pushSlot("zero-other"); !ok {
		t.Error("another sandbox got no slot")
	}
	w = httptest.NewRecorder()
	gateHandler(w, post())
	if !strings.Contains(w.Body.String(), "pushes of this sandbox are running") {
		t.Errorf("push beyond the limit: body %q", w.Body)
	}
	for _, free := range frees {
		free()
	}
	if free, ok := pushSlot("zero-team"); !ok {
		t.Error("no slot after the pushes ended")
	} else {
		free()
	}

	// limitReader stops at the limit and reports it
	touched := 0
	l := &limitReader{r: strings.NewReader("0123456789"), n: 4, touch: func() { touched++ }}
	b, err := io.ReadAll(l)
	if string(b) != "0123" || err != errPushTooLarge || !l.hit || touched == 0 {
		t.Errorf("limitReader: %q, %v, hit %v, touched %d", b, err, l.hit, touched)
	}
	l = &limitReader{r: strings.NewReader("0123"), n: 4}
	if b, err := io.ReadAll(l); string(b) != "0123" || err != nil || l.hit {
		t.Errorf("limitReader at the limit: %q, %v", b, err)
	}
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// Every forbidden pattern of spec §6 is left out; .zeroignore and symlinks too.
func TestScanInputExcludes(t *testing.T) {
	top := t.TempDir()
	secret := []string{".env", ".ENV", "ID_RSA", "Server.PEM", ".AWS/credentials", ".env.local", "sub/.env.prod", "tls/server.pem", "tls/server.key", "id_rsa", "id_ed25519.pub",
		".git-credentials", ".netrc", ".ssh/config", ".aws/credentials", ".kube/config", "deep/.ssh/known_hosts"}
	files := map[string]string{"openapi.yaml": "a", "docs/README.md": "b", "draft.txt": "c", "private/x.txt": "d",
		".zeroignore": "# comment\ndraft.txt\nprivate/\n.env\n"}
	for _, f := range secret {
		files[f] = "secret"
	}
	writeTree(t, top, files)
	if err := os.Symlink("/etc/passwd", filepath.Join(top, "link")); err != nil {
		t.Fatal(err)
	}
	got, skip, err := scanInput(top)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range got {
		names = append(names, f.rel)
	}
	slices.Sort(names)
	if want := []string{".zeroignore", "docs/README.md", "openapi.yaml"}; !slices.Equal(names, want) {
		t.Fatalf("copied %v, want %v", names, want)
	}
	why := map[string]string{}
	for _, k := range skip {
		why[k.rel] = k.why
	}
	for rel, want := range map[string]string{".env": "secret pattern", ".env.local": "secret pattern", "sub/.env.prod": "secret pattern",
		"tls/server.pem": "secret pattern", "tls/server.key": "secret pattern", "id_rsa": "secret pattern", "id_ed25519.pub": "secret pattern",
		".git-credentials": "secret pattern", ".netrc": "secret pattern", ".ssh": "secret pattern", ".aws": "secret pattern",
		".kube": "secret pattern", "deep/.ssh": "secret pattern", "draft.txt": ".zeroignore", "private": ".zeroignore", "link": "symlink"} {
		if why[rel] != want {
			t.Errorf("%s: skipped as %q, want %q", rel, why[rel], want)
		}
	}
}

func TestBundleRepoWithoutStash(t *testing.T) {
	top := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := core.Sh("git", append([]string{"-C", top, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return out
	}
	git("init", "-q", "-b", "main")
	git("commit", "-q", "--allow-empty", "-m", "a")
	git("branch", "feat")
	git("tag", "v1")
	os.WriteFile(filepath.Join(top, ".env"), []byte("TOKEN=x"), 0o600)
	git("stash", "-u", "-q")
	git("checkout", "-q", "--detach")
	file := filepath.Join(t.TempDir(), "b")
	if err := bundleRepo(top, file); err != nil {
		t.Fatal(err)
	}
	heads := git("bundle", "list-heads", file)
	for _, want := range []string{"refs/heads/main", "refs/heads/feat", "refs/tags/v1", "HEAD"} {
		if !strings.Contains(heads, want) {
			t.Errorf("bundle lacks %s: %s", want, heads)
		}
	}
	if strings.Contains(heads, "stash") {
		t.Errorf("bundle carries the stash: %s", heads)
	}
}

func TestSecretPatterns(t *testing.T) {
	files := []string{".env", ".envrc", ".env.local", "prod.env", "ca.pem", "tls.key", "cert.p12", "cert.pfx", "app.jks", "release.keystore",
		"host.ppk", "pw.kdbx", "private.asc", "office.ovpn", "id_rsa", "id_ed25519.pub", "deploy_rsa", "old_dsa", "host_ecdsa", "ci_ed25519",
		".git-credentials", ".netrc", "_netrc", ".npmrc", ".pypirc", ".pgpass", ".htpasswd", ".dockercfg", ".s3cfg",
		"kubeconfig", "prod.kubeconfig", "terraform.tfstate", "terraform.tfstate.backup", "prod.tfvars", "prod.tfvars.json",
		".terraformrc", "terraform.rc", ".bash_history", ".psql_history"}
	if len(files) < len(secretFiles) {
		t.Fatalf("%d examples for %d patterns", len(files), len(secretFiles))
	}
	for _, p := range secretFiles {
		if !slices.ContainsFunc(files, func(f string) bool { ok, _ := filepath.Match(p, f); return ok }) {
			t.Errorf("pattern %s has no example", p)
		}
	}
	dirs := []string{".ssh", ".aws", ".kube", ".gnupg", ".docker", ".azure", ".terraform", ".git"}
	if !slices.Equal(dirs, secretDirs) {
		t.Fatalf("examples %v, patterns %v", dirs, secretDirs)
	}
	top := t.TempDir()
	tree := map[string]string{"openapi.yaml": "a", "environment.md": "b", "keys.md": "c", "history.txt": "d", "docker/compose.yaml": "e"}
	for _, f := range files {
		tree["sub/"+strings.ToUpper(f)] = "secret"
		tree[f] = "secret"
	}
	for _, d := range dirs {
		tree["sub/"+d+"/config"] = "secret"
	}
	writeTree(t, top, tree)
	got, skip, err := scanInput(top)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range got {
		names = append(names, f.rel)
	}
	slices.Sort(names)
	if want := []string{"docker/compose.yaml", "environment.md", "history.txt", "keys.md", "openapi.yaml"}; !slices.Equal(names, want) {
		t.Fatalf("copied %v, want %v", names, want)
	}
	if want := 2*len(files) + len(dirs); len(skip) != want {
		t.Fatalf("%d skipped, want %d", len(skip), want)
	}
}

func TestSyncInputs(t *testing.T) {
	old := inputRoot
	inputRoot = t.TempDir()
	defer func() { inputRoot = old }()
	specs := filepath.Join(t.TempDir(), "specs")
	writeTree(t, specs, map[string]string{"openapi.yaml": "openapi", ".env": "DB_PASSWORD=x", "bin/run.sh": "#!/bin/sh"})
	os.Chmod(filepath.Join(specs, "bin/run.sh"), 0o700)
	s := &Sandbox{Name: "zero-t", Folders: []string{specs}, State: t.TempDir()}
	var out strings.Builder
	if err := s.syncInputs(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "input specs: 2 files, 1 KiB, read-only") || !strings.Contains(out.String(), "excluded: .env (secret pattern)") {
		t.Fatalf("summary: %q", out.String())
	}
	stage := filepath.Join(inputRoot, "zero-t", "specs")
	if core.FileExists(filepath.Join(stage, ".env")) {
		t.Fatal(".env was copied")
	}
	for name, mode := range map[string]os.FileMode{"openapi.yaml": 0o644, "bin/run.sh": 0o755, "bin": 0o755} {
		fi, err := os.Stat(filepath.Join(stage, name))
		if err != nil || fi.Mode().Perm() != mode {
			t.Fatalf("%s: %v %v, want mode %o", name, fi, err, mode)
		}
	}

	// A file deleted on the host is gone after the next sync.
	os.Remove(filepath.Join(specs, "openapi.yaml"))
	if err := s.syncInputs(&out); err != nil {
		t.Fatal(err)
	}
	if core.FileExists(filepath.Join(stage, "openapi.yaml")) {
		t.Fatal("deleted file still staged")
	}

	// Over the limit: error, the staged copy stays as it was.
	writeTree(t, specs, map[string]string{"big.bin": strings.Repeat("x", 4096)})
	s.MaxInput = 1024
	if err := s.syncInputs(&out); err == nil || !strings.Contains(err.Error(), "limit is 1 KiB") {
		t.Fatalf("limit: %v", err)
	}
	if core.FileExists(filepath.Join(stage, "big.bin")) {
		t.Fatal("copied despite the limit")
	}

	// A directory that left the workspace loses its copy.
	s.Folders, s.MaxInput = nil, 0
	if err := s.syncInputs(&out); err != nil {
		t.Fatal(err)
	}
	if core.FileExists(stage) {
		t.Fatal("copy of a removed directory kept")
	}
}

func TestUnpushedScript(t *testing.T) {
	dir := t.TempDir()
	run := func(script string) string {
		t.Helper()
		out, err := core.Sh("sh", "-c", "cd "+core.ShellQuote(dir)+" && "+script)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	run("git init -q -b main . && git config user.email t@t && git config user.name t && " +
		"git commit -q --allow-empty -m base && git update-ref refs/remotes/origin/main HEAD")
	if got := run(unpushedScript); got != "" {
		t.Errorf("clean checkout: %q, want nothing", got)
	}
	run("git checkout -q -b feat/login && git commit -q --allow-empty -m a && git commit -q --allow-empty -m b && echo x > new.txt")
	want := "feat/login (2 commits)\nuncommitted changes (1 files)"
	if got := run(unpushedScript); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// A pushed branch has a remote branch at the same commit.
	run("git update-ref refs/remotes/origin/zero/x HEAD && rm new.txt")
	if got := run(unpushedScript); got != "" {
		t.Errorf("after push: %q, want nothing", got)
	}
}

func TestUntil(t *testing.T) {
	for d, want := range map[time.Duration]string{
		-time.Hour:                   "expired",
		20*time.Hour + time.Minute:   "20h",
		72*time.Hour + 5*time.Minute: "3d",
	} {
		if got := until(time.Now().Add(d)); got != want {
			t.Errorf("until(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestUncommitted(t *testing.T) {
	top := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if out, err := core.Sh("git", append([]string{"-C", top, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...); err != nil {
			t.Fatal(out, err)
		}
	}
	git("init", "-q")
	os.WriteFile(filepath.Join(top, "a"), []byte("1"), 0o644)
	os.WriteFile(filepath.Join(top, ".gitignore"), []byte("ignored\n"), 0o644)
	git("add", ".")
	git("commit", "-qm", "one")
	if n := uncommitted(top); n != 0 {
		t.Fatalf("clean repo: %d", n)
	}
	os.WriteFile(filepath.Join(top, "a"), []byte("2"), 0o644)
	os.WriteFile(filepath.Join(top, "new"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(top, "ignored"), []byte("x"), 0o644)
	if n := uncommitted(top); n != 2 {
		t.Fatalf("changed + untracked: %d, want 2", n)
	}
}

func TestGatewayPinned(t *testing.T) {
	t.Setenv("OPENSHELL_GATEWAY", "other")
	s := &Sandbox{State: t.TempDir()}
	s.setGateway("")
	if got := os.Getenv("OPENSHELL_GATEWAY"); got != openshell.DefaultGateway || s.gw != "" {
		t.Fatalf("setGateway(\"\"): env %q, gw %q", got, s.gw)
	}
	os.Setenv("OPENSHELL_GATEWAY", "other")
	var in []string
	for _, gw := range []string{"", openshell.VMGateway} {
		withGateway(gw, func() { in = append(in, os.Getenv("OPENSHELL_GATEWAY")) })
	}
	if !slices.Equal(in, []string{openshell.DefaultGateway, openshell.VMGateway}) || os.Getenv("OPENSHELL_GATEWAY") != "other" {
		t.Fatalf("withGateway: %v, after %q", in, os.Getenv("OPENSHELL_GATEWAY"))
	}
}

func TestSwitchKey(t *testing.T) {
	for in, want := range map[string]string{
		"k": "up", "\x1b[A": "up", "j": "down", "\x1b[B": "down", "\r": "use",
		"q": "quit", "\x03": "quit", "x": "",
	} {
		if got := switchKey([]byte(in)); got != want {
			t.Errorf("switchKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderSwitch(t *testing.T) {
	o := &overview{rows: []wsRow{
		{Name: "api", Status: "running", Token: "gitlab 20h", Policy: "abc123", Folders: 2, Current: true},
		{Name: "web", Status: "none", Token: "-", Policy: "-", Folders: 1},
	}, warn: []string{"gateway not reachable"}}
	var b bytes.Buffer
	renderSwitch(&b, o, 1)
	lines := strings.Split(b.String(), "\n")
	if !strings.HasPrefix(lines[1], "  * api") || !strings.Contains(lines[1], "running") {
		t.Errorf("current row: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], ">   web") {
		t.Errorf("selected row: %q", lines[2])
	}
	if !strings.Contains(b.String(), "warning: gateway not reachable") || !strings.Contains(b.String(), switchHelp) {
		t.Errorf("warning or help missing:\n%s", b.String())
	}
}

func TestNextKey(t *testing.T) {
	var got []string
	for b := []byte("jj\x1b[A\x1bOBx\r"); len(b) > 0; {
		a, n := nextKey(b)
		got, b = append(got, a), b[n:]
	}
	if want := "down down up down  use"; strings.Join(got, " ") != want {
		t.Errorf("keys = %q, want %q", strings.Join(got, " "), want)
	}
	if a, _ := nextKey([]byte("\x1b")); a != "quit" {
		t.Errorf("lone escape = %q, want quit", a)
	}
	for in, want := range map[string]bool{"\x1b": true, "j\x1b[": true, "\x1bO": true, "\x1b[A": false, "j": false} {
		if cutEscape([]byte(in)) != want {
			t.Errorf("cutEscape(%q) != %v", in, want)
		}
	}
}

func TestSwitchByName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	c := &Config{Current: "api", Workspaces: map[string]*Workspace{
		"api": {Folders: []string{dir}}, "web": {Folders: []string{dir}}}}
	if err := c.save(); err != nil {
		t.Fatal(err)
	}
	current := func() string {
		c, err := loadConfig()
		if err != nil {
			t.Fatal(err)
		}
		return c.Current
	}
	if err := Switch(nil, nil, []string{"web"}); err != nil || current() != "web" {
		t.Errorf("switch web: err %v, current %q", err, current())
	}
	if err := Switch(nil, nil, []string{"nope"}); err == nil || current() != "web" {
		t.Errorf("switch nope: err %v, current %q", err, current())
	}
	// Without a name and without a terminal: an error, nothing changed.
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := Switch(f, f, nil); err == nil || !strings.Contains(err.Error(), "terminal") || current() != "web" {
		t.Errorf("switch without terminal: err %v, current %q", err, current())
	}
}

func TestGitlabToolPermissions(t *testing.T) {
	got, err := gitlabToolPermissions([]string{"get_project", "list_commits", "get_commit", "list_branches"})
	if err != nil || strings.Join(got, " ") != "read_project read_code read_branch" {
		t.Fatalf("got %v, %v", got, err)
	}
	if got, _ := gitlabToolPermissions([]string{"list_branches"}); strings.Join(got, " ") != "read_branch" {
		t.Fatalf("one tool: %v", got)
	}
	if _, err := gitlabToolPermissions([]string{"get_project", "save_note"}); err == nil {
		t.Fatal("tool without entry accepted")
	}
	// the list pocs/15-gitlab-mcp/tools-check.sh was run with (#131)
	const all = "read_project read_code read_repository read_branch read_repository_tag read_merge_request read_pipeline read_job read_work_item read_namespace"
	if got, err := gitlabToolPermissions(gitlabDefaultTools); err != nil || strings.Join(got, " ") != all {
		t.Fatalf("default list: %v, %v", got, err)
	}
	if len(gitlabTools) != len(gitlabDefaultTools) {
		t.Fatalf("%d tools in the table, %d in the default list", len(gitlabTools), len(gitlabDefaultTools))
	}
	b, err := os.ReadFile("../../pocs/15-gitlab-mcp/default-tools.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(strings.Fields(string(b)), gitlabDefaultTools) {
		t.Fatal("default list differs from pocs/15-gitlab-mcp/default-tools.txt")
	}
}

func TestWorkspaceTools(t *testing.T) {
	got, err := mcpTools([]string{"get_project", "default", "get_project"})
	if err != nil || len(got) != len(gitlabDefaultTools) || got[0] != "get_project" {
		t.Fatalf("default with names: %v, %v", got, err)
	}
	if got, err := mcpTools(nil); err != nil || got != nil {
		t.Fatalf("no list: %v, %v", got, err)
	}
	if _, err := mcpTools([]string{"default", "save_note"}); err == nil || !strings.Contains(err.Error(), `"save_note"`) {
		t.Fatalf("tool without entry: %v", err)
	}

	caps := []string{"git.read", "git.push"}
	base, err := (gitlab{}).Permissions(caps, nil)
	if err != nil || strings.Join(base, " ") != "read_project read_code download_code push_code" {
		t.Fatalf("without tools: %v, %v", base, err)
	}
	with, err := (gitlab{}).Permissions(caps, []string{"get_project", "list_branches"})
	if err != nil || strings.Join(with, " ") != strings.Join(base, " ")+" read_branch" {
		t.Fatalf("with tools: %v, %v", with, err)
	}
	if _, err := (gitlab{}).Permissions(caps, []string{"save_note"}); err == nil {
		t.Fatal("permissions for a tool without entry")
	}

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_STATE_HOME", dir)
	write := func(tools string) {
		os.MkdirAll(filepath.Join(dir, "zero"), 0o700)
		os.WriteFile(configPath(), []byte("workspaces:\n    a:\n        folders: ["+dir+"]\n"+tools), 0o600)
	}
	write("        tools: [default, nope]\n")
	if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), `"nope"`) {
		t.Fatalf("config with unknown tool: %v", err)
	}
	write("")
	c, err := loadConfig()
	if err != nil || c.sandbox("a").Tools != nil {
		t.Fatalf("config without tools: %v", err)
	}
	write("        tools: [list_branches, get_project]\n")
	if c, err = loadConfig(); err != nil {
		t.Fatal(err)
	}
	s := c.sandbox("a")
	s.State = filepath.Join(dir, "state")
	os.MkdirAll(s.State, 0o700)
	if ch, err := s.syncTools(); err != nil || !ch || s.read("tools") != "list_branches get_project" {
		t.Fatalf("first sync: %v, %v, %q", ch, err, s.read("tools"))
	}
	if ch, _ := s.syncTools(); !ch {
		t.Fatal("list whose mint did not finish reported as unchanged")
	}
	s.forgeSynced()
	if ch, _ := s.syncTools(); ch {
		t.Fatal("unchanged list reported as changed")
	}
	s.Tools = nil
	if ch, _ := s.syncTools(); !ch || s.read("tools") != "" {
		t.Fatal("removed list not recorded")
	}
	if err := c.save(); err != nil {
		t.Fatal(err)
	}
	if c, _ = loadConfig(); len(c.Workspaces["a"].Tools) != 2 {
		t.Fatal("tools lost on save")
	}
}

func TestDefinitionFile(t *testing.T) {
	for f, want := range map[string]bool{
		".gitlab-ci.yml": true, "ci/build.yaml": true, "deploy/app.YML": true, ".gitlab/CODEOWNERS": true,
		"ci/pipeline.conf": true, "main.go": false, "Makefile": false, "docs/yml.md": false,
	} {
		if got := gitlabDefinitionFile(f, "ci/pipeline.conf"); got != want {
			t.Errorf("%s: got %v, want %v", f, got, want)
		}
	}
	if !gitlabDefinitionFile("ci/pipeline.conf", "/ci/pipeline.conf") {
		t.Error("ci_config_path with a leading / not matched")
	}
	if gitlabDefinitionFile("ci/pipeline.conf", "") {
		t.Error("ci/pipeline.conf is no definition file without ci_config_path")
	}
}

func TestDefinitionChanged(t *testing.T) {
	var compare, config string
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("authorization %q", r.Header.Get("Authorization"))
		}
		switch r.URL.EscapedPath() {
		case "/projects/g%2Fp":
			fmt.Fprintf(w, `{"default_branch":"main","ci_config_path":%q}`, config)
		case "/projects/g%2Fp/repository/compare":
			if q := r.URL.Query(); q.Get("from") != "main" || q.Get("to") != "abc" || q.Get("straight") != "false" {
				t.Errorf("query %s", r.URL.RawQuery)
			}
			w.WriteHeader(status)
			io.WriteString(w, compare)
		default:
			t.Errorf("path %s", r.URL.EscapedPath())
		}
	}))
	defer srv.Close()
	many := `{"diffs":[` + strings.Repeat(`{"old_path":"a","new_path":"a"},`, maxCompareFiles-1) + `{"old_path":"a","new_path":"a"}]}`
	for _, c := range []struct {
		name, compare, config string
		status                int
		file                  string
		err                   bool
	}{
		{"code only", `{"diffs":[{"old_path":"main.go","new_path":"main.go"}]}`, "", 200, "", false},
		{"ci file", `{"diffs":[{"old_path":"main.go","new_path":"main.go"},{"old_path":".gitlab-ci.yml","new_path":".gitlab-ci.yml"}]}`, "", 200, ".gitlab-ci.yml", false},
		{"renamed away", `{"diffs":[{"old_path":"ci.yml","new_path":"ci.txt"}]}`, "", 200, "ci.yml", false},
		{"config path", `{"diffs":[{"old_path":"build/ci","new_path":"build/ci"}]}`, "build/ci", 200, "build/ci", false},
		{"timeout", `{"compare_timeout":true,"diffs":[]}`, "", 200, "", true},
		{"too many", many, "", 200, "", true},
		{"refused", `{}`, "", 403, "", true},
		{"no json", `<html>`, "", 200, "", true},
	} {
		compare, config, status = c.compare, c.config, c.status
		file, err := gitlabDefinitionChanged(srv.URL, "tok", "g/p", "abc")
		if file != c.file || (err != nil) != c.err {
			t.Errorf("%s: got %q, %v", c.name, file, err)
		}
	}
}

func TestCheckedPushParts(t *testing.T) {
	if got := hex.EncodeToString(emptyPack()); got != "5041434b0000000200000000029d08823bd8a8eab510ad6ac75c823cfd3ed31e" {
		t.Errorf("empty pack %s", got)
	}
	ref := "refs/heads/zero-check/x"
	for in, ok := range map[string]bool{
		pktLine("unpack ok\n") + pktLine("ok "+ref+"\n") + "0000":                                    true,
		pktLine("unpack ok\n") + pktLine("ng "+ref+" pre-receive hook\n") + "0000":                   false,
		pktLine("unpack index-pack failed\n") + "0000":                                               false,
		pktLine("unpack ok\n") + pktLine("ok refs/heads/other\n") + "0000":                           false,
		pktLine("unpack ok\n") + pktLine("ok refs/heads/other\n") + pktLine("ok "+ref+"\n") + "0000": true,
		pktLine("ok "+ref+"\n") + pktLine("unpack ok\n") + "0000":                                    false,
		"": false,
	} {
		if err := readReport(bufio.NewReader(strings.NewReader(in)), ref); (err == nil) != ok {
			t.Errorf("%q: %v", in, err)
		}
	}
	resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("0000"))}
	gateNote("report-status side-band-64k", "hello")(resp)
	if b, _ := io.ReadAll(resp.Body); string(b) != pktLine("\x02hello\n")+"0000" {
		t.Errorf("note %q", b)
	}
	resp = &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("0000"))}
	gateNote("report-status", "hello")(resp)
	if b, _ := io.ReadAll(resp.Body); string(b) != "0000" {
		t.Errorf("note without side band %q", b)
	}
}

// A token minted by a setup that stopped before its id was written is found
// by its label and revoked (#138).
func TestRevokeForgeByLabel(t *testing.T) {
	bin, state := t.TempDir(), t.TempDir()
	calls := filepath.Join(bin, "calls")
	list := fmt.Sprintf(`[{"id":77,"name":%q,"expires_at":"2099-01-01"},{"id":78,"name":"other@host","expires_at":"2099-01-01"}]`, tokenLabel("zero-x"))
	glab := "#!/bin/sh\necho \"$*\" >> " + calls + "\ncase \"$*\" in *paginate*) cat <<'J'\n" + list + "\nJ\n;; esac\n"
	for name, body := range map[string]string{"glab": glab, "openshell": "#!/bin/sh\n"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	s := &Sandbox{Name: "zero-x", State: state}

	s.write("forge.id", "5", 0o600)
	if err := s.revokeForge(); err == nil {
		t.Error("token id without a recorded forge: no error")
	}
	if !core.FileExists(s.path("forge.id")) {
		t.Error("state dropped although nothing was revoked")
	}

	os.Remove(s.path("forge.id"))
	s.write("forge", "gitlab", 0o600)
	if err := s.revokeForge(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(calls); strings.Contains(string(b), "personal_access_tokens") {
		t.Errorf("no mint was open, yet the forge was asked:\n%s", b)
	}
	s.write("forge.minting", "", 0o600)
	if err := s.pauseForge(); err != nil {
		t.Fatal(err)
	}
	if core.FileExists(s.path("forge.minting")) {
		t.Error("marker kept after the revoke")
	}
	b, _ := os.ReadFile(calls)
	if got := string(b); !strings.Contains(got, "/personal_access_tokens/77") || strings.Contains(got, "/personal_access_tokens/78") {
		t.Errorf("glab calls:\n%s", got)
	}
}

// A workspace whose repositories are gone keeps no token: zero up revokes it,
// and without forge.id upkeep has nothing to renew (#147).
func TestSyncForgePathsRevokes(t *testing.T) {
	bin, state := t.TempDir(), t.TempDir()
	calls := filepath.Join(bin, "calls")
	if err := os.WriteFile(filepath.Join(bin, "glab"), []byte("#!/bin/sh\necho \"$*\" >> "+calls+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	s := &Sandbox{Name: "zero-x", State: state}
	s.write("forge", "gitlab", 0o600)
	s.write("forge.host", "gitlab.example", 0o600)
	s.write("forge.path", "g/p", 0o600)
	s.write("forge.id", "9", 0o600)

	_, err := s.syncForgePaths()
	if err == nil || !strings.Contains(err.Error(), "its token is revoked") || !strings.Contains(err.Error(), "was g/p, now none") {
		t.Fatalf("err = %v", err)
	}
	if core.FileExists(s.path("forge.id")) {
		t.Error("forge.id kept: zero-cp would renew the token")
	}
	if b, _ := os.ReadFile(calls); !strings.Contains(string(b), "/personal_access_tokens/9") {
		t.Errorf("glab calls:\n%s", b)
	}
	if err := s.renewForge([]string{"git.read"}, false); err != nil {
		t.Errorf("upkeep renew without a token: %v", err)
	}
	if b, _ := os.ReadFile(calls); strings.Count(string(b), "\n") != 1 {
		t.Errorf("upkeep asked the forge:\n%s", b)
	}

	// The revoke fails: forge.id goes all the same, so nothing renews the token.
	os.WriteFile(filepath.Join(bin, "glab"), []byte("#!/bin/sh\nexit 1\n"), 0o755)
	s.write("forge.id", "10", 0o600)
	if _, err := s.syncForgePaths(); err == nil || !strings.Contains(err.Error(), "could not be revoked") {
		t.Fatalf("err = %v", err)
	}
	if core.FileExists(s.path("forge.id")) {
		t.Error("forge.id kept after a failed revoke")
	}

	// Forge not recorded: the message does not claim a revoke.
	os.Remove(s.path("forge"))
	s.write("forge.id", "11", 0o600)
	if _, err := s.syncForgePaths(); err == nil || !strings.Contains(err.Error(), "token 11 is not revoked") {
		t.Fatalf("err = %v", err)
	}
}

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"main\n\tahead 2":                      "main\n\tahead 2",
		"grüße 日本 ✓":                           "grüße 日本 ✓",
		"a\x1b[2J\x1b[Hb":                      "a[2J[Hb",                    // CSI: clear screen, home
		"a\x1b]0;title\x07b":                   "a]0;titleb",                 // OSC: window title
		"a\u009b2Jb\u009d0;t\u009cc":           "a2Jb0;tc",                   // C1 forms of CSI, OSC, ST
		"line\rdelete zero-api? [y/N]\x08\x7f": "linedelete zero-api? [y/N]", // carriage return, backspace, DEL
		"a\x00b":                               "ab",
		"é\xc2":                                "é",                 // cut-off character
		"a\x9b2J\xffb\xc0\x9bc":                "a2Jbc",             // bytes that are not UTF-8
		"zero-api\u202eipa-orez\u2066x\u2069":  "zero-apiipa-orezx", // writing direction
	} {
		if got := core.Clean(in); got != want {
			t.Errorf("clean(%q) = %q, want %q", in, got, want)
		}
	}
	// a C1 character split across two writes is removed all the same
	var b bytes.Buffer
	w := &core.CleanWriter{W: &b}
	for _, p := range []string{"a\xc2", "\x9b31mb\xc2", "\xa9\x1b"} {
		if n, err := w.Write([]byte(p)); n != len(p) || err != nil {
			t.Fatalf("Write(%q) = %d, %v", p, n, err)
		}
	}
	if b.String() != "a31mb©" {
		t.Errorf("cleanWriter = %q", b.String())
	}
}

func TestApprovePolicy(t *testing.T) {
	pol := func(hosts []string, caps ...string) *Policy {
		p := &Policy{Capabilities: append(slices.Clone(defaultCaps), caps...)}
		p.Network.Allow = hosts
		return p
	}
	for _, c := range []struct {
		name     string
		approved *Policy // nil: nothing approved yet
		damaged  bool
		p        *Policy
		approve  bool
		err      string
		want     policyChange
	}{
		{name: "default needs nothing", p: pol(nil)},
		{name: "wider without a terminal is refused", p: pol([]string{"a.example"}), err: "no terminal to ask at"},
		{name: "wider capability without a terminal is refused", p: pol(nil, "ci.run"), err: "no terminal to ask at"},
		{name: "wider with --approve", p: pol([]string{"a.example"}, "ci.run"), approve: true,
			want: policyChange{caps: true, newHosts: []string{"a.example"}}},
		{name: "narrower needs nothing", approved: pol([]string{"a.example", "b.example"}, "ci.run"), p: pol([]string{"a.example"}),
			want: policyChange{caps: true, goneHosts: true}},
		{name: "host swapped is wider", approved: pol([]string{"a.example"}), p: pol([]string{"b.example"}), err: "not approved"},
		{name: "same as approved", approved: pol([]string{"a.example"}), p: pol([]string{"a.example"})},
		{name: "damaged record is refused", damaged: true, p: pol(nil), approve: true, err: "policy.approved"},
	} {
		s := &Sandbox{Name: "zero-x", State: t.TempDir(), Approve: c.approve}
		if c.approved != nil {
			if err := s.saveApproved(c.approved); err != nil {
				t.Fatal(err)
			}
		}
		if c.damaged {
			s.write("policy.approved", "capabilities: [\n", 0o600)
		}
		ch, err := s.approvePolicy(c.p)
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%s: err = %v, want %q", c.name, err, c.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
		} else if !reflect.DeepEqual(ch, c.want) {
			t.Errorf("%s: change = %+v, want %+v", c.name, ch, c.want)
		}
	}
}

func TestProjectPolicy(t *testing.T) {
	base := []byte("version: 1\nnetwork_policies:\n  zen:\n    name: zen\n  project:\n    name: project\n    endpoints:\n      - host: old.example\n")
	rule := projectPolicy([]string{"a.example", "b.example"})
	eps := rule["endpoints"].([]map[string]any)
	if len(eps) != 2 || fmt.Sprint(rule["binaries"]) != "[map[path:/**]]" {
		t.Fatalf("rule = %v", rule)
	}
	for _, e := range eps {
		if e["port"] != 443 || e["protocol"] != "rest" || e["enforcement"] != "enforce" || e["access"] != "read-only" || len(e) != 5 {
			t.Errorf("endpoint = %v", e)
		}
	}
	b, err := openshell.PolicyWith(base, projectRule, projectPolicy([]string{"a.example"}))
	if err != nil {
		t.Fatal(err)
	}
	if out := string(b); strings.Contains(out, "old.example") || !strings.Contains(out, "host: a.example") ||
		!strings.Contains(out, "access: read-only") || !strings.Contains(out, "zen") {
		t.Errorf("rule not replaced:\n%s", out)
	}
	b, err = openshell.PolicyWith(base, projectRule, projectPolicy(nil))
	if err != nil {
		t.Fatal(err)
	}
	if out := string(b); strings.Contains(out, "project") || !strings.Contains(out, "zen") {
		t.Errorf("rule not removed:\n%s", out)
	}
}

func TestCtxRemoveMissingFolder(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	root, err := realPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, d := range []string{a, b} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := Ctx(&out, "", []string{"new", "w", a, b}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(b); err != nil {
		t.Fatal(err)
	}
	if err := Ctx(&out, "w", []string{"remove", b}); err != nil {
		t.Fatalf("remove of a directory that is gone: %v", err)
	}
	c, err := loadConfig()
	if err != nil || !slices.Equal(c.Workspaces["w"].Folders, []string{a}) {
		t.Fatalf("directories = %v, %v", c.Workspaces["w"], err)
	}
	err = Ctx(&out, "w", []string{"remove", filepath.Join(root, "c")})
	if err == nil || !strings.Contains(err.Error(), "zero -c w ctx remove PATH") {
		t.Errorf("unknown directory: %v", err)
	}
	// A directory that was replaced by a link to another directory of the
	// workspace is taken out itself, not the directory it links to.
	if err := Ctx(&out, "w", []string{"add", b2(t, root)}); err != nil {
		t.Fatal(err)
	}
	d := filepath.Join(root, "d")
	if err := os.RemoveAll(d); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a, d); err != nil {
		t.Fatal(err)
	}
	if err := Ctx(&out, "w", []string{"remove", d}); err != nil {
		t.Fatal(err)
	}
	if c, _ := loadConfig(); !slices.Equal(c.Workspaces["w"].Folders, []string{a}) {
		t.Errorf("after removing the linked directory: %v", c.Workspaces["w"].Folders)
	}
}

func b2(t *testing.T, root string) string {
	d := filepath.Join(root, "d")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDoctorLines(t *testing.T) {
	for remote, want := range map[string]string{
		"":                                "has no origin",
		"/srv/git/x.git":                  "is not on GitLab",
		"git@github.com:Breee/z3r0.git\n": "github.com:Breee/z3r0.git is not on GitLab",
		"https://git.example.org/a/b.git": "glab auth login --hostname git.example.org",
		"\x1b[2Jgit@github.com:a/b.git":   "",
	} {
		got := noForgeHere(remote)
		if !strings.Contains(got, want) || strings.ContainsRune(got, 0x1b) || !strings.Contains(got, "cannot push") {
			t.Errorf("noForgeHere(%q) = %q", remote, got)
		}
	}
	export := `[{"integrationID":"github-copilot","active":true,"value":{"type":"oauth","access":"tok"}}]`
	if copilotToken([]byte(export)) != "tok" || copilotToken([]byte(`[]`)) != "" || copilotToken([]byte(`x`)) != "" {
		t.Error("copilotToken")
	}
	// The check names the fix for each way the login can be missing.
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if p := modelLogin(); !strings.Contains(p, "not on PATH") {
		t.Errorf("no opencode: %q", p)
	}
	fake := func(script string) {
		if err := os.WriteFile(filepath.Join(bin, "opencode"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	fake("echo '[]'")
	if p := modelLogin(); !strings.Contains(p, "opencode auth login") {
		t.Errorf("no login: %q", p)
	}
	fake("echo secret; echo secret >&2; exit 1")
	if p := modelLogin(); strings.Contains(p, "secret") || !strings.Contains(p, "opencode auth list") {
		t.Errorf("export fails: %q", p)
	}
	fake("echo '" + export + "'")
	if p := modelLogin(); p != "" {
		t.Errorf("login present: %q", p)
	}
}

func TestPolicyState(t *testing.T) {
	top := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", top).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	s := &Sandbox{Name: "zero-x", State: t.TempDir(), Folders: []string{top}}
	if got := s.policyState(); got != "ok" {
		t.Errorf("no file, nothing approved: got %s, want ok", got)
	}
	os.MkdirAll(filepath.Join(top, ".zero"), 0o755)
	file := filepath.Join(top, ".zero", "policy.yaml")
	os.WriteFile(file, []byte("network:\n  allow: [a.example]\n"), 0o644)
	if got := s.policyState(); got != "changed" {
		t.Errorf("host not approved: got %s, want changed", got)
	}
	p, err := s.loadPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.saveApproved(p); err != nil {
		t.Fatal(err)
	}
	if got := s.policyState(); got != "ok" {
		t.Errorf("approved: got %s, want ok", got)
	}
	os.WriteFile(file, []byte("nonsense: 1\n"), 0o644)
	if got := s.policyState(); got != "changed" {
		t.Errorf("file that cannot be read: got %s, want changed", got)
	}
}

func TestRmLeftover(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	bin := filepath.Join(dir, "bin")
	os.MkdirAll(bin, 0o755)
	os.WriteFile(filepath.Join(bin, "openshell"), []byte("#!/bin/sh\nexit 1\n"), 0o755)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	var out strings.Builder
	if err := Ctx(&out, "", []string{"rm", "-y", "gone"}); err == nil || !strings.Contains(err.Error(), "nothing left") {
		t.Errorf("nothing there: %v", err)
	}
	if err := Ctx(&out, "", []string{"rm", "-y", "../x"}); err == nil {
		t.Error("name with a path accepted")
	}
	state := filepath.Join(core.StateRoot(), "zero-gone")
	os.MkdirAll(state, 0o700)
	if err := Ctx(&out, "", []string{"rm", "-y", "gone"}); err != nil {
		t.Fatal(err)
	}
	if core.FileExists(state) || !strings.Contains(out.String(), "removed sandbox zero-gone") {
		t.Errorf("state left or no message: %q", out.String())
	}
}

func TestHasSession(t *testing.T) {
	for in, want := range map[string]bool{
		`{"data":[],"cursor":{"previous":null,"next":null}}`: false,
		`{"data":[{"id":"ses_1"}]}`:                          true,
	} {
		if got, err := hasSession(in); err != nil || got != want {
			t.Errorf("hasSession(%s) = %v, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "Unauthorized", `{"error":"x"}`, `{"data":null}`} {
		if _, err := hasSession(in); err == nil {
			t.Errorf("hasSession(%q): no error", in)
		}
	}
}

func TestGuidance(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "review.md"), []byte("be brief"), 0o644)
	os.MkdirAll(filepath.Join(d, "go-review"), 0o755)
	os.WriteFile(filepath.Join(d, "go-review", "SKILL.md"), []byte("---\ndescription: x\n---\n"), 0o644)
	os.MkdirAll(filepath.Join(d, "empty"), 0o755)
	os.MkdirAll(filepath.Join(d, "other"), 0o755)
	os.WriteFile(filepath.Join(d, "other", "review.md"), []byte("x"), 0o644)
	p := func(n string) string { return filepath.Join(d, n) }

	instr, skills, err := guidance(&Workspace{Instructions: []string{p("review.md")}, Skills: []string{p("go-review")}})
	if err != nil || len(instr) != 1 || len(skills) != 1 {
		t.Fatalf("guidance = %v %v %v", instr, skills, err)
	}
	for _, c := range []struct {
		w    Workspace
		want string
	}{
		{Workspace{Instructions: []string{p("gone.md")}}, "not found"},
		{Workspace{Instructions: []string{p("go-review")}}, "not a file"},
		{Workspace{Instructions: []string{p("review.md"), p("other/review.md")}}, "listed twice"},
		{Workspace{Skills: []string{p("empty")}}, "SKILL.md"},
		{Workspace{Skills: []string{p("review.md")}}, "SKILL.md"},
	} {
		if _, _, err := guidance(&c.w); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("guidance(%v) = %v, want %q", c.w, err, c.want)
		}
	}

	if got := guidanceConfig(nil, nil, nil, "", false); got != "" {
		t.Errorf("no guidance: config %q", got)
	}
	got := guidanceConfig(instr, skills, nil, "", false)
	for _, want := range []string{`"/tmp/zero-input/.zero/instructions/review.md"`, `"/tmp/zero-input/.zero/skills"`, `"$schema"`} {
		if !strings.Contains(got, want) {
			t.Errorf("config lacks %s:\n%s", want, got)
		}
	}
	if strings.Contains(guidanceConfig(instr, nil, nil, "", false), "skills") {
		t.Error("config names skills without a skill")
	}

	// Staging: copied, then dropped when the entries leave the workspace.
	old := inputRoot
	inputRoot = t.TempDir()
	defer func() { inputRoot = old }()
	oldRT := rt
	rt = noSandbox{rt}
	defer func() { rt = oldRT }()
	s := &Sandbox{Name: "zero-g", State: t.TempDir(), Instructions: instr, Skills: skills}
	var out bytes.Buffer
	if err := s.syncInputs(&out); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{".zero/instructions/review.md", ".zero/skills/go-review/SKILL.md"} {
		if !core.FileExists(filepath.Join(s.stage(), f)) {
			t.Errorf("%s not staged", f)
		}
	}
	if !strings.Contains(out.String(), "instructions: review.md, read-only") || !strings.Contains(out.String(), "skills: go-review, read-only") {
		t.Errorf("output: %s", out.String())
	}
	s.Instructions, s.Skills = nil, nil
	if err := s.syncInputs(&out); err != nil {
		t.Fatal(err)
	}
	if core.FileExists(filepath.Join(s.stage(), ".zero")) {
		t.Error(".zero kept after the entries left")
	}
	s.Instructions, s.MaxInput = instr, 1
	if err := s.syncInputs(&out); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("limit: %v", err)
	}
}

// noSandbox is a runtime in which no sandbox exists.
type noSandbox struct{ core.Runtime }

func (noSandbox) Exists(string) bool { return false }

func TestCopilotScript(t *testing.T) {
	got := copilotScript("/tmp/zero/a b", []string{"-p", "it's; rm -rf /"})
	want := `cd '/tmp/zero/a b' && COPILOT_AUTO_UPDATE=false COPILOT_GITHUB_TOKEN="$COPILOT_TOKEN" exec /usr/local/bin/copilot --no-auto-update '-p' 'it'\''s; rm -rf /'`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestModelRoutes(t *testing.T) {
	p := fmt.Sprint(modelPolicy("zero-a-models"))
	for _, want := range []string{"path:/v1/models", "path:/v1/chat/completions", "path:/v1/messages", "path:/v1/responses"} {
		if !strings.Contains(p, want) {
			t.Errorf("models rule lacks %s: %s", want, p)
		}
	}
	for _, not := range []string{"access:", "/key/", "/user", "/credentials", "/v1/mcp", "**"} {
		if strings.Contains(p, not) {
			t.Errorf("models rule holds %q: %s", not, p)
		}
	}
}

func TestCopilotProxyScript(t *testing.T) {
	got := copilotProxyScript("/tmp/zero/a", []string{"--model", "github_copilot/x", "-p", "it's"})
	for _, w := range []string{
		`cd '/tmp/zero/a' && [ -n "$LITELLM_API_KEY" ] || `,
		"COPILOT_OFFLINE=true COPILOT_PROVIDER_BASE_URL=http://host.openshell.internal:18096/v1 COPILOT_PROVIDER_API_KEY=\"$LITELLM_API_KEY\" exec /usr/local/bin/copilot --no-auto-update '--model' 'github_copilot/x' '-p' 'it'\\''s'",
	} {
		if !strings.Contains(got, w) {
			t.Errorf("script lacks %q:\n%s", w, got)
		}
	}
	if strings.Contains(got, "COPILOT_GITHUB_TOKEN") {
		t.Errorf("script passes a GitHub login: %s", got)
	}
	for args, want := range map[string]bool{"--model x": true, "--model=x": true, "-p hi": false, "--model": false, "--model=": false} {
		if got := hasModelArg(strings.Fields(args)); got != want {
			t.Errorf("hasModelArg(%q) = %v", args, got)
		}
	}
	pol := fmt.Sprint(modelPolicy("zero-a-models"))
	if !strings.Contains(pol, copilotBin) {
		t.Error("models rule does not name the Copilot CLI")
	}
	if !strings.Contains(pol, "credential_binding:map[provider:zero-a-models]") {
		t.Errorf("models rule does not bind the key: %s", pol)
	}
	if strings.Contains(got, "service.json") {
		t.Errorf("script reads the service settings: %s", got)
	}
}

func TestCheckModels(t *testing.T) {
	ws := func(m ...string) map[string]*Workspace {
		return map[string]*Workspace{"a": {Folders: []string{"/x"}, Models: m}}
	}
	ok := []Config{
		{Workspaces: ws()},
		{Parts: Parts{Models: "litellm"}, Providers: []string{"github_copilot"}, Workspaces: ws()},
		{Parts: Parts{Models: "litellm"}, Providers: []string{"github_copilot"}, Workspaces: ws("github_copilot/*", "github_copilot/claude-haiku-4.5")},
	}
	for i, c := range ok {
		if err := c.checkModels(); err != nil {
			t.Errorf("ok %d: %v", i, err)
		}
	}
	bad := map[string]Config{
		"unknown value":    {Parts: Parts{Models: "other"}},
		"unknown provider": {Parts: Parts{Models: "litellm"}, Providers: []string{"nope"}},
		"needs parts":      {Providers: []string{"github_copilot"}},
		"needs providers":  {Parts: Parts{Models: "litellm"}},
		"models needs":     {Workspaces: ws("github_copilot/*")},
		"want PROVIDER":    {Parts: Parts{Models: "litellm"}, Providers: []string{"github_copilot"}, Workspaces: ws("openai/gpt")},
		"not usable":       {Parts: Parts{Models: "litellm"}, Providers: []string{"github_copilot"}, Workspaces: ws("github_copilot/a b")},
		"listed twice":     {Parts: Parts{Models: "litellm"}, Providers: []string{"github_copilot"}, Workspaces: ws("github_copilot/*", "github_copilot/*")},
	}
	for want, c := range bad {
		if err := c.checkModels(); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v", want, err)
		}
	}
}

func TestModelProxyParts(t *testing.T) {
	cfg := modelConfig([]string{"github_copilot"}, nil)
	for _, want := range []string{`model_name: "github_copilot/*"`, "per_user_oauth", "credential_name: github-copilot", "require_key_mcp_access_defined: true", "master_key: os.environ/LITELLM_MASTER_KEY"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config lacks %q", want)
		}
	}
	var c map[string]any
	if err := yaml.Unmarshal([]byte(cfg), &c); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(partsCompose, &c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(partsCompose), `"127.0.0.1:18096:4000"`) || strings.Count(string(partsCompose), "@sha256:") != 2 {
		t.Error("compose: loopback port and two images by digest wanted")
	}
	req := keyRequest("zero-a@h", []string{"github_copilot/*"}, "", nil)
	b, _ := json.Marshal(req)
	for _, want := range []string{`"no-mcp-servers"`, `"user_id":"default_user_id"`, `"models":["github_copilot/*"]`, `"key_alias":"zero-a@h"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("key request lacks %s: %s", want, b)
		}
	}
	ids, err := proxyModels([]byte(`{"data":[{"id":"github_copilot/*"},{"id":"github_copilot/gpt-5","max_input_tokens":400000,"max_output_tokens":128000},{"id":"github_copilot/emb","mode":"embedding"},{"id":"github_copilot/img","mode":"image_generation"},{"id":"github_copilot/half","mode":"chat","max_input_tokens":1000},{"id":"github_copilot/claude-haiku-4.5"}]}`))
	if err != nil || len(ids) != 3 || ids[2].ID != "github_copilot/half" || ids[0].ID != "github_copilot/claude-haiku-4.5" || ids[1].In != 400000 {
		t.Errorf("models: %v %v", ids, err)
	}
	if _, err := proxyModels([]byte("nope")); err == nil {
		t.Error("unreadable list accepted")
	}
	got := guidanceConfig(nil, nil, ids, "", false)
	if strings.Contains(got, `"context": 1000`) || strings.Contains(got, "emb") || strings.Contains(got, "img") {
		t.Errorf("opencode config has a half limit or a model that cannot chat: %s", got)
	}
	if ms, err := proxyModels([]byte(`{"data":[]}`)); err != nil || len(ms) != 0 {
		t.Errorf("empty list: %v %v", ms, err)
	}
	for _, want := range []string{`"litellm"`, `"baseURL": "http://host.openshell.internal:18096/v1"`, `"apiKey": "{env:LITELLM_API_KEY}"`, `"cacheFormat": "anthropic"`, `"context": 400000`, `"output": 128000`} {
		if !strings.Contains(got, want) {
			t.Errorf("opencode config lacks %s", want)
		}
	}
	if n := notConnected([]byte(`{"connections":[{"credential_name":"github-copilot","connected":false},{"credential_name":"x","connected":true}]}`)); len(n) != 1 || n[0] != "github-copilot" {
		t.Errorf("not connected: %v", n)
	}
	if strings.Contains(got, "sk-") {
		t.Error("key in the opencode config")
	}
}

func TestCompose(t *testing.T) {
	t.Setenv("ZERO_COMPOSE", "")
	if got := strings.Join(compose(), " "); got != "docker compose" {
		t.Errorf("default: %q", got)
	}
	t.Setenv("ZERO_COMPOSE", "podman compose")
	if got := strings.Join(compose(), " "); got != "podman compose" {
		t.Errorf("override: %q", got)
	}
	t.Setenv("ZERO_COMPOSE", "true")
	if got := composeProblem(); got != "" {
		t.Errorf("command that runs: %q", got)
	}
	t.Setenv("ZERO_COMPOSE", "false")
	if got := composeProblem(); !strings.Contains(got, "false does not run") || !strings.Contains(got, "ZERO_COMPOSE") {
		t.Errorf("command that fails: %q", got)
	}
}

func TestClaudeScript(t *testing.T) {
	got := claudeScript("/tmp/zero/a b", []string{"--model", "github_copilot/x", "-p", "it's"})
	for _, want := range []string{"cd '/tmp/zero/a b' &&", `[ -n "$LITELLM_API_KEY" ]`, "ANTHROPIC_BASE_URL=http://host.openshell.internal:18096 ", `ANTHROPIC_AUTH_TOKEN="$LITELLM_API_KEY"`, "DISABLE_AUTOUPDATER=1", "exec " + claudeBin + " '--model' 'github_copilot/x' '-p' 'it'\\''s'"} {
		if !strings.Contains(got, want) {
			t.Errorf("script %q lacks %q", got, want)
		}
	}
	if strings.Contains(got, "ANTHROPIC_API_KEY") || strings.Contains(got, "/v1") {
		t.Errorf("script %q: x-api-key form or /v1 in the address", got)
	}
	if !strings.Contains(fmt.Sprint(modelPolicy("p")), claudeBin) {
		t.Error("models rule does not name Claude Code")
	}
}

func TestModelArg(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--model", "a/b", "-p", "x"}, "a/b"},
		{[]string{"-p", "x", "--model=a/b"}, "a/b"},
		{[]string{"--model", "-p", "x"}, ""},
		{[]string{"--model", ""}, ""},
		{[]string{"--model="}, ""},
		{[]string{"--model"}, ""},
		{[]string{"-p", "x"}, ""},
		{[]string{"-p", "x", "--", "--model", "a/b"}, ""},
		{[]string{"--model", "a/b", "--", "--model", "c/d"}, "a/b"},
	} {
		if got := modelArg(c.args); got != c.want {
			t.Errorf("modelArg(%q) = %q, want %q", c.args, got, c.want)
		}
	}
	s := claudeScript("/tmp/zero/a", []string{"--model", "a/b", "-p", "x"})
	for _, w := range []string{"ANTHROPIC_MODEL='a/b'", "ANTHROPIC_DEFAULT_HAIKU_MODEL='a/b'", "DISABLE_UPDATES=1", "CLAUDE_CODE_DISABLE_OFFICIAL_MARKETPLACE_AUTOINSTALL=1"} {
		if !strings.Contains(s, w) {
			t.Errorf("claude script lacks %s: %s", w, s)
		}
	}
	if err := ClaudeNeedsTerminal(nil); err == nil {
		t.Error("zero claude without arguments and without a terminal is not refused")
	}
}

func TestUI(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	get := func(method, host, path string, h map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://"+host+path, nil)
		r.Host = host
		for k, v := range h {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		uiHandler(w, r)
		return w
	}
	want := func(what string, w *httptest.ResponseRecorder, code int) {
		t.Helper()
		if w.Code != code {
			t.Fatalf("%s: got %d, want %d (%s)", what, w.Code, code, w.Body.String())
		}
	}
	want("no session", get("GET", uiAddr(), "/api/workspaces", nil), 401)
	want("no session, page", get("GET", uiAddr(), "/ui/", nil), 401)
	want("other host name", get("GET", "host.openshell.internal:18097", "/ui/", nil), 403)
	want("foreign origin", get("GET", uiAddr(), "/ui/", map[string]string{"Origin": "http://evil.example"}), 403)
	want("write", get("POST", uiAddr(), "/api/workspaces", nil), 405)
	want("no code", get("GET", uiAddr(), "/ui/enter?code=", nil), 401)

	code := filepath.Join(core.StateRoot(), uiCodeFile)
	if err := os.MkdirAll(core.StateRoot(), 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(code, []byte("abc"), 0o600)
	want("wrong code", get("GET", uiAddr(), "/ui/enter?code=abd", nil), 401)
	if core.FileExists(code) {
		t.Fatal("a wrong code must use the link up")
	}
	os.WriteFile(code, []byte("abc"), 0o600)
	old := time.Now().Add(-uiCodeLife - time.Minute)
	os.Chtimes(code, old, old)
	want("old code", get("GET", uiAddr(), "/ui/enter?code=abc", nil), 401)
	os.WriteFile(code, []byte("abc"), 0o600)
	want("probe of the link", get("HEAD", uiAddr(), "/ui/enter?code=abc", nil), 405)
	want("model proxy page, no session", get("GET", uiAddr(), "/litellm/ui/", nil), 401)
	w := get("GET", uiAddr(), "/ui/enter?code=abc", nil)
	want("code", w, 303)
	c := w.Result().Cookies()
	if len(c) != 1 || c[0].Name != uiCookie || !c[0].HttpOnly || c[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie: %+v", c)
	}
	want("code twice", get("GET", uiAddr(), "/ui/enter?code=abc", nil), 401)
	ck := map[string]string{"Cookie": uiCookie + "=" + c[0].Value}
	want("unknown path", get("GET", uiAddr(), "/api/nope", ck), 404)
	want("model proxy page, no model proxy", get("GET", uiAddr(), "/litellm/ui/", ck), 404)
	uiMu.Lock()
	uiSessions["gone"] = time.Now().Add(-time.Minute)
	uiMu.Unlock()
	want("session that ended", get("GET", uiAddr(), "/api/parts", map[string]string{"Cookie": uiCookie + "=gone"}), 401)
	want("made-up session", get("GET", uiAddr(), "/api/parts", map[string]string{"Cookie": uiCookie + "=x"}), 401)
	w = get("GET", uiAddr(), "/api/parts", ck)
	want("parts", w, 200)
	if !strings.Contains(w.Body.String(), "push gate") {
		t.Fatalf("parts: %s", w.Body.String())
	}

	// What a workspace shows: no token id, no key; text from the sandbox cleaned.
	cfg := &Config{Workspaces: map[string]*Workspace{"a": {Folders: []string{"/x/a"}, Models: []string{"github_copilot/*"}}}}
	st := filepath.Join(core.StateRoot(), "zero-a")
	os.MkdirAll(st, 0o700)
	for name, val := range map[string]string{
		"caps": "git.read git.push", "forge.id": "123456", "gate.key": "SECRETKEY", "tools": "get_project",
		"gate.log": `{"time":"1","ref":"refs/heads/zero/a","decision":"allow","reason":"first","key":"SECRETKEY"}` + "\n" +
			`{"time":"2","ref":"refs/heads/main","decision":"deny","reason":"second\u001b[2J"}` + "\n",
	} {
		os.WriteFile(filepath.Join(st, name), []byte(val), 0o600)
	}
	b, _ := json.Marshal(uiDetail(cfg, wsRow{Name: "a", Status: "running"}))
	d := uiDecisions(cfg, "a")
	bd, _ := json.Marshal(d)
	for _, bad := range []string{"SECRETKEY", "123456", "\u001b"} {
		if strings.Contains(string(b), bad) || strings.Contains(string(bd), bad) {
			t.Fatalf("%q in an answer: %s %s", bad, b, bd)
		}
	}
	if !strings.Contains(string(b), "git.push") || !strings.Contains(string(b), "get_project") || !strings.Contains(string(b), "zero/a") {
		t.Fatalf("detail: %s", b)
	}
	if len(d) != 2 || d[0]["reason"] != "second[2J" || d[1]["reason"] != "first" {
		t.Fatalf("decisions newest first, cleaned: %v", d)
	}
}

func TestHostLoginNeeded(t *testing.T) {
	proxy := &Workspace{Folders: []string{"/a"}, Models: []string{"github_copilot/*"}}
	plain := &Workspace{Folders: []string{"/b"}}
	on := Parts{Models: partLiteLLM}
	for _, tc := range []struct {
		name string
		c    *Config
		want bool
	}{
		{"no config", nil, true},
		{"no proxy", &Config{Workspaces: map[string]*Workspace{"a": plain}}, true},
		{"proxy, no workspace", &Config{Parts: on}, true},
		{"proxy, one workspace without models", &Config{Parts: on, Workspaces: map[string]*Workspace{"a": proxy, "b": plain}}, true},
		{"proxy, every workspace with models", &Config{Parts: on, Workspaces: map[string]*Workspace{"a": proxy}}, false},
	} {
		if got := hostLoginNeeded(tc.c); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestDefaultModel(t *testing.T) {
	list := []string{"github_copilot/*", "github_copilot/a"}
	for m, want := range map[string]bool{
		"github_copilot/claude-haiku-4.5": true, "github_copilot/a": true,
		"github_copilot/*": false, "other/x": false, "nomodel": false, "github_copilot/": false,
	} {
		if got := modelListed(m, list); got != want {
			t.Errorf("modelListed(%q) = %v", m, got)
		}
	}
	if modelListed("github_copilot/b", []string{"github_copilot/a"}) {
		t.Error("a model outside a list without wildcard counts")
	}
	c := &Config{Parts: Parts{Models: partLiteLLM}, Providers: []string{"github_copilot"},
		Workspaces: map[string]*Workspace{"a": {Folders: []string{"/x"}, Models: []string{"github_copilot/a"}, Model: "github_copilot/b"}}}
	if err := c.checkModels(); err == nil || !strings.Contains(err.Error(), "is not one of its models") {
		t.Errorf("default model outside the list: %v", err)
	}
	c.Workspaces["a"].Model = "github_copilot/a"
	if err := c.checkModels(); err != nil {
		t.Errorf("default model in the list: %v", err)
	}
	c.Workspaces["a"].Models = nil
	if err := c.checkModels(); err == nil {
		t.Error("default model without models accepted")
	}
	ms := []proxyModel{{ID: "github_copilot/a"}}
	if got := guidanceConfig(nil, nil, ms, "github_copilot/a", false); !strings.Contains(got, `"model": "litellm/github_copilot/a"`) {
		t.Errorf("default model not written:\n%s", got)
	}
	if got := guidanceConfig(nil, nil, ms, "", false); strings.Contains(got, `"model":`) {
		t.Errorf("model written without a default:\n%s", got)
	}
	s := &Sandbox{Model: "github_copilot/a"}
	if got := strings.Join(s.withModel([]string{"-p", "hi"}), " "); got != "--model github_copilot/a -p hi" {
		t.Errorf("withModel = %q", got)
	}
	if got := strings.Join(s.withModel([]string{"--model", "x/y", "-p", "hi"}), " "); got != "--model x/y -p hi" {
		t.Errorf("withModel with a model = %q", got)
	}
	if got := (&Sandbox{}).withModel([]string{"-p"}); len(got) != 1 {
		t.Errorf("withModel without default = %q", got)
	}
}

func TestMCPProxy(t *testing.T) {
	if got := mcpName("zero-a-b"); got != "gitlab_a_b" {
		t.Errorf("mcpName = %q", got)
	}
	if got := mcpEnvName("zero-a-b"); got != "ZERO_MCP_A_B" {
		t.Errorf("mcpEnvName = %q", got)
	}
	e := []mcpEntry{{Name: "gitlab_a", Host: "gitlab.example", Env: "ZERO_MCP_A", Token: "SECRETTOKEN"}}
	c := mcpConfig(e)
	for _, want := range []string{"mcp_servers:", "gitlab_a:", `url: "https://gitlab.example/api/v4/mcp"`, "auth_value: os.environ/ZERO_MCP_A"} {
		if !strings.Contains(c, want) {
			t.Errorf("mcpConfig lacks %q:\n%s", want, c)
		}
	}
	if strings.Contains(c, "SECRETTOKEN") {
		t.Error("token in the proxy config")
	}
	if mcpConfig(nil) != "" {
		t.Error("mcpConfig without entries is not empty")
	}
	if got := mcpEnv(e); got != "ZERO_MCP_A=SECRETTOKEN\n" {
		t.Errorf("mcpEnv = %q", got)
	}
	if !strings.Contains(modelConfig([]string{"github_copilot"}, e), "mcp_servers:") {
		t.Error("modelConfig lacks the servers")
	}
	if strings.Contains(modelConfig([]string{"github_copilot"}, nil), "mcp_servers:") {
		t.Error("modelConfig names servers without entries")
	}

	with := fmt.Sprint(keyRequest("k", []string{"m"}, "gitlab_a", []string{"get_project"})["object_permission"])
	if !strings.Contains(with, "gitlab_a") || !strings.Contains(with, "get_project") || strings.Contains(with, "no-mcp-servers") {
		t.Errorf("key with tools: %s", with)
	}
	for _, none := range []string{
		fmt.Sprint(keyRequest("k", []string{"m"}, "", nil)["object_permission"]),
		fmt.Sprint(keyRequest("k", []string{"m"}, "gitlab_a", nil)["object_permission"]),
		fmt.Sprint(keyRequest("k", []string{"m"}, "", []string{"get_project"})["object_permission"]),
	} {
		if !strings.Contains(none, "no-mcp-servers") {
			t.Errorf("key without tools: %s", none)
		}
	}

	ms := []proxyModel{{ID: "github_copilot/a"}}
	g := guidanceConfig(nil, nil, ms, "", true)
	for _, want := range []string{`"mcp"`, `"gitlab"`, "/mcp", "Bearer {env:LITELLM_API_KEY}"} {
		if !strings.Contains(g, want) {
			t.Errorf("opencode config lacks %q:\n%s", want, g)
		}
	}
	if strings.Contains(guidanceConfig(nil, nil, ms, "", false), `"mcp"`) {
		t.Error("opencode config names the server without tools")
	}

	route := false
	for _, r := range modelRoutes {
		route = route || r.Method == "POST" && r.Path == "/mcp"
	}
	if !route {
		t.Error("no POST /mcp in the model routes")
	}

	s := &Sandbox{Name: "zero-a", State: t.TempDir()}
	s.write("tools", "get_project list_commits", 0o600)
	if got := s.tokenTools(); len(got) != 2 {
		t.Errorf("tokenTools without the proxy = %v", got)
	}
	s.Models, s.Tools = []string{"github_copilot/*"}, []string{"get_project"}
	if got := s.tokenTools(); got != nil {
		t.Errorf("tokenTools with the proxy = %v", got)
	}
	if !s.proxyTools() {
		t.Error("proxyTools false with models and tools")
	}

	t.Setenv("XDG_STATE_HOME", t.TempDir())
	d := filepath.Join(core.StateRoot(), "zero-b")
	os.MkdirAll(d, 0o700)
	os.WriteFile(filepath.Join(d, "mcp.token"), []byte("T"), 0o600)
	os.WriteFile(filepath.Join(d, "forge.host"), []byte("gitlab.example"), 0o600)
	os.MkdirAll(filepath.Join(core.StateRoot(), "zero-c"), 0o700)
	got, err := mcpEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "gitlab_b" || got[0].Env != "ZERO_MCP_B" || got[0].Token != "T" || got[0].Host != "gitlab.example" {
		t.Errorf("mcpEntries = %+v", got)
	}
	// A host that would break out of its line in the proxy's config is refused.
	bad := filepath.Join(core.StateRoot(), "zero-bad")
	os.MkdirAll(bad, 0o700)
	os.WriteFile(filepath.Join(bad, "mcp.token"), []byte("tok"), 0o600)
	os.WriteFile(filepath.Join(bad, "forge.host"), []byte("a\"b"), 0o600)
	if _, err := mcpEntries(); err == nil {
		t.Error("mcpEntries accepted a host with a quote")
	}
	if (&Sandbox{State: bad}).mcpAged() == false {
		t.Error("a tool token without a date counts as fresh")
	}

}

func TestProviderKeys(t *testing.T) {
	c := &Config{Parts: Parts{Models: partLiteLLM}, Providers: []string{"openrouter"},
		Workspaces: map[string]*Workspace{"a": {Folders: []string{"/x"}, Models: []string{"openrouter/*"}}}}
	if err := c.checkModels(); err == nil || !strings.Contains(err.Error(), "provider_keys.openrouter") {
		t.Errorf("key provider without a command: %v", err)
	}
	c.ProviderKeys = map[string]string{"openrouter": "echo k"}
	if err := c.checkModels(); err != nil {
		t.Errorf("key provider with a command: %v", err)
	}
	c.ProviderKeys["github_copilot"] = "echo k"
	if err := c.checkModels(); err == nil {
		t.Error("a key command for a provider without key is accepted")
	}
	delete(c.ProviderKeys, "github_copilot")
	c.ProviderKeys["openai"] = "echo k"
	if err := c.checkModels(); err == nil {
		t.Error("a key command for a provider that is not listed is accepted")
	}

	cfg := modelConfig([]string{"github_copilot", "openrouter"}, nil)
	for _, want := range []string{`model_name: "openrouter/*"`, `api_key: os.environ/OPENROUTER_API_KEY`, `model_name: "github_copilot/*"`} {
		if !strings.Contains(cfg, want) {
			t.Errorf("proxy config lacks %q:\n%s", want, cfg)
		}
	}

	keys, err := providerKeys(map[string]string{"openrouter": "printf sk-one\\nrest"})
	if err != nil || keys["openrouter"] != "sk-one" {
		t.Errorf("key command: %v %v", keys, err)
	}
	if strings.Contains(cfg, "sk-one") {
		t.Error("key in the proxy config")
	}
	_, err = providerKeys(map[string]string{"openrouter": "false"})
	if err == nil || strings.Contains(err.Error(), "sk-") || !strings.Contains(err.Error(), "openrouter") {
		t.Errorf("failing key command: %v", err)
	}
	if _, err := providerKeys(map[string]string{"openrouter": "true"}); err == nil {
		t.Error("a command that prints nothing gives a key")
	}
	a, b := keysSum(map[string]string{"x": "1"}), keysSum(map[string]string{"x": "2"})
	if a == b || a != keysSum(map[string]string{"x": "1"}) || strings.Contains(a, "1=") {
		t.Errorf("key sums: %s %s", a, b)
	}
	// The error names the provider, never the command or what it printed.
	if _, err := providerKeys(map[string]string{"openrouter": "echo sk-inline"}); err != nil {
		t.Errorf("inline key: %v", err)
	}
	d := t.TempDir()
	script := filepath.Join(d, "k")
	os.WriteFile(script, []byte("#!/bin/sh\necho sk-printed\nexit 1\n"), 0o755)
	if _, err := providerKeys(map[string]string{"openrouter": script}); err == nil || strings.Contains(err.Error(), "sk-printed") || strings.Contains(err.Error(), script) {
		t.Errorf("failing key command leaks: %v", err)
	}
	c.ProviderKeys = map[string]string{"openrouter": "sh -c 'echo k'"}
	if err := c.checkModels(); err == nil || !strings.Contains(err.Error(), "without a shell") {
		t.Errorf("command with quotes: %v", err)
	}
	// Every key provider's variable is handed to the proxy container.
	for p, env := range keyProviders {
		if !strings.Contains(string(partsCompose), env+": ${ZERO_KEY_"+strings.ToUpper(p)+"-}") {
			t.Errorf("compose file lacks %s for %s", env, p)
		}
	}
	if providerKeysProblem(map[string]string{"x": "false"}) == "" || providerKeysProblem(map[string]string{"x": "echo k"}) != "" {
		t.Error("doctor check of the key commands")
	}
}

func TestParseComputeDriver(t *testing.T) {
	for in, want := range map[string]string{
		"Gateway Info\n  Compute drivers:\n    docker\n      Driver version: 0.1.8\n": "docker",
		"Gateway Info\n  Compute drivers:\n    podman\n":                              "podman",
		"":                                "",
		"  Extensions:\n    docker (x)\n": "",
		"  Compute drivers:\n    vm\n":    "",
	} {
		if got := openshell.ParseComputeDriver(in); got != want {
			t.Errorf("openshell.ParseComputeDriver(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStep(t *testing.T) {
	var b bytes.Buffer
	old, oldTTY := progress.out, progress.tty
	defer func() { progress.out, progress.tty = old, oldTTY }()
	sec := regexp.MustCompile(`\d+\.\ds`)

	progress.out, progress.tty = &b, false
	step("one")(nil)
	end := step("bad")
	end(fmt.Errorf("x"))
	end(nil) // a second call prints nothing
	want := "one ...\none: done (T)\nbad ...\nbad: failed (T)\n"
	if got := sec.ReplaceAllString(b.String(), "T"); got != want {
		t.Errorf("plain: got %q, want %q", got, want)
	}

	b.Reset()
	progress.tty = true
	var o bytes.Buffer
	end = step("slow")
	time.Sleep(250 * time.Millisecond) // the spinner repaints meanwhile
	fmt.Fprintln(stepWriter{&o}, "a line of the step")
	end(nil)
	step("broken")(fmt.Errorf("x"))
	progress.Lock()
	got := sec.ReplaceAllString(b.String(), "T")
	progress.Unlock()
	for _, w := range []string{"\r\x1b[K\x1b[32m✓\x1b[0m slow \x1b[2mT\x1b[0m\n", "\r\x1b[K\x1b[31m✗\x1b[0m broken \x1b[2mT\x1b[0m\n", "⠙"} {
		if !strings.Contains(got, w) {
			t.Errorf("terminal: %q not in %q", w, got)
		}
	}
	if o.String() != "a line of the step\n" {
		t.Errorf("step output: %q", o.String())
	}
}

func TestTook(t *testing.T) {
	for d, want := range map[time.Duration]string{400 * time.Millisecond: "0.4s", 12 * time.Second: "12s", 65 * time.Second: "1m05s"} {
		if got := took(time.Now().Add(-d)); got != want {
			t.Errorf("%v: got %s, want %s", d, got, want)
		}
	}
}

func TestFirstRunPolicy(t *testing.T) {
	d := t.TempDir()
	for _, f := range []string{"go.mod", "package.json"} {
		if err := os.WriteFile(filepath.Join(d, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	found := detectPresets(d)
	if len(found) != 2 || found[0].Name != "Go modules" || found[1].Name != "npm" {
		t.Fatalf("detectPresets = %v", found)
	}
	if got := detectPresets(t.TempDir()); len(got) != 0 {
		t.Fatalf("empty directory: %v", got)
	}
	p := firstRunPolicy(found, []string{"Go modules"}, defaultCaps)
	if !slices.Equal(p.Network.Allow, []string{"proxy.golang.org", "sum.golang.org"}) {
		t.Fatalf("hosts = %v", p.Network.Allow)
	}
	back, err := parsePolicy(renderPolicy(p))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(back.Network.Allow, p.Network.Allow) || !slices.Equal(back.Capabilities, p.Capabilities) {
		t.Fatalf("round trip: %+v != %+v", back, p)
	}
	if strings.Contains(string(renderPolicy(firstRunPolicy(nil, nil, defaultCaps))), "network") {
		t.Fatal("network block without hosts")
	}
	if checkChosenCaps(nil) == nil || checkChosenCaps([]string{capMRCreate}) == nil {
		t.Fatal("empty set or mr.create without ci.run accepted")
	}
	if err := checkChosenCaps([]string{"git.read", capMRCreate, capCIRun}); err != nil {
		t.Fatal(err)
	}
	for f, want := range map[string]string{"requirements.txt": "Python", "pyproject.toml": "Python", "Cargo.toml": "Rust"} {
		d := t.TempDir()
		if err := os.WriteFile(filepath.Join(d, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if got := detectPresets(d); len(got) != 1 || got[0].Name != want {
			t.Errorf("%s: %v", f, got)
		}
	}
	caps := append(slices.Clone(defaultCaps), capCIRun, capMRCreate)
	if p := firstRunPolicy(nil, nil, caps); !slices.Equal(p.Capabilities, caps) {
		t.Fatalf("caps = %v", p.Capabilities)
	}
	u := unionPolicy(&Policy{Capabilities: []string{"git.read"}}, p, p)
	if !slices.Equal(u.Network.Allow, p.Network.Allow) || len(u.Capabilities) != len(defaultCaps) {
		t.Fatalf("union = %+v", u)
	}
}

// The yes at the first run covers the policy shown, not a wider one that a
// directory of the workspace brings along unseen.
func TestFirstRunApproval(t *testing.T) {
	shown := &Policy{Capabilities: defaultCaps}
	shown.Network.Allow = []string{"proxy.golang.org"}
	s := &Sandbox{Name: "zero-t", State: t.TempDir(), shown: shown}
	if _, err := s.approvePolicy(shown); err != nil {
		t.Fatalf("the shown policy asked again: %v", err)
	}
	wider := &Policy{Capabilities: append(slices.Clone(defaultCaps), capCIRun)}
	wider.Network.Allow = []string{"proxy.golang.org", "evil.example.com"}
	if _, err := s.approvePolicy(wider); err == nil {
		t.Fatal("a policy wider than the shown one passed without approval")
	}
	s.shown = nil
	if _, err := s.approvePolicy(shown); err == nil {
		t.Fatal("widening passed without first run or approval")
	}
}

func TestHomeKeys(t *testing.T) {
	press := func(m tea.Model, keys ...rune) (home, bool) {
		var cmd tea.Cmd
		for _, k := range keys {
			msg := tea.KeyPressMsg{Code: k}
			if k >= ' ' && k < 0x7f {
				msg.Text = string(k)
			}
			m, cmd = m.Update(msg)
		}
		return m.(home), cmd != nil
	}
	plain := func(m home) string {
		return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.frame(), "")
	}
	o := &overview{c: &Config{Workspaces: map[string]*Workspace{
		"apps": {Folders: []string{"/src/apps"}},
		"dev":  {Folders: []string{"/src/one", "/src/two"}, Model: "m1"},
	}}, rows: []wsRow{
		{Name: "apps", Status: "none", Token: "-", Policy: "-", Folders: 1},
		{Name: "dev", Status: "running", Token: "ok", Policy: "default", Folders: 2, Current: true},
		{Name: "docs", Status: "stopped", Token: "-", Policy: "-", Folders: 1},
	}}
	n, _ := home{w: 100, h: 24}.Update(homeLoaded{o: o})
	m := n.(home)
	if r := m.cur(); r == nil || r.Name != "dev" {
		t.Fatalf("first load selects the current workspace: %+v", r)
	}

	for _, size := range [][2]int{{100, 24}, {80, 24}, {50, 18}} {
		m.w, m.h = size[0], size[1]
		out := plain(m)
		for _, l := range strings.Split(out, "\n") {
			if lipgloss.Width(l) > m.w {
				t.Errorf("%dx%d: line wider than the window: %q", m.w, m.h, l)
			}
		}
		for _, want := range []string{"NAME", "workspaces(all)[3]", "dev*", "█"} {
			if !strings.Contains(out, want) {
				t.Errorf("%dx%d: no %q in\n%s", m.w, m.h, want, out)
			}
		}
		if hints := strings.Contains(out, "<v>"); hints != (m.w >= 78) {
			t.Errorf("%dx%d: key hints shown = %v", m.w, m.h, hints)
		}
	}
	m.w, m.h = 15, 5
	if !strings.Contains(plain(m), "too small") {
		t.Error("small window not said")
	}
	m.w, m.h = 100, 24

	if f, _ := press(m, '/', 'd', 'o'); f.mode != homeFilter || len(f.shown()) != 1 || !strings.Contains(plain(f), "workspaces(/do)[1]") {
		t.Errorf("filter: %d rows\n%s", len(f.shown()), plain(f))
	} else if f, _ = press(f, tea.KeyEscape); len(f.shown()) != 3 {
		t.Error("esc does not clear the filter")
	}
	if c, _ := press(m, ':', 'd', 'o', 'c', 's', tea.KeyEnter); c.cur().Name != "docs" || c.mode != homeList {
		t.Errorf("command jumps to a workspace: %s", c.cur().Name)
	}
	if c, _ := press(m, ':', 'x', tea.KeyEnter); !strings.Contains(plain(c), "no command or workspace: x") {
		t.Error("unknown command not said")
	}
	if c, quit := press(m, ':', 'q', tea.KeyEnter); !quit || c.cmd != "" {
		t.Error(":q does not quit")
	}
	if d, _ := press(m, 'd'); !strings.Contains(plain(d), "describe(dev)") || !strings.Contains(plain(d), "/src/two") {
		t.Errorf("describe:\n%s", plain(d))
	}
	if d, _ := press(m, 'd'); !strings.Contains(plain(d), "Permissions") || !strings.Contains(plain(d), "git.push") {
		t.Errorf("describe without permissions:\n%s", plain(d))
	}
	if c := m.sub("switch dev", "switch", "dev"); c == nil {
		t.Error("no switch command")
	}
	if h, _ := press(m, '?'); !strings.Contains(plain(h), "help") {
		t.Error("no help")
	}

	for k, want := range map[rune]string{tea.KeyEnter: "opencode", 'o': "opencode", 'v': "vscode", 'n': "new", 'q': ""} {
		if c, quit := press(m, k); c.cmd != want || !quit {
			t.Errorf("key %q: cmd %q, quit %v", k, c.cmd, quit)
		}
	}
	if c, quit := press(m, tea.KeyEscape); c.cmd != "" || quit {
		t.Error("esc on the list quits")
	}
	for _, k := range "usrd" {
		if c, run := press(home{w: 100, h: 24, ready: true}, k); run || c.mode != homeList {
			t.Errorf("key %q with no workspace does something", k)
		}
	}
	if _, run := press(m, 's'); !run {
		t.Error("stop runs nothing")
	}

	m, _ = press(m, 'j')
	o.rows = slices.Insert(o.rows, 0, wsRow{Name: "aaa", Status: "none"})
	n, _ = m.Update(homeLoaded{o: o})
	if r := n.(home).cur(); r.Name != "docs" {
		t.Errorf("selection after a reload: %s", r.Name)
	}
	n, _ = n.(home).Update(homeRan{what: "rm docs", last: "zero: no", err: fmt.Errorf("exit status 1")})
	if !strings.Contains(plain(n.(home)), "rm docs: zero: no") {
		t.Error("failed command not shown")
	}
	// the selection stays on its workspace while the filter changes
	fm := m
	fm.filter, fm.mode = "", homeList
	fm.selectName("dev")
	if f, _ := press(fm, '/', 'd', 'o', tea.KeyBackspace); f.cur() == nil || f.cur().Name != "dev" {
		t.Errorf("selection after a filter edit: %+v", f.cur())
	}
	// the help fits the box at 80x24
	hm := m
	hm.w, hm.h = 80, 24
	if h, _ := press(hm, '?'); !strings.Contains(plain(h), "esc back") || !strings.Contains(plain(h), ":NAME") {
		t.Errorf("help cut off:\n%s", plain(h))
	}
	// commands carry no approval of their own
	if c := m.exec("dev", "rm"); c == nil {
		t.Error("no command")
	}
	src, _ := os.ReadFile("tui.go")
	if !bytes.Contains(src, []byte(`m.sub(cmd+" "+name, "-c", name, cmd)`)) || bytes.Contains(src, []byte(`"-y"`)) || bytes.Contains(src, []byte(`--approve`)) {
		t.Error("the screen must run zero -c NAME CMD and nothing more")
	}
	// long and wide names stay within the window; many rows keep the selection in sight
	big := &overview{c: &Config{Workspaces: map[string]*Workspace{}}}
	for i := 0; i < 40; i++ {
		big.rows = append(big.rows, wsRow{Name: fmt.Sprintf("ws%02d", i), Status: "none", Token: "-", Policy: "-"})
	}
	big.rows = append(big.rows, wsRow{Name: "日本語-" + strings.Repeat("x", 120), Status: "none", Token: "-", Policy: "-"})
	n, _ = home{w: 60, h: 20}.Update(homeLoaded{o: big})
	b, _ := press(n, 'G')
	for _, l := range strings.Split(plain(b), "\n") {
		if lipgloss.Width(l) > 60 {
			t.Errorf("long name: line wider than the window: %q", l)
		}
	}
	if !strings.Contains(plain(b), "日本語-") || strings.Contains(plain(b), "ws00") {
		t.Errorf("last row not scrolled into sight:\n%s", plain(b))
	}
	if d, _ := press(b, 'd'); strings.Count(strings.Split(plain(d), "\n")[6], "x") > 60 || lipgloss.Width(strings.Split(plain(d), "\n")[6]) > 60 {
		t.Errorf("describe title wider than the window")
	}
	if b, _ = press(b, 'k', 'k', 'g'); !strings.Contains(plain(b), "ws00") {
		t.Errorf("first row not shown:\n%s", plain(b))
	}
	// one load at a time: an answer that a newer request overtook is dropped
	l := home{w: 100, h: 24, loading: true}
	n, cmd := l.Update(homeRan{what: "stop dev"})
	if l = n.(home); cmd != nil || !l.again {
		t.Error("second load started while one runs")
	}
	n, cmd = l.Update(homeLoaded{o: o})
	if l = n.(home); cmd == nil || len(l.rows) != 0 || !l.loading {
		t.Errorf("stale answer used: %d rows", len(l.rows))
	}
	if n, _ = l.Update(homeLoaded{o: o}); len(n.(home).rows) != len(o.rows) || n.(home).loading {
		t.Error("fresh answer not used")
	}
}

func TestHomeFirstRun(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	press := func(m tea.Model, keys ...rune) (home, bool) {
		var cmd tea.Cmd
		for _, k := range keys {
			msg := tea.KeyPressMsg{Code: k}
			if k >= ' ' && k < 0x7f {
				msg.Text = string(k)
			}
			m, cmd = m.Update(msg)
		}
		return m.(home), cmd != nil
	}
	plain := func(m home) string {
		return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.frame(), "")
	}
	o := &overview{c: &Config{Workspaces: map[string]*Workspace{"new": {Folders: []string{repo}}}},
		rows: []wsRow{{Name: "new", Status: "none", Token: "-", Policy: "-", Folders: 1, Current: true}}}
	n, _ := home{w: 80, h: 24}.Update(homeLoaded{o: o})
	file := filepath.Join(repo, ".zero", "policy.yaml")

	m, _ := press(n, tea.KeyEnter)
	if m.mode != homeFirst || m.cmd != "" {
		t.Fatalf("enter on a new workspace did not open the first run: mode %v cmd %q", m.mode, m.cmd)
	}
	for _, size := range [][2]int{{80, 24}, {50, 18}} {
		m.w, m.h = size[0], size[1]
		for i := range m.fr.items {
			m.fr.cur = i
			out := plain(m)
			for _, l := range strings.Split(out, "\n") {
				if lipgloss.Width(l) > m.w {
					t.Errorf("%dx%d: line wider than the window: %q", m.w, m.h, l)
				}
			}
			if !strings.Contains(out, m.fr.items[i].key) {
				t.Errorf("%dx%d: option %s not visible in\n%s", m.w, m.h, m.fr.items[i].key, out)
			}
			if !strings.Contains(out, "esc back") {
				t.Errorf("%dx%d: no key line in\n%s", m.w, m.h, out)
			}
		}
	}
	m.w, m.h, m.fr.cur = 80, 24, 0
	out := plain(m)
	for _, want := range []string{"first run(new)", hostsTitle, capsTitle, "[x] Go modules: proxy.golang.org", "[x] git.push: push, only to its own branch", "[ ] ci.run:", "[ ] gitlab.mr.create:"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}

	// gitlab.mr.create alone is refused
	last, fr := m, *m.fr
	fr.items = slices.Clone(fr.items)
	fr.cur = len(fr.items) - 1
	last.fr = &fr
	e, _ := press(last, ' ', tea.KeyEnter)
	if e.fr == nil || e.fr.preview || !strings.Contains(plain(e), capMRCreate+" needs "+capCIRun) {
		t.Errorf("gitlab.mr.create without ci.run got through:\n%s", plain(e))
	}

	// esc writes nothing
	x, _ := press(m, tea.KeyEnter, tea.KeyEscape, tea.KeyEscape)
	if x.mode != homeList || core.FileExists(file) {
		t.Errorf("esc: mode %v, file written %v", x.mode, core.FileExists(file))
	}

	// space unticks the host; the preview shows the file; enter writes and starts
	p, _ := press(m, ' ', tea.KeyEnter)
	if !p.fr.preview || !strings.Contains(plain(p), file) || strings.Contains(plain(p), "proxy.golang.org") {
		t.Fatalf("preview:\n%s", plain(p))
	}
	if core.FileExists(file) {
		t.Fatal("file written before the yes")
	}
	d, quit := press(p, tea.KeyEnter)
	if d.cmd != "opencode" || !quit {
		t.Fatalf("after yes: cmd %q quit %v msg %q", d.cmd, quit, d.msg)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	pol, err := parsePolicy(b)
	if err != nil || len(pol.Network.Allow) != 0 || !slices.Equal(pol.Capabilities, defaultCaps) {
		t.Errorf("written policy: %v %+v", err, pol)
	}

	// the yes is handed to the child once, and covers only what was shown
	s := d.firstSandbox("new")
	if _, err := s.approvePolicy(pol); err != nil {
		t.Errorf("the shown policy asked again: %v", err)
	}
	if s.takeShown() != nil {
		t.Error("policy.shown was not used up")
	}
	wider := &Policy{Capabilities: defaultCaps}
	wider.Network.Allow = []string{"evil.example.com"}
	s = m.firstSandbox("new")
	screenShown[s.Name] = pol
	if _, err := s.approvePolicy(wider); err == nil {
		t.Error("a policy wider than the shown one passed")
	}
	// a child gets it through its environment, by name, once
	v, _ := shownValue(wider)
	t.Setenv(shownEnv, "zero-other\n"+v)
	if m.firstSandbox("new").takeShown() != nil || os.Getenv(shownEnv) != "" {
		t.Error("a yes for another sandbox was taken, or stayed in the environment")
	}
	t.Setenv(shownEnv, s.Name+"\n"+v)
	s = m.firstSandbox("new")
	if _, err := s.approvePolicy(wider); err != nil {
		t.Errorf("the yes from the environment was not used: %v", err)
	}
	if m.firstSandbox("new").takeShown() != nil {
		t.Error("the yes can be used twice")
	}

	// a small window: enter pages through the preview before it writes
	os.RemoveAll(filepath.Dir(file))
	sm, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 12})
	q, _ := press(sm.(home), tea.KeyEnter)
	if !strings.Contains(plain(q), "enter more") {
		t.Fatalf("cut preview does not say so:\n%s", plain(q))
	}
	for i := 0; i < 10 && !core.FileExists(file); i++ {
		if !strings.Contains(plain(q), "enter more") && !strings.Contains(plain(q), "gitlab.pipeline.read") {
			t.Errorf("enter would write without the end shown:\n%s", plain(q))
		}
		q, _ = press(q, tea.KeyEnter)
	}
	if !core.FileExists(file) {
		t.Error("paged preview never wrote")
	}

	// with the file in place the screen starts at once
	g, quit := press(n, 'v')
	if g.mode == homeFirst || g.cmd != "vscode" || !quit {
		t.Errorf("second run: mode %v cmd %q", g.mode, g.cmd)
	}
}
