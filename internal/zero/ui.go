package zero

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Breee/z3r0/internal/core"
)

// The read-only API and web UI of zero-cp (ADR 0053). They have a listener of
// their own: the gate's port is named in sandbox rules, this one is in none.
const (
	uiPort     = 18097
	uiCookie   = "zero_ui"
	uiCodeFile = "ui.code"
	uiCodeLife = 5 * time.Minute
	uiSession  = 12 * time.Hour
	uiMaxLog   = 200
)

var (
	uiMu       sync.Mutex
	uiSessions = map[string]time.Time{}
)

func uiAddr() string { return fmt.Sprintf("127.0.0.1:%d", uiPort) }

func uiToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// UI writes a code that zero-cp takes once and prints the link that carries it.
func UI(out io.Writer) error {
	if err := ensureServe(); err != nil {
		return err
	}
	code, err := uiToken()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(core.StateRoot(), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(core.StateRoot(), uiCodeFile), []byte(code), 0o600); err != nil {
		return err
	}
	url := fmt.Sprintf("http://%s/ui/enter?code=%s", uiAddr(), code)
	fmt.Fprintf(out, "open (works once, 5 minutes): %s\n", link(out, url))
	exec.Command("xdg-open", url).Run()
	return nil
}

// uiEnter takes the code of `zero ui`: right, not older than five minutes, once.
func uiEnter(code string) bool {
	uiMu.Lock()
	defer uiMu.Unlock()
	path := filepath.Join(core.StateRoot(), uiCodeFile)
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	want, err := os.ReadFile(path)
	os.Remove(path)
	if err != nil || time.Since(st.ModTime()) > uiCodeLife || len(want) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare(want, []byte(code)) == 1
}

func uiOpen() (string, error) {
	id, err := uiToken()
	if err != nil {
		return "", err
	}
	uiMu.Lock()
	defer uiMu.Unlock()
	for k, t := range uiSessions {
		if time.Now().After(t) {
			delete(uiSessions, k)
		}
	}
	uiSessions[id] = time.Now().Add(uiSession)
	return id, nil
}

func uiKnown(r *http.Request) bool {
	c, err := r.Cookie(uiCookie)
	if err != nil {
		return false
	}
	uiMu.Lock()
	defer uiMu.Unlock()
	t, ok := uiSessions[c.Value]
	if ok && !time.Now().Before(t) {
		delete(uiSessions, c.Value)
		return false
	}
	return ok
}

