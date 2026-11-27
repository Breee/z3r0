#!/bin/sh
# PoC 31 (#185): first-run questions and workspace list with Charm v2.
# Drives the program through a pseudo terminal (ptyrun.py) and piped.
# Needs: go, python3. Touches nothing outside a temp dir.
set -eu
cd "$(dirname "$0")"
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
go build -o "$T/poc31" .
mkdir "$T/repo"; touch "$T/repo/go.mod" "$T/repo/package.json"

fails=0
has() { if printf %s "$2" | grep -q -- "$3"; then echo "PASS $1"; else echo "FAIL $1: $(printf %s "$2" | tail -n 5)"; fails=$((fails+1)); fi; }
not() { if printf %s "$2" | grep -q -- "$3"; then echo "FAIL $1"; fails=$((fails+1)); else echo "PASS $1"; fi; }
pty() { python3 ptyrun.py "$@" 2>&1 || true; }

o=$(pty '\r|\r|\r' "$T/poc31" firstrun "$T/repo")
has "terminal, Enter three times: detected hosts kept" "$o" "proxy.golang.org"
has "terminal, Enter three times: npm host kept" "$o" "registry.npmjs.org"
has "terminal, Enter three times: safe permissions" "$o" "gitlab.pipeline.read"
not "terminal, Enter three times: ci.run stays off" "$o" "  - ci.run"
has "terminal, Enter three times: exit 0" "$o" "^rc 0"

o=$(pty '\r|G| |\r|\x03' "$T/poc31" firstrun "$T/repo")
has "gitlab.mr.create without ci.run refused in the form" "$o" "gitlab.mr.create needs ci.run"

o=$(pty '\r|\r|n' "$T/poc31" firstrun "$T/repo")
has "answer no: nothing written" "$o" "nothing written"

o=$("$T/poc31" firstrun "$T/repo" </dev/null 2>&1)
has "no terminal: no questions" "$o" "no questions, defaults used"
has "no terminal: defaults printed" "$o" "sum.golang.org"

o=$(pty 'j|v' "$T/poc31" tui)
has "list: down, v picks the second workspace for VS Code" "$o" "chosen: v cve"
o=$(pty 'q' "$T/poc31" tui)
has "list: q leaves" "$o" "^rc 0"
rc=0; "$T/poc31" tui </dev/null >/dev/null 2>&1 || rc=$?
has "list without a terminal: exit 2" "rc=$rc" "rc=2"

echo "fails: $fails"
[ $fails -eq 0 ]
