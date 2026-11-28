package zero

import (
	"io"
	"os"
	"strings"
)

// Colour is added only on a terminal (ADR 0059): the text and its columns
// are the same with and without it.
const (
	sgrReset = "\x1b[0m"
	sgrBold  = "1"
	sgrFaint = "2"
	sgrCyan  = "36"
)

var statusSGR = map[string]string{"running": "32", "paused": "33", "none": sgrFaint, "unknown": sgrFaint}

func sgr(codes ...string) string {
	var on []string
	for _, c := range codes {
		if c != "" {
			on = append(on, c)
		}
	}
	if len(on) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(on, ";") + "m"
}

// paint wraps s in the codes; without codes s stays as it is.
func paint(s string, codes ...string) string {
	on := sgr(codes...)
	if on == "" || s == "" {
		return s
	}
	return on + s + sgrReset
}

// Fancy reports whether out is a terminal that takes colour.
func Fancy(out io.Writer) bool {
	f, ok := out.(*os.File)
	return ok && fancy(f)
}

// StyleHelp colours the help text: the first line and every heading
// (a line that starts at the left edge, up to its colon if it has one)
// bold, command names cyan. The lines under Examples stay as they are.
func StyleHelp(s string) string {
	lines := strings.Split(s, "\n")
	examples := false
	for i, l := range lines {
		switch {
		case l == "":
		case i == 0:
			lines[i] = paint(l, sgrBold)
		case l[0] != ' ':
			examples = l == "Examples"
			if label, rest, ok := strings.Cut(l, ":"); ok {
				lines[i] = paint(label+":", sgrBold) + rest
			} else {
				lines[i] = paint(l, sgrBold)
			}
		case !examples && len(l) > 2 && strings.HasPrefix(l, "  ") && l[2] != ' ':
			name, rest, cut := strings.Cut(l[2:], " ")
			lines[i] = "  " + paint(name, sgrCyan)
			if cut {
				lines[i] += " " + rest
			}
		}
	}
	return strings.Join(lines, "\n")
}

// styleRows colours the table writeRows made: header faint, the current
// workspace bold, the status by its meaning.
func styleRows(table string, rows []wsRow) string {
	lines := strings.Split(table, "\n")
	for i, l := range lines {
		if i == 0 && l != "" {
			lines[i] = sgr(sgrFaint) + l + sgrReset
			continue
		}
		if i < 1 || i > len(rows) {
			continue
		}
		r := rows[i-1]
		bold := ""
		if r.Current {
			bold = sgrBold
		}
		// the status is the first word after the name's column
		from := 2 + len(r.Name)
		if r.Status == "" || from > len(l) {
			lines[i] = paint(l, bold)
			continue
		}
		at := from + len(l[from:]) - len(strings.TrimLeft(l[from:], " "))
		if !strings.HasPrefix(l[at:], r.Status) {
			lines[i] = paint(l, bold)
			continue
		}
		lines[i] = paint(l[:at], bold) + paint(r.Status, bold, statusSGR[r.Status]) + paint(l[at+len(r.Status):], bold)
	}
	return strings.Join(lines, "\n")
}
