package zero

import "io"

// Text that comes out of a sandbox is written by the agent. Before it reaches
// the host terminal the control characters are taken out (all but newline and
// tab), so it cannot move the cursor, repaint a question or set the window
// title (#140).

// cleaner removes control characters from a byte stream: C0 except \n and \t,
// DEL, and C1 (U+0080..U+009F, in UTF-8 the bytes C2 80..C2 9F).
type cleaner struct{ c2 bool }

func (c *cleaner) filter(p []byte) []byte {
	out := make([]byte, 0, len(p)+1)
	for _, b := range p {
		if c.c2 {
			c.c2 = false
			if b >= 0x80 && b <= 0x9f {
				continue
			}
			out = append(out, 0xc2)
		}
		switch {
		case b == 0xc2:
			c.c2 = true
		case b == '\n' || b == '\t' || b >= 0x20 && b != 0x7f:
			out = append(out, b)
		}
	}
	return out
}

// clean is the filter for a whole text.
func clean(s string) string {
	var c cleaner
	out := c.filter([]byte(s))
	if c.c2 {
		out = append(out, 0xc2)
	}
	return string(out)
}

// cleanWriter filters what is written through it.
type cleanWriter struct {
	w io.Writer
	c cleaner
}

func (w *cleanWriter) Write(p []byte) (int, error) {
	if _, err := w.w.Write(w.c.filter(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}
