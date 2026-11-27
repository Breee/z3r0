#!/bin/sh
# PoC 32 (#181): a k9s-style workspace screen with bubbletea v2 and lipgloss v2.
# Builds the program, checks drawn frames at several sizes and drives the
# real screen through a pseudo terminal. Rows are made up; nothing is changed.
set -eu
cd "$(dirname "$0")"
T=$(mktemp -d)
trap 'rm -rf "$T"' EXIT
go build -o "$T/poc32" .

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $(printf %s "$2" | tail -n 6)"; fails=$((fails+1)); }
has() { if printf %s "$2" | grep -q -- "$3"; then pass "$1"; else fail "$1" "$2"; fi; }
not() { if printf %s "$2" | grep -q -- "$3"; then fail "$1" "$2"; else pass "$1"; fi; }
frame() { "$T/poc32" frame "$@" | sed 's/\x1b\[[0-9;]*m//g'; }
fits() { # NAME WIDTH TEXT
	w=$(printf '%s\n' "$3" | LC_ALL=C.UTF-8 wc -L)
	if [ "$w" -le "$2" ]; then pass "$1"; else fail "$1" "widest line $w"; fi
}
pty() { python3 ptyrun.py "$@" 2>&1 || true; }

f=$(frame 100 24)
has "logo in the header" "$f" "▄█████▄"
has "key hints in the header" "$f" "<d>"
has "table header" "$f" "NAME.*STATUS.*DIRS.*TOKEN.*POLICY"
has "frame title counts the workspaces" "$f" "workspaces(all)\[4\]"
has "current workspace marked" "$f" "developer\*"
fits "100x24: every line fits" 100 "$f"
[ "$(printf '%s\n' "$f" | wc -l)" -le 24 ] && pass "100x24: fits the height" || fail "100x24: fits the height" "$f"

f=$(frame 80 24)
fits "80x24: every line fits" 80 "$f"
has "80x24: table still there" "$f" "developer"

f=$(frame 50 18)
fits "50x18: every line fits" 50 "$f"
not "50x18: hints dropped" "$f" "<d>"

has "too small is said" "$(frame 15 5)" "too small"

f=$(frame 80 20 / d e)
has "filter prompt" "$f" "/de"
has "filter in the title" "$f" "workspaces(/de)\[1\]"
not "filter narrows" "$f" "apps"

f=$(frame 80 20 d)
has "describe shows the directories" "$f" "Directories"
has "describe names the workspace" "$f" "describe(developer)"

has "help view" "$(frame 80 20 '?')" "help"
has "unknown command is said" "$(frame 80 20 : x enter)" "no command or workspace: x"
has "command jumps to a workspace" "$(frame 80 20 : a p p s enter d)" "describe(apps)"

p=$(SIZE=24x100 pty 'j|:|q|\r' "$T/poc32")
has "terminal: screen drawn" "$p" "workspaces(all)"
has "terminal: :q quits" "$p" "rc 0"
has "terminal: enter picks opencode" "$(pty '\r' "$T/poc32")" "would run: zero -c developer opencode"
has "terminal: v picks vscode" "$(pty 'k|v' "$T/poc32")" "would run: zero -c cve vscode"
p=$(pty 'r|developer\r|q' "$T/poc32")
has "terminal: remove asks outside the screen" "$p" "type workspace name to delete"
has "terminal: back on the screen after it" "$p" "rm developer: done"
p=$(pty 'r|nope\r|q' "$T/poc32")
has "terminal: failed remove is said" "$p" "rm developer: exit status"
not "terminal: failed remove is not done" "$p" "rm developer: done"

set +e; "$T/poc32" </dev/null >"$T/o" 2>&1; rc=$?; set -e
[ $rc = 2 ] && pass "no terminal: exit 2" || fail "no terminal: exit 2" "rc $rc $(cat "$T/o")"

echo "fails: $fails"
[ $fails -eq 0 ]
