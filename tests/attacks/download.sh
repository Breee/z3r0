#!/bin/sh
# Malicious workspace -> host via `sandbox download`. Runs on HOST. Usage: download.sh <sandbox>
# PASS=blocked, GAP=known (caveats.md). Never runs downloaded code except the git fsmonitor probe (touch only).
N="$1"; T=$(mktemp -d); TGT="$HOME/.z3r0-dl-target"; M="$T/PWNED"; rc=0
mkdir -p "$TGT"; trap 'rm -rf "$T" "$TGT"' EXIT
X() { timeout 40 openshell sandbox exec -n "$N" -- sh -c "$1" >/dev/null 2>&1; }
D() { openshell sandbox download "$N" "$1" "$2" >/dev/null 2>&1; }
res() { if [ "$2" = hit ]; then echo "$3  $1${4:+ ($4)}"; [ "$3" = FAIL ] && rc=1; else echo "PASS $1"; fi; return 0; }

X "rm -rf /sandbox/o && mkdir -p /sandbox/o && cd /sandbox/o && ln -s $TGT link && git init -q && git config core.fsmonitor 'touch $M; false' && echo a > a"
D /sandbox/o "$T/o"
[ -L "$T/o/link" ] && res dl:abs-symlink-kept hit GAP C21 || res dl:abs-symlink-kept no
git -C "$T/o" status >/dev/null 2>&1; [ -e "$M" ] && res dl:git-fsmonitor-host-exec hit GAP C20 || res dl:git-fsmonitor-host-exec no
X "cd /sandbox/o && rm link && mkdir link && echo PWN > link/pwn.txt"
D /sandbox/o "$T/o"; [ -e "$TGT/pwn.txt" ] && res dl:write-through-symlink hit FAIL || res dl:write-through-symlink no
X "mkdir -p /sandbox/t && cd /sandbox/t && ln -s ../../../../../../../$TGT up && echo x > f"
D /sandbox/t "$T/t"; [ -n "$(ls -A "$TGT")" ] && res dl:relative-traversal hit FAIL || res dl:relative-traversal no
exit $rc
