#!/usr/bin/env python3
# GitLab authorization code + PKCE on a loopback redirect; stores the refresh token in OpenBao.
import base64, hashlib, http.server, json, os, secrets, sys, urllib.parse, urllib.request, webbrowser

app, bao, btok, uid = sys.argv[1:5]
redirect = "http://localhost:8250/oidc/callback"
verifier = secrets.token_urlsafe(48)
challenge = base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).rstrip(b"=").decode()
state = secrets.token_urlsafe(16)
url = "https://gitlab.com/oauth/authorize?" + urllib.parse.urlencode({
    "client_id": app, "redirect_uri": redirect, "response_type": "code", "scope": "api",
    "state": state, "code_challenge": challenge, "code_challenge_method": "S256"})
got = {}

class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        got.update(urllib.parse.parse_qsl(urllib.parse.urlparse(self.path).query))
        self.send_response(200); self.end_headers(); self.wfile.write(b"zero: done, close this tab")
    def log_message(self, *a): pass

srv = http.server.HTTPServer(("127.0.0.1", 8250), H)
print("open:", url, file=sys.stderr); webbrowser.open(url)
while "code" not in got and "error" not in got:
    srv.handle_request()
if got.get("state") != state or "code" not in got:
    sys.exit("login failed: %s" % got)

def post(u, data, headers={}):
    r = urllib.request.Request(u, data=data, headers=headers)
    return json.load(urllib.request.urlopen(r))

tok = post("https://gitlab.com/oauth/token", urllib.parse.urlencode({
    "client_id": app, "code": got["code"], "grant_type": "authorization_code",
    "redirect_uri": redirect, "code_verifier": verifier}).encode())
post(bao + "/v1/secret/data/zero/users/%s/gitlab" % uid,
     json.dumps({"data": {"refresh_token": tok["refresh_token"]}}).encode(),
     {"X-Vault-Token": btok, "Content-Type": "application/json"})
print("stored refresh token in OpenBao; access token expires_in", tok["expires_in"], "scope", tok["scope"])