type uiWorkspace struct {
	Name         string   `json:"name"`
	Current      bool     `json:"current"`
	Status       string   `json:"status"`
	Folders      []string `json:"folders"`
	Token        string   `json:"token,omitempty"`
	Projects     []string `json:"projects,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Hosts        []string `json:"hosts,omitempty"`
	Tools        []string `json:"tools,omitempty"`
	Models       []string `json:"models,omitempty"`
	Instructions []string `json:"instructions,omitempty"`
	Skills       []string `json:"skills,omitempty"`
	Branch       string   `json:"branch,omitempty"`
	Policy       string   `json:"policy,omitempty"`
}

type uiPart struct {
	Name  string `json:"name"`
	On    bool   `json:"on"`
	State string `json:"state,omitempty"`
	URL   string `json:"url,omitempty"`
	Note  string `json:"note,omitempty"`
}

func cleanAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, core.Clean(s))
	}
	return out
}

func bases(paths []string) []string {
	var l []string
	for _, p := range paths {
		l = append(l, filepath.Base(p))
	}
	return l
}

func uiWorkspaces() ([]uiWorkspace, string, error) {
	o, err := loadOverview()
	if err != nil {
		return nil, "", err
	}
	var list []uiWorkspace
	for _, row := range o.rows {
		list = append(list, uiDetail(o.c, row))
	}
	return list, strings.Join(o.warn, "; "), nil
}

// uiDetail is what `zero status` shows, as data. No token id, no key.
func uiDetail(c *Config, row wsRow) uiWorkspace {
	w := uiWorkspace{Name: row.Name, Current: row.Current, Status: row.Status, Token: row.Token, Policy: row.Policy}
	ws := c.Workspaces[row.Name]
	if ws == nil {
		return w
	}
	s := c.sandbox(row.Name)
	w.Folders = cleanAll(ws.Folders)
	w.Tools = cleanAll(strings.Fields(s.read("tools")))
	w.Models = cleanAll(ws.Models)
	w.Instructions = cleanAll(bases(s.Instructions))
	w.Skills = cleanAll(bases(s.Skills))
	if s.read("forge.id") != "" {
		w.Projects = cleanAll(s.forgePaths())
	}
	w.Capabilities = cleanAll(strings.Fields(s.read("caps")))
	if b, err := os.ReadFile(s.path("policy.approved")); err == nil {
		if p, err := parsePolicy(b); err == nil {
			w.Hosts = cleanAll(p.Network.Allow)
		}
	}
	if s.read("gate.key") != "" {
		w.Branch = strings.TrimPrefix(sessionBranch(s.Name), "refs/heads/")
	}
	return w
}

// uiLogFields are the fields of a gate log line the UI passes on; nothing else.
var uiLogFields = []string{"time", "decision", "ref", "reason", "method", "path", "old", "new"}

// uiDecisions gives the last lines of a workspace's gate log, newest first.
func uiDecisions(c *Config, name string) []map[string]string {
	if c.Workspaces[name] == nil {
		return nil
	}
	f, err := os.Open(c.sandbox(name).path("gate.log"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var all []map[string]string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var m map[string]string
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		d := map[string]string{}
		for _, k := range uiLogFields {
			if v := m[k]; v != "" {
				d[k] = core.Clean(v)
			}
		}
		all = append(all, d)
		if len(all) > uiMaxLog {
			all = all[1:]
		}
	}
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	return all
}

func uiParts(c *Config) []uiPart {
	gate := uiPart{Name: "push gate", On: true, State: "running"}
	models := uiPart{Name: "model proxy", On: c.Parts.Models != ""}
	if models.On {
		models.State = "ready"
		if p := proxyLogin(); p != "" {
			models.State = p
		}
		// LiteLLM's own page, passed on below uiProxyPath, with LiteLLM's own
		// login: zero cannot sign the user in there (C59).
		models.URL = uiProxyPath + "/ui/"
		models.Note = "its own login: user admin, password in " + filepath.Join(partsDir(), "models.key")
	}
	return []uiPart{gate, models}
}

// uiAllowed keeps the UI to this machine's browser: the right Host name, no
// foreign Origin, and nothing but reads.
func uiAllowed(r *http.Request) (int, string) {
	if r.Host != uiAddr() && r.Host != fmt.Sprintf("localhost:%d", uiPort) {
		return http.StatusForbidden, "wrong host name"
	}
	if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host {
		return http.StatusForbidden, "foreign origin"
	}
	return 0, ""
}

// uiProxyPath is where the model proxy's own page sits on the UI port. It is
// LiteLLM's page with LiteLLM's login, so it is the one place that can change
// something; zero's own pages and API only read.
const uiProxyPath = "/litellm"

var uiProxy = httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", modelPort)})

// uiPassOn hands a request below uiProxyPath to the model proxy, without
// zero's session cookie.
func uiPassOn(w http.ResponseWriter, r *http.Request) {
	var keep []string
	for _, c := range r.Cookies() {
		if c.Name != uiCookie {
			keep = append(keep, c.String())
		}
	}
	r.Header.Del("Cookie")
	if len(keep) > 0 {
		r.Header.Set("Cookie", strings.Join(keep, "; "))
	}
	uiProxy.ServeHTTP(w, r)
}

func uiHandler(w http.ResponseWriter, r *http.Request) {
	if code, why := uiAllowed(r); code != 0 {
		http.Error(w, why, code)
		return
	}
	if r.URL.Path == uiProxyPath || strings.HasPrefix(r.URL.Path, uiProxyPath+"/") {
		if !uiKnown(r) {
			http.Error(w, "no session; run: zero ui", http.StatusUnauthorized)
			return
		}
		if c, err := loadConfig(); err != nil || c.Parts.Models == "" {
			http.NotFound(w, r)
			return
		}
		uiPassOn(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'none'; base-uri 'none'")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "the UI only reads", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path == "/ui/enter" {
		if r.Method != http.MethodGet {
			http.Error(w, "the link is opened, not probed", http.StatusMethodNotAllowed)
			return
		}
		if !uiEnter(r.URL.Query().Get("code")) {
			http.Error(w, "link used or too old; run: zero ui", http.StatusUnauthorized)
			return
		}
		id, err := uiOpen()
		if err != nil {
			http.Error(w, "no session", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: uiCookie, Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: int(uiSession.Seconds())})
		http.Redirect(w, r, "/ui/", http.StatusSeeOther)
		return
	}
	if !uiKnown(r) {
		http.Error(w, "no session; run: zero ui", http.StatusUnauthorized)
		return
	}
	c, err := loadConfig()
	if err != nil {
		http.Error(w, core.Clean(err.Error()), http.StatusInternalServerError)
		return
	}
	name := r.URL.Query().Get("workspace")
	switch r.URL.Path {
	case "/api/workspaces":
		list, warn, err := uiWorkspaces()
		if err != nil {
			http.Error(w, core.Clean(err.Error()), http.StatusInternalServerError)
			return
		}
		uiJSON(w, map[string]any{"workspaces": list, "warning": core.Clean(warn)})
	case "/api/decisions":
		if c.Workspaces[name] == nil {
			http.Error(w, "no such workspace", http.StatusNotFound)
			return
		}
		uiJSON(w, map[string]any{"workspace": name, "decisions": uiDecisions(c, name)})
	case "/api/parts":
		uiJSON(w, map[string]any{"parts": uiParts(c)})
	case "/ui/", "/ui":
		list, warn, err := uiWorkspaces()
		if err != nil {
			http.Error(w, core.Clean(err.Error()), http.StatusInternalServerError)
			return
		}
		uiPage(w, "Workspaces", map[string]any{"List": list, "Warn": core.Clean(warn)})
	case "/ui/workspace":
		list, _, err := uiWorkspaces()
		if err != nil {
			http.Error(w, core.Clean(err.Error()), http.StatusInternalServerError)
			return
		}
		for _, ws := range list {
			if ws.Name == name {
				uiPage(w, "Workspace", map[string]any{"W": ws, "Log": uiDecisions(c, name)})
				return
			}
		}
		http.Error(w, "no such workspace", http.StatusNotFound)
	case "/ui/parts":
		uiPage(w, "Parts", map[string]any{"Parts": uiParts(c)})
	default:
		if strings.HasPrefix(r.URL.Path, "/api/workspace/") {
			name = strings.TrimPrefix(r.URL.Path, "/api/workspace/")
			list, _, err := uiWorkspaces()
			if err != nil {
				http.Error(w, core.Clean(err.Error()), http.StatusInternalServerError)
				return
			}
			for _, ws := range list {
				if ws.Name == name {
					uiJSON(w, ws)
					return
				}
			}
		}
		http.NotFound(w, r)
	}
}

func uiJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func uiPage(w http.ResponseWriter, title string, data map[string]any) {
	data["Title"] = title
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := uiTmpl.ExecuteTemplate(w, title, data); err != nil {
		log.Printf("ui: %v", err)
	}
}

var uiTmpl = template.Must(template.New("ui").Funcs(template.FuncMap{"join": strings.Join}).Parse(`
{{define "head"}}<!doctype html><html lang="en"><head><meta charset="utf-8"><title>zero: {{.Title}}</title>
<style>body{font:15px/1.5 system-ui,sans-serif;margin:2rem auto;max-width:60rem;padding:0 1rem;color:#222}
nav a{margin-right:1rem}table{border-collapse:collapse;width:100%}th,td{text-align:left;padding:.3rem .6rem;border-bottom:1px solid #ddd;vertical-align:top}
th{font-weight:600}code{background:#f3f3f3;padding:0 .2rem}.warn{color:#a40}.dim{color:#777}</style></head><body>
<nav><a href="/ui/">Workspaces</a><a href="/ui/parts">Parts</a></nav><h1>{{.Title}}</h1>{{end}}
{{define "foot"}}<p class="dim">Read-only. Changes are made in zero's config file and with the zero command.</p></body></html>{{end}}

{{define "Workspaces"}}{{template "head" .}}
{{if .Warn}}<p class="warn">{{.Warn}}</p>{{end}}
<table><tr><th>Workspace</th><th>Sandbox</th><th>Directories</th><th>Token</th><th>Policy</th></tr>
{{range .List}}<tr><td><a href="/ui/workspace?workspace={{.Name}}">{{.Name}}</a>{{if .Current}} (current){{end}}</td><td>{{.Status}}</td><td>{{len .Folders}}</td><td>{{.Token}}</td><td>{{.Policy}}</td></tr>
{{else}}<tr><td colspan="5">no workspace yet</td></tr>{{end}}</table>
{{template "foot" .}}{{end}}

{{define "Workspace"}}{{template "head" .}}{{with .W}}
<h2>{{.Name}}</h2>
<table>
<tr><th>Sandbox</th><td>{{.Status}}</td></tr>
<tr><th>Directories</th><td>{{range .Folders}}<code>{{.}}</code><br>{{end}}</td></tr>
<tr><th>Forge token</th><td>{{if .Token}}{{.Token}}{{else}}none{{end}}</td></tr>
<tr><th>Projects it reaches</th><td>{{if .Projects}}{{join .Projects ", "}}{{else}}none{{end}}</td></tr>
<tr><th>Capabilities</th><td>{{if .Capabilities}}{{join .Capabilities ", "}}{{else}}none recorded{{end}}</td></tr>
<tr><th>Pushes to</th><td>{{if .Branch}}<code>{{.Branch}}</code> and below{{else}}nothing{{end}}</td></tr>
<tr><th>Other hosts</th><td>{{if .Hosts}}{{join .Hosts ", "}}{{else}}none{{end}}</td></tr>
<tr><th>Models</th><td>{{if .Models}}{{join .Models ", "}} (through the model proxy){{else}}not through the model proxy{{end}}</td></tr>
<tr><th>Tools</th><td>{{if .Tools}}{{join .Tools ", "}}{{else}}none{{end}}</td></tr>
<tr><th>Instructions</th><td>{{if .Instructions}}{{join .Instructions ", "}}{{else}}none{{end}}</td></tr>
<tr><th>Skills</th><td>{{if .Skills}}{{join .Skills ", "}}{{else}}none{{end}}</td></tr>
<tr><th>Policy</th><td>{{.Policy}}</td></tr>
</table>{{end}}
<h2>Decisions of the push gate</h2>
<table><tr><th>Time</th><th>Decision</th><th>Ref</th><th>Reason</th></tr>
{{range .Log}}<tr><td>{{index . "time"}}</td><td>{{index . "decision"}}</td><td>{{index . "ref"}}{{index . "path"}}</td><td>{{index . "reason"}}</td></tr>
{{else}}<tr><td colspan="4">none recorded</td></tr>{{end}}</table>
{{template "foot" .}}{{end}}

{{define "Parts"}}{{template "head" .}}
<table><tr><th>Part</th><th>On</th><th>State</th><th>Its own page</th></tr>
{{range .Parts}}<tr><td>{{.Name}}</td><td>{{if .On}}yes{{else}}no{{end}}</td><td>{{.State}}</td><td>{{if .URL}}<a href="{{.URL}}" rel="noreferrer">{{.URL}}</a><br><span class="dim">{{.Note}}</span>{{end}}</td></tr>{{end}}</table>
{{template "foot" .}}{{end}}
`))

// serveUI runs the UI listener next to the gate. A failure is logged, the gate goes on.
func serveUI() {
	srv := &http.Server{
		Addr:              uiAddr(),
		Handler:           http.HandlerFunc(uiHandler),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      time.Minute,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    64 << 10,
	}
	log.Printf("zero-cp: read-only API and UI on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Printf("zero-cp: UI listener: %v", err)
	}
}
