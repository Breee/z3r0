package zero

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// Input directories (ADR 0039): a workspace directory that is no git repo is copied,
// filtered, to a staging directory on the host. The staging directory of the
// sandbox is bind-mounted read-only at inputMount.

// inputRoot is created by zero install; the gateway user can read it.
var inputRoot = "/var/lib/zero/input"

const (
	inputMount      = "/tmp/zero-input"
	defaultMaxInput = 100 << 20
)

// secretFiles and secretDirs are never copied; .zeroignore cannot allow them.
// Matched on the lower-cased base name at any depth. Sources per pattern:
// docs/spec/workspaces.md section 6.1.
var (
	secretFiles = []string{
		".env*", "*.env",
		"*.pem", "*.key", "*.p12", "*.pfx", "*.jks", "*.keystore", "*.ppk", "*.kdbx", "*.asc", "*.ovpn",
		"id_*", "*_rsa", "*_dsa", "*_ecdsa", "*_ed25519",
		".git-credentials", ".netrc", "_netrc", ".npmrc", ".pypirc", ".pgpass", ".htpasswd", ".dockercfg", ".s3cfg",
		"kubeconfig", "*.kubeconfig",
		"*.tfstate", "*.tfstate.*", "*.tfvars", "*.tfvars.json", ".terraformrc", "terraform.rc",
		".*_history",
	}
	secretDirs = []string{".ssh", ".aws", ".kube", ".gnupg", ".docker", ".azure", ".terraform", ".git"}
)

func isInput(top string) bool { return !fileExists(filepath.Join(top, ".git")) }

func (s *Sandbox) inputs() []string {
	var out []string
	for _, top := range s.Folders {
		if isInput(top) {
			out = append(out, top)
		}
	}
	return out
}

// repos are the directories that are cloned into the sandbox.
func (s *Sandbox) repos() []string {
	var out []string
	for _, top := range s.Folders {
		if !isInput(top) {
			out = append(out, top)
		}
	}
	return out
}

func (s *Sandbox) stage() string { return filepath.Join(inputRoot, s.Name) }

type inputFile struct {
	rel  string
	size int64
	mode fs.FileMode
}

type skipped struct{ rel, why string }

// matchAny reports whether name or rel matches one of the patterns.
func matchAny(patterns []string, name, rel string) bool {
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, name); ok {
			return true
		}
		if ok, _ := filepath.Match(p, rel); ok {
			return true
		}
	}
	return false
}

// readIgnore reads .zeroignore: one pattern per line (file name or path
// relative to the directory, shell wildcards), # starts a comment.
func readIgnore(top string) ([]string, error) {
	f, err := os.Open(filepath.Join(top, ".zeroignore"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSuffix(strings.TrimSpace(sc.Text()), "/")
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if _, err := filepath.Match(l, ""); err != nil {
			return nil, fmt.Errorf("%s/.zeroignore: bad pattern %q", top, l)
		}
		out = append(out, l)
	}
	return out, sc.Err()
}

// scanInput lists the files of an input directory that may be copied and the
// entries that are left out. Symlinks and special files are never copied.
func scanInput(top string) ([]inputFile, []skipped, error) {
	ignore, err := readIgnore(top)
	if err != nil {
		return nil, nil, err
	}
	var files []inputFile
	var skip []skipped
	err = filepath.WalkDir(top, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == top {
			return nil
		}
		rel, _ := filepath.Rel(top, p)
		why := ""
		switch {
		case d.IsDir() && matchAny(secretDirs, strings.ToLower(d.Name()), ""), !d.IsDir() && matchAny(secretFiles, strings.ToLower(d.Name()), ""):
			why = "secret pattern"
		case matchAny(ignore, d.Name(), rel):
			why = ".zeroignore"
		case d.Type()&fs.ModeSymlink != 0:
			why = "symlink"
		case !d.IsDir() && !d.Type().IsRegular():
			why = "special file"
		}
		if why != "" {
			skip = append(skip, skipped{rel, why})
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, inputFile{rel, fi.Size(), fi.Mode()})
		return nil
	})
	return files, skip, err
}

// copyInput writes the files into dst. Everything is world-readable: the
// sandbox user is "nobody" on the host; the parent of inputRoot keeps other
// local users out.
func copyInput(top, dst string, files []inputFile) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, f := range files {
		to := filepath.Join(dst, f.rel)
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if f.mode&0o111 != 0 {
			mode = 0o755
		}
		if err := copyFile(filepath.Join(top, f.rel), to, mode, f.size); err != nil {
			return err
		}
	}
	// MkdirAll and OpenFile are subject to the umask.
	return filepath.WalkDir(dst, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.Chmod(p, 0o755)
		}
		return nil
	})
}

