#!/bin/sh
# P1 check (as login user, after sudo setup.sh). Pass: sandbox on the container gateway works like on the native one.
G=openshell-ctr; N=p1-check; ok=0
if openshell -g $G gateway info >/dev/null 2>&1; then echo "pass gateway reachable"; else echo "FAIL gateway reachable"; ok=1; fi
openshell -g $G sandbox delete $N >/dev/null 2>&1
if openshell -g $G sandbox create --name $N --from localhost/z3r0-dev:latest --policy internal/zero/policy.yaml --detach >/dev/null 2>&1 \
  && openshell -g $G sandbox ssh-config $N > /tmp/opencode/p1.ssh \
  && ssh -F /tmp/opencode/p1.ssh "$(awk '/^Host /{print $2;exit}' /tmp/opencode/p1.ssh)" id | grep -q .; then
  echo "pass sandbox create + ssh"; else echo "FAIL sandbox create + ssh"; ok=1; fi
if ssh -F /tmp/opencode/p1.ssh "$(awk '/^Host /{print $2;exit}' /tmp/opencode/p1.ssh)" 'curl -s -m 5 https://example.com' >/dev/null 2>&1; then
  echo "FAIL egress default deny"; ok=1; else echo "pass egress default deny"; fi
openshell -g $G sandbox delete $N >/dev/null 2>&1
exit $ok
