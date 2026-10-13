# Shared helpers for PoC scripts. Source it: . "$(dirname "$0")/../lib.sh"
# shellcheck shell=sh

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DEFAULT_MODEL="opencode/nemotron-3.5-lightning-free"
GO_PATH="PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin"   # caveat C09
RC=0

# ---- output -------------------------------------------------------------

step() { printf '\n== %s\n' "$*"; }
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1${2:+  ($2)}"; RC=1; }
gap()  { echo "GAP  $1"; }
info() { echo "INFO $1"; }

# expect NAME CMD...   -> PASS if CMD succeeds, else FAIL
expect() {
  name="$1"; shift
  if "$@"; then pass "$name"; else fail "$name"; fi
}

# expect_not NAME CMD...   -> PASS if CMD fails, else FAIL
expect_not() {
  name="$1"; shift
  if "$@"; then fail "$name"; else pass "$name"; fi
}

# contains TEXT PATTERN   -> grep -q on a string
contains() { printf '%s\n' "$1" | grep -q -- "$2"; }

# ---- sandbox ------------------------------------------------------------

# run SCRIPT inside $SANDBOX (set SANDBOX_ENV to pass e.g. "$GO_PATH")
in_sandbox() {
  openshell sandbox exec -n "$SANDBOX" ${SANDBOX_ENV:+--env "$SANDBOX_ENV"} -- sh -c "$1" 2>&1
}

delete_sandbox() { openshell sandbox delete "$SANDBOX" >/dev/null 2>&1; }

# copy the host opencode binary into the sandbox at /sandbox/ocbin/opencode
# and allow it (only it) to reach opencode.ai
upload_opencode() {
  mkdir -p "$1/ocbin"
  cp -L "$(command -v opencode)" "$1/ocbin/opencode"
  openshell sandbox upload "$SANDBOX" "$1/ocbin" /sandbox >/dev/null || return 1
  openshell policy update "$SANDBOX" \
    --rule-name zen \
    --binary /sandbox/ocbin/opencode \
    --add-endpoint opencode.ai:443:read-write:rest:enforce \
    --wait >/dev/null
}

# commit everything in a sandbox repo as a throwaway identity
git_init_in_sandbox() {
  in_sandbox "cd $1 && git init -q && git add -A \
    && git -c user.name=z3r0 -c user.email=z3r0@local commit -qm init" >/dev/null
}
