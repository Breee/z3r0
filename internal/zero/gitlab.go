package zero

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Breee/z3r0/internal/core"

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
	capCIRun:               {"read_repository"}, // the gate compares the pushed commit with the default branch (#126)
	"gitlab.issue.read":    {"read_work_item"},
	"gitlab.mr.read":       {"read_merge_request"},
	"gitlab.mr.create":     {"create_merge_request"},
	"gitlab.pipeline.read": {"read_pipeline", "read_job"},
}

// gitlabTools maps the tools of GitLab's MCP endpoint that zero offers to
// the fine-grained token permissions GitLab checks for them (ADR 0043,
// MG14; checked with pocs/15-gitlab-mcp/tools-check.sh).
var gitlabTools = map[string][]string{
	"get_project":          {"read_project"},
	"get_repository_file":  {"read_project", "read_code", "read_repository"},
	"list_repository_tree": {"read_project", "read_code", "read_repository"},
	"list_branches":        {"read_branch"},
	"list_tags":            {"read_repository_tag"},
	"list_commits":         {"read_project", "read_code"},
	"get_commit":           {"read_project", "read_code"},
	"list_merge_requests":  {"read_project", "read_merge_request"},
	"get_merge_request":    {"read_project", "read_merge_request"},
	"list_pipelines":       {"read_pipeline"},
	"get_pipeline":         {"read_project", "read_pipeline"},
	"get_job":              {"read_job"},
	"list_work_items":      {"read_work_item", "read_namespace"},
	"get_work_item":        {"read_work_item"},
	"search_labels":        {"read_project"},
}

// gitlabDefaultTools is the tool list of a workspace that sets none.
var gitlabDefaultTools = []string{
	"get_project", "get_repository_file", "list_repository_tree", "list_branches",
	"list_tags", "list_commits", "get_commit", "list_merge_requests",
	"get_merge_request", "list_pipelines", "get_pipeline", "get_job",
	"list_work_items", "get_work_item", "search_labels",
}

