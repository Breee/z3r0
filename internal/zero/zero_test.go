package zero

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
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
	if got, want := stateRoot(), "/tmp/xdg/zero"; got != want {
		t.Errorf("stateRoot() = %q, want %q", got, want)
	}
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/u")
	if got, want := stateRoot(), "/home/u/.local/state/zero"; got != want {
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
		if got := shellQuote(in); got != want {
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
	g := gitlab{"git.example.org"}
	y := string(g.Profile())
	if g.ProfileID() != "zero-gitlab-git-example-org" || !strings.Contains(y, "id: zero-gitlab-git-example-org\n") ||
		!strings.Contains(y, "host: git.example.org\n") || strings.Contains(y, "host: gitlab.com") {
		t.Errorf("profile for other host wrong:\n%s", y)
	}
	if string(gitlab{gitlabCom}.Profile()) != string(gitlabProfileYAML) {
		t.Error("gitlab.com profile changed")
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
	if _, err := (gitlab{}).Permissions([]string{"container.build"}); err == nil {
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
	// The input folder c is no policy source.
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

func TestGateSeveralRepos(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := filepath.Join(stateRoot(), "zero-team")
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
		out, err := sh("git", append([]string{"-C", top, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
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
	if fileExists(filepath.Join(stage, ".env")) {
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
	if fileExists(filepath.Join(stage, "openapi.yaml")) {
		t.Fatal("deleted file still staged")
	}

	// Over the limit: error, the staged copy stays as it was.
	writeTree(t, specs, map[string]string{"big.bin": strings.Repeat("x", 4096)})
	s.MaxInput = 1024
	if err := s.syncInputs(&out); err == nil || !strings.Contains(err.Error(), "limit is 1 KiB") {
		t.Fatalf("limit: %v", err)
	}
	if fileExists(filepath.Join(stage, "big.bin")) {
		t.Fatal("copied despite the limit")
	}

	// A folder that left the workspace loses its copy.
	s.Folders, s.MaxInput = nil, 0
	if err := s.syncInputs(&out); err != nil {
		t.Fatal(err)
	}
	if fileExists(stage) {
		t.Fatal("copy of a removed folder kept")
	}
}

func TestUnpushedScript(t *testing.T) {
	dir := t.TempDir()
	run := func(script string) string {
		t.Helper()
		out, err := sh("sh", "-c", "cd "+shellQuote(dir)+" && "+script)
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
		if out, err := sh("git", append([]string{"-C", top, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...); err != nil {
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
