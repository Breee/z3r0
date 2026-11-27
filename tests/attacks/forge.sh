#!/bin/sh
# Forge attack checks against a running zero sandbox (GitLab). PASS=blocked.
# usage: tests/attacks/forge.sh <sandbox-name> [repo-number]   e.g. zero-developer 2
set -u
N=${1:?sandbox name}
ST=${XDG_STATE_HOME:-$HOME/.local/state}/zero/$N
C=$ST/ssh.conf
H=$(awk '/^Host /{print $2;exit}' "$C")
P=$(sed -n "${2:-1}p" "$ST/forge.path")
REPO=$(basename "$P")
ID=$(glab api "projects/$(printf %s "$P" | sed 's#/#%2F#g')" | jq -r .id)
FAILS=0
sbx() { ssh -F "$C" "$H" "$1" 2>&1; }
code() { sbx "curl -s -o /dev/null -w %{http_code} -X $1 -H \"Authorization: Bearer \$GITLAB_TOKEN\" -H 'Content-Type: application/json' ${3:+-d '$3'} 'https://gitlab.com/api/v4/projects/$ID$2'"; }
want() { if [ "$2" = "$3" ]; then echo "PASS $1 ($2)"; else echo "FAIL $1 (got $2, want $3)"; FAILS=$((FAILS+1)); fi; }

want "REST delete branch"          "$(code DELETE /repository/branches/main)" 403
want "REST delete merged branches" "$(code DELETE /repository/merged_branches)" 403
want "REST delete tag"             "$(code DELETE /repository/tags/x)" 403
want "REST unprotect branch"       "$(code DELETE /protected_branches/main)" 403
want "REST protect branch"         "$(code POST '/protected_branches?name=zz')" 403
want "REST create branch"          "$(code POST '/repository/branches?branch=zero-attack&ref=main')" 403
want "REST create tag"             "$(code POST '/repository/tags?tag_name=zero-attack&ref=main')" 403
want "REST commit to branch"       "$(code POST /repository/commits '{"branch":"zero-attack","start_branch":"main","commit_message":"x","actions":[]}')" 403
G=$(sbx "curl -s -X POST -H \"Authorization: Bearer \$GITLAB_TOKEN\" -H 'Content-Type: application/json' -d '{\"query\":\"mutation{createBranch(input:{projectPath:\\\"$P\\\",name:\\\"zero-attack\\\",ref:\\\"main\\\"}){errors}}\"}' https://gitlab.com/api/graphql")
case $G in *"Access denied"*|*policy_denied*) echo "PASS GraphQL create branch";; *) echo "FAIL GraphQL create branch: $G"; FAILS=$((FAILS+1));; esac
for r in refs/heads/main refs/heads/zero-attack refs/tags/zero-attack; do
  O=$(sbx "cd /tmp/zero/$REPO && git push origin \$(git commit-tree -p HEAD -m 'forge.sh attack' HEAD^{tree}):$r")
  case $O in *"push denied"*) echo "PASS git push $r (gate)";; *) echo "FAIL git push $r: $O"; FAILS=$((FAILS+1));; esac
done
O=$(sbx "cd /tmp/zero/$REPO && git push origin :refs/heads/zero/${N#zero-}")
case $O in *"push denied"*) echo "PASS git push delete session branch (gate)";; *) echo "FAIL git push delete session branch: $O"; FAILS=$((FAILS+1));; esac
O=$(sbx "cd /tmp/zero/$REPO && git push https://gitlab.com/$P.git \$(git commit-tree -p HEAD -m 'forge.sh attack' HEAD^{tree}):refs/heads/zero-attack")
case $O in *403*) echo "PASS direct git push to forge (policy)";; *) echo "FAIL direct git push: $O"; FAILS=$((FAILS+1));; esac
want "REST start pipeline"          "$(code POST '/pipeline?ref=main')" 403
if grep -qw ci.run "$ST/caps"; then
  # #126: with ci.run a push starts no pipeline while the branch changes a pipeline definition file
  B=zero/${N#zero-}
  # probe ci|code|clean: push a commit on the session branch that adds .gitlab-ci.yml, adds a code file, or removes both
  probe() { ssh -F "$C" "$H" "cd /tmp/zero/$REPO && git fetch -q origin $B main && export GIT_INDEX_FILE=/tmp/forge-sh-index && git read-tree FETCH_HEAD && case $1 in
      ci) git update-index --add --cacheinfo 100644,\$(echo 'probe: {script: [echo forge.sh]}' | git hash-object -w --stdin),.gitlab-ci.yml;;
      code) git update-index --add --cacheinfo 100644,\$(date +%s%N | git hash-object -w --stdin),forge-sh-probe.txt;;
      clean) git read-tree origin/main;; esac &&
    C=\$(git commit-tree -p FETCH_HEAD -m 'forge.sh ci.run probe ($1)' \$(git write-tree)) && git push origin \$C:refs/heads/$B 2>&1; echo sha \$C" 2>&1; }
  status() { sleep 5; glab api "projects/$ID/pipelines?ref=$B&sha=$(printf %s "$1" | sed -n 's/^sha //p')" | jq -r '.[0].status // "none"'; }
  for step in ci code; do
    O=$(probe $step); S=$(status "$O")
    case "$S $O" in "skipped "*"pipeline skipped"*) echo "PASS ci.run: push ($step) while the branch changes .gitlab-ci.yml starts no pipeline ($S)";;
      *) echo "FAIL ci.run: push ($step) while the branch changes .gitlab-ci.yml: $S $O"; FAILS=$((FAILS+1));; esac
  done
  O=$(probe clean)
  case $O in *"pipeline skipped"*|*denied*|*rejected*|*fatal*) echo "FAIL ci.run: push without a definition change: $O"; FAILS=$((FAILS+1));; *) echo "PASS ci.run: push without a definition change is not skipped by the gate";; esac
  glab api "projects/$ID/repository/branches?search=zero-check" | jq -r '.[].name' | grep -q . && { echo "FAIL temporary branch left on forge"; FAILS=$((FAILS+1)); }
