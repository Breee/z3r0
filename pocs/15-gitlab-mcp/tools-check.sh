#!/usr/bin/env bash
# Mints a token with the given permissions for one project and calls every
# tool of default-tools.txt on the instance MCP endpoint (no gateway).
# Prints ok, empty (an empty list: no data, or GitLab hiding it) or refused.
# usage: tools-check.sh PERMISSION...   env: FORGE_HOST, PROJECT, TOOLS (default: all of default-tools.txt)
set -euo pipefail
HOST=${FORGE_HOST:-gitlab.com}
PROJECT=${PROJECT:-corewire/technology-and-research/developer}
HERE=$(cd "$(dirname "$0")" && pwd)
api() { glab api "$@" --hostname "$HOST"; }
enc=$(jq -rn --arg p "$PROJECT" '$p|@uri')
PID=$(api "projects/$enc" | jq .id)
REF=$(api "projects/$PID" | jq -r .default_branch)
SHA=$(api "projects/$PID/repository/commits?per_page=1" | jq -r '.[0].id // empty')
MR=$(api "projects/$PID/merge_requests?per_page=1&state=all" | jq -r '.[0].iid // empty')
PIPE=$(api "projects/$PID/pipelines?per_page=1" | jq -r '.[0].id // empty')
JOB=$(api "projects/$PID/jobs?per_page=1" | jq -r '.[0].id // empty')
WI=$(api "projects/$PID/issues?per_page=1" | jq -r '.[0].iid // empty')
FILE=$(api "projects/$PID/repository/tree?per_page=100" | jq -r '[.[] | select(.type=="blob")][0].path // empty')

body=$(jq -n --arg n "zero-poc15-check" --arg e "$(date -u -d tomorrow +%F)" --argjson id "$PID" \
    '{name:$n, expires_at:$e, granular_scopes:[{access:"selected_memberships", permissions:$ARGS.positional, project_ids:[$id]}, {access:"user", permissions:["execute_mcp_tool"]}]}' --args "$@")
out=$(printf %s "$body" | api -X POST /user/personal_access_tokens -H 'Content-Type: application/json' --input -)
TID=$(printf %s "$out" | jq -er .id)
H=$(mktemp); chmod 600 "$H"
trap 'rm -f "$H"; api -X DELETE "/personal_access_tokens/$TID" >/dev/null 2>&1 || true' EXIT
printf %s "$out" | jq -er '"Authorization: Bearer " + .token' > "$H"

args() {
    case $1 in
    get_project|list_repository_tree|list_tags|list_commits|list_work_items|search_labels)
        echo "{\"project_id\":\"$PID\"}" ;;
    list_merge_requests) echo "{\"project_id\":\"$PID\",\"state\":\"all\"}" ;;
    get_repository_file) echo "{\"project_id\":\"$PID\",\"file_path\":\"$FILE\",\"ref\":\"$REF\"}" ;;
    list_branches|list_pipelines) echo "{\"id\":\"$PID\"}" ;;
    get_commit) echo "{\"project_id\":\"$PID\",\"commit_sha\":\"$SHA\"}" ;;
    get_merge_request) echo "{\"project_id\":\"$PID\",\"merge_request_iid\":${MR:-0}}" ;;
    get_pipeline) echo "{\"id\":\"$PID\",\"pipeline_id\":${PIPE:-0}}" ;;
    get_job) echo "{\"id\":\"$PID\",\"job_id\":${JOB:-0}}" ;;
    get_work_item) echo "{\"project_id\":\"$PID\",\"work_item_iid\":${WI:-0}}" ;;
    esac
}

echo "host=$HOST project=$PROJECT permissions: $*"
echo "data: commit=${SHA:0:8} file=$FILE mr=${MR:-none} pipeline=${PIPE:-none} job=${JOB:-none} work_item=${WI:-none}"
bad=0; empty=0
while read -r t; do
    if [ "$t" = get_work_item ] && [ -z "$WI" ]; then
        echo "skipped  $t  project has no work item"; continue
    fi
    r=$(curl -s --max-time 60 "https://$HOST/api/v4/mcp" -H @"$H" \
        -H 'content-type: application/json' -H 'accept: application/json, text/event-stream' \
        -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"$t\",\"arguments\":$(args "$t")}}")
    text=$(printf %s "$r" | jq -r '(.result.content[0].text // .error.message // "no answer")' 2>/dev/null | tr '\n' ' ' | cut -c1-260)
    if ! printf %s "$r" | jq -e '.result' >/dev/null 2>&1; then
        echo "error    $t  $text"; bad=$((bad + 1))
    elif [ "$(printf %s "$r" | jq -r '.result.isError // false')" = false ]; then
        if printf %s "$r" | jq -e '.result.content[0].text | fromjson | (.nodes // .tags // .work_items // .) == []' >/dev/null 2>&1; then
            echo "empty    $t  $text"; empty=$((empty + 1))
        else
            echo "ok       $t  $text"
        fi
    else
        echo "refused  $t  $text"; bad=$((bad + 1))
    fi
done < <(printf '%s\n' ${TOOLS:-$(cat "$HERE/default-tools.txt")})
echo "refused or error: $bad, empty (not proven): $empty"
[ "$bad" = 0 ]
