#!/usr/bin/env bash
# PoC 11 (#93): push service limits the agent to its session branch.
# Uses the active podman gateway and a real GitLab project. Mints one
# fine-grained token (glab), binds it to a sandbox, starts bin/zero-push on the
# host. Cleans up sandbox, provider, token, branch and MR.
# Needs: go, glab (logged in), openshell, jq.
# Usage: [GATEWAY=openshell] [PROJECT=group/project] run.sh
set -u
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
PROJECT=${PROJECT:-corewire/technology-and-research/developer}
IMAGE=${IMAGE:-$(sed -n 's/^const devImage = "\(.*\)"/\1/p' "$ROOT/internal/zero/zero.go")}
O="openshell -g ${GATEWAY:-openshell}"
PORT=18095
BRANCH=zero/poc11-$$
GITLAB=https://gitlab.com/$PROJECT.git
PUSH=http://host.openshell.internal:$PORT/$PROJECT.git
W=$(mktemp -d)
FAILED=0

pass() { echo "PASS $1"; }
fail() { echo "FAIL $1"; FAILED=1; }
check() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1 (got $2, want $3)"; fi; }
sbx() { $O sandbox exec --name push-a -- sh -c "$1" 2>&1; }
remote_has() { glab api "projects/$PID_/repository/branches/$(jq -rn --arg b "$1" '$b|@uri')" >/dev/null 2>&1 && echo yes || echo no; }

cleanup() {
    $O sandbox delete push-a >/dev/null 2>&1
    sleep 3
    $O provider delete push-poc >/dev/null 2>&1
    [ -z "${TID:-}" ] || glab api -X DELETE "/personal_access_tokens/$TID" >/dev/null 2>&1
    for mr in $(glab api "projects/$PID_/merge_requests?state=opened&source_branch=$BRANCH" 2>/dev/null | jq -r '.[].iid'); do
        glab api -X PUT "projects/$PID_/merge_requests/$mr" -f state_event=close >/dev/null 2>&1
    done
    glab api -X DELETE "projects/$PID_/repository/branches/$(jq -rn --arg b "$BRANCH" '$b|@uri')" >/dev/null 2>&1
    kill "${SVC:-}" 2>/dev/null
    rm -rf "$W"
}
trap cleanup EXIT

(cd "$ROOT" && go build -o bin/zero-push ./cmd/zero-push) || exit 1
PID_=$(glab api "projects/$(jq -rn --arg p "$PROJECT" '$p|@uri')" | jq -r .id)
[ -n "$PID_" ] && [ "$PID_" != null ] || { echo "project $PROJECT not found"; exit 1; }

"$ROOT/bin/zero-push" -addr "${BIND:-0.0.0.0}:$PORT" -repo "/$PROJECT.git" -ref "refs/heads/$BRANCH" -session poc11 -log "$W/push.log" &
SVC=$!

# One token, same permissions zero mints today (includes push_code).
TOK=$(jq -n --argjson id "$PID_" --arg exp "$(date -u -d tomorrow +%F)" '{name:"zero-poc11", expires_at:$exp,
    granular_scopes:[{access:"selected_memberships", project_ids:[$id], permissions:
    ["read_project","read_code","download_code","push_code","create_branch","create_merge_request","read_merge_request","read_job","read_pipeline"]}]}' |
    glab api -X POST /user/personal_access_tokens -H 'Content-Type: application/json' --input -)
TID=$(echo "$TOK" | jq -r .id)
[ "$TID" != null ] || { echo "mint failed: $TOK"; exit 1; }

$O profile import --file "$(dirname "$0")/profile.yaml" >/dev/null 2>&1
GITLAB_TOKEN=$(echo "$TOK" | jq -r .token) $O provider create --name push-poc --type zero-gitlab-push --credential GITLAB_TOKEN >/dev/null || exit 1
unset TOK
$O sandbox create --name push-a --from "$IMAGE" --policy "$ROOT/internal/zero/policy.yaml" --provider push-poc \
    --env GLAB_IS_OAUTH2=true --detach >/dev/null || exit 1
sleep 10

HELPER='!f() { echo username=oauth2; echo "password=$GITLAB_TOKEN"; }; f'
OUT=$(sbx "git -c credential.helper='$HELPER' clone -q $GITLAB /tmp/dev && cd /tmp/dev &&
    git config credential.helper '$HELPER' && git config user.name poc && git config user.email poc@example.com &&
    git remote set-url --push origin $PUSH && git checkout -q -b $BRANCH && git commit -q --allow-empty -m 'poc 11' && echo cloned")
case $OUT in *cloned) pass "clone/fetch directly from gitlab.com";; *) fail "clone: $OUT";; esac

check "glab API works in sandbox" "$(sbx "cd /tmp/dev && glab api projects/$PID_" | jq -r .id)" "$PID_"

OUT=$(sbx "cd /tmp/dev && git push $GITLAB HEAD:refs/heads/zero/direct-$$")
check "direct push to gitlab.com blocked" "$(remote_has "zero/direct-$$")" no
echo "     $(echo "$OUT" | tail -1 | cut -c1-150)"

for spec in "HEAD:refs/heads/zero/other-$$" "HEAD:refs/heads/main" "HEAD:refs/heads/$BRANCH HEAD:refs/heads/zero/other-$$"; do
    OUT=$(sbx "cd /tmp/dev && git push origin $spec")
    case $OUT in *"zero-push: denied"*) pass "service denies: $spec";; *) fail "service denies: $spec: $OUT";; esac
done
check "foreign branch not created" "$(remote_has "zero/other-$$")" no
check "session branch not created by mixed push" "$(remote_has "$BRANCH")" no

OUT=$(sbx "cd /tmp/dev && git push -o merge_request.create -o merge_request.title=poc11 origin HEAD:refs/heads/$BRANCH")
check "session branch pushed through the service" "$(remote_has "$BRANCH")" yes
check "push option merge_request.create passes" "$(glab api "projects/$PID_/merge_requests?state=opened&source_branch=$BRANCH" | jq length)" 1

sbx "cd /tmp/dev && git commit -q --allow-empty -m two && git push origin HEAD:refs/heads/$BRANCH" >/dev/null
check "update of session branch" "$(glab api "projects/$PID_/repository/branches/$(jq -rn --arg b "$BRANCH" '$b|@uri')" | jq -r .commit.title)" two

OUT=$(sbx "cd /tmp/dev && git push origin :refs/heads/$BRANCH")
case $OUT in *"delete not allowed"*) pass "service denies delete";; *) fail "service denies delete: $OUT";; esac
check "session branch still exists" "$(remote_has "$BRANCH")" yes

SEEN=$(sbx 'printf %s "$GITLAB_TOKEN"')
case $SEEN in openshell:resolve:*) pass "agent sees only the placeholder";; *) fail "agent sees: $SEEN";; esac
check "decision log: allows" "$(grep -c '"decision":"allow"' "$W/push.log")" 2
check "decision log: denies" "$(grep -c '"decision":"deny"' "$W/push.log")" 5
echo "--- decision log"
jq -c '{ref, decision, reason}' "$W/push.log"

exit $FAILED