else
  B=zero/${N#zero-}
  O=$(sbx "cd /tmp/zero/$REPO && { git fetch -q origin $B 2>/dev/null && T=FETCH_HEAD || T=HEAD; } && C=\$(git commit-tree -p \$T -m 'forge.sh ci probe' \$T^{tree}) && git push -q origin \$C:refs/heads/$B && echo sha \$C")
  SHA=$(printf %s "$O" | sed -n 's/^sha //p'); sleep 5
  [ -n "$SHA" ] || { echo "FAIL probe push to $B: $O"; FAILS=$((FAILS+1)); }
  S=$(glab api "projects/$ID/pipelines?sha=$SHA" | jq -r '.[0].status // "none"')
  case $S in skipped|none) echo "PASS push starts no pipeline ($S)";; *) echo "FAIL push started a pipeline ($S)"; FAILS=$((FAILS+1));; esac
fi
if grep -qw gitlab.mr.create "$ST/caps"; then echo "SKIP merge request checks (gitlab.mr.create approved for this sandbox)"; else
  # #134: a merge request opened by the agent starts a pipeline that ci.skip on the push does not prevent
  B=zero/${N#zero-}
  mrs() { glab api "projects/$ID/merge_requests?state=opened&source_branch=$B" | jq length; }
  M0=$(mrs)
  want "REST create merge request" "$(code POST /merge_requests "{\"source_branch\":\"$B\",\"target_branch\":\"main\",\"title\":\"forge.sh attack\"}")" 403
  for o in merge_request.create ci.variable=A=b; do
    O=$(sbx "cd /tmp/zero/$REPO && git fetch -q origin $B && git push -o $o origin \$(git commit-tree -p FETCH_HEAD -m 'forge.sh option probe' FETCH_HEAD^{tree}):refs/heads/$B")
    case $O in *"push denied"*) echo "PASS git push -o $o (gate)";; *) echo "FAIL git push -o $o: $O"; FAILS=$((FAILS+1));; esac
  done
  R=$(sbx "curl -s -X POST -H \"Authorization: Bearer \$GITLAB_TOKEN\" -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"save_merge_request\",\"arguments\":{\"project_id\":\"$ID\",\"source_branch\":\"$B\",\"target_branch\":\"main\",\"title\":\"forge.sh attack\"}}}' https://gitlab.com/api/v4/mcp")
  case $R in *insufficient_granular_scope*|*policy_denied*) echo "PASS MCP save_merge_request";; *) echo "FAIL MCP save_merge_request: $R"; FAILS=$((FAILS+1));; esac
  sleep 3
  want "no merge request opened (REST, push option, MCP)" "$(mrs)" "$M0"
