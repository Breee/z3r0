#!/bin/sh
# PoC 33 (#186), step 2: what a GitHub App installation token can be limited
# to. Needs ./setup.sh done, the app installed on REPO, and gh logged in
# (only to read the repository id).
#   REPO=owner/name ./run.sh          # a repository the app is installed on
#   OTHER=owner/other                 # optional: second repository of the
#                                     # installation, must stay out of reach
# No token is printed. The token is revoked at the end.
set -eu
cd "$(dirname "$0")"

DIR=${DIR:-$HOME/.config/zero-poc33}
REPO=${REPO:?set REPO=owner/name}
OTHER=${OTHER:-}
API=https://api.github.com
T=$(mktemp -d); TOKEN=
for b in openssl curl python3 git gh; do command -v $b >/dev/null || { echo "missing: $b"; exit 1; }; done
[ -r "$DIR/app.pem" ] || { echo "run ./setup.sh first"; exit 1; }

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | cut -c1-200)"; fails=$((fails+1)); }
is() { if [ "$2" = "$3" ]; then pass "$1"; else fail "$1" "got $2, want $3"; fi; }
js() { python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }
b64() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }
# api METHOD PATH [BODY] with the header file $T/auth; prints the status, body in $T/body
api() { curl -s -o "$T/body" -w '%{http_code}' -X "$1" -H @"$T/auth" -H 'Accept: application/vnd.github+json' ${3:+-d "$3"} "$API$2"; }

cleanup() {
	if [ -n "$TOKEN" ]; then
		printf 'Authorization: Bearer %s\n' "$TOKEN" > "$T/auth"
		[ "$(api DELETE /installation/token)" = 204 ] || echo "CLEANUP FAILED: token not revoked (it ends within the hour)"
	fi
	rm -rf "$T"
}
trap cleanup EXIT
trap 'exit 1' INT TERM HUP
umask 077

# 1. the app signs a short JWT with its private key
APP=$(js 'd["id"]' < "$DIR/app.json")
now=$(date +%s)
h=$(printf '{"alg":"RS256","typ":"JWT"}' | b64)
p=$(printf '{"iat":%s,"exp":%s,"iss":"%s"}' $((now-30)) $((now+300)) "$APP" | b64)
sig=$(printf '%s.%s' "$h" "$p" | openssl dgst -sha256 -sign "$DIR/app.pem" -binary | b64)
printf 'Authorization: Bearer %s.%s.%s\n' "$h" "$p" "$sig" > "$T/auth"

owner=${REPO%%/*}
is "the app finds its installation on $owner" "$(api GET "/users/$owner/installation")" 200
INST=$(js 'd["id"]' < "$T/body")
RID=$(gh api "repos/$REPO" --jq .id)

# 2. a token for one repository and fewer permissions than the app has
body=$(printf '{"repository_ids":[%s],"permissions":{"contents":"write","issues":"read","pull_requests":"read","metadata":"read"}}' "$RID")
is "token minted for one repository" "$(api POST "/app/installations/$INST/access_tokens" "$body")" 201
TOKEN=$(js 'd["token"]' < "$T/body")
is "it names one repository" "$(js 'len(d["repositories"])' < "$T/body")" 1
is "it holds only the permissions asked for" "$(js '",".join(sorted(d["permissions"]))' < "$T/body")" "contents,issues,metadata,pull_requests"
exp=$(js 'd["expires_at"]' < "$T/body")
left=$(( $(date -d "$exp" +%s) - now ))
if [ $left -le 3600 ] && [ $left -gt 3000 ]; then pass "it ends within the hour"; else fail "it ends within the hour" "$left s"; fi
printf 'Authorization: Bearer %s\n' "$TOKEN" > "$T/auth"

# 3. what it reaches
is "reads its repository" "$(api GET "/repos/$REPO")" 200
is "reads issues" "$(api GET "/repos/$REPO/issues")" 200
is "cannot open a pull request (not asked for)" "$(api POST "/repos/$REPO/pulls" '{"title":"x","head":"zero/poc33","base":"main"}')" 403
is "cannot start a workflow (not asked for)" "$(api GET "/repos/$REPO/actions/runs")" 403
is "the token lists one repository" "$(api GET /installation/repositories >/dev/null; js 'd["total_count"]' < "$T/body")" 1
if [ -n "$OTHER" ]; then
	s=$(api GET "/repos/$OTHER")
	case $s in 403|404) pass "a second repository of the installation is out of reach ($s)";; *) fail "second repository out of reach" "$s";; esac
fi

# 4. git over https with the token; the token goes in through a helper, not argv
cat > "$T/askpass" <<EOF
#!/bin/sh
case \$1 in Username*) echo x-access-token;; *) sed 's/^Authorization: Bearer //' "$T/auth";; esac
EOF
chmod 700 "$T/askpass"
g() { GIT_ASKPASS="$T/askpass" GIT_TERMINAL_PROMPT=0 git -c credential.helper= "$@"; }
if g clone -q "https://github.com/$REPO.git" "$T/r" 2>"$T/err"; then pass "git clone with x-access-token"; else fail "git clone" "$(cat "$T/err")"; fi
br=zero/poc33-$now
if (cd "$T/r" && git checkout -q -b "$br" && git -c user.name=poc33 -c user.email=poc33@example.invalid commit -q --allow-empty -m "PoC 33" && g push -q origin "$br") 2>"$T/err"; then
	pass "git push to its own branch"
	(cd "$T/r" && g push -q origin ":$br") 2>/dev/null || echo "NOTE: branch $br left on $REPO"
else fail "git push to its own branch" "$(cat "$T/err")"; fi
mkdir -p "$T/r/.github/workflows"; echo 'on: push' > "$T/r/.github/workflows/poc33.yml"
if (cd "$T/r" && git checkout -q -b "$br-wf" && git add . && git -c user.name=poc33 -c user.email=poc33@example.invalid commit -q -m wf && g push -q origin "$br-wf") 2>"$T/err"; then
	fail "a push that adds a workflow file is refused" "it went through"
	(cd "$T/r" && g push -q origin ":$br-wf") 2>/dev/null || true
else pass "a push that adds a workflow file is refused (no workflows permission)"; fi

# 5. rules of the default branch are readable with the token
def=$(gh api "repos/$REPO" --jq .default_branch)
s=$(api GET "/repos/$REPO/rules/branches/$def")
is "branch rules readable" "$s" 200
echo "rules on $def: $(js '",".join(r["type"] for r in d) or "none"' < "$T/body")"

# 6. revoked means dead
is "token revoked" "$(api DELETE /installation/token)" 204
is "revoked token is refused" "$(api GET "/repos/$REPO")" 401
TOKEN=

echo "fails: $fails"
[ $fails -eq 0 ]
