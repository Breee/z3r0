package zero

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"github.com/Breee/z3r0/internal/core"
)

const switchHelp = "up/down or k/j: move  enter: switch  q: leave unchanged"

// switchKey maps the bytes of one key press to an action; "" for other keys.
func switchKey(b []byte) string {
	switch string(b) {
	case "k", "\x1b[A", "\x1bOA":
		return "up"
	case "j", "\x1b[B", "\x1bOB":
		return "down"
	case "\r", "\n":
		return "use"
	case "q", "\x1b", "\x03":
		return "quit"
	}
	return ""
}

// renderSwitch draws the `zero ls` table with a cursor on row sel.
func renderSwitch(out io.Writer, o *overview, sel int) {
	var table bytes.Buffer
	writeRows(&table, o.rows)
	var b strings.Builder
	b.WriteString("\x1b[H\x1b[2J")
	t := table.String()
	if Fancy(out) {
		t = styleRows(t, o.rows)
	}
	for i, l := range strings.Split(strings.TrimRight(t, "\n"), "\n") {
		cur := "  "
		if i == sel+1 {
			cur = "> "
		}
		b.WriteString(cur + l + "\n")
	}
	for _, w := range o.warn {
		b.WriteString("warning: " + w + "\n")
	}
	b.WriteString("\n" + switchHelp + "\n")
	io.WriteString(out, b.String())
}

// nextKey takes the first key press off b: its action and its length. An
// escape that starts no known sequence counts as the Esc key.
func nextKey(b []byte) (string, int) {
	if b[0] != 0x1b {
		return switchKey(b[:1]), 1
	}
	if len(b) >= 3 && (b[1] == '[' || b[1] == 'O') {
		return switchKey(b[:3]), 3
	}
	return "quit", len(b)
}

// cutEscape reports whether b ends inside an escape sequence, as happens
// when a terminal delivers an arrow key in two reads.
func cutEscape(b []byte) bool {
	n := len(b)
	return b[n-1] == 0x1b || n >= 2 && b[n-2] == 0x1b && (b[n-1] == '[' || b[n-1] == 'O')
}

func termios(f *os.File, req uintptr, t *syscall.Termios) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), req, uintptr(unsafe.Pointer(t))); e != 0 {
		return e
	}
	return nil
}

// Switch makes a workspace the current one, like `zero ctx use`: the one
// named in args, or without a name the one picked from the list.
func Switch(in, out *os.File, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: zero switch [NAME]")
	}
	name := ""
	if len(args) == 1 {
		name = args[0]
	} else {
		var err error
		if name, err = pickWorkspace(in, out); err != nil || name == "" {
			return err
		}
	}
	c, err := loadConfig()
	if err != nil {
		return err
	}
	if c.Workspaces[name] == nil {
		return fmt.Errorf("no workspace %q (zero ls lists them)", name)
	}
	c.Current = name
	if err := c.Save(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "workspace: %s (switch)\n", name)
	return nil
}

// pickWorkspace shows the list and returns the chosen name; "" if the user left.
func pickWorkspace(in, out *os.File) (string, error) {
	var cooked syscall.Termios
	if err := termios(in, syscall.TCGETS, &cooked); err != nil || !core.IsTerminal(out) {
		return "", fmt.Errorf("zero switch without a name needs a terminal (zero switch NAME; zero ls lists them)")
	}
	o, err := loadOverview()
	if err != nil {
		return "", err
	}
	if len(o.rows) == 0 {
		return "", fmt.Errorf("no workspaces (zero ctx new NAME [PATH...])")
	}
	sel := 0
	for i, r := range o.rows {
		if r.Current {
			sel = i
		}
	}
	raw := cooked
	raw.Lflag &^= syscall.ICANON | syscall.ECHO | syscall.ISIG
	raw.Cc[syscall.VMIN], raw.Cc[syscall.VTIME] = 1, 0
	// rest waits a tenth of a second for the rest of an escape sequence.
	rest := raw
	rest.Cc[syscall.VMIN], rest.Cc[syscall.VTIME] = 0, 1
	if err := termios(in, syscall.TCSETS, &raw); err != nil {
		return "", err
	}
	defer termios(in, syscall.TCSETS, &cooked)
	defer io.WriteString(out, "\x1b[H\x1b[2J")
	buf := make([]byte, 8)
	for {
		renderSwitch(out, o, sel)
		n, err := in.Read(buf)
		if err != nil {
			return "", err
		}
		keys := buf[:n]
		if cutEscape(keys) {
			more := make([]byte, 8)
			termios(in, syscall.TCSETS, &rest)
			m, _ := in.Read(more)
			termios(in, syscall.TCSETS, &raw)
			keys = append(keys, more[:m]...)
		}
		for len(keys) > 0 {
			action, k := nextKey(keys)
			keys = keys[k:]
			switch action {
			case "up":
				sel = max(0, sel-1)
			case "down":
				sel = min(sel+1, len(o.rows)-1)
			case "use":
				return o.rows[sel].Name, nil
			case "quit":
				return "", nil
			}
		}
	}
}
