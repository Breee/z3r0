package zero

import "runtime/debug"

// Version is the commit this binary was built from, as the Go toolchain
// records it: 12 characters, the commit date, "+changes" for a build from
// a changed checkout.
func Version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	rev, date, dirty := "", "", ""
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.time":
			date = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "+changes"
			}
		}
	}
	if rev == "" {
		return "unknown (built without git information)"
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	if len(date) >= 10 {
		date = " " + date[:10]
	}
	return rev + dirty + date
}
