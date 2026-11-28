"""PoC 36: the smallest OpenID provider LiteLLM's sign-in accepts.
One local user, approved without a question (the PoC stands in for
"the person already holds the zero UI session"). Codes work once."""
import base64, json, os, secrets, urllib.parse
from http.server import BaseHTTPRequestHandler, HTTPServer

CID, SEC = os.environ["IDP_CLIENT_ID"], os.environ["IDP_CLIENT_SECRET"]
REDIRECT = os.environ["IDP_REDIRECT"]
USER = {"sub": "zero-local", "name": "zero"}
codes, tokens = set(), set()


class H(BaseHTTPRequestHandler):
    def out(self, code, body, headers=()):
        b = json.dumps(body).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(b)))
        for k, v in headers:
            self.send_header(k, v)
        self.end_headers()
        self.wfile.write(b)

    def do_GET(self):
        u = urllib.parse.urlparse(self.path)
        q = urllib.parse.parse_qs(u.query)
        if u.path == "/authorize":
            if q.get("client_id") != [CID] or q.get("redirect_uri") != [REDIRECT]:
                return self.out(400, {"error": "invalid_request"})
            c = secrets.token_urlsafe(24)
            codes.add(c)
            to = REDIRECT + "?" + urllib.parse.urlencode({"code": c, "state": q.get("state", [""])[0]})
            return self.out(302, {}, [("Location", to)])
        if u.path == "/userinfo":
            t = self.headers.get("Authorization", "").removeprefix("Bearer ")
            if t not in tokens:
                return self.out(401, {"error": "invalid_token"})
            return self.out(200, USER)
        self.out(404, {})

    def do_POST(self):
        if self.path != "/token":
            return self.out(404, {})
        form = urllib.parse.parse_qs(self.rfile.read(int(self.headers.get("Content-Length", 0))).decode())
        want = "Basic " + base64.b64encode(f"{CID}:{SEC}".encode()).decode()
        if not secrets.compare_digest(self.headers.get("Authorization", ""), want):
            return self.out(401, {"error": "invalid_client"})
        c = form.get("code", [""])[0]
        if c not in codes or form.get("redirect_uri") != [REDIRECT]:
            return self.out(400, {"error": "invalid_grant"})
        codes.discard(c)
        t = secrets.token_urlsafe(32)
        tokens.add(t)
        self.out(200, {"access_token": t, "token_type": "Bearer", "expires_in": 300})

    def log_message(self, *a):
        pass


HTTPServer(("0.0.0.0", 8000), H).serve_forever()
