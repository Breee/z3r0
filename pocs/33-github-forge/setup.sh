#!/bin/sh
# PoC 33 (#186), step 1: make a GitHub App of your own through the manifest
# flow. Needs a browser once. Writes the app id and its private key to
# $DIR (default ~/.config/zero-poc33, mode 700); nothing is printed.
#   ./setup.sh            # opens a local page with one button
# Afterwards: install the app on your account (the link is printed), then
# run ./run.sh.
set -eu
cd "$(dirname "$0")"

DIR=${DIR:-$HOME/.config/zero-poc33}
PORT=${PORT:-18733}
NAME=${NAME:-zero-poc33-$(id -un)}

for b in python3 curl; do command -v $b >/dev/null || { echo "missing: $b"; exit 1; }; done
[ -e "$DIR/app.json" ] && { echo "already set up: $DIR/app.json (remove the directory to start again)"; exit 0; }
umask 077
mkdir -p "$DIR"

echo "open http://127.0.0.1:$PORT/ in your browser and press the button"
NAME="$NAME" PORT="$PORT" DIR="$DIR" python3 - <<'PY'
import http.server, json, os, secrets, urllib.parse, urllib.request
name, port, d = os.environ["NAME"], int(os.environ["PORT"]), os.environ["DIR"]
state = secrets.token_urlsafe(16)
base = f"http://127.0.0.1:{port}"
manifest = {
    "name": name, "url": "https://github.com/Breee/z3r0", "public": False,
    "redirect_url": base + "/done",
    "hook_attributes": {"url": "https://example.invalid/none", "active": False},
    "default_permissions": {"contents": "write", "issues": "read", "pull_requests": "write",
                            "actions": "write", "metadata": "read"},
    "default_events": [],
}
page = ("<form action='https://github.com/settings/apps/new?state=%s' method='post'>"
        "<input type='hidden' name='manifest' value='%s'>"
        "<button>Create the GitHub App %s</button></form>") % (
        state, json.dumps(manifest).replace("'", "&#39;"), name)

class H(http.server.BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_GET(self):
        u = urllib.parse.urlparse(self.path); q = urllib.parse.parse_qs(u.query)
        if u.path != "/done":
            self.send_response(200); self.send_header("Content-Type", "text/html"); self.end_headers()
            self.wfile.write(page.encode()); return
        if q.get("state", [""])[0] != state or "code" not in q:
            self.send_response(400); self.end_headers(); self.wfile.write(b"wrong state"); return
        req = urllib.request.Request(
            "https://api.github.com/app-manifests/%s/conversions" % q["code"][0],
            method="POST", headers={"Accept": "application/vnd.github+json"})
        app = json.load(urllib.request.urlopen(req))
        with open(os.path.join(d, "app.pem"), "w") as f: f.write(app["pem"])
        with open(os.path.join(d, "app.json"), "w") as f:
            json.dump({"id": app["id"], "slug": app["slug"], "html_url": app["html_url"]}, f)
        self.send_response(200); self.end_headers()
        self.wfile.write(b"done; go back to the terminal")
        self.server.done = True

s = http.server.HTTPServer(("127.0.0.1", port), H); s.done = False
while not s.done: s.handle_request()
print("app made:", json.load(open(os.path.join(d, "app.json")))["slug"])
print("install it on your account, 'Only select repositories':")
print("  https://github.com/apps/%s/installations/new" % json.load(open(os.path.join(d, "app.json")))["slug"])
PY
