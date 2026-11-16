package zero

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

//go:embed gitlab-profile.yaml
var gitlabProfileYAML []byte

// gitlabCaps maps policy capabilities to fine-grained token permissions
// (D20, docs/architecture/07a). git.commit is local only.
var gitlabCaps = map[string][]string{
	"git.read":             {"read_project", "read_code", "download_code"},
	"git.commit":           {},
	"git.push":             {"push_code"},
	capCIRun:               {},
	"gitlab.issue.read":    {"read_work_item"},
	"gitlab.mr.read":       {"read_merge_request"},
	"gitlab.mr.create":     {"create_merge_request"},
	"gitlab.pipeline.read": {"read_pipeline", "read_job"},
}

// remoteRe splits an origin URL into host and project path.
var remoteRe = regexp.MustCompile(`^(?:git@([^:/]+):|ssh://git@([^:/]+)(?::\d+)?/|https://(?:[^@/]+@)?([^/]+)/)(.+?)(?:\.git)?/?$`)

func parseRemote(remote string) (host, path string, ok bool) {
	m := remoteRe.FindStringSubmatch(remote)
	if m == nil {
		return "", "", false
	}
	return m[1] + m[2] + m[3], m[4], true
}

const gitlabCom = "gitlab.com"

// gitlab mints tokens through the host's glab session for one GitLab
// instance: gitlab.com or any host glab is logged in to (ADR 0038).
type gitlab struct{ host string }

func (gitlab) Name() string     { return "gitlab" }
func (g gitlab) Host() string   { return g.host }
func (gitlab) TokenEnv() string { return "GITLAB_TOKEN" }
func (gitlab) GitUser() string  { return "oauth2" }
func (g gitlab) MCP() string    { return "https://" + g.host + "/api/v4/mcp" }

// Profiles are per instance: the endpoint host is part of the profile.
func (g gitlab) ProfileID() string {
	if g.host == gitlabCom {
		return "zero-gitlab"
	}
	return "zero-gitlab-" + regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(g.host), "-")
}

func (g gitlab) Profile() []byte {
	y := strings.Replace(string(gitlabProfileYAML), "id: zero-gitlab\n", "id: "+g.ProfileID()+"\n", 1)
	return []byte(strings.ReplaceAll(y, "host: "+gitlabCom+"\n", "host: "+g.host+"\n"))
}

// glab sends PATs as PRIVATE-TOKEN, which OpenShell >= 0.1.8 does not
// substitute; as OAuth it sends Authorization: Bearer (ADR 0026, C46, #65).
func (gitlab) Env() []string { return []string{"GLAB_IS_OAUTH2=true"} }

func (g gitlab) LoggedIn() error {
	if _, err := sh("glab", "auth", "status", "--hostname", g.host); err != nil {
		return fmt.Errorf("glab is not logged in to %s; run: glab auth login --hostname %s --use-keyring", g.host, g.host)
	}
	return nil
}

// PlaintextLogin reports whether glab keeps this host's token in its config
// file instead of the OS keyring.
func (g gitlab) PlaintextLogin() bool {
	dir, err := os.UserConfigDir()
	if err != nil {
		return false
	}
	b, err := os.ReadFile(filepath.Join(dir, "glab-cli", "config.yml"))
	if err != nil {
		return false
	}
	var c struct {
		Hosts map[string]struct {
			// glab writes "token: !!null <value>"; only the raw node keeps the value.
			Token yaml.Node `yaml:"token"`
		} `yaml:"hosts"`
	}
	return yaml.Unmarshal(b, &c) == nil && c.Hosts[g.host].Token.Value != ""
}

func (g gitlab) glab(args ...string) (string, error) {
	return sh("glab", append(args, "--hostname", g.host)...)
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

func (g gitlab) Mint(label, path string, perms []string) (string, string, error) {
	out, err := g.glab("api", "projects/"+url.PathEscape(path))
	if err != nil {
		return "", "", fmt.Errorf("project %s: %w", path, err)
	}
	var proj struct{ ID int64 }
	if err := json.Unmarshal([]byte(out), &proj); err != nil || proj.ID == 0 {
		return "", "", fmt.Errorf("project %s: bad response", path)
	}
	body, _ := json.Marshal(map[string]any{
		"name":       label,
		"expires_at": time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02"),
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
		"-H", "Content-Type: application/json", "--input", "-", "--hostname", g.host)
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

func (g gitlab) Revoke(id string) error {
	_, err := g.glab("api", "-X", "DELETE", "/personal_access_tokens/"+id)
	if err != nil && !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "401") {
		return err
	}
	return nil
}

func (g gitlab) Tokens() ([]Token, error) {
	out, err := g.glab("api", "--paginate", "/personal_access_tokens?state=active&per_page=100")
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	var list []struct {
		ID        int64
		Name      string
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return nil, fmt.Errorf("list tokens: bad response")
	}
	var toks []Token
	for _, t := range list {
		// GitLab tokens expire at the start of expires_at (UTC).
		exp, _ := time.Parse("2006-01-02", t.ExpiresAt)
		toks = append(toks, Token{ID: fmt.Sprint(t.ID), Name: t.Name, Expires: exp})
	}
	return toks, nil
}

func (g gitlab) Guardrails(path string) ([]string, error) {
	out, err := g.glab("api", "projects/"+url.PathEscape(path))
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
	out, err = g.glab("api", "projects/"+url.PathEscape(path)+"/protected_branches/"+url.PathEscape(p.DefaultBranch))
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
	// A pipeline on an agent branch can read every CI variable that is not protected (P14).
	if out, err = g.glab("api", "--paginate", "projects/"+url.PathEscape(path)+"/variables"); err == nil {
		var vars []struct {
			Key       string `json:"key"`
			Protected bool   `json:"protected"`
		}
		if json.Unmarshal([]byte(out), &vars) == nil {
			for _, v := range vars {
				if !v.Protected {
					missing = append(missing, "CI variable "+v.Key+" not protected")
				}
			}
		}
	}
	return missing, nil
}
