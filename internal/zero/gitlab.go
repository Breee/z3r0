package zero

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
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

// gitlabPath returns the project path of a gitlab.com origin, or "".
func gitlabPath(remote string) string {
	m := gitlabRemoteRe.FindStringSubmatch(strings.TrimSpace(remote))
	if m == nil {
		return ""
	}
	return m[1]
}

func (s *Sandbox) gitlabProvider() string { return s.Name + "-gitlab" }

// setupGitLab mints the token and stores it as an OpenShell provider. It
// returns the provider name, or "" when origin is not on gitlab.com.
func (s *Sandbox) setupGitLab() (string, error) {
	origin, _ := sh("git", "-C", s.Top, "remote", "get-url", "origin")
	path := gitlabPath(origin)
	if path == "" {
		return "", nil
	}
	if err := importGitLabProfile(); err != nil {
		return "", err
	}
	// A leftover token from a failed create is revoked before minting anew.
	if err := s.revokeGitLab(); err != nil {
		return "", err
	}
	out, err := sh("glab", "api", "projects/"+url.PathEscape(path))
	if err != nil {
		return "", fmt.Errorf("gitlab project %s: %w", path, err)
	}
	var proj struct{ ID int64 }
	if err := json.Unmarshal([]byte(out), &proj); err != nil || proj.ID == 0 {
		return "", fmt.Errorf("gitlab project %s: bad response", path)
	}
	tok, id, err := mintGitLabToken(s.Name, proj.ID)
	if err != nil {
		return "", err
	}
	if err := s.write("gitlab.id", fmt.Sprint(id), 0o600); err != nil {
		return "", err
	}
	if err := s.write("gitlab.path", path, 0o600); err != nil {
		return "", err
	}
	sh("openshell", "provider", "delete", s.gitlabProvider())
	// The token goes in via env lookup, never in argv.
	cmd := exec.Command("openshell", "provider", "create", "--name", s.gitlabProvider(),
		"--type", "zero-gitlab", "--credential", "GITLAB_TOKEN")
	cmd.Env = append(os.Environ(), "GITLAB_TOKEN="+tok)
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("provider create: %v: %s", err, strings.TrimSpace(string(b)))
	}
	return s.gitlabProvider(), nil
}

func importGitLabProfile() error {
	f, err := os.CreateTemp("", "zero-gitlab-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(gitlabProfileYAML); err != nil {
		return err
	}
	f.Close()
	// An existing profile is kept; after editing gitlab-profile.yaml run
	// `openshell profile delete zero-gitlab` once.
	if _, err := sh("openshell", "profile", "import", "-f", f.Name()); err != nil &&
		!strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("import gitlab profile: %w", err)
	}
	return nil
}

func mintGitLabToken(name string, projectID int64) (string, int64, error) {
	body, _ := json.Marshal(map[string]any{
		"name":       name,
		"expires_at": time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02"),
		"granular_scopes": []map[string]any{{
			"access":      "selected_memberships",
			"permissions": gitlabPermissions,
			"project_ids": []int64{projectID},
		}},
	})
	cmd := exec.Command("glab", "api", "-X", "POST", "/user/personal_access_tokens",
		"-H", "Content-Type: application/json", "--input", "-")
	cmd.Stdin = strings.NewReader(string(body))
	b, err := cmd.Output()
	if err != nil {
		return "", 0, fmt.Errorf("mint gitlab token: %w", err)
	}
	var r struct {
		ID    int64
		Token string
	}
	if err := json.Unmarshal(b, &r); err != nil || r.Token == "" {
		return "", 0, fmt.Errorf("mint gitlab token: bad response")
	}
	return r.Token, r.ID, nil
}

// revokeGitLab revokes the stored token and deletes the provider. Revoking an
// already expired or revoked token is not an error.
func (s *Sandbox) revokeGitLab() error {
	id := s.read("gitlab.id")
	if id == "" {
		return nil
	}
	if _, err := sh("glab", "api", "-X", "DELETE", "/personal_access_tokens/"+id); err != nil &&
		!strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "401") {
		return fmt.Errorf("revoke gitlab token %s: %w", id, err)
	}
	sh("openshell", "provider", "delete", s.gitlabProvider())
	os.Remove(s.path("gitlab.id"))
	return nil
}

// configureGitLabRemote points origin in the sandbox at https and makes git
// send the placeholder from GITLAB_TOKEN; the proxy substitutes the token.
func (s *Sandbox) configureGitLabRemote() error {
	path := s.read("gitlab.path")
	if path == "" {
		return nil
	}
	script := "cd " + shellQuote(s.Dir) + " && " +
		"git remote set-url origin " + shellQuote("https://gitlab.com/"+path+".git") + " && " +
		"git config credential.helper " +
		shellQuote(`!f() { echo username=oauth2; echo "password=$GITLAB_TOKEN"; }; f`)
	if _, err := s.ssh(script); err != nil {
		return fmt.Errorf("configure gitlab remote: %w", err)
	}
	return nil
}
