package zero

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"
)

//go:embed gitlab-profile.yaml
var gitlabProfileYAML []byte

// gitlabCaps maps policy capabilities to fine-grained token permissions
// (D20, docs/architecture/07a). git.commit is local only.
var gitlabCaps = map[string][]string{
	"git.read":             {"read_project", "read_code", "download_code"},
	"git.commit":           {},
	"git.push":             {"push_code", "create_branch"},
	"gitlab.issue.read":    {"read_work_item"},
	"gitlab.mr.read":       {"read_merge_request"},
	"gitlab.mr.create":     {"create_merge_request"},
	"gitlab.pipeline.read": {"read_pipeline", "read_job"},
}

var gitlabRemoteRe = regexp.MustCompile(`^(?:git@gitlab\.com:|ssh://git@gitlab\.com/|https://(?:[^@/]+@)?gitlab\.com/)(.+?)(?:\.git)?/?$`)

// gitlab mints tokens through the host's glab session.
type gitlab struct{}

func (gitlab) Name() string     { return "gitlab" }
func (gitlab) Host() string     { return "gitlab.com" }
func (gitlab) Profile() []byte  { return gitlabProfileYAML }
func (gitlab) TokenEnv() string { return "GITLAB_TOKEN" }
func (gitlab) GitUser() string  { return "oauth2" }
func (gitlab) MCP() string      { return "https://gitlab.com/api/v4/mcp" }

// glab sends PATs as PRIVATE-TOKEN, which OpenShell >= 0.1.8 does not
// substitute; as OAuth it sends Authorization: Bearer (ADR 0026, C46, #65).
func (gitlab) Env() []string { return []string{"GLAB_IS_OAUTH2=true"} }

func (gitlab) Match(remote string) (string, bool) {
	m := gitlabRemoteRe.FindStringSubmatch(remote)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func (gitlab) Permissions(caps []string) ([]string, error) {
	perms := []string{"read_project"}
	for _, c := range caps {
		p, ok := gitlabCaps[c]
		if !ok {
			return nil, fmt.Errorf("unknown capability %q for gitlab", c)
		}
		for _, x := range p {
			if !slices.Contains(perms, x) {
				perms = append(perms, x)
			}
		}
	}
	return perms, nil
}

func (gitlab) Mint(label, path string, perms []string) (string, string, error) {
	out, err := sh("glab", "api", "projects/"+url.PathEscape(path))
	if err != nil {
		return "", "", fmt.Errorf("project %s: %w", path, err)
	}
	var proj struct{ ID int64 }
	if err := json.Unmarshal([]byte(out), &proj); err != nil || proj.ID == 0 {
		return "", "", fmt.Errorf("project %s: bad response", path)
	}
	body, _ := json.Marshal(map[string]any{
		"name":       label,
		"expires_at": time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02"),
		"granular_scopes": []map[string]any{{
			"access":      "selected_memberships",
			"permissions": perms,
			"project_ids": []int64{proj.ID},
		}, {
			"access":      "user",
			"permissions": []string{"execute_mcp_tool"},
		}},
	})
	cmd := exec.Command("glab", "api", "-X", "POST", "/user/personal_access_tokens",
		"-H", "Content-Type: application/json", "--input", "-")
	cmd.Stdin = strings.NewReader(string(body))
	b, err := cmd.Output()
	if err != nil {
		// %v, not %w: a wrapped ExitError would make main exit silently.
		var stderr []byte
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = ee.Stderr
		}
		return "", "", fmt.Errorf("mint token (permissions %v): %v: %s", perms, err, strings.TrimSpace(string(stderr)))
	}
	var r struct {
		ID    int64
		Token string
	}
	if err := json.Unmarshal(b, &r); err != nil || r.Token == "" {
		return "", "", fmt.Errorf("mint token: bad response")
	}
	return r.Token, fmt.Sprint(r.ID), nil
}

func (gitlab) Revoke(id string) error {
	_, err := sh("glab", "api", "-X", "DELETE", "/personal_access_tokens/"+id)
	if err != nil && !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "401") {
		return err
	}
	return nil
}

func (gitlab) Guardrails(path string) ([]string, error) {
	out, err := sh("glab", "api", "projects/"+url.PathEscape(path))
	if err != nil {
		return nil, fmt.Errorf("project %s: %w", path, err)
	}
	var p struct {
		DefaultBranch                    string `json:"default_branch"`
		OnlyAllowMergeIfPipelineSucceeds bool   `json:"only_allow_merge_if_pipeline_succeeds"`
	}
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		return nil, fmt.Errorf("project %s: bad response", path)
	}
	var missing []string
	if !p.OnlyAllowMergeIfPipelineSucceeds {
		missing = append(missing, "merge allowed without green pipeline")
	}
	out, err = sh("glab", "api", "projects/"+url.PathEscape(path)+"/protected_branches/"+url.PathEscape(p.DefaultBranch))
	if err != nil {
		return append(missing, p.DefaultBranch+" not protected"), nil
	}
	var b struct {
		Push []struct {
			AccessLevel int `json:"access_level"`
		} `json:"push_access_levels"`
		AllowForcePush bool `json:"allow_force_push"`
	}
	if err := json.Unmarshal([]byte(out), &b); err != nil {
		return nil, fmt.Errorf("protected branch: bad response")
	}
	for _, l := range b.Push {
		if l.AccessLevel != 0 {
			missing = append(missing, p.DefaultBranch+" allows direct push")
			break
		}
	}
	if b.AllowForcePush {
		missing = append(missing, p.DefaultBranch+" allows force push")
	}
	return missing, nil
}
