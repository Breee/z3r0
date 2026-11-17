package zero

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Input folders (ADR 0039): a workspace folder that is no git repo is copied,
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

// repos are the folders that are cloned into the sandbox.
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
// relative to the folder, shell wildcards), # starts a comment.
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

// scanInput lists the files of an input folder that may be copied and the
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

// syncInputs refreshes the host copy of every input folder and drops copies
// of folders that left the workspace. The size limit holds for all input
// folders together and is checked before anything is copied.
func (s *Sandbox) syncInputs(out io.Writer) error {
	inputs := s.inputs()
	if !fileExists(inputRoot) {
		if len(inputs) > 0 {
			return fmt.Errorf("%s is an input folder (no git repo); input folders need %s: sudo zero install", inputs[0], inputRoot)
		}
		return nil
	}
	if len(inputs) > 0 {
		if s.gw != "" {
			return fmt.Errorf("%s is an input folder (no git repo); input folders are not supported on the %s gateway (ADR 0039)", inputs[0], s.gw)
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
	var scans []scan
	var total int64
	for _, top := range inputs {
		files, skip, err := scanInput(top)
		if err != nil {
			return fmt.Errorf("input %s: %w", top, err)
		}
		sc := scan{top: top, files: files, skip: skip}
		for _, f := range files {
			sc.size += f.size
		}
		total += sc.size
		scans = append(scans, sc)
	}
	if total > limit {
		return fmt.Errorf("input folders have %s, limit is %s; raise it: zero up --max-input <MiB>", kib(total), kib(limit))
	}
	stage := s.stage()
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(stage, 0o755); err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, sc := range scans {
		base := filepath.Base(sc.top)
		keep[base] = true
		dst, tmp, old := filepath.Join(stage, base), filepath.Join(stage, "."+base+".new"), filepath.Join(stage, "."+base+".old")
		os.RemoveAll(tmp)
		os.RemoveAll(old)
		if err := copyInput(sc.top, tmp, sc.files); err != nil {
			os.RemoveAll(tmp)
			return fmt.Errorf("copy input %s: %w", sc.top, err)
		}
		if err := os.Rename(dst, old); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return err
		}
		os.RemoveAll(old)
		fmt.Fprintf(out, "input %s: %d files, %s, read-only\n", base, len(sc.files), kib(sc.size))
		for i, k := range sc.skip {
			if i == 10 {
				fmt.Fprintf(out, "  excluded: %d more\n", len(sc.skip)-i)
				break
			}
			fmt.Fprintf(out, "  excluded: %s (%s)\n", k.rel, k.why)
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
