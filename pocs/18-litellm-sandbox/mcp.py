# Stand-in MCP server (streamable HTTP, plain JSON answers): two tools, needs its own bearer token.
import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

TOKEN = os.environ["MCP_UPSTREAM_TOKEN"]
TOOLS = [
    {"name": n, "description": d,
     "inputSchema": {"type": "object", "properties": {"text": {"type": "string"}}, "required": ["text"]}}
    for n, d in (("read_note", "Read a note."), ("write_note", "Write a note."))
]


class H(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def send(self, code, obj=None):
        body = b"" if obj is None else json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self):
        req = json.loads(self.rfile.read(int(self.headers.get("content-length", 0))) or b"{}")
        if self.headers.get("authorization") != "Bearer " + TOKEN:
            return self.send(401, {"error": "upstream token missing"})
        if "id" not in req:
            return self.send(202)
        m, p = req.get("method"), req.get("params") or {}
        if m == "initialize":
            r = {"protocolVersion": p.get("protocolVersion", "2025-06-18"),
                 "capabilities": {"tools": {}}, "serverInfo": {"name": "notes", "version": "1"}}
        elif m == "tools/list":
            r = {"tools": TOOLS}
        elif m == "tools/call":
            r = {"content": [{"type": "text", "text": "%s: %s" % (p.get("name"), (p.get("arguments") or {}).get("text"))}]}
        else:
            return self.send(200, {"jsonrpc": "2.0", "id": req["id"], "error": {"code": -32601, "message": "unknown method"}})
        self.send(200, {"jsonrpc": "2.0", "id": req["id"], "result": r})

    def do_GET(self):
        self.send(405)

    def log_message(self, *a):
        pass


ThreadingHTTPServer(("0.0.0.0", 18102), H).serve_forever()
