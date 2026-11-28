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

	"github.com/Breee/z3r0/internal/agent/opencode"
	"github.com/Breee/z3r0/internal/core"
	"github.com/Breee/z3r0/internal/guidance/hostfiles"
	"github.com/Breee/z3r0/internal/model/litellm"
)

// Instructions and skills of a workspace (ADR 0049): plain files on the host,
// copied read-only into the sandbox next to the input directories and named in
// the opencode user config of the sandbox.

// guidanceConfig is the opencode config that names the copied instructions
// and skills; empty when the workspace has none.
func guidanceConfig(instr, skills []string, models []litellm.Model, model string, tools bool) string {
	if len(instr)+len(skills)+len(models) == 0 {
		return ""
	}
	root := inputMount + "/" + hostfiles.Dir
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
		c["providers"] = map[string]any{"litellm": litellm.Provider(models)}
		if model != "" {
			c["model"] = "litellm/" + model
		}
		if tools {
			// The workspace's tools: the proxy's MCP endpoint, same key (ADR 0055).
			c["mcp"] = map[string]any{"servers": map[string]any{"gitlab": map[string]any{
				"type": "remote", "url": fmt.Sprintf("http://%s:%d/mcp", gateHost, litellm.Port), "oauth": false,
				"headers": map[string]string{"Authorization": "Bearer {env:" + litellm.KeyEnv + "}"},
			}}}
		}
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return string(b) + "\n"
}

// syncGuidance writes, replaces or removes the opencode user config in the
// sandbox. It only touches a file zero wrote itself (state opencode.sha).
func (s *Sandbox) syncGuidance(out io.Writer, models []litellm.Model) error {
	want := guidanceConfig(s.Instructions, s.Skills, models, s.Model, s.read("mcp.id") != "")
	mine := s.read("opencode.sha")
	if want == "" && mine == "" {
		return nil
	}
	cur, err := s.ssh("f=" + opencode.ConfigPath + "; test -e \"$f\" && sha256sum \"$f\" | cut -d' ' -f1 || true")
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
		if _, err := s.ssh("rm -f " + opencode.ConfigPath); err != nil {
			return err
		}
		return os.Remove(s.path("opencode.sha"))
	}
	sum := sha256.Sum256([]byte(want))
	sha := hex.EncodeToString(sum[:])
	if sha == cur {
		return nil
	}
	if _, err := s.ssh("f=" + opencode.ConfigPath + "; mkdir -p \"$(dirname \"$f\")\" && printf %s " + core.ShellQuote(want) + " > \"$f\""); err != nil {
		return err
	}
	return s.write("opencode.sha", sha, 0o600)
}
