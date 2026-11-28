// Package hostfiles gives instructions and skills from plain files on the
// host (ADR 0049).
package hostfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Breee/z3r0/internal/core"
)

// Dir is the entry of the staging directory that holds them.
const Dir = ".zero"

// Resolve checks the instructions and skills of a workspace and returns
// them as absolute paths.
func Resolve(instructions, skillDirs []string) (instr, skills []string, err error) {
	resolve := func(kind, p string, seen map[string]bool) (string, os.FileInfo, error) {
		q := p
		if strings.HasPrefix(q, "~/") {
			q = filepath.Join(os.Getenv("HOME"), q[2:])
		}
		q, err := core.RealPath(q)
		if err != nil {
			return "", nil, fmt.Errorf("%s %s: not found", kind, p)
		}
		st, err := os.Stat(q)
		if err != nil {
			return "", nil, fmt.Errorf("%s %s: %w", kind, p, err)
		}
		base := filepath.Base(q)
		if strings.HasPrefix(base, ".") || base != core.Clean(base) {
			return "", nil, fmt.Errorf("%s %s: the name %q is not usable", kind, p, base)
		}
		if seen[base] {
			return "", nil, fmt.Errorf("%s %s: the name %s is listed twice", kind, p, base)
		}
		seen[base] = true
		return q, st, nil
	}
	seen := map[string]bool{}
	for _, p := range instructions {
		q, st, err := resolve("instruction", p, seen)
		if err != nil {
			return nil, nil, err
		}
		if !st.Mode().IsRegular() {
			return nil, nil, fmt.Errorf("instruction %s: not a file", p)
		}
		instr = append(instr, q)
	}
	seen = map[string]bool{}
	for _, p := range skillDirs {
		q, st, err := resolve("skill", p, seen)
		if err != nil {
			return nil, nil, err
		}
		if !st.IsDir() || !core.FileExists(filepath.Join(q, "SKILL.md")) {
			return nil, nil, fmt.Errorf("skill %s: not a directory with a SKILL.md", p)
		}
		skills = append(skills, q)
	}
	return instr, skills, nil
}

// BaseNames joins the base names, for messages.
func BaseNames(paths []string) string {
	var l []string
	for _, p := range paths {
		l = append(l, filepath.Base(p))
	}
	return strings.Join(l, ", ")
}
