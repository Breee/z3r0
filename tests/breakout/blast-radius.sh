#!/usr/bin/env bash
# Simulated breakout: what can the given host user reach? Prints names only, never contents.
# Usage: tests/breakout/blast-radius.sh [user]   (default: current user; other user via sudo -u)
set -u
U="${1:-$(id -un)}"
if [ "$U" != "$(id -un)" ]; then exec sudo -u "$U" -H bash "$0" "$U"; fi
H="$HOME"; R="/run/user/$(id -u)"; n=0
hit() { echo "FOUND $1"; n=$((n+1)); }
chk() { [ -r "$2" ] && hit "$1: $2"; }  # readable file/dir

echo "# blast radius as $(id -un) (uid $(id -u)), HOME=$H"
chk gateway-db       "$H/.local/state/openshell/gateway/openshell.db"
chk gateway-kek      "$H/.local/state/openshell/gateway/credentials/key-encryption-key.bin"
for f in "$H"/.config/openshell/gateways/*/mtls/tls.key; do chk openshell-client-key "$f"; done
for f in "$H"/.ssh/id_*; do case "$f" in *.pub) ;; *) chk ssh-key "$f";; esac; done
chk ssh-agent        "${SSH_AUTH_SOCK:-/nonexistent}"
chk kubeconfig       "$H/.kube/config"
chk gh-token         "$H/.config/gh/hosts.yml"
chk glab-token       "$H/.config/glab-cli/config.yml"
chk git-credentials  "$H/.git-credentials"
chk docker-auth      "$H/.docker/config.json"
chk podman-auth      "$R/containers/auth.json"
chk pass-store       "$H/.password-store"
chk gnupg            "$H/.gnupg"
chk aws              "$H/.aws/credentials"
chk netrc            "$H/.netrc"
chk podman-socket    "$R/podman/podman.sock"
[ -S /var/run/docker.sock ] && [ -w /var/run/docker.sock ] && hit "docker-socket (root-equivalent): /var/run/docker.sock"
id -nG | grep -qw docker && hit "group docker (root-equivalent)"
sudo -n true 2>/dev/null && hit "passwordless sudo"
r=$(find "$H/repos" -maxdepth 3 -name .git -type d 2>/dev/null | wc -l); [ "$r" -gt 0 ] && hit "git repos under ~/repos: $r"
s=$(ls "$H"/.mozilla/firefox/*/cookies.sqlite "$H"/.config/google-chrome/*/Cookies 2>/dev/null | wc -l); [ "$s" -gt 0 ] && hit "browser cookie stores: $s"
echo "# total findings: $n"