// gitlabToolPermissions returns the token permissions the tools need.
func gitlabToolPermissions(tools []string) ([]string, error) {
	var perms []string
	for _, t := range tools {
		p, ok := gitlabTools[t]
		if !ok {
			return nil, fmt.Errorf("unknown gitlab tool %q", t)
		}
		for _, x := range p {
			if !slices.Contains(perms, x) {
				perms = append(perms, x)
			}
		}
	}
	return perms, nil
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

// The profile holds the credential only; the endpoints are in the sandbox's
// policy (forgePolicy), so one profile serves every instance.
func (gitlab) ProfileID() string { return "zero-gitlab-token" }
func (gitlab) Profile() []byte   { return gitlabProfileYAML }

// Egress limits the forge host to the workspace's projects: the REST API by
// project id and by encoded path, and git fetch. Everything else on the host
// is refused: other projects, user and group API, GraphQL, MCP, snippets,
// web pages (#52). Pushes go through the push gate only (#95).
func (g gitlab) Egress(paths []string) (allow, deny []EgressRule, err error) {
	for _, path := range paths {
		if strings.ContainsAny(path, "*?[]{}\\") {
			return nil, nil, fmt.Errorf("project path %q: not usable in an egress rule", path)
		}
		id, err := g.projectID(path)
		if err != nil {
			return nil, nil, err
		}
		allow = append(allow, gitlabProjectRules(path, id)...)
	}
	deny = []EgressRule{
		{Method: "*", Path: "/**/git-receive-pack"},
		{Method: "GET", Path: "/**/info/refs", Query: map[string]string{"service": "git-receive-pack"}},
	}
	return allow, deny, nil
}

func gitlabProjectRules(path string, id int64) (allow []EgressRule) {
	for _, p := range []string{fmt.Sprint(id), url.PathEscape(path)} {
		base := "/api/v4/projects/" + p
		allow = append(allow, EgressRule{Method: "GET", Path: base})
		for _, m := range []string{"GET", "HEAD", "POST"} {
			allow = append(allow, EgressRule{Method: m, Path: base + "/**"})
		}
	}
	return append(allow,
		EgressRule{Method: "GET", Path: "/" + path + ".git/info/refs"},
		EgressRule{Method: "POST", Path: "/" + path + ".git/git-upload-pack"})
}

func (g gitlab) projectID(path string) (int64, error) {
	out, err := g.glab("api", "projects/"+url.PathEscape(path))
	if err != nil {
		return 0, fmt.Errorf("project %s: %w", path, err)
	}
	var proj struct{ ID int64 }
	if err := json.Unmarshal([]byte(out), &proj); err != nil || proj.ID == 0 {
		return 0, fmt.Errorf("project %s: bad response", path)
	}
	return proj.ID, nil
}

// glab sends PATs as PRIVATE-TOKEN, which OpenShell >= 0.1.8 does not
// substitute; as OAuth it sends Authorization: Bearer (ADR 0026, C46, #65).
func (gitlab) Env() []string { return []string{"GLAB_IS_OAUTH2=true"} }

func (g gitlab) LoggedIn() error {
	if _, err := core.Sh("glab", "auth", "status", "--hostname", g.host); err != nil {
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
	return core.Sh("glab", append(args, "--hostname", g.host)...)
}

func (gitlab) Permissions(caps, tools []string) ([]string, error) {
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
	tp, err := gitlabToolPermissions(tools)
	if err != nil {
		return nil, err
	}
	for _, x := range tp {
		if !slices.Contains(perms, x) {
			perms = append(perms, x)
		}
	}
	return perms, nil
}

func (g gitlab) Mint(label string, paths []string, perms []string) (string, string, error) {
	var ids []int64
	for _, path := range paths {
		id, err := g.projectID(path)
		if err != nil {
			return "", "", err
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return "", "", fmt.Errorf("mint token: no project")
	}
	body, _ := json.Marshal(map[string]any{
		"name":       label,
		"expires_at": time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02"),
		"granular_scopes": []map[string]any{{
			"access":      "selected_memberships",
			"permissions": perms,
			"project_ids": ids,
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

func (g gitlab) DefinitionChanged(token, path, sha string) (string, error) {
	return gitlabDefinitionChanged("https://"+g.host+"/api/v4", token, path, sha)
}

// maxCompareFiles: a comparison with this many files or more counts as not
// known. gitlab.com listed 633 files in full and cut at 3000 with
// compare_timeout set (2026-11-23); the margin is for instances set lower.
const maxCompareFiles = 300

// gitlabDefinitionChanged compares sha with the project's default branch from
// their merge base, so the definition the pipeline starts from was on the
// default branch once.
func gitlabDefinitionChanged(api, token, path, sha string) (string, error) {
	get := func(rest string, v any) error {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+"/projects/"+url.PathEscape(path)+rest, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("forge not reached")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("forge answered %d", resp.StatusCode)
		}
		return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(v)
	}
	var p struct {
		DefaultBranch string `json:"default_branch"`
		CIConfigPath  string `json:"ci_config_path"`
	}
	if err := get("", &p); err != nil {
		return "", err
	}
	if p.DefaultBranch == "" {
		return "", fmt.Errorf("no default branch")
	}
	var c struct {
		Timeout bool `json:"compare_timeout"`
		Diffs   []struct {
			Old string `json:"old_path"`
			New string `json:"new_path"`
		} `json:"diffs"`
	}
	q := url.Values{"from": {p.DefaultBranch}, "to": {sha}, "straight": {"false"}}
	if err := get("/repository/compare?"+q.Encode(), &c); err != nil {
		return "", err
	}
	if c.Timeout || len(c.Diffs) >= maxCompareFiles {
		return "", fmt.Errorf("comparison incomplete")
	}
	for _, d := range c.Diffs {
		for _, f := range []string{d.Old, d.New} {
			if gitlabDefinitionFile(f, p.CIConfigPath) {
				return f, nil
			}
		}
	}
	return "", nil
}

// gitlabDefinitionFile reports whether a change of file can change what a
// pipeline does before any job runs: the project's CI config file, anything
// under .gitlab/, and every YAML file (a local include must be one).
func gitlabDefinitionFile(file, ciConfig string) bool {
	if ciConfig = strings.TrimPrefix(ciConfig, "/"); ciConfig == "" {
		ciConfig = ".gitlab-ci.yml"
	}
	l := strings.ToLower(file)
	return file == ciConfig || strings.HasPrefix(file, ".gitlab/") ||
		strings.HasSuffix(l, ".yml") || strings.HasSuffix(l, ".yaml")
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
