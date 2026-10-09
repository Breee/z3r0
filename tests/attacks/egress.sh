#!/bin/sh
# Egress attacks. Runs INSIDE the sandbox. Usage: egress.sh deny|allow
# deny: no network policy. allow: curl may GET httpbin.org:443 (read-only, rest, enforce).
M="$1"; rc=0
blocked() { n="$1"; shift; if "$@" >/dev/null 2>&1; then echo "FAIL $n"; rc=1; else echo "PASS $n"; fi; }
allowed() { n="$1"; shift; if "$@" >/dev/null 2>&1; then echo "PASS $n"; else echo "FAIL $n"; rc=1; fi; }
c() { curl -sf --max-time 8 "$@"; }
cp /usr/bin/curl /tmp/curl2   # same binary, other path

if [ "$M" = deny ]; then
  blocked "egress:deny-httpbin"        c https://httpbin.org/get
else
  allowed "control:allow-get"          c https://httpbin.org/get
  blocked "egress:post-readonly"       c -X POST -d x=1 https://httpbin.org/post
  blocked "egress:other-host"          c https://example.com
  blocked "egress:plain-http-80"       c http://httpbin.org/get
  blocked "egress:other-port"          c https://httpbin.org:8443/get
  blocked "egress:connect-to-redirect" c --connect-to httpbin.org:443:example.com:443 https://httpbin.org/get
  blocked "egress:plaintext-over-443"  c -p http://httpbin.org:443/get
  blocked "egress:copied-binary"       /tmp/curl2 -sf --max-time 8 https://httpbin.org/get
  blocked "egress:sh-wrapper-wget"     sh -c "wget -q -T 8 -O- https://httpbin.org/get"
fi
exit $rc
