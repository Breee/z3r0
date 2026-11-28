package core

import (
	"io"
	"unicode"
	"unicode/utf8"
)

// Text that comes out of a sandbox is written by the agent. Before it reaches
// the host terminal the control characters are taken out (all but newline and
// tab), so it cannot move the cursor, repaint a question or set the window
// title (#140).

// Cleaner removes from a UTF-8 byte stream: control characters (C0 except \n
// and \t, DEL, C1), the characters that change the writing direction
// (U+202A..U+202E, U+2066..U+2069) and bytes that are not UTF-8, which a
// terminal with an 8-bit character set could read as C1.
type Cleaner struct{ rest []byte } // start of a character cut off by the end of a write

func (c *Cleaner) Filter(p []byte) []byte {
	p = append(c.rest, p...)
	c.rest = nil
	out := make([]byte, 0, len(p))
	for len(p) > 0 {
		r, n := utf8.DecodeRune(p)
		switch {
		case r == utf8.RuneError && n == 1:
			if !utf8.FullRune(p) {
				c.rest = append(c.rest, p...)
				return out
			}
		case unicode.IsControl(r) && r != '\n' && r != '\t', r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069:
		default:
			out = append(out, p[:n]...)
		}
		p = p[n:]
	}
	return out
}

// Clean is the filter for a whole text.
func Clean(s string) string {
	var c Cleaner
	return string(c.Filter([]byte(s)))
}

// CleanWriter filters what is written through it.
type CleanWriter struct {
	W io.Writer
	c Cleaner
}

func (w *CleanWriter) Write(p []byte) (int, error) {
	if _, err := w.W.Write(w.c.Filter(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}
