#!/bin/sh
# Forge attack checks against a running zero sandbox (GitLab). PASS=blocked.
# usage: tests/attacks/forge.sh <sandbox-name>     e.g. zero-developer
set -u
N=${1:?sandbox name}
ST=${XDG_STATE_HOME:-$HOME/.local/state}/zero/$N
C=$ST/ssh.conf
H=$(awk '/^Host /{print $2;exit}' "$C")
P=$(cat "$ST/forge.path")
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
case $G in *"Access denied"*) echo "PASS GraphQL create branch (token)";; *) echo "FAIL GraphQL create branch: $G"; FAILS=$((FAILS+1));; esac
for r in refs/heads/main refs/heads/zero-attack refs/tags/zero-attack; do
  O=$(sbx "cd /tmp/zero/$REPO && git push origin HEAD:$r")
  case $O in *"push denied"*) echo "PASS git push $r (gate)";; *) echo "FAIL git push $r: $O"; FAILS=$((FAILS+1));; esac
done
O=$(sbx "cd /tmp/zero/$REPO && git push https://gitlab.com/$P.git HEAD:refs/heads/zero-attack")
case $O in *403*) echo "PASS direct git push to forge (policy)";; *) echo "FAIL direct git push: $O"; FAILS=$((FAILS+1));; esac
want "REST start pipeline"          "$(code POST '/pipeline?ref=main')" 403
if grep -qw ci.run "$ST/caps"; then echo "SKIP ci.skip check (ci.run approved for this sandbox)"; else
  B=zero/${N#zero-}
  O=$(sbx "cd /tmp/zero/$REPO && git commit -q --allow-empty -m 'forge.sh ci probe' && git push origin HEAD:refs/heads/$B && git rev-parse HEAD")
  SHA=$(printf %s "$O" | tail -1); sleep 5
  S=$(glab api "projects/$ID/pipelines?sha=$SHA" | jq -r '.[0].status // "none"')
  case $S in skipped|none) echo "PASS push starts no pipeline ($S)";; *) echo "FAIL push started a pipeline ($S)"; FAILS=$((FAILS+1));; esac
fi
want "control: read MRs works"     "$(code GET /merge_requests)" 200
glab api "projects/$ID/repository/branches" | jq -r '.[].name' | grep -q zero-attack && { echo "FAIL branch zero-attack exists on forge"; FAILS=$((FAILS+1)); }
echo "fails: $FAILS"; [ "$FAILS" = 0 ]
