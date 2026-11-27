# Stand-in for a provider that wants an API key: lists two models and answers
# a chat completion, both only with "Authorization: Bearer $ACME_API_KEY".
import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

KEY = os.environ["ACME_API_KEY"]


class H(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def send(self, code, obj):
        body = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def allowed(self):
        if self.headers.get("authorization") == "Bearer " + KEY:
            return True
        self.send(401, {"error": {"message": "stand-in: wrong or missing key", "type": "invalid_request_error"}})
        return False

    def do_GET(self):
        if self.allowed():
            self.send(200, {"object": "list", "data": [
                {"id": "small", "object": "model", "owned_by": "acme"},
                {"id": "large", "object": "model", "owned_by": "acme"}]})

    def do_POST(self):
        req = json.loads(self.rfile.read(int(self.headers.get("content-length", 0))) or b"{}")
        if self.allowed():
            self.send(200, {
                "id": "chatcmpl-acme", "object": "chat.completion", "created": 0, "model": req.get("model", ""),
                "choices": [{"index": 0, "finish_reason": "stop",
                             "message": {"role": "assistant", "content": "acme says 42 as " + req.get("model", "")}}],
                "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}})

    def log_message(self, *a):
        pass


ThreadingHTTPServer(("0.0.0.0", 18101), H).serve_forever()
