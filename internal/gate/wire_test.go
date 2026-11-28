package gate

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestCheckRefs(t *testing.T) {
	b := SessionBranch("zero-dev")
	a, n := strings.Repeat("a", 40), strings.Repeat("b", 40)
	ok := [][]Update{
		{{a, n, "refs/heads/zero/dev"}},
		{{ZeroSHA, n, "refs/heads/zero/dev/fix"}, {a, n, "refs/heads/zero/dev"}},
	}
	bad := [][]Update{
		nil,
		{{a, n, "refs/heads/main"}},
		{{a, n, "refs/heads/zero/developer"}},
		{{a, n, "refs/heads/zero/other"}},
		{{a, n, "refs/tags/v1"}},
		{{a, ZeroSHA, "refs/heads/zero/dev"}},
		{{a, n, "refs/heads/zero/dev"}, {a, n, "refs/heads/main"}},
		{{a, n, "refs/heads/zero/dev/../../main"}},
	}
	for _, u := range ok {
		if err := CheckRefs(u, b); err != nil {
			t.Errorf("%v: %v", u, err)
		}
	}
	for _, u := range bad {
		if CheckRefs(u, b) == nil {
			t.Errorf("%v allowed", u)
		}
	}
}

func TestReadCommands(t *testing.T) {
	a, n := strings.Repeat("a", 40), strings.Repeat("b", 40)
	line := a + " " + n + " refs/heads/x\x00report-status\n"
	in := fmt.Sprintf("%04x%s0000PACK", len(line)+4, line)
	ups, _, err := ReadCommands(bufio.NewReader(strings.NewReader(in)))
	if err != nil || len(ups) != 1 || ups[0].Ref != "refs/heads/x" {
		t.Fatalf("%v %v", ups, err)
	}
	if _, _, err := ReadCommands(bufio.NewReader(strings.NewReader("0005"))); err == nil {
		t.Fatal("truncated accepted")
	}
}

func TestWriteCommands(t *testing.T) {
	a, n := strings.Repeat("a", 40), strings.Repeat("b", 40)
	ups := []Update{{a, n, "refs/heads/zero/x"}}
	cmd := func(caps string) string { return PktLine(a + " " + n + " refs/heads/zero/x\x00" + caps + "\n") }
	skip := PktLine("ci.skip\n")
	for _, c := range []struct{ caps, skipCI, want string }{
		{"report-status", "n", cmd("report-status") + "0000"},
		{"report-status", "y", cmd("report-status push-options") + "0000" + skip + "0000"},
		{"report-status push-options", "y", cmd("report-status push-options") + "0000" + skip},
	} {
		if got := WriteCommands(ups, c.caps, c.skipCI == "y"); got != c.want {
			t.Errorf("caps %q skip %s:\n got %q\nwant %q", c.caps, c.skipCI, got, c.want)
		}
	}
	// what was written parses back to the same updates
	got, _, err := ReadCommands(bufio.NewReader(strings.NewReader(WriteCommands(ups, "x", true))))
	if err != nil || len(got) != 1 || got[0] != ups[0] {
		t.Fatalf("%v %v", got, err)
	}
}

func TestPushOptions(t *testing.T) {
	in := PktLine("merge_request.create\n") + PktLine("merge_request.target=main") + "0000PACK"
	br := bufio.NewReader(strings.NewReader(in))
	opts, err := ReadOptions(br)
	if err != nil || !slices.Equal(opts, []string{"merge_request.create", "merge_request.target=main"}) {
		t.Fatalf("%q %v", opts, err)
	}
	if rest, _ := io.ReadAll(br); string(rest) != "PACK" {
		t.Errorf("rest %q", rest)
	}
	if got := WriteOptions(opts); got != PktLine("merge_request.create\n")+PktLine("merge_request.target=main\n")+"0000" {
		t.Errorf("written %q", got)
	}
	if CheckOptions(opts, false) == nil {
		t.Error("merge_request.create passes without gitlab.mr.create")
	}
	if err := CheckOptions(opts, true); err != nil {
		t.Error(err)
	}
	for _, o := range []string{"ci.variable=A=b", "ci.skip", "merge_request.title=x\nci.variable=A=b", ""} {
		if CheckOptions([]string{o}, true) == nil {
			t.Errorf("option %q passes", o)
		}
	}
	for caps, ok := range map[string]bool{"report-status push-options": true, "report-status": true, "push-options=1": false, "a Push-Options": false, "push-options-x": false} {
		if (CheckCaps(caps) == nil) != ok {
			t.Errorf("checkCaps(%q) ok = %v", caps, !ok)
		}
	}
	if CheckOptions(nil, false) != nil {
		t.Error("no options refused")
	}
	for _, bad := range []string{"", "0003", "zzzz", "0009ab"} {
		if _, err := ReadOptions(bufio.NewReader(strings.NewReader(bad))); err == nil {
			t.Errorf("readOptions(%q) passes", bad)
		}
	}
}

// FuzzGateCommands feeds arbitrary bytes to the gate's parser (#100): it must
// not panic, and whatever it accepts must survive the rewrite unchanged.

// FuzzGateCommands feeds arbitrary bytes to the gate's parser (#100): it must
// not panic, and whatever it accepts must survive the rewrite unchanged.
func FuzzGateCommands(f *testing.F) {
	a, n := strings.Repeat("a", 40), strings.Repeat("b", 40)
	f.Add(PktLine(a+" "+n+" refs/heads/zero/dev\x00report-status push-options\n") + "0000PACK")
	f.Add(PktLine(a+" "+n+" refs/heads/main\n") + PktLine(a+" "+ZeroSHA+" refs/heads/zero/dev\n") + "0000")
	f.Add("0000")
	f.Add("ffff")
	f.Fuzz(func(t *testing.T, in string) {
		ups, caps, err := ReadCommands(bufio.NewReader(strings.NewReader(in)))
		if err != nil {
			return
		}
		if len(ups) > MaxRefUpdates {
			t.Fatalf("%d updates accepted", len(ups))
		}
		CheckRefs(ups, "refs/heads/zero/dev")
		for _, skip := range []bool{false, true} {
			again, _, err := ReadCommands(bufio.NewReader(strings.NewReader(WriteCommands(ups, caps, skip))))
			if err != nil || !slices.Equal(again, ups) {
				t.Fatalf("rewrite changed the commands: %v %v -> %v", err, ups, again)
			}
		}
	})
}

func TestReadCommandsLimits(t *testing.T) {
	a := strings.Repeat("a", 40)
	one := PktLine(a + " " + a + " refs/heads/zero/dev\n")
	if _, _, err := ReadCommands(bufio.NewReader(strings.NewReader(strings.Repeat(one, MaxRefUpdates+1) + "0000"))); err == nil {
		t.Error("too many ref updates accepted")
	}
	long := PktLine(a + " " + a + " refs/heads/zero/" + strings.Repeat("x", MaxCommandLine) + "\n")
	if _, _, err := ReadCommands(bufio.NewReader(strings.NewReader(long + "0000"))); err == nil {
		t.Error("overlong command accepted")
	}
}
