package zero

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
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
