package zero

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

//go:embed gitlab-profile.yaml
var gitlabProfileYAML []byte

// gitlabPermissions is the fine-grained scope of the per-sandbox token (D20):
// clone, push branches, open MRs, read pipelines and job logs.
var gitlabPermissions = []string{
	"read_project", "read_code", "download_code", "push_code", "create_branch",
	"create_merge_request", "read_merge_request", "read_job", "read_pipeline",
}

var gitlabRemoteRe = regexp.MustCompile(`^(?:git@gitlab\.com:|ssh://git@gitlab\.com/|https://(?:[^@/]+@)?gitlab\.com/)(.+?)(?:\.git)?/?$`)

// gitlab mints tokens through the host's glab session.
type gitlab struct{}

func (gitlab) Name() string     { return "gitlab" }
func (gitlab) Host() string     { return "gitlab.com" }
func (gitlab) Profile() []byte  { return gitlabProfileYAML }
func (gitlab) TokenEnv() string { return "GITLAB_TOKEN" }
func (gitlab) GitUser() string  { return "oauth2" }

func (gitlab) Match(remote string) (string, bool) {
	m := gitlabRemoteRe.FindStringSubmatch(remote)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func (gitlab) Mint(label, path string) (string, string, error) {
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
			"permissions": gitlabPermissions,
			"project_ids": []int64{proj.ID},
		}},
	})
	cmd := exec.Command("glab", "api", "-X", "POST", "/user/personal_access_tokens",
		"-H", "Content-Type: application/json", "--input", "-")
	cmd.Stdin = strings.NewReader(string(body))
	b, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("mint token: %w", err)
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
