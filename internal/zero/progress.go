package zero

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/Breee/z3r0/internal/core"
)

// progress shows the slow parts of a command on stderr: on a terminal a
// spinner with the running time, replaced by a result line; otherwise plain
// lines without control characters.
var progress = struct {
	sync.Mutex
	out  io.Writer
	tty  bool
	name string // step on the spinner line, "" when none
}{out: os.Stderr, tty: fancy(os.Stderr)}

func fancy(f *os.File) bool {
	return colourWanted(core.IsTerminal(f), os.Getenv("NO_COLOR"), os.Getenv("TERM"))
}

// colourWanted is the plain rule: colour only on a terminal, and not with
// NO_COLOR set or TERM=dumb.
func colourWanted(terminal bool, noColor, term string) bool {
	return terminal && noColor == "" && term != "dumb"
}

var spinner = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// step names what happens next; the returned func ends it with its result.
func step(name string) func(error) {
	p := &progress
	t0 := time.Now()
	stop := make(chan struct{})
	p.Lock()
	if p.tty {
		p.name = name
		fmt.Fprintf(p.out, "\r\x1b[K\x1b[36m%c\x1b[0m %s", spinner[0], name)
		go func() {
			tick := time.NewTicker(100 * time.Millisecond)
			defer tick.Stop()
			for i := 1; ; i++ {
				select {
				case <-stop:
					return
				case <-tick.C:
				}
				p.Lock()
				select {
				case <-stop:
					p.Unlock()
					return
				default:
				}
				p.name = name
				fmt.Fprintf(p.out, "\r\x1b[K\x1b[36m%c\x1b[0m %s \x1b[2m%s\x1b[0m", spinner[i%len(spinner)], name, took(t0))
				p.Unlock()
			}
		}()
	} else {
		fmt.Fprintf(p.out, "%s ...\n", name)
	}
	p.Unlock()
	ended := false
	return func(err error) {
		p.Lock()
		defer p.Unlock()
		if ended {
			return
		}
		ended = true
		close(stop)
		switch {
		case !p.tty && err != nil:
			fmt.Fprintf(p.out, "%s: failed (%s)\n", name, took(t0))
		case !p.tty:
			fmt.Fprintf(p.out, "%s: done (%s)\n", name, took(t0))
		case err != nil:
			p.name = ""
			fmt.Fprintf(p.out, "\r\x1b[K\x1b[31m✗\x1b[0m %s \x1b[2m%s\x1b[0m\n", name, took(t0))
		default:
			p.name = ""
			fmt.Fprintf(p.out, "\r\x1b[K\x1b[32m✓\x1b[0m %s \x1b[2m%s\x1b[0m\n", name, took(t0))
		}
	}
}

func took(t0 time.Time) string {
	d := time.Since(t0)
	if d < 10*time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// stepWriter keeps lines a step prints out of the spinner line.
type stepWriter struct{ w io.Writer }

func (s stepWriter) Write(b []byte) (int, error) {
	p := &progress
	p.Lock()
	defer p.Unlock()
	if p.tty && p.name != "" {
		fmt.Fprint(p.out, "\r\x1b[K")
		p.name = ""
	}
	return s.w.Write(b)
}

var lapLast time.Time

// lap prints the time since the previous lap when ZERO_TIMING is set: where
// the time of a command goes, phase by phase. An empty name starts the clock.
func lap(name string) {
	if os.Getenv("ZERO_TIMING") == "" {
		return
	}
	now := time.Now()
	if name != "" && !lapLast.IsZero() {
		fmt.Fprintf(os.Stderr, "timing: %-22s %s\n", name, now.Sub(lapLast).Round(time.Millisecond))
	}
	lapLast = now
}
