package zero

import "testing"

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

func TestGitlabPath(t *testing.T) {
	for in, want := range map[string]string{
		"git@gitlab.com:corewire/technology-and-research/developer.git": "corewire/technology-and-research/developer",
		"https://gitlab.com/a/b.git":                                    "a/b",
		"https://user:x@gitlab.com/a/b":                                 "a/b",
		"ssh://git@gitlab.com/a/b.git":                                  "a/b",
		"git@github.com:Breee/z3r0.git":                                 "",
	} {
		if got := gitlabPath(in); got != want {
			t.Errorf("gitlabPath(%q) = %q, want %q", in, got, want)
		}
	}
}
