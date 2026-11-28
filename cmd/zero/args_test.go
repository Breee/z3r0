package main

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/Breee/z3r0/internal/zero"
)

func TestCheckArgs(t *testing.T) {
	ok := [][]string{
		{"up"}, {"up", "--approve", "--vm", "--max-input", "200"},
		{"opencode"}, {"opencode", "--tui"}, {"opencode", "--web"}, {"vscode"}, {"web"}, {"status"}, {"stop"}, {"pause"},
		{"logs"}, {"logs", "-f"}, {"logs", "--follow"},
		{"rm"}, {"rm", "-y"}, {"ls"}, {"switch"}, {"switch", "api"},
		{"completion", "bash"}, {"install"}, {"install", "--podman"}, {"install", "--docker-rootless"}, {"install", "--docker"}, {"doctor"}, {"version"}, {"help"},
		{"run", "fix", "--the", "bug"}, {"copilot"}, {"copilot", "-p", "fix it", "--allow-all-tools"},
		{"claude", "--model", "a/b", "-p", "fix it"},
		{"ctx"}, {"ctx", "list"}, {"ctx", "--prompt"}, {"ctx", "use", "api"},
		{"ctx", "new", "api"}, {"ctx", "new", "api", ".", "../b"},
		{"ctx", "add"}, {"ctx", "add", "."}, {"ctx", "remove", "../b"},
		{"ctx", "rm", "api"}, {"ctx", "rm", "-y", "api"}, {"ctx", "rm", "api", "-y"},
	}
	for _, c := range ok {
		if err := checkArgs(c[0], c[1:]); err != nil {
			t.Errorf("%v: %v", c, err)
		}
	}
	bad := []struct {
		args []string
		want string
	}{
		{[]string{"up", "--bogus"}, `zero up: unknown option "--bogus" (options: --approve, --vm, --max-input N)`},
		{[]string{"opencode", "--tui", "--web"}, "--tui or --web, not both"},
		{[]string{"up", "--aprove"}, `unknown option "--aprove"`},
		{[]string{"up", "x"}, `zero up: takes no argument, got "x"`},
		{[]string{"up", "--max-input"}, "--max-input needs a value"},
		{[]string{"up", "--max-input", "big"}, "needs a number above 0"},
		{[]string{"up", "--max-input", "0"}, "needs a number above 0"},
		{[]string{"opencode", "-x"}, `zero opencode: unknown option "-x" (options: --tui, --web)`},
		{[]string{"vscode", "x"}, "takes no argument"},
		{[]string{"web", "--x"}, "unknown option"},
		{[]string{"status", "-v"}, "unknown option"},
		{[]string{"stop", "now"}, "takes no argument"},
		{[]string{"pause", "-y"}, "unknown option"},
		{[]string{"logs", "-F"}, `(options: -f, --follow)`},
		{[]string{"rm", "--yes"}, `zero rm: unknown option "--yes" (options: -y)`},
		{[]string{"rm", "api"}, "takes no argument"},
		{[]string{"ls", "x"}, "takes no argument"},
		{[]string{"ls", "-a"}, "unknown option"},
		{[]string{"switch", "a", "b"}, "usage: zero switch NAME"},
		{[]string{"switch", "-x"}, "unknown option"},
		{[]string{"completion"}, "usage: zero completion bash"},
		{[]string{"completion", "bash", "x"}, "usage: zero completion bash"},
		{[]string{"install", "-f"}, "unknown option"},
		{[]string{"doctor", "x"}, "takes no argument"},
		{[]string{"version", "-v"}, "unknown option"},
		{[]string{"help", "--bogus"}, "unknown option"},
		{[]string{"run"}, "needs a prompt"},
		{[]string{"ctx", "nope"}, `unknown subcommand "nope"`},
		{[]string{"ctx", "--prompt", "x"}, "takes no argument"},
		{[]string{"ctx", "list", "x"}, "takes no argument"},
		{[]string{"ctx", "use"}, "usage: zero ctx use NAME"},
		{[]string{"ctx", "use", "-x"}, "unknown option"},
		{[]string{"ctx", "new"}, "usage: zero ctx new NAME [PATH...]"},
		{[]string{"ctx", "new", "-f", "a"}, "unknown option"},
		{[]string{"ctx", "add", "-y"}, "unknown option"},
		{[]string{"ctx", "remove", "--all"}, "unknown option"},
		{[]string{"ctx", "rm"}, "usage: zero ctx rm [-y] NAME"},
		{[]string{"ctx", "rm", "-f", "api"}, `(options: -y)`},
	}
	for _, c := range bad {
		err := checkArgs(c.args[0], c.args[1:])
		var u usageError
		if !errors.As(err, &u) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: got %v, want %q", c.args, err, c.want)
		}
	}
	for cmd := range argSpecs {
		if strings.HasPrefix(cmd, "ctx ") {
			continue
		}
		if n := strings.Count(usageText, "\n"); n > 40 {
			t.Fatalf("usage text has %d lines, want at most 40", n)
		}
		if !strings.Contains(usageText, "\n  "+cmd) {
			t.Errorf("%s is not in the usage text", cmd)
		}
	}
}

func TestContextFlag(t *testing.T) {
	ctx, rest, err := contextFlag([]string{"-c", "api", "run", "-c", "x"})
	if err != nil || ctx != "api" || strings.Join(rest, " ") != "run -c x" {
		t.Errorf("got %q %v %v", ctx, rest, err)
	}
	ctx, rest, err = contextFlag([]string{"-c", "api", "copilot", "-c", "x"})
	if err != nil || ctx != "api" || strings.Join(rest, " ") != "copilot -c x" {
		t.Errorf("got %q %v %v", ctx, rest, err)
	}
	ctx, rest, err = contextFlag([]string{"-c", "api", "claude", "-c", "x"})
	if err != nil || ctx != "api" || strings.Join(rest, " ") != "claude -c x" {
		t.Errorf("got %q %v %v", ctx, rest, err)
	}
	ctx, rest, err = contextFlag([]string{"up", "--context=api", "--vm"})
	if err != nil || ctx != "api" || strings.Join(rest, " ") != "up --vm" {
		t.Errorf("got %q %v %v", ctx, rest, err)
	}
	for _, in := range [][]string{{"up", "-c"}, {"-c", "--vm", "up"}, {"--context=", "up"}, {"--context", "-y", "rm"}} {
		var u usageError
		if _, _, err := contextFlag(in); !errors.As(err, &u) {
			t.Errorf("%v: got %v, want a usage error", in, err)
		}
	}
}

func TestStyledHelpSameText(t *testing.T) {
	styled := zero.StyleHelp(usageText)
	if styled == usageText {
		t.Fatal("help not styled")
	}
	if got := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(styled, ""); got != usageText {
		t.Errorf("styled help differs from the plain text:\n%s", got)
	}
	if !strings.Contains(styled, "\x1b[36mopencode\x1b[0m") || !strings.Contains(styled, "\x1b[1;36mWorkspaces\x1b[0m") {
		t.Errorf("command or heading not coloured:\n%q", styled)
	}
}
