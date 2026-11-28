package zero

import (
	"io"
	"os"

	"github.com/Breee/z3r0/internal/core"
)

// link returns url as an OSC 8 hyperlink when out is a terminal, so terminals
// make it clickable; otherwise the plain url.
func link(out io.Writer, url string) string {
	if w, ok := out.(stepWriter); ok {
		out = w.w
	}
	if f, ok := out.(*os.File); !ok || !core.IsTerminal(f) {
		return url
	}
	return "\x1b]8;;" + url + "\x1b\\" + url + "\x1b]8;;\x1b\\"
}
