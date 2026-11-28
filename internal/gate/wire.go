// Package gate holds the wire format of a git push as the push gate reads and
// writes it, and the checks on what a sandbox may push.
package gate

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

// ZeroSHA is the object name git uses for "no object".
const ZeroSHA = "0000000000000000000000000000000000000000"

// Limits on what the gate parses from a sandbox (#100). A normal push names a
// handful of refs in short lines.
const (
	MaxRefUpdates  = 32
	MaxCommandLine = 1024
)

type Update struct{ Old, New, Ref string }

// ReadCommands reads the ref update commands of a receive-pack request up to
// the first flush packet. caps is the capability list of the first command.
func ReadCommands(r *bufio.Reader) (ups []Update, caps string, err error) {
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
		if n > MaxCommandLine {
			return nil, "", fmt.Errorf("command line longer than %d bytes", MaxCommandLine)
		}
		if len(ups) == MaxRefUpdates {
			return nil, "", fmt.Errorf("more than %d ref updates in one push", MaxRefUpdates)
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
		ups = append(ups, Update{f[0], f[1], f[2]})
	}
}

// ReadOptions reads the push options that follow the commands when the
// client announced push-options, up to their flush packet.
func ReadOptions(r *bufio.Reader) ([]string, error) {
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
		if n > MaxCommandLine || len(opts) == MaxRefUpdates {
			return nil, fmt.Errorf("push options too long or too many")
		}
		line := make([]byte, n-4)
		if _, err := io.ReadFull(r, line); err != nil {
			return nil, fmt.Errorf("truncated request")
		}
		opts = append(opts, strings.TrimSuffix(string(line), "\n"))
	}
}

// CheckCaps refuses a capability that git reads as push-options but the gate
// would not ("push-options=x"): its options would reach the forge unchecked.
func CheckCaps(caps string) error {
	for _, c := range strings.Fields(caps) {
		if c != "push-options" && strings.HasPrefix(strings.ToLower(c), "push-options") {
			return fmt.Errorf("capability %q not allowed", c)
		}
	}
	return nil
}

// CheckOptions refuses the agent's push options: the forge acts on them with
// only push permission (merge_request.create opens a merge request, whose
// pipeline ci.skip does not prevent, #134). merge_request.* passes when
// gitlab.mr.create and ci.run are granted (ADR 0044).
func CheckOptions(opts []string, mrCreate bool) error {
	for _, o := range opts {
		if !mrCreate || !strings.HasPrefix(o, "merge_request.") || strings.ContainsAny(o, "\n\x00") {
			return fmt.Errorf("push option %q not allowed", o)
		}
	}
	return nil
}

func PktLine(s string) string { return fmt.Sprintf("%04x%s", len(s)+4, s) }

// WriteCommands is the command section of a receive-pack request for ups.
// With skipCI the push option ci.skip is added (#102): the forge records the
// pipeline as skipped, so agent-written CI code runs only when a human starts
// it. The rest of the request must follow: the client's own push options if
// it announced push-options (WriteOptions), then the pack.
func WriteCommands(ups []Update, caps string, skipCI bool) string {
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
		b.WriteString(PktLine(l + "\n"))
	}
	b.WriteString("0000")
	if skipCI {
		b.WriteString(PktLine("ci.skip\n"))
		if !hadOpts {
			b.WriteString("0000")
		}
	}
	return b.String()
}

// WriteOptions is the client's push option section after WriteCommands.
func WriteOptions(opts []string) string {
	var b strings.Builder
	for _, o := range opts {
		b.WriteString(PktLine(o + "\n"))
	}
	return b.String() + "0000"
}

// CheckRefs allows only updates of branch (refs/heads/zero/<name>) and
// branches below it; no deletes.
func CheckRefs(ups []Update, branch string) error {
	if len(ups) == 0 {
		return fmt.Errorf("no ref updates")
	}
	for _, u := range ups {
		if u.Ref != branch && !strings.HasPrefix(u.Ref, branch+"/") || strings.Contains(u.Ref, "..") {
			return fmt.Errorf("%s: only %s and %s/* may be pushed", u.Ref, branch, branch)
		}
		if u.New == ZeroSHA {
			return fmt.Errorf("%s: delete not allowed", u.Ref)
		}
	}
	return nil
}

// ReadReport is nil when a report-status answer says the pack was unpacked
// and ref was updated; otherwise the error carries the forge's line.
func ReadReport(r *bufio.Reader, ref string) error {
	lines, err := ReadOptions(r)
	if err != nil {
		return fmt.Errorf("forge answer not understood")
	}
	if len(lines) > 0 && lines[0] == "unpack ok" && slices.Contains(lines[1:], "ok "+ref) {
		return nil
	}
	return fmt.Errorf("forge refused the push: %s", strings.Join(lines, "; "))
}

// EmptyPack is a pack without objects: the objects of a checked push are
// already on the forge.
func EmptyPack() []byte {
	p := []byte("PACK\x00\x00\x00\x02\x00\x00\x00\x00")
	sum := sha1.Sum(p)
	return append(p, sum[:]...)
}

// SessionBranch is the ref a sandbox may push.
func SessionBranch(sandbox string) string {
	return "refs/heads/zero/" + strings.TrimPrefix(sandbox, "zero-")
}
