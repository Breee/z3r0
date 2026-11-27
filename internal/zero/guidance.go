package zero

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Instructions and skills of a workspace (ADR 0049): plain files on the host,
// copied read-only into the sandbox next to the input directories and named in
// the opencode user config of the sandbox.

// guidanceDir is the entry of the staging directory that holds them.
const guidanceDir = ".zero"

// opencodeConfig is the opencode user config inside the sandbox.
const opencodeConfig = "$HOME/.config/opencode/opencode.json"

// guidance checks the instructions and skills of a workspace and returns
// them as absolute paths.
func guidance(w *Workspace) (instr, skills []string, err error) {
	resolve := func(kind, p string, seen map[string]bool) (string, os.FileInfo, error) {
		q := p
		if strings.HasPrefix(q, "~/") {
			q = filepath.Join(os.Getenv("HOME"), q[2:])
		}
		q, err := realPath(q)
		if err != nil {
			return "", nil, fmt.Errorf("%s %s: not found", kind, p)
		}
		st, err := os.Stat(q)
		if err != nil {
			return "", nil, fmt.Errorf("%s %s: %w", kind, p, err)
		}
		base := filepath.Base(q)
		if strings.HasPrefix(base, ".") || base != clean(base) {
			return "", nil, fmt.Errorf("%s %s: the name %q is not usable", kind, p, base)
		}
		if seen[base] {
			return "", nil, fmt.Errorf("%s %s: the name %s is listed twice", kind, p, base)
		}
		seen[base] = true
		return q, st, nil
	}
	seen := map[string]bool{}
	for _, p := range w.Instructions {
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
	for _, p := range w.Skills {
		q, st, err := resolve("skill", p, seen)
		if err != nil {
			return nil, nil, err
		}
		if !st.IsDir() || !fileExists(filepath.Join(q, "SKILL.md")) {
			return nil, nil, fmt.Errorf("skill %s: not a directory with a SKILL.md", p)
		}
		skills = append(skills, q)
	}
	return instr, skills, nil
}

// guidanceConfig is the opencode config that names the copied instructions
// and skills; empty when the workspace has none.
func guidanceConfig(instr, skills []string, models []proxyModel, model string, tools bool) string {
	if len(instr)+len(skills)+len(models) == 0 {
		return ""
	}
	root := inputMount + "/" + guidanceDir
	c := map[string]any{"$schema": "https://opencode.ai/config.json"}
	if len(instr) > 0 {
		var l []string
		for _, f := range instr {
			l = append(l, root+"/instructions/"+filepath.Base(f))
		}
		c["instructions"] = l
	}
	if len(skills) > 0 {
		c["skills"] = []string{root + "/skills"}
	}
	if len(models) > 0 {
		c["providers"] = map[string]any{"litellm": modelProvider(models)}
		if model != "" {
			c["model"] = "litellm/" + model
		}
		if tools {
			// The workspace's tools: the proxy's MCP endpoint, same key (ADR 0055).
			c["mcp"] = map[string]any{"servers": map[string]any{"gitlab": map[string]any{
				"type": "remote", "url": fmt.Sprintf("http://%s:%d/mcp", gateHost, modelPort), "oauth": false,
				"headers": map[string]string{"Authorization": "Bearer {env:" + modelKeyEnv + "}"},
			}}}
		}
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return string(b) + "\n"
}

// syncGuidance writes, replaces or removes the opencode user config in the
// sandbox. It only touches a file zero wrote itself (state opencode.sha).
func (s *Sandbox) syncGuidance(out io.Writer, models []proxyModel) error {
	want := guidanceConfig(s.Instructions, s.Skills, models, s.Model, s.read("mcp.id") != "")
	mine := s.read("opencode.sha")
	if want == "" && mine == "" {
		return nil
	}
	cur, err := s.ssh("f=" + opencodeConfig + "; test -e \"$f\" && sha256sum \"$f\" | cut -d' ' -f1 || true")
	if err != nil {
		return err
	}
	cur = strings.TrimSpace(cur)
	if cur != "" && cur != mine {
		if want != "" {
			fmt.Fprintf(out, "instructions, skills and models: not announced to opencode, the sandbox has an opencode config zero did not write (~/.config/opencode/opencode.json); remove it there and run zero up\n")
			return nil
		}
		fmt.Fprintf(out, "opencode config in the sandbox was changed there (~/.config/opencode/opencode.json); zero leaves it\n")
		return os.Remove(s.path("opencode.sha"))
	}
	if want == "" {
		if _, err := s.ssh("rm -f " + opencodeConfig); err != nil {
			return err
		}
		return os.Remove(s.path("opencode.sha"))
	}
	sum := sha256.Sum256([]byte(want))
	sha := hex.EncodeToString(sum[:])
	if sha == cur {
		return nil
	}
	if _, err := s.ssh("f=" + opencodeConfig + "; mkdir -p \"$(dirname \"$f\")\" && printf %s " + shellQuote(want) + " > \"$f\""); err != nil {
		return err
	}
	return s.write("opencode.sha", sha, 0o600)
}

// base names for messages.
func baseNames(paths []string) string {
	var l []string
	for _, p := range paths {
		l = append(l, filepath.Base(p))
	}
	return strings.Join(l, ", ")
}
