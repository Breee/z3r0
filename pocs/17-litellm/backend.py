# Stand-in model backend: answers every request at once with a fixed text,
# as a chat completion or, on /responses, as a Responses API object.
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class H(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def do_POST(self):
        self.rfile.read(int(self.headers.get("content-length", 0)))
        if self.path.endswith("/responses"):
            # LiteLLM turns an Anthropic-form request for an openai/ model into a Responses API call.
            answer = {
                "id": "resp-stand-in", "object": "response", "created_at": 0, "status": "completed",
                "model": "stand-in",
                "output": [{"type": "message", "id": "msg-stand-in", "status": "completed", "role": "assistant",
                            "content": [{"type": "output_text", "text": "pong", "annotations": []}]}],
                "usage": {"input_tokens": 1, "output_tokens": 1, "total_tokens": 2},
            }
        else:
            answer = {
                "id": "chatcmpl-stand-in", "object": "chat.completion", "created": 0, "model": "stand-in",
                "choices": [{"index": 0, "finish_reason": "stop",
                             "message": {"role": "assistant", "content": "pong"}}],
                "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
            }
        body = json.dumps(answer).encode()
        self.send_response(200)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *a):
        pass


ThreadingHTTPServer(("0.0.0.0", 18101), H).serve_forever()