func copyFile(from, to string, mode fs.FileMode, size int64) error {
	// O_NOFOLLOW: a file swapped for a symlink after the scan is not followed.
	in, err := os.OpenFile(from, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	// At most the scanned size: a file that grew after the scan cannot pass the size limit.
	if _, err := io.Copy(out, io.LimitReader(in, size)); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(to, mode)
}

func kib(n int64) string {
	if n >= 1<<20 {
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d KiB", (n+1023)/1024)
}

// syncInputs refreshes the host copy of every input directory and of the
// workspace's instructions and skills, and drops copies of what left the
// workspace. The size limit holds for all of them together and is checked
// before anything is copied.
func (s *Sandbox) syncInputs(out io.Writer) error {
	inputs := s.inputs()
	what := ""
	if len(inputs) > 0 {
		what = inputs[0] + " is an input directory (no git repo); input directories"
	} else if len(s.Instructions)+len(s.Skills) > 0 {
		what = "the workspace has instructions or skills; they"
	}
	if !fileExists(inputRoot) {
		if what != "" {
			return fmt.Errorf("%s need %s: sudo zero install", what, inputRoot)
		}
		return nil
	}
	if what != "" {
		if s.gw != "" {
			return fmt.Errorf("%s are not supported on the %s gateway; use zero up without --vm", what, s.gw)
		}
		if s.exists() && s.read("input.mount") == "" {
			return fmt.Errorf("%s was created without the input mount; recreate: zero rm && zero up", s.Name)
		}
	}
	limit := s.MaxInput
	if limit == 0 {
		limit = defaultMaxInput
	}
	type scan struct {
		top   string
		files []inputFile
		skip  []skipped
		size  int64
	}
	var scans, skills []scan
	var total int64
	for i, top := range append(slices.Clone(inputs), s.Skills...) {
		files, skip, err := scanInput(top)
		if err != nil {
			return fmt.Errorf("input %s: %w", top, err)
		}
		sc := scan{top: top, files: files, skip: skip}
		for _, f := range files {
			sc.size += f.size
		}
		total += sc.size
		if i < len(inputs) {
			scans = append(scans, sc)
		} else {
			skills = append(skills, sc)
		}
	}
	var instr []inputFile
	for _, f := range s.Instructions {
		st, err := os.Stat(f)
		if err != nil {
			return err
		}
		instr = append(instr, inputFile{rel: f, size: st.Size(), mode: 0o644})
		total += st.Size()
	}
	if total > limit {
		return fmt.Errorf("input directories, instructions and skills have %s, limit is %s; raise it: zero up --max-input <MiB>", kib(total), kib(limit))
	}
	stage := s.stage()
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(stage, 0o755); err != nil {
		return err
	}
	keep := map[string]bool{}
	// swap puts the finished copy tmp in place of stage/base.
	swap := func(base, tmp string) error {
		dst, old := filepath.Join(stage, base), filepath.Join(stage, "."+base+".old")
		os.RemoveAll(old)
		if err := os.Rename(dst, old); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return err
		}
		return os.RemoveAll(old)
	}
	for _, sc := range scans {
		base := filepath.Base(sc.top)
		keep[base] = true
		tmp := filepath.Join(stage, "."+base+".new")
		os.RemoveAll(tmp)
		if err := copyInput(sc.top, tmp, sc.files); err != nil {
			os.RemoveAll(tmp)
			return fmt.Errorf("copy input %s: %w", sc.top, err)
		}
		if err := swap(base, tmp); err != nil {
			return err
		}
		fmt.Fprintf(out, "input %s: %d files, %s, read-only\n", base, len(sc.files), kib(sc.size))
		for i, k := range sc.skip {
			if i == 10 {
				fmt.Fprintf(out, "  excluded: %d more\n", len(sc.skip)-i)
				break
			}
			fmt.Fprintf(out, "  excluded: %s (%s)\n", k.rel, k.why)
		}
	}
	if len(instr)+len(skills) > 0 {
		keep[guidanceDir] = true
		tmp := filepath.Join(stage, "."+guidanceDir+".new")
		os.RemoveAll(tmp)
		fail := func(err error) error {
			os.RemoveAll(tmp)
			return fmt.Errorf("copy instructions and skills: %w", err)
		}
		for _, d := range []string{"instructions", "skills"} {
			if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
				return fail(err)
			}
		}
		var names []string
		for _, f := range instr {
			if err := copyFile(f.rel, filepath.Join(tmp, "instructions", filepath.Base(f.rel)), f.mode, f.size); err != nil {
				return fail(err)
			}
			names = append(names, filepath.Base(f.rel))
		}
		if len(names) > 0 {
			fmt.Fprintf(out, "instructions: %s, read-only\n", strings.Join(names, ", "))
		}
		names = nil
		for _, sc := range skills {
			if err := copyInput(sc.top, filepath.Join(tmp, "skills", filepath.Base(sc.top)), sc.files); err != nil {
				return fail(err)
			}
			names = append(names, filepath.Base(sc.top))
		}
		if len(names) > 0 {
			fmt.Fprintf(out, "skills: %s, read-only\n", strings.Join(names, ", "))
		}
		if err := swap(guidanceDir, tmp); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !keep[e.Name()] {
			if err := os.RemoveAll(filepath.Join(stage, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
