package zero

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

func isTerminal(f *os.File) bool {
	var t syscall.Termios
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TCGETS, uintptr(unsafe.Pointer(&t)))
	return e == 0
}

// link returns url as an OSC 8 hyperlink when out is a terminal, so terminals
// make it clickable; otherwise the plain url.
func link(out io.Writer, url string) string {
	if w, ok := out.(stepWriter); ok {
		out = w.w
	}
	if f, ok := out.(*os.File); !ok || !isTerminal(f) {
		return url
	}
	return "\x1b]8;;" + url + "\x1b\\" + url + "\x1b]8;;\x1b\\"
}
