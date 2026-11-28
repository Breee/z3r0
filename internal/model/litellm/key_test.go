package litellm

import (
	"encoding/json"
	"testing"
	"time"
)

// keyList is what /key/list answers for a key made from req.
func keyList(t *testing.T, req map[string]any, expires time.Time, blocked bool, n int) []byte {
	t.Helper()
	k := map[string]any{"expires": expires, "blocked": blocked, "models": req["models"], "object_permission": req["object_permission"], "user_id": User, "team_id": nil, "allowed_routes": []string{}}
	keys := []any{}
	for i := 0; i < n; i++ {
		keys = append(keys, k)
	}
	b, err := json.Marshal(map[string]any{"keys": keys})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestKeyState(t *testing.T) {
	in := time.Now().Add(48 * time.Hour)
	for _, c := range []struct {
		models []string
		entry  string
		tools  []string
	}{
		{[]string{"github_copilot/*"}, "", nil},
		{[]string{"a/b", "c/*"}, "zero-x", []string{"list_issues", "get_issue"}},
		{[]string{"a/b"}, "zero-x", nil},
	} {
		req := KeyRequest("alias", c.models, c.entry, c.tools)
		left, allows := keyState(keyList(t, req, in, false, 1))
		if left < 47*time.Hour {
			t.Errorf("%v: left %v", c, left)
		}
		if want := KeyAllows(c.models, c.entry, c.tools); allows != want {
			t.Errorf("%v: key allows %s, request says %s", c, allows, want)
		}
	}
	req := KeyRequest("alias", []string{"a/b"}, "zero-x", []string{"t1"})
	_, allows := keyState(keyList(t, req, in, false, 1))
	for name, other := range map[string]string{
		"models": KeyAllows([]string{"a/b", "c/d"}, "zero-x", []string{"t1"}),
		"tools":  KeyAllows([]string{"a/b"}, "zero-x", []string{"t1", "t2"}),
		"entry":  KeyAllows([]string{"a/b"}, "", nil),
	} {
		if allows == other {
			t.Errorf("a change of %s is not seen", name)
		}
	}
	// the shape the proxy answers with (LiteLLM, 2026-11-28), and widened keys
	real := func(user, team, extra string) []byte {
		return []byte(`{"keys":[{"expires":"` + in.Format(time.RFC3339) + `","blocked":null,"models":["github_copilot/*"],"user_id":"` + user + `","team_id":` + team + `,"allowed_routes":[],"object_permission":{"agent_access_groups":[],"agents":[],"blocked_tools":[],"mcp_access_groups":[` + extra + `],"mcp_servers":["no-mcp-servers"],"mcp_tool_permissions":null,"mcp_tool_search_enabled":null,"mcp_toolsets":[],"models":[],"object_permission_id":"7cb2","search_tools":[],"skills":[],"vector_stores":[]}}],"total_count":1}`)
	}
	if left, allows := keyState(real(User, "null", "")); left < 47*time.Hour || allows != KeyAllows([]string{"github_copilot/*"}, "", nil) {
		t.Errorf("real answer: left %v allows %s", left, allows)
	}
	if a, b := KeyAllows([]string{"b", "a"}, "e", []string{"y", "x"}), KeyAllows([]string{"a", "b"}, "e", []string{"x", "y"}); a != b {
		t.Errorf("order changes the sum: %s %s", a, b)
	}
	for name, b := range map[string][]byte{
		"another owner": real("someone", "null", ""),
		"a team":        real(User, `"t1"`, ""),
		"access group":  real(User, "null", `"all-tools"`),
		"blocked":       keyList(t, req, in, true, 1),
		"two keys":      keyList(t, req, in, false, 2),
		"no key":        keyList(t, req, in, false, 0),
		"garbage":       []byte("{"),
	} {
		if left, allows := keyState(b); left != 0 || allows != "" {
			t.Errorf("%s: left %v allows %q, want none", name, left, allows)
		}
	}
}