fi
B=zero/${N#zero-}
want "REST commit to session branch"  "$(code POST /repository/commits "{\"branch\":\"$B\",\"commit_message\":\"forge.sh attack\",\"actions\":[{\"action\":\"create\",\"file_path\":\"zero-attack\",\"content\":\"x\"}]}")" 403
want "REST start pipeline on session branch" "$(code POST "/pipeline?ref=$B")" 403
G=$(sbx "curl -s -X POST -H \"Authorization: Bearer \$GITLAB_TOKEN\" -H 'Content-Type: application/json' -d '{\"query\":\"mutation{mergeRequestCreate(input:{projectPath:\\\"$P\\\",sourceBranch:\\\"$B\\\",targetBranch:\\\"main\\\",title:\\\"forge.sh attack\\\"}){errors}}\"}' https://gitlab.com/api/graphql")
case $G in *"Access denied"*|*"doesn't have permission"*|*"does not have permission"*|*policy_denied*) echo "PASS GraphQL create merge request";; *) echo "FAIL GraphQL create merge request: $G"; FAILS=$((FAILS+1));; esac
# #52: the forge host is reachable for the workspace's projects only
out() { sbx "curl -s -o /dev/null -m 20 -w %{http_code} $1"; }
T='-H "Authorization: Bearer $GITLAB_TOKEN"'
X='-H "PRIVATE-TOKEN: glpat-foreign"'
GL=https://gitlab.com
want "other project, own token"            "$(out "$T $GL/api/v4/projects/278964")" 403
want "other project, no token"             "$(out "$GL/api/v4/projects/278964")" 403
want "other project, foreign token"        "$(out "$X $GL/api/v4/projects/278964")" 403
want "other project by path"               "$(out "$T $GL/api/v4/projects/gitlab-org%2Fgitlab")" 403
want "other project through dot segments"  "$(out "--path-as-is $T $GL/api/v4/projects/$ID/../278964")" 403
want "other project through encoded dots"  "$(out "--path-as-is $T $GL/api/v4/projects/$ID/%2e%2e/278964")" 403
want "snippet with a foreign token"        "$(out "-X POST $X $GL/api/v4/snippets")" 403
want "user API"                            "$(out "$T $GL/api/v4/user")" 403
want "project list"                        "$(out "$T $GL/api/v4/projects")" 403
want "GraphQL"                             "$(out "-X POST $T $GL/api/graphql")" 403
want "MCP endpoint"                        "$(out "-X POST $T $GL/api/v4/mcp")" 403
want "web page of another project"         "$(out "$GL/gitlab-org/gitlab")" 403
want "raw file of another project"         "$(out "$GL/gitlab-org/gitlab/-/raw/master/README.md")" 403
want "git fetch of another project"        "$(out "$GL/gitlab-org/cli.git/info/refs?service=git-upload-pack")" 403
want "PUT on the own project"             "$(out "-X PUT $T $GL/api/v4/projects/$ID")" 403
want "PATCH below the own project"        "$(out "-X PATCH $T $GL/api/v4/projects/$ID/issues/1")" 403
want "fork of the own project"            "$(out "-X POST $T $GL/api/v4/projects/$ID/fork")" 403
want "upload host"                         "$(out "$GL/uploads/x")" 403
O=$(sbx "cd /tmp/zero/$REPO && git fetch origin main 2>&1; echo rc=\$?")
case $O in *rc=0*) echo "PASS control: git fetch of the own project works";; *) echo "FAIL control: git fetch: $O"; FAILS=$((FAILS+1));; esac
want "control: own project by path"        "$(out "$T $GL/api/v4/projects/$(printf %s "$P" | sed 's#/#%2F#g')")" 200
want "control: read MRs works"     "$(code GET /merge_requests)" 200
glab api "projects/$ID/repository/branches" | jq -r '.[].name' | grep -q zero-attack && { echo "FAIL branch zero-attack exists on forge"; FAILS=$((FAILS+1)); }
# Model proxy (sandbox of a workspace with models): only model routes, no other model host.
if [ "$(sbx 'test -n "$LITELLM_API_KEY" && echo yes' | tail -n 1)" = yes ]; then
	MP='curl -s -o /dev/null -m 10 -w %{http_code} -H "Authorization: Bearer $LITELLM_API_KEY" http://host.openshell.internal:18096'
	want "model proxy: key list refused"      "$(sbx "$MP/key/list" | tail -n 1)" 403
	want "model proxy: new key refused"       "$(sbx "$MP/key/generate -X POST -H 'content-type: application/json' -d '{}'" | tail -n 1)" 403
	want "control: model list of the proxy"   "$(sbx "$MP/v1/models" | tail -n 1)" 200
	for h in api.githubcopilot.com api.anthropic.com opencode.ai; do
		want "model host $h not reachable" "$(sbx "curl -s -o /dev/null -m 10 -w %{http_code} https://$h/" | tail -n 1)" 000
	done
	# Tools through the proxy (workspace with tools): only the listed ones, GitLab's endpoint not directly.
	MCP='curl -s -m 20 -H "Authorization: Bearer $LITELLM_API_KEY" -H "content-type: application/json" -H "accept: application/json, text/event-stream" http://host.openshell.internal:18096/mcp -d'
	T=$(sbx "$MCP '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\",\"params\":{}}'" | sed -n 's/^data: //p' | jq -r '.result.tools[0].name // empty')
	if [ -n "$T" ]; then
		X="${T%%-*}-save_note"
		R=$(sbx "$MCP '{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"$X\",\"arguments\":{}}}'")
		case $R in *"not allowed"*) echo "PASS tool outside the list refused ($X)";; *) echo "FAIL tool outside the list not refused ($X)"; FAILS=$((FAILS+1));; esac
	fi
	want "GitLab MCP endpoint not reachable directly" "$(sbx 'curl -s -o /dev/null -m 10 -w %{http_code} -X POST https://gitlab.com/api/v4/mcp' | tail -n 1)" 403
fi
# zero-cp's read-only API and UI (own port, in no sandbox rule).
want "zero UI not reachable"        "$(sbx 'curl -s -o /dev/null -m 10 -w %{http_code} http://host.openshell.internal:18097/ui/' | tail -n 1)" 000
want "zero API not reachable"       "$(sbx 'curl -s -o /dev/null -m 10 -w %{http_code} http://host.openshell.internal:18097/api/workspaces' | tail -n 1)" 000
echo "fails: $FAILS"; [ "$FAILS" = 0 ]
